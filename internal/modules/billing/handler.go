package billing

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// Handler exposes billing HTTP endpoints.
type Handler struct{ service Service }

func NewHandler(service Service) *Handler { return &Handler{service: service} }

func (h *Handler) RegisterRoutes(r *chi.Mux) {
	r.Route("/api/v1/billing", func(r chi.Router) {
		// Subscription endpoints
		r.Post("/subscriptions", h.createSubscription)                                    // POST   /api/v1/billing/subscriptions
		r.Get("/subscriptions/vendor/{vendor_id}", h.getSubscription)                     // GET    /api/v1/billing/subscriptions/vendor/{id}
		r.Patch("/subscriptions/vendor/{vendor_id}/tier", h.changeTier)                   // PATCH  /api/v1/billing/subscriptions/vendor/{id}/tier
		r.Post("/subscriptions/vendor/{vendor_id}/cancel", h.cancelSubscription)          // POST   /api/v1/billing/subscriptions/vendor/{id}/cancel
		r.Patch("/subscriptions/vendor/{vendor_id}/status", h.updateStatus)               // PATCH  /api/v1/billing/subscriptions/vendor/{id}/status

		// Invoice endpoints
		r.Post("/invoices/vendor/{vendor_id}/generate", h.generateInvoice)                // POST   /api/v1/billing/invoices/vendor/{id}/generate
		r.Get("/invoices/{id}", h.getInvoice)                                             // GET    /api/v1/billing/invoices/{id}
		r.Get("/invoices/number/{number}", h.getInvoiceByNumber)                          // GET    /api/v1/billing/invoices/number/{number}
		r.Get("/invoices/vendor/{vendor_id}", h.listVendorInvoices)                       // GET    /api/v1/billing/invoices/vendor/{id}
		r.Post("/invoices/{id}/pay", h.markPaid)                                          // POST   /api/v1/billing/invoices/{id}/pay
		r.Post("/invoices/{id}/void", h.voidInvoice)                                      // POST   /api/v1/billing/invoices/{id}/void
	})
}

func (h *Handler) createSubscription(w http.ResponseWriter, r *http.Request) {
	var req CreateSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	sub, err := h.service.CreateSubscription(r.Context(), req)
	if err != nil {
		code := http.StatusInternalServerError
		msg := err.Error()
		if strings.Contains(msg, "required") || strings.Contains(msg, "not found") {
			code = http.StatusBadRequest
		} else if strings.Contains(msg, "already has") {
			code = http.StatusConflict
		}
		respond(w, code, map[string]string{"error": msg})
		return
	}
	respond(w, http.StatusCreated, sub)
}

func (h *Handler) getSubscription(w http.ResponseWriter, r *http.Request) {
	vendorID := chi.URLParam(r, "vendor_id")
	sub, err := h.service.GetSubscription(r.Context(), vendorID)
	if err != nil {
		respond(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, sub)
}

func (h *Handler) changeTier(w http.ResponseWriter, r *http.Request) {
	vendorID := chi.URLParam(r, "vendor_id")
	var req ChangeTierRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	sub, err := h.service.ChangeTier(r.Context(), vendorID, req)
	if err != nil {
		code := http.StatusInternalServerError
		msg := err.Error()
		if strings.Contains(msg, "required") || strings.Contains(msg, "not found") || strings.Contains(msg, "already on") {
			code = http.StatusBadRequest
		} else if strings.Contains(msg, "cannot change tier") {
			code = http.StatusUnprocessableEntity
		}
		respond(w, code, map[string]string{"error": msg})
		return
	}
	respond(w, http.StatusOK, sub)
}

func (h *Handler) cancelSubscription(w http.ResponseWriter, r *http.Request) {
	vendorID := chi.URLParam(r, "vendor_id")
	var req CancelSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	sub, err := h.service.CancelSubscription(r.Context(), vendorID, req)
	if err != nil {
		code := http.StatusInternalServerError
		msg := err.Error()
		if strings.Contains(msg, "already cancelled") {
			code = http.StatusConflict
		} else if strings.Contains(msg, "cannot cancel") || strings.Contains(msg, "not found") {
			code = http.StatusUnprocessableEntity
		}
		respond(w, code, map[string]string{"error": msg})
		return
	}
	respond(w, http.StatusOK, sub)
}

func (h *Handler) updateStatus(w http.ResponseWriter, r *http.Request) {
	vendorID := chi.URLParam(r, "vendor_id")
	var req UpdateSubStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	sub, err := h.service.UpdateStatus(r.Context(), vendorID, req)
	if err != nil {
		code := http.StatusInternalServerError
		msg := err.Error()
		if strings.Contains(msg, "cannot transition") {
			code = http.StatusUnprocessableEntity
		} else if strings.Contains(msg, "required") || strings.Contains(msg, "not found") {
			code = http.StatusBadRequest
		}
		respond(w, code, map[string]string{"error": msg})
		return
	}
	respond(w, http.StatusOK, sub)
}

func (h *Handler) generateInvoice(w http.ResponseWriter, r *http.Request) {
	vendorID := chi.URLParam(r, "vendor_id")
	idempotencyKey := r.Header.Get("Idempotency-Key")
	inv, err := h.service.GenerateInvoice(r.Context(), vendorID, idempotencyKey)
	if err != nil {
		code := http.StatusInternalServerError
		msg := err.Error()
		if strings.Contains(msg, "not found") || strings.Contains(msg, "cancelled") {
			code = http.StatusBadRequest
		}
		respond(w, code, map[string]string{"error": msg})
		return
	}
	respond(w, http.StatusCreated, inv)
}

func (h *Handler) getInvoice(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	inv, err := h.service.GetInvoice(r.Context(), id)
	if err != nil {
		respond(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, inv)
}

func (h *Handler) getInvoiceByNumber(w http.ResponseWriter, r *http.Request) {
	number := chi.URLParam(r, "number")
	inv, err := h.service.GetInvoiceByNumber(r.Context(), number)
	if err != nil {
		respond(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, inv)
}

func (h *Handler) listVendorInvoices(w http.ResponseWriter, r *http.Request) {
	vendorID := chi.URLParam(r, "vendor_id")
	invs, err := h.service.ListVendorInvoices(r.Context(), vendorID)
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, invs)
}

func (h *Handler) markPaid(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req MarkPaidRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	inv, err := h.service.MarkInvoicePaid(r.Context(), id, req)
	if err != nil {
		code := http.StatusInternalServerError
		msg := err.Error()
		if strings.Contains(msg, "already") || strings.Contains(msg, "voided") {
			code = http.StatusConflict
		} else if strings.Contains(msg, "not found") {
			code = http.StatusNotFound
		}
		respond(w, code, map[string]string{"error": msg})
		return
	}
	respond(w, http.StatusOK, inv)
}

func (h *Handler) voidInvoice(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	inv, err := h.service.VoidInvoice(r.Context(), id)
	if err != nil {
		code := http.StatusInternalServerError
		msg := err.Error()
		if strings.Contains(msg, "paid") || strings.Contains(msg, "already voided") {
			code = http.StatusConflict
		} else if strings.Contains(msg, "not found") {
			code = http.StatusNotFound
		}
		respond(w, code, map[string]string{"error": msg})
		return
	}
	respond(w, http.StatusOK, inv)
}

func respond(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}
