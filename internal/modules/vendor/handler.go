package vendor

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(router *chi.Mux) {
	router.Post("/vendor/onboard", h.onboardVendor)
	router.Get("/vendor/profile", h.getVendor)
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

	vendor, err := h.service.GetVendor(r.Context(), ownerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(vendor)
}
