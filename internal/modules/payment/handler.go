package payment

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/georgemunganga/printa-backend/internal/middleware"
	"github.com/georgemunganga/printa-backend/internal/modules/order"
	"github.com/georgemunganga/printa-backend/internal/modules/vendor"
	"github.com/go-chi/chi/v5"
)

// Handler exposes payment HTTP endpoints.
type Handler struct {
	service       Service
	vendorService vendor.Service
	orderService  order.Service
}

func NewHandler(service Service, vendorService vendor.Service, orderService order.Service) *Handler {
	return &Handler{service: service, vendorService: vendorService, orderService: orderService}
}

// RegisterRoutes registers all routes (legacy — kept for backward compatibility).
func (h *Handler) RegisterRoutes(r chi.Router) {
	h.RegisterWebhookRoutes(r)
	h.RegisterProtectedRoutes(r)
}

// RegisterWebhookRoutes registers provider callback endpoints without JWT authentication.
// In production these endpoints fail closed until an ingress-provided shared secret is configured.
func (h *Handler) RegisterWebhookRoutes(r chi.Router) {
	r.Route("/api/v1/webhooks", func(r chi.Router) {
		r.Post("/mtn-momo", h.webhookMTN)
		r.Post("/airtel-money", h.webhookAirtel)
	})
}

// RegisterProtectedRoutes registers payment endpoints that require JWT auth.
func (h *Handler) RegisterProtectedRoutes(r chi.Router) {
	r.Route("/api/v1/payments", func(r chi.Router) {
		r.Post("/", h.initiate)
		r.Get("/{id}", h.getByID)
		r.Post("/{id}/verify", h.verify)
		r.Post("/{id}/refund", h.refund)
		r.Get("/reference/{ref_type}/{ref_id}", h.listByReference)
		r.Get("/vendor/{vendor_id}", h.listByVendor)
	})
}

func (h *Handler) initiate(w http.ResponseWriter, r *http.Request) {
	var req InitiatePaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if middleware.GetRole(r) == middleware.RoleCustomer {
		if strings.ToUpper(req.ReferenceType) != "ORDER" {
			respond(w, http.StatusBadRequest, map[string]string{"error": "customers may initiate payments only for orders"})
			return
		}
		o, err := h.orderService.GetOrder(r.Context(), req.ReferenceID)
		if err != nil || o.CustomerID == nil || o.CustomerID.String() != middleware.GetUserID(r) {
			respond(w, http.StatusForbidden, map[string]string{"error": "order is not accessible to the authenticated customer"})
			return
		}
		req.Amount = o.Total
		req.Currency = o.Currency
		req.VendorID = ""
	} else if !h.bindVendorRequest(w, r, &req.VendorID) {
		return
	}
	if headerKey := r.Header.Get("Idempotency-Key"); headerKey != "" {
		req.IdempotencyKey = headerKey
	}
	tx, err := h.service.Initiate(r.Context(), req)
	if err != nil {
		code := http.StatusInternalServerError
		msg := err.Error()
		if strings.Contains(msg, "required") || strings.Contains(msg, "invalid") || strings.Contains(msg, "greater than") {
			code = http.StatusBadRequest
		} else if strings.Contains(msg, "duplicate") {
			code = http.StatusConflict
		}
		respond(w, code, map[string]string{"error": msg})
		return
	}
	respond(w, http.StatusCreated, tx)
}

func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	tx, ok := h.requirePaymentAccess(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	respond(w, http.StatusOK, tx)
}

func (h *Handler) verify(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, ok := h.requirePaymentAccess(w, r, id); !ok {
		return
	}
	tx, err := h.service.Verify(r.Context(), id)
	if err != nil {
		code := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			code = http.StatusNotFound
		}
		respond(w, code, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, tx)
}

func (h *Handler) refund(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	tx, err := h.service.Refund(r.Context(), id)
	if err != nil {
		code := http.StatusInternalServerError
		msg := err.Error()
		if strings.Contains(msg, "not found") {
			code = http.StatusNotFound
		} else if strings.Contains(msg, "only COMPLETED") {
			code = http.StatusUnprocessableEntity
		}
		respond(w, code, map[string]string{"error": msg})
		return
	}
	respond(w, http.StatusOK, tx)
}

// listByReference is retained for operational support. Reference-level access depends
// on the referenced order/invoice relationship and is therefore administrator-only.
func (h *Handler) listByReference(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	refType := ReferenceType(strings.ToUpper(chi.URLParam(r, "ref_type")))
	txs, err := h.service.ListByReference(r.Context(), refType, chi.URLParam(r, "ref_id"))
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if txs == nil {
		txs = make([]*PaymentTransaction, 0)
	}
	respond(w, http.StatusOK, txs)
}

func (h *Handler) listByVendor(w http.ResponseWriter, r *http.Request) {
	vendorID := chi.URLParam(r, "vendor_id")
	if !h.requireVendorAccess(w, r, vendorID) {
		return
	}
	txs, err := h.service.ListByVendor(r.Context(), vendorID)
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if txs == nil {
		txs = make([]*PaymentTransaction, 0)
	}
	respond(w, http.StatusOK, txs)
}

// ── Webhook Handlers ──────────────────────────────────────────────────────────

func (h *Handler) webhookMTN(w http.ResponseWriter, r *http.Request) {
	if !h.authorizeWebhook(w, r) {
		return
	}
	var raw map[string]interface{}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&raw); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
		return
	}
	payload := WebhookPayload{
		Provider:    string(ProviderMTNMomo),
		ExternalRef: stringFromMap(raw, "externalId", "referenceId", "financialTransactionId"),
		Status:      stringFromMap(raw, "status"),
		Amount:      floatFromMap(raw, "amount"),
		Currency:    stringFromMap(raw, "currency"),
		PhoneNumber: stringFromMap(raw, "payer.partyId", "payer.msisdn"),
		RawPayload:  raw,
	}
	h.processWebhook(w, r, payload)
}

