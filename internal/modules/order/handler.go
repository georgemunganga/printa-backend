package order

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/georgemunganga/printa-backend/internal/middleware"
	"github.com/georgemunganga/printa-backend/internal/modules/delivery"
	"github.com/go-chi/chi/v5"
)

// Handler exposes order HTTP endpoints.
type Handler struct {
	service Service
	db      *sql.DB
	pricing delivery.PricingService
}

func NewHandler(service Service, db *sql.DB, pricing delivery.PricingService) *Handler {
	return &Handler{service: service, db: db, pricing: pricing}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/v1/orders", func(r chi.Router) {
		r.Post("/", h.placeOrder)                              // POST   /api/v1/orders
		r.Get("/{id}", h.getOrder)                             // GET    /api/v1/orders/{id}
		r.Get("/number/{number}", h.getOrderByNumber)          // GET    /api/v1/orders/number/{number}
		r.Patch("/{id}/status", h.updateStatus)                // PATCH  /api/v1/orders/{id}/status
		r.Delete("/{id}", h.cancelOrder)                       // DELETE /api/v1/orders/{id}
		r.Get("/store/{store_id}", h.listStoreOrders)          // GET    /api/v1/orders/store/{store_id}?status=PENDING
		r.Get("/customer/{customer_id}", h.listCustomerOrders) // GET /api/v1/orders/customer/{customer_id}
	})
}

func (h *Handler) RegisterStorefrontRoutes(r chi.Router) {
	r.Post("/api/v1/storefront/order-quote", h.quoteOrder)
}

