package delivery

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/georgemunganga/printa-backend/internal/middleware"
	"github.com/go-chi/chi/v5"
)

// Handler exposes delivery-location endpoints to authenticated customers.
type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api/v1/delivery/locations", h.listLocations)
	r.Post("/api/v1/delivery/locations", h.createLocation)
	r.Patch("/api/v1/delivery/locations/{id}", h.updateLocation)
	r.Delete("/api/v1/delivery/locations/{id}", h.deleteLocation)
	r.Put("/api/v1/delivery/locations/{id}/default", h.setDefaultLocation)
}

func (h *Handler) listLocations(w http.ResponseWriter, r *http.Request) {
	if !requireCustomer(w, r) {
		return
	}
	locations, err := h.service.ListLocations(r.Context(), middleware.GetUserID(r))
	if err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, locations)
}

func (h *Handler) createLocation(w http.ResponseWriter, r *http.Request) {
	if !requireCustomer(w, r) {
		return
	}
	var req UpsertLocationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	location, err := h.service.CreateLocation(r.Context(), middleware.GetUserID(r), req)
	if err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusCreated, location)
}

func (h *Handler) updateLocation(w http.ResponseWriter, r *http.Request) {
	if !requireCustomer(w, r) {
		return
	}
	var req UpsertLocationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	location, err := h.service.UpdateLocation(r.Context(), chi.URLParam(r, "id"), middleware.GetUserID(r), req)
	if err != nil {
		respondLocationError(w, err)
		return
	}
	respond(w, http.StatusOK, location)
}

func (h *Handler) deleteLocation(w http.ResponseWriter, r *http.Request) {
	if !requireCustomer(w, r) {
		return
	}
	if err := h.service.DeleteLocation(r.Context(), chi.URLParam(r, "id"), middleware.GetUserID(r)); err != nil {
		respondLocationError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) setDefaultLocation(w http.ResponseWriter, r *http.Request) {
	if !requireCustomer(w, r) {
		return
	}
	location, err := h.service.SetDefaultLocation(r.Context(), chi.URLParam(r, "id"), middleware.GetUserID(r))
	if err != nil {
		respondLocationError(w, err)
		return
	}
	respond(w, http.StatusOK, location)
}

func requireCustomer(w http.ResponseWriter, r *http.Request) bool {
	if middleware.GetRole(r) != middleware.RoleCustomer {
		respond(w, http.StatusForbidden, map[string]string{"error": "customer role is required"})
		return false
	}
	return true
}

func respondLocationError(w http.ResponseWriter, err error) {
	if err == sql.ErrNoRows {
		respond(w, http.StatusNotFound, map[string]string{"error": "delivery location not found"})
		return
	}
	respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
}

func respond(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
