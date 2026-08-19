package wallet

import (
	"encoding/json"
	"net/http"

	"github.com/georgemunganga/printa-backend/internal/middleware"
	"github.com/georgemunganga/printa-backend/internal/modules/vendor"
	"github.com/go-chi/chi/v5"
)

// Handler exposes only read-only vendor wallet reporting. Fund movement, account
// activation, fee-policy administration, and withdrawals remain outside this API
// until their operational and compliance gates are approved.
type Handler struct {
	service       Service
	vendorService vendor.Service
}

func NewHandler(service Service, vendorService vendor.Service) *Handler {
	return &Handler{service: service, vendorService: vendorService}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api/v1/vendor/wallet", h.overview)
}

func (h *Handler) overview(w http.ResponseWriter, r *http.Request) {
	if middleware.GetRole(r) != middleware.RoleVendor {
		respond(w, http.StatusForbidden, map[string]string{"error": "vendor wallet access is restricted to vendor accounts"})
		return
	}
	vendorProfile, err := h.vendorService.GetVendor(r.Context(), middleware.GetUserID(r))
	if err != nil {
		respond(w, http.StatusNotFound, map[string]string{"error": "vendor profile setup is required before wallet reporting is available"})
		return
	}
	overview, err := h.service.GetOverviewByVendor(r.Context(), vendorProfile.ID)
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": "unable to load wallet overview"})
		return
	}
	respond(w, http.StatusOK, overview)
}

func respond(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
