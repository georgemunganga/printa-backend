package order

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/georgemunganga/printa-backend/internal/middleware"
	"github.com/go-chi/chi/v5"
)

// Handler exposes order HTTP endpoints.
type Handler struct {
	service Service
	db      *sql.DB
}

func NewHandler(service Service, db *sql.DB) *Handler { return &Handler{service: service, db: db} }

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
	if middleware.GetRole(r) == middleware.RoleCustomer {
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
			AssetID string `json:"asset_id"`
		}
		if err := json.Unmarshal(item.Customisation, &customisation); err != nil {
			return err
		}
		if customisation.AssetID == "" {
			continue
		}
		var exists bool
		if err := h.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM design_assets WHERE id=$1 AND owner_id=$2 AND deleted_at IS NULL)`, customisation.AssetID, middleware.GetUserID(r)).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("design asset is not available to the authenticated customer")
		}
	}
	return nil
}

type customerDeliveryInput struct {
	Method     string `json:"method"`
	LocationID string `json:"location_id"`
}

type canonicalCustomerDelivery struct {
	Method         string   `json:"method"`
	LocationID     string   `json:"location_id"`
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
}

// validateCustomerDelivery accepts a customer delivery order only when its referenced saved location belongs to the
// authenticated customer and the requested store has an active matching city-level zone. Client address text and
// coordinates are never persisted; the order receives a canonical server snapshot instead.
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
		if input.LocationID == "" {
			return fmt.Errorf("delivery orders require a saved delivery location")
		}
	default:
		return fmt.Errorf("delivery_address.method must be pickup or delivery")
	}

	var snapshot canonicalCustomerDelivery
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
	var covered bool
	if err := h.db.QueryRowContext(r.Context(), `
		SELECT EXISTS(
			SELECT 1 FROM store_delivery_zones
			WHERE store_id=$1 AND is_active=true AND LOWER(city)=LOWER($2) AND LOWER(country)=LOWER($3)
		)`, req.StoreID, snapshot.City, snapshot.Country).Scan(&covered); err != nil {
		return err
	}
	if !covered {
		return fmt.Errorf("the selected store does not cover this saved delivery location")
	}
	if latitude.Valid {
		snapshot.Latitude = &latitude.Float64
	}
	if longitude.Valid {
		snapshot.Longitude = &longitude.Float64
	}
	snapshot.Method = "delivery"
	snapshot.Coverage = "CITY_LEVEL"
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
	if middleware.GetRole(r) == middleware.RoleCustomer && customerID != middleware.GetUserID(r) {
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
