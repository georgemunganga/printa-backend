package auth

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Handler handles HTTP requests for the auth module.
type Handler struct {
	service Service
}

// NewHandler creates a new auth handler.
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers auth routes on a chi.Mux (public — no auth required).
func (h *Handler) RegisterRoutes(router *chi.Mux) {
	router.Post("/api/v1/auth/login", h.login)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	token, err := h.service.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		respond(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, map[string]string{
		"token":      token,
		"token_type": "Bearer",
	})
}

func respond(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}
