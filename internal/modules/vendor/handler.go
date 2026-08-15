package vendor

import (
	"encoding/json"
	"net/http"

	"github.com/georgemunganga/printa-backend/internal/middleware"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
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

	vendor, err := h.service.OnboardVendor(r.Context(), req.OwnerID, req.BusinessName, req.TaxID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
