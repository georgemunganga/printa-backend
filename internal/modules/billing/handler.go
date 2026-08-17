package billing

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/georgemunganga/printa-backend/internal/middleware"
	"github.com/georgemunganga/printa-backend/internal/modules/vendor"
	"github.com/go-chi/chi/v5"
)

// Handler exposes billing HTTP endpoints.
type Handler struct {
	service       Service
	vendorService vendor.Service
}

func NewHandler(service Service, vendorService vendor.Service) *Handler {
	return &Handler{service: service, vendorService: vendorService}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/v1/billing", func(r chi.Router) {
		r.Post("/subscriptions", h.createSubscription)
		r.Get("/subscriptions/vendor/{vendor_id}", h.getSubscription)
		r.Patch("/subscriptions/vendor/{vendor_id}/tier", h.changeTier)
		r.Post("/subscriptions/vendor/{vendor_id}/cancel", h.cancelSubscription)
		r.Patch("/subscriptions/vendor/{vendor_id}/status", h.updateStatus)

		r.Post("/invoices/vendor/{vendor_id}/generate", h.generateInvoice)
		r.Get("/invoices/{id}", h.getInvoice)
		r.Get("/invoices/number/{number}", h.getInvoiceByNumber)
		r.Get("/invoices/vendor/{vendor_id}", h.listVendorInvoices)
		r.Post("/invoices/{id}/pay", h.markPaid)
		r.Post("/invoices/{id}/void", h.voidInvoice)
	})
}

func (h *Handler) createSubscription(w http.ResponseWriter, r *http.Request) {
	var req CreateSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if !h.bindVendorRequest(w, r, &req.VendorID) {
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
	if !h.requireVendorAccess(w, r, vendorID) {
		return
	}
	sub, err := h.service.GetSubscription(r.Context(), vendorID)
	if err != nil {
		respond(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, sub)
}

func (h *Handler) changeTier(w http.ResponseWriter, r *http.Request) {
	vendorID := chi.URLParam(r, "vendor_id")
	if !h.requireVendorAccess(w, r, vendorID) {
		return
	}
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
	if !h.requireVendorAccess(w, r, vendorID) {
		return
	}
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
	if !h.requireAdmin(w, r) {
		return
	}
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
	if !h.requireVendorAccess(w, r, vendorID) {
		return
	}
	inv, err := h.service.GenerateInvoice(r.Context(), vendorID, r.Header.Get("Idempotency-Key"))
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
	inv, ok := h.requireInvoiceAccess(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	respond(w, http.StatusOK, inv)
}

func (h *Handler) getInvoiceByNumber(w http.ResponseWriter, r *http.Request) {
	inv, err := h.service.GetInvoiceByNumber(r.Context(), chi.URLParam(r, "number"))
	if err != nil {
		respond(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	if !h.requireVendorAccess(w, r, inv.VendorID.String()) {
		return
	}
	respond(w, http.StatusOK, inv)
}

func (h *Handler) listVendorInvoices(w http.ResponseWriter, r *http.Request) {
	vendorID := chi.URLParam(r, "vendor_id")
	if !h.requireVendorAccess(w, r, vendorID) {
		return
	}
	invs, err := h.service.ListVendorInvoices(r.Context(), vendorID)
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if invs == nil {
		invs = make([]*BillingInvoice, 0)
	}
	respond(w, http.StatusOK, invs)
}

func (h *Handler) markPaid(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
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
	if !h.requireAdmin(w, r) {
		return
	}
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

func (h *Handler) requireInvoiceAccess(w http.ResponseWriter, r *http.Request, invoiceID string) (*BillingInvoice, bool) {
	inv, err := h.service.GetInvoice(r.Context(), invoiceID)
	if err != nil {
		respond(w, http.StatusNotFound, map[string]string{"error": "invoice not found"})
		return nil, false
	}
	if !h.requireVendorAccess(w, r, inv.VendorID.String()) {
		return nil, false
	}
	return inv, true
}

func (h *Handler) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if middleware.GetRole(r) != middleware.RoleAdmin {
		respond(w, http.StatusForbidden, map[string]string{"error": "administrator access is required"})
		return false
	}
	return true
}

func respond(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