func (h *Handler) webhookAirtel(w http.ResponseWriter, r *http.Request) {
	if !h.authorizeWebhook(w, r) {
		return
	}
	var raw map[string]interface{}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&raw); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
		return
	}
	txData, _ := raw["transaction"].(map[string]interface{})
	if txData == nil {
		txData = raw
	}
	payload := WebhookPayload{
		Provider:    string(ProviderAirtel),
		ExternalRef: stringFromMap(txData, "id", "airtel_money_id"),
		Status:      stringFromMap(txData, "status"),
		Amount:      floatFromMap(txData, "amount"),
		Currency:    stringFromMap(txData, "currency"),
		PhoneNumber: stringFromMap(txData, "msisdn", "subscriber.msisdn"),
		RawPayload:  raw,
	}
	h.processWebhook(w, r, payload)
}

func (h *Handler) processWebhook(w http.ResponseWriter, r *http.Request, payload WebhookPayload) {
	tx, err := h.service.HandleWebhook(r.Context(), payload)
	if err != nil {
		// Return 200 for an accepted-but-ignored callback to avoid retry storms while
		// keeping the detailed reason out of the public response.
		respond(w, http.StatusOK, map[string]string{"status": "ignored"})
		return
	}
	respond(w, http.StatusOK, map[string]interface{}{"status": "processed", "transaction_id": tx.ID})
}

func (h *Handler) authorizeWebhook(w http.ResponseWriter, r *http.Request) bool {
	if !strings.EqualFold(os.Getenv("APP_ENV"), "production") {
		return true
	}
	secret := os.Getenv("PAYMENT_WEBHOOK_SHARED_SECRET")
	if secret == "" {
		respond(w, http.StatusServiceUnavailable, map[string]string{"error": "payment webhook receiver is not configured"})
		return false
	}
	provided := r.Header.Get("X-Printa-Webhook-Token")
	if provided == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(secret)) != 1 {
		respond(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized webhook"})
		return false
	}
	return true
}

// ── Access control helpers ────────────────────────────────────────────────────

func (h *Handler) bindVendorRequest(w http.ResponseWriter, r *http.Request, vendorID *string) bool {
	if middleware.GetRole(r) == middleware.RoleAdmin {
		if *vendorID == "" {
			respond(w, http.StatusBadRequest, map[string]string{"error": "vendor_id is required"})
			return false
		}
		return true
	}
	if middleware.GetRole(r) != middleware.RoleVendor {
		respond(w, http.StatusForbidden, map[string]string{"error": "insufficient permissions"})
		return false
	}
	currentVendor, err := h.vendorService.GetVendor(r.Context(), middleware.GetUserID(r))
	if err != nil {
		respond(w, http.StatusForbidden, map[string]string{"error": "authenticated vendor profile is required"})
		return false
	}
	*vendorID = currentVendor.ID.String()
	return true
}

func (h *Handler) requireVendorAccess(w http.ResponseWriter, r *http.Request, vendorID string) bool {
	if middleware.GetRole(r) == middleware.RoleAdmin {
		return true
	}
	if middleware.GetRole(r) != middleware.RoleVendor {
		respond(w, http.StatusForbidden, map[string]string{"error": "insufficient permissions"})
		return false
	}
	currentVendor, err := h.vendorService.GetVendor(r.Context(), middleware.GetUserID(r))
	if err != nil {
		respond(w, http.StatusForbidden, map[string]string{"error": "authenticated vendor profile is required"})
		return false
	}
	if currentVendor.ID.String() != vendorID {
		respond(w, http.StatusForbidden, map[string]string{"error": "vendor scope does not match authenticated user"})
		return false
	}
	return true
}

func (h *Handler) requirePaymentAccess(w http.ResponseWriter, r *http.Request, paymentID string) (*PaymentTransaction, bool) {
	tx, err := h.service.GetByID(r.Context(), paymentID)
	if err != nil {
		respond(w, http.StatusNotFound, map[string]string{"error": "payment transaction not found"})
		return nil, false
	}
	if middleware.GetRole(r) == middleware.RoleAdmin {
		return tx, true
	}
	if tx.VendorID == nil || !h.requireVendorAccess(w, r, tx.VendorID.String()) {
		return nil, false
	}
	return tx, true
}

func (h *Handler) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if middleware.GetRole(r) != middleware.RoleAdmin {
		respond(w, http.StatusForbidden, map[string]string{"error": "administrator access is required"})
		return false
	}
	return true
}

// ── Payload helpers ───────────────────────────────────────────────────────────

func respond(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func stringFromMap(m map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		var value interface{} = m
		for _, part := range strings.Split(key, ".") {
			object, ok := value.(map[string]interface{})
			if !ok {
				value = nil
				break
			}
			value = object[part]
		}
		if text, ok := value.(string); ok && text != "" {
			return text
		}
	}
	return ""
}

func floatFromMap(m map[string]interface{}, key string) float64 {
	value := interface{}(m)
	for _, part := range strings.Split(key, ".") {
		object, ok := value.(map[string]interface{})
		if !ok {
			return 0
		}
		value = object[part]
	}
	switch number := value.(type) {
	case float64:
		return number
	case float32:
		return float64(number)
	case int:
		return float64(number)
	case string:
		var parsed float64
		if err := json.Unmarshal([]byte(number), &parsed); err == nil {
			return parsed
		}
	}
	return 0
}
