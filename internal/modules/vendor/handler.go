package vendor

import (
	"encoding/json"
	"net/http"

	"github.com/georgemunganga/printa-backend/internal/middleware"
	"github.com/georgemunganga/printa-backend/internal/modules/policyconsent"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service        Service
	consentService policyconsent.Service
}

func NewHandler(service Service, consentService policyconsent.Service) *Handler {
	return &Handler{service: service, consentService: consentService}
}

func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Route("/api/v1/vendor", func(r chi.Router) {
		r.Post("/onboard", h.onboardVendor)
		r.Get("/profile", h.getVendor)
	})
}

func (h *Handler) onboardVendor(w http.ResponseWriter, r *http.Request) {
	type request struct {
		OwnerID      string `json:"owner_id"`
		BusinessName string `json:"business_name"`
		TaxID        string `json:"tax_id"`
	}

	var req request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if middleware.GetRole(r) != middleware.RoleAdmin {
		if middleware.GetRole(r) != middleware.RoleVendor {
			http.Error(w, "insufficient permissions", http.StatusForbidden)
			return
		}
		req.OwnerID = middleware.GetUserID(r)
	}

	accepted, err := h.consentService.HasRequiredAcceptance(r.Context(), req.OwnerID)
	if err != nil {
		http.Error(w, "unable to verify required vendor policy acceptance", http.StatusInternalServerError)
		return
	}
	if !accepted {
		http.Error(w, "acceptance of the current Vendor Terms and Privacy Notice is required before onboarding", http.StatusPreconditionRequired)
		return
	}

	vendor, err := h.service.OnboardVendor(r.Context(), req.OwnerID, req.BusinessName, req.TaxID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := h.consentService.AttachAcceptedPoliciesToVendor(r.Context(), req.OwnerID, vendor.ID.String(), policyconsent.RequestIP(r), r.UserAgent()); err != nil {
		http.Error(w, "vendor profile was created but consent evidence could not be attached; retry onboarding", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(vendor)
}

func (h *Handler) getVendor(w http.ResponseWriter, r *http.Request) {
	ownerID := r.URL.Query().Get("owner_id")
	if middleware.GetRole(r) != middleware.RoleAdmin {
		if middleware.GetRole(r) != middleware.RoleVendor {
			http.Error(w, "insufficient permissions", http.StatusForbidden)
			return
		}
		ownerID = middleware.GetUserID(r)
	}
	if ownerID == "" {
		http.Error(w, "owner_id is required", http.StatusBadRequest)
		return
	}

	vendor, err := h.service.GetVendor(r.Context(), ownerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(vendor)
}
