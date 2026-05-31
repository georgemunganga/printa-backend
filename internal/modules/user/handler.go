package user

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Handler handles HTTP requests for the user module.
type Handler struct {
	service Service
}

// NewHandler creates a new user handler.
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers all user routes (legacy — used for backward compat).
func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Post("/api/v1/users/register", h.register)
	router.Get("/api/v1/users/{id}", h.getUser)
	router.Get("/api/v1/users", h.listUsers)
}

// RegisterPublicRoutes registers routes that do not require authentication.
func (h *Handler) RegisterPublicRoutes(router *chi.Mux) {
	router.Post("/api/v1/users/register", h.register)
}

// RegisterProtectedRoutes registers routes that require authentication.
func (h *Handler) RegisterProtectedRoutes(router chi.Router) {
	router.Get("/api/v1/users/{id}", h.getUser)
	router.Get("/api/v1/users", h.listUsers)
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email     string `json:"email"`
		Password  string `json:"password"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Role      string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	u, err := h.service.RegisterUser(r.Context(), req.Email, req.Password, req.FirstName, req.LastName, req.Role)
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusCreated, u)
}

func (h *Handler) getUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	u, err := h.service.GetUser(r.Context(), id)
	if err != nil {
		respond(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}
	respond(w, http.StatusOK, u)
}

func (h *Handler) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.service.ListUsers(r.Context())
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, map[string]interface{}{"total": len(users), "users": users})
}

func respond(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}
