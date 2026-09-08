package paymentmethod

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/georgemunganga/printa-backend/internal/middleware"
	"github.com/go-chi/chi/v5"
)

type Handler struct{ service Service }

func NewHandler(service Service) *Handler { return &Handler{service: service} }
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api/v1/payment-methods", h.list)
	r.Post("/api/v1/payment-methods", h.create)
	r.Patch("/api/v1/payment-methods/{id}", h.update)
	r.Delete("/api/v1/payment-methods/{id}", h.delete)
	r.Put("/api/v1/payment-methods/{id}/default", h.setDefault)
}
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	methods, err := h.service.List(r.Context(), middleware.GetUserID(r))
	if err != nil {
		respondError(w, err)
		return
	}
	respond(w, http.StatusOK, methods)
}
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req UpsertRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	method, err := h.service.Create(r.Context(), middleware.GetUserID(r), req)
	if err != nil {
		respondError(w, err)
		return
	}
	respond(w, http.StatusCreated, method)
}
func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	var req UpsertRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	method, err := h.service.Update(r.Context(), chi.URLParam(r, "id"), middleware.GetUserID(r), req)
	if err != nil {
		respondError(w, err)
		return
	}
	respond(w, http.StatusOK, method)
}
func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Delete(r.Context(), chi.URLParam(r, "id"), middleware.GetUserID(r)); err != nil {
		respondError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h *Handler) setDefault(w http.ResponseWriter, r *http.Request) {
	method, err := h.service.SetDefault(r.Context(), chi.URLParam(r, "id"), middleware.GetUserID(r))
	if err != nil {
		respondError(w, err)
		return
	}
	respond(w, http.StatusOK, method)
}
func respondError(w http.ResponseWriter, err error) {
	if err == sql.ErrNoRows {
		respond(w, http.StatusNotFound, map[string]string{"error": "payment method not found"})
		return
	}
	respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
}
func respond(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