func (h *Handler) quoteOrder(w http.ResponseWriter, r *http.Request) {
	var req QuoteOrderRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": "invalid quote request"})
		return
	}
	method := strings.ToLower(strings.TrimSpace(req.Fulfilment.Method))
	if method == "" {
		method = "pickup"
	}
	var fee, distance float64
	if method == "delivery" {
		if strings.TrimSpace(req.Fulfilment.City) == "" || strings.TrimSpace(req.Fulfilment.Country) == "" || req.Fulfilment.Latitude == nil || req.Fulfilment.Longitude == nil {
			respond(w, http.StatusBadRequest, map[string]string{"error": "delivery city, country, latitude, and longitude are required"})
			return
		}
		var covered, hasZones bool
		if err := h.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM store_delivery_zones WHERE store_id=$1 AND is_active=true AND LOWER(city)=LOWER($2) AND LOWER(country)=LOWER($3)), EXISTS(SELECT 1 FROM store_delivery_zones WHERE store_id=$1)`, req.StoreID, req.Fulfilment.City, req.Fulfilment.Country).Scan(&covered, &hasZones); err != nil {
			respond(w, http.StatusInternalServerError, map[string]string{"error": "delivery coverage could not be checked"})
			return
		}
		if hasZones && !covered {
			respond(w, http.StatusUnprocessableEntity, map[string]string{"error": "the selected store does not cover this delivery location"})
			return
		}
		pricingQuote, err := h.pricing.Quote(r.Context(), req.StoreID, *req.Fulfilment.Latitude, *req.Fulfilment.Longitude)
		if err != nil {
			respond(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
			return
		}
		fee, distance = pricingQuote.Fee, pricingQuote.DistanceKM
	} else if method != "pickup" {
		respond(w, http.StatusBadRequest, map[string]string{"error": "fulfilment method must be pickup or delivery"})
		return
	}
	quote, err := h.service.Quote(r.Context(), req.StoreID, req.Items, req.Discount, fee, distance)
	if err != nil {
		code := http.StatusBadRequest
		if strings.Contains(err.Error(), "unavailable") || strings.Contains(err.Error(), "not found") {
			code = http.StatusUnprocessableEntity
		}
		respond(w, code, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, quote)
}

func (h *Handler) placeOrder(w http.ResponseWriter, r *http.Request) {
	var req PlaceOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	req.IdempotencyKey = strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if len(req.IdempotencyKey) > 128 {
		respond(w, http.StatusBadRequest, map[string]string{"error": "Idempotency-Key must not exceed 128 characters"})
		return
	}
	if strings.EqualFold(req.Channel, "ONLINE") || middleware.GetRole(r) == middleware.RoleCustomer {
		req.CustomerID = middleware.GetUserID(r)
		if err := h.validateCustomerAssets(r, req); err != nil {
			respond(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
			return
		}
		if err := h.validateCustomerDelivery(r, &req); err != nil {
			respond(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
			return
		}
	}
	o, err := h.service.PlaceOrder(r.Context(), req)
	if err != nil {
		code := http.StatusInternalServerError
		msg := err.Error()
		if strings.Contains(msg, "unavailable") || strings.Contains(msg, "not found in this store") {
			code = http.StatusUnprocessableEntity
		} else if strings.Contains(msg, "required") || strings.Contains(msg, "invalid") || strings.Contains(msg, "at least one") {
			code = http.StatusBadRequest
		}
		respond(w, code, map[string]string{"error": msg})
		return
	}
	respond(w, http.StatusCreated, o)
}

func (h *Handler) validateCustomerAssets(r *http.Request, req PlaceOrderRequest) error {
	for _, item := range req.Items {
		if len(item.Customisation) == 0 {
			continue
		}
		var customisation struct {
			AssetID        string `json:"asset_id"`
			UploadedAssets []struct {
				AssetID string `json:"asset_id"`
			} `json:"uploaded_assets"`
		}
		if err := json.Unmarshal(item.Customisation, &customisation); err != nil {
			return err
		}
		assetIDs := make([]string, 0, len(customisation.UploadedAssets)+1)
		if customisation.AssetID != "" {
			assetIDs = append(assetIDs, customisation.AssetID)
		}
		for _, asset := range customisation.UploadedAssets {
			if asset.AssetID != "" {
				assetIDs = append(assetIDs, asset.AssetID)
			}
		}
		seen := make(map[string]struct{}, len(assetIDs))
		for _, assetID := range assetIDs {
			if _, duplicate := seen[assetID]; duplicate {
				continue
			}
			seen[assetID] = struct{}{}
			var exists bool
			if err := h.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM design_assets WHERE id=$1 AND owner_id=$2 AND deleted_at IS NULL)`, assetID, middleware.GetUserID(r)).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return fmt.Errorf("design asset is not available to the authenticated customer")
			}
		}
	}
	return nil
}

type customerDeliveryInput struct {
	Method         string   `json:"method"`
	LocationID     string   `json:"location_id"`
	RecipientName  string   `json:"recipient_name"`
	RecipientPhone string   `json:"recipient_phone"`
	AddressLine1   string   `json:"address_line1"`
	AddressLine2   string   `json:"address_line2"`
	City           string   `json:"city"`
	Country        string   `json:"country"`
	Latitude       *float64 `json:"latitude"`
	Longitude      *float64 `json:"longitude"`
}

type canonicalCustomerDelivery struct {
	Method         string   `json:"method"`
	LocationID     string   `json:"location_id,omitempty"`
	Label          string   `json:"label"`
	RecipientName  string   `json:"recipient_name"`
	RecipientPhone string   `json:"recipient_phone"`
	AddressLine1   string   `json:"address_line1"`
	AddressLine2   string   `json:"address_line2,omitempty"`
	City           string   `json:"city"`
	Country        string   `json:"country"`
	Latitude       *float64 `json:"latitude,omitempty"`
	Longitude      *float64 `json:"longitude,omitempty"`
	Coverage       string   `json:"coverage"`
	DistanceKM     float64  `json:"distance_km"`
	Fee            float64  `json:"fee"`
	Currency       string   `json:"currency"`
	PricingRuleID  string   `json:"pricing_rule_id"`
}

