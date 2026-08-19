package policyconsent

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"

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
	router.Route("/api/v1/vendor/policies", func(r chi.Router) {
		r.Get("/status", h.status)
		r.Post("/accept", h.accept)
	})
}

func (h *Handler) status(w http.ResponseWriter, r *http.Request) {
	if middleware.GetRole(r) != middleware.RoleVendor {
		respondError(w, http.StatusForbidden, "vendor access is required")
		return
	}
	status, err := h.service.GetStatus(r.Context(), middleware.GetUserID(r))
	if err != nil {
		respondError(w, http.StatusInternalServerError, "unable to load vendor policy status")
		return
	}
	respondJSON(w, http.StatusOK, status)
}

func (h *Handler) accept(w http.ResponseWriter, r *http.Request) {
	if middleware.GetRole(r) != middleware.RoleVendor {
		respondError(w, http.StatusForbidden, "vendor access is required")
		return
	}
	var request struct {
		Policies []PolicyAcceptance `json:"policies"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		respondError(w, http.StatusBadRequest, "invalid acceptance request")
		return
	}
	status, err := h.service.Accept(
		r.Context(),
		middleware.GetUserID(r),
		"",
		request.Policies,
		RequestIP(r),
		r.UserAgent(),
		"VENDOR_ONBOARDING",
	)
	if err != nil {
		switch {
		case errors.Is(err, ErrAcceptanceUnavailable):
			respondError(w, http.StatusConflict, err.Error())
		case errors.Is(err, ErrIncompleteAcceptance), errors.Is(err, ErrInvalidAcceptance):
			respondError(w, http.StatusBadRequest, err.Error())
		default:
			respondError(w, http.StatusInternalServerError, "unable to record policy acceptance")
		}
		return
	}
	respondJSON(w, http.StatusOK, status)
}

func RequestIP(r *http.Request) net.IP {
	if forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]); forwarded != "" {
		if parsed := net.ParseIP(forwarded); parsed != nil {
			return parsed
		}
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil {
		return net.ParseIP(host)
	}
	return net.ParseIP(strings.TrimSpace(r.RemoteAddr))
}

func respondJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}
