package operatingstatus

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/georgemunganga/printa-backend/internal/middleware"
	"github.com/georgemunganga/printa-backend/internal/modules/vendor"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service       Service
	vendorService vendor.Service
}

func NewHandler(service Service, vendorService vendor.Service) *Handler {
	return &Handler{service: service, vendorService: vendorService}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/v1/vendor/operating-status", func(r chi.Router) {
		r.Get("/", h.getCurrentStatus)
		r.Post("/grace-request", h.requestGrace)
	})
	r.Patch("/api/v1/admin/vendors/{vendor_id}/compliance", h.decideCompliance)
}

func (h *Handler) getCurrentStatus(w http.ResponseWriter, r *http.Request) {
	vendorProfile, ok := h.requireCurrentVendor(w, r)
	if !ok {
		return
	}
	status, err := h.service.GetStatus(r.Context(), vendorProfile.ID.String())
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "unable to load vendor operating status"})
		return
	}
	respondJSON(w, http.StatusOK, status)
}

func (h *Handler) requestGrace(w http.ResponseWriter, r *http.Request) {
	vendorProfile, ok := h.requireCurrentVendor(w, r)
	if !ok {
		return
	}
	result, err := h.service.RequestGrace(r.Context(), vendorProfile.ID.String(), middleware.GetUserID(r))
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "unable to request subscription grace period"})
		return
	}
	if !result.Granted {
		respondJSON(w, http.StatusConflict, result)
		return
	}
	respondJSON(w, http.StatusCreated, result)
}

func (h *Handler) decideCompliance(w http.ResponseWriter, r *http.Request) {
	if middleware.GetRole(r) != middleware.RoleAdmin {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "administrator access is required"})
		return
	}
	var req ComplianceDecisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid compliance decision payload"})
		return
	}
	req.Status = strings.ToUpper(strings.TrimSpace(req.Status))
	req.Reason = strings.TrimSpace(req.Reason)
	decision, err := h.service.DecideCompliance(r.Context(), chi.URLParam(r, "vendor_id"), middleware.GetUserID(r), req)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, decision)
}

func (h *Handler) requireCurrentVendor(w http.ResponseWriter, r *http.Request) (*vendor.Vendor, bool) {
	if middleware.GetRole(r) != middleware.RoleVendor {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "vendor access is required"})
		return nil, false
	}
	vendorProfile, err := h.vendorService.GetVendor(r.Context(), middleware.GetUserID(r))
	if err != nil {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "authenticated vendor profile is required"})
		return nil, false
	}
	return vendorProfile, true
}

func respondJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