// validateCustomerDelivery accepts either a customer-owned saved location or a validated one-time address. The order
// receives a canonical snapshot, so a later location edit cannot change an order already in production.
func (h *Handler) validateCustomerDelivery(r *http.Request, req *PlaceOrderRequest) error {
	if len(req.DeliveryAddress) == 0 || string(req.DeliveryAddress) == "null" {
		return nil
	}
	var input customerDeliveryInput
	if err := json.Unmarshal(req.DeliveryAddress, &input); err != nil {
		return fmt.Errorf("delivery_address must be valid JSON")
	}
	switch strings.ToLower(strings.TrimSpace(input.Method)) {
	case "", "pickup":
		canonical, err := json.Marshal(map[string]string{"method": "pickup", "store_id": req.StoreID})
		if err != nil {
			return err
		}
		req.DeliveryAddress = canonical
		return nil
	case "delivery":
	default:
		return fmt.Errorf("delivery_address.method must be pickup or delivery")
	}

	var snapshot canonicalCustomerDelivery
	if strings.TrimSpace(input.LocationID) == "" {
		snapshot = canonicalCustomerDelivery{
			Method: "delivery", Label: "Current location", RecipientName: strings.TrimSpace(input.RecipientName),
			RecipientPhone: strings.TrimSpace(input.RecipientPhone), AddressLine1: strings.TrimSpace(input.AddressLine1),
			AddressLine2: strings.TrimSpace(input.AddressLine2), City: strings.TrimSpace(input.City), Country: strings.TrimSpace(input.Country),
			Latitude: input.Latitude, Longitude: input.Longitude,
		}
		if snapshot.RecipientName == "" || snapshot.RecipientPhone == "" || snapshot.AddressLine1 == "" || snapshot.City == "" || snapshot.Country == "" {
			return fmt.Errorf("one-time delivery requires recipient_name, recipient_phone, address_line1, city, and country")
		}
		if (snapshot.Latitude == nil) != (snapshot.Longitude == nil) {
			return fmt.Errorf("delivery latitude and longitude must be provided together")
		}
		if snapshot.Latitude != nil && (*snapshot.Latitude < -90 || *snapshot.Latitude > 90 || *snapshot.Longitude < -180 || *snapshot.Longitude > 180) {
			return fmt.Errorf("delivery coordinates are invalid")
		}
	} else {
		var latitude, longitude sql.NullFloat64
		err := h.db.QueryRowContext(r.Context(), `
		SELECT id, label, recipient_name, recipient_phone, address_line1, COALESCE(address_line2, ''), city, country, latitude, longitude
		FROM customer_delivery_locations
		WHERE id=$1 AND customer_id=$2`, input.LocationID, req.CustomerID).
			Scan(&snapshot.LocationID, &snapshot.Label, &snapshot.RecipientName, &snapshot.RecipientPhone, &snapshot.AddressLine1, &snapshot.AddressLine2, &snapshot.City, &snapshot.Country, &latitude, &longitude)
		if err == sql.ErrNoRows {
			return fmt.Errorf("saved delivery location is not available to the authenticated customer")
		}
		if err != nil {
			return err
		}
		if latitude.Valid {
			snapshot.Latitude = &latitude.Float64
		}
		if longitude.Valid {
			snapshot.Longitude = &longitude.Float64
		}
	}
	var covered, hasConfiguredZones bool
	if err := h.db.QueryRowContext(r.Context(), `
		SELECT
			EXISTS(SELECT 1 FROM store_delivery_zones WHERE store_id=$1 AND is_active=true AND LOWER(city)=LOWER($2) AND LOWER(country)=LOWER($3)),
			EXISTS(SELECT 1 FROM store_delivery_zones WHERE store_id=$1)
		`, req.StoreID, snapshot.City, snapshot.Country).Scan(&covered, &hasConfiguredZones); err != nil {
		return err
	}
	if hasConfiguredZones && !covered {
		return fmt.Errorf("the selected store does not cover this delivery location")
	}
	if snapshot.Latitude == nil || snapshot.Longitude == nil {
		return fmt.Errorf("exact delivery coordinates are required to calculate the delivery fee")
	}
	quote, err := h.pricing.Quote(r.Context(), req.StoreID, *snapshot.Latitude, *snapshot.Longitude)
	if err != nil {
		return err
	}
	snapshot.Method = "delivery"
	snapshot.Coverage = "PRICING_ENGINE"
	if covered {
		snapshot.Coverage = "CITY_LEVEL"
	}
	snapshot.DistanceKM = quote.DistanceKM
	snapshot.Fee = quote.Fee
	snapshot.Currency = quote.Currency
	snapshot.PricingRuleID = quote.RuleID
	req.DeliveryFee = quote.Fee
	req.DeliveryDistanceKM = quote.DistanceKM
	canonical, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	req.DeliveryAddress = canonical
	return nil
}

