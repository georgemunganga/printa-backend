package payment

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// Handler exposes payment HTTP endpoints.
type Handler struct{ service Service }

func NewHandler(service Service) *Handler { return &Handler{service: service} }

// RegisterRoutes registers all routes (legacy — kept for backward compat).
func (h *Handler) RegisterRoutes(r chi.Router) {
	h.RegisterWebhookRoutes(r)
	h.RegisterProtectedRoutes(r)
}

// RegisterWebhookRoutes registers provider webhook endpoints (no JWT required).
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
	id := chi.URLParam(r, "id")
	tx, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		respond(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, tx)
}

func (h *Handler) verify(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
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

func (h *Handler) listByReference(w http.ResponseWriter, r *http.Request) {
	refType := ReferenceType(strings.ToUpper(chi.URLParam(r, "ref_type")))
	refID := chi.URLParam(r, "ref_id")
	txs, err := h.service.ListByReference(r.Context(), refType, refID)
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, txs)
}

func (h *Handler) listByVendor(w http.ResponseWriter, r *http.Request) {
	vendorID := chi.URLParam(r, "vendor_id")
	txs, err := h.service.ListByVendor(r.Context(), vendorID)
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, txs)
}

// ── Webhook Handlers ──────────────────────────────────────────────────────────

func (h *Handler) webhookMTN(w http.ResponseWriter, r *http.Request) {
	var raw map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
		return
	}
	payload := WebhookPayload{
		Provider:    string(ProviderMTNMomo),
		ExternalRef: stringFromMap(raw, "externalId", "referenceId", "financialTransactionId"),
		Status:      stringFromMap(raw, "status"),
		Amount:      floatFromMap(raw, "amount"),
		Currency:    stringFromMap(raw, "currency"),
		PhoneNumber: stringFromMap(raw, "payer.partyId"),
		RawPayload:  raw,
	}
	tx, err := h.service.HandleWebhook(r.Context(), payload)
	if err != nil {
		respond(w, http.StatusOK, map[string]string{"status": "ignored", "reason": err.Error()})
		return
	}
	respond(w, http.StatusOK, map[string]interface{}{"status": "processed", "transaction_id": tx.ID})
}

func (h *Handler) webhookAirtel(w http.ResponseWriter, r *http.Request) {
	var raw map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
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
		PhoneNumber: stringFromMap(txData, "msisdn"),
		RawPayload:  raw,
	}
	tx, err := h.service.HandleWebhook(r.Context(), payload)
	if err != nil {
		respond(w, http.StatusOK, map[string]string{"status": "ignored", "reason": err.Error()})
		return
	}
	respond(w, http.StatusOK, map[string]interface{}{"status": "processed", "transaction_id": tx.ID})
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func respond(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

func stringFromMap(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

func floatFromMap(m map[string]interface{}, key string) float64 {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case float64:
			return n
		case int:
			return float64(n)
		}
	}
	return 0
}
