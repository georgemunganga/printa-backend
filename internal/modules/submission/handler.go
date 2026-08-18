package submission

import (
	"encoding/json"
	"net/http"

	"github.com/georgemunganga/printa-backend/internal/middleware"
	"github.com/go-chi/chi/v5"
)

type Handler struct{ service Service }

func NewHandler(service Service) *Handler { return &Handler{service: service} }

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/v1/submissions", func(r chi.Router) {
		r.Get("/mine", h.listOwn)
		r.Post("/support", h.createSupport)
		r.Post("/feedback", h.createFeedback)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole(middleware.RoleAdmin))
			r.Get("/customer", h.listCustomer)
			r.Get("/vendor", h.listVendor)
		})
	})
}

func (h *Handler) createSupport(w http.ResponseWriter, r *http.Request) {
	role, ok := requesterRole(w, r)
	if !ok {
		return
	}
	var req CreateSupportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	record, err := h.service.CreateSupport(r.Context(), middleware.GetUserID(r), role, req)
	if err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusCreated, record)
}
func (h *Handler) createFeedback(w http.ResponseWriter, r *http.Request) {
	role, ok := requesterRole(w, r)
	if !ok {
		return
	}
	var req CreateFeedbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	record, err := h.service.CreateFeedback(r.Context(), middleware.GetUserID(r), role, req)
	if err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusCreated, record)
}
func (h *Handler) listOwn(w http.ResponseWriter, r *http.Request) {
	role, ok := requesterRole(w, r)
	if !ok {
		return
	}
	records, err := h.service.ListOwn(r.Context(), middleware.GetUserID(r), role)
	if err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, records)
}
func (h *Handler) listCustomer(w http.ResponseWriter, r *http.Request) {
	h.listRole(w, r, RequesterRoleCustomer)
}
func (h *Handler) listVendor(w http.ResponseWriter, r *http.Request) {
	h.listRole(w, r, RequesterRoleVendor)
}
func (h *Handler) listRole(w http.ResponseWriter, r *http.Request, role RequesterRole) {
	records, err := h.service.ListByRole(r.Context(), role)
	if err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, records)
}
func requesterRole(w http.ResponseWriter, r *http.Request) (RequesterRole, bool) {
	switch middleware.GetRole(r) {
	case middleware.RoleCustomer:
		return RequesterRoleCustomer, true
	case middleware.RoleVendor:
		return RequesterRoleVendor, true
	}
	respond(w, http.StatusForbidden, map[string]string{"error": "customer or vendor permission is required"})
	return "", false
}
func respond(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