func (h *Handler) getOrder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	o, err := h.service.GetOrder(r.Context(), id)
	if err != nil {
		respond(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	if !h.requireCustomerOrderAccess(w, r, o) {
		return
	}
	respond(w, http.StatusOK, o)
}

func (h *Handler) getOrderByNumber(w http.ResponseWriter, r *http.Request) {
	number := chi.URLParam(r, "number")
	o, err := h.service.GetOrderByNumber(r.Context(), number)
	if err != nil {
		respond(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	if !h.requireCustomerOrderAccess(w, r, o) {
		return
	}
	respond(w, http.StatusOK, o)
}

func (h *Handler) updateStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req UpdateStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	o, err := h.service.UpdateStatus(r.Context(), id, req)
	if err != nil {
		code := http.StatusInternalServerError
		if strings.Contains(err.Error(), "cannot transition") {
			code = http.StatusUnprocessableEntity
		} else if strings.Contains(err.Error(), "not found") {
			code = http.StatusNotFound
		}
		respond(w, code, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, o)
}

func (h *Handler) cancelOrder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	o, err := h.service.GetOrder(r.Context(), id)
	if err != nil {
		respond(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	if !h.requireCustomerOrderAccess(w, r, o) {
		return
	}
	if err := h.service.CancelOrder(r.Context(), id); err != nil {
		code := http.StatusInternalServerError
		if strings.Contains(err.Error(), "only PENDING") {
			code = http.StatusUnprocessableEntity
		} else if strings.Contains(err.Error(), "not found") {
			code = http.StatusNotFound
		}
		respond(w, code, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, map[string]string{"status": "order cancelled"})
}

func (h *Handler) listStoreOrders(w http.ResponseWriter, r *http.Request) {
	storeID := chi.URLParam(r, "store_id")
	status := r.URL.Query().Get("status")
	orders, err := h.service.ListStoreOrders(r.Context(), storeID, status)
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if orders == nil {
		orders = make([]*Order, 0)
	}
	respond(w, http.StatusOK, orders)
}

func (h *Handler) listCustomerOrders(w http.ResponseWriter, r *http.Request) {
	customerID := chi.URLParam(r, "customer_id")
	if customerID != middleware.GetUserID(r) {
		respond(w, http.StatusForbidden, map[string]string{"error": "customer scope does not match authenticated user"})
		return
	}
	orders, err := h.service.ListCustomerOrders(r.Context(), customerID)
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if orders == nil {
		orders = make([]*Order, 0)
	}
	respond(w, http.StatusOK, orders)
}

func (h *Handler) requireCustomerOrderAccess(w http.ResponseWriter, r *http.Request, o *Order) bool {
	if middleware.GetRole(r) != middleware.RoleCustomer {
		return true
	}
	if o.CustomerID == nil || o.CustomerID.String() != middleware.GetUserID(r) {
		respond(w, http.StatusForbidden, map[string]string{"error": "order does not belong to authenticated customer"})
		return false
	}
	return true
}

func respond(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}
