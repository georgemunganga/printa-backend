package auth

import (
	"encoding/json"
	"net/http"
	"net/url"

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
	router.Post("/api/v1/auth/otp/request", h.requestOTP)
	router.Post("/api/v1/auth/otp/verify", h.verifyOTP)
	router.Get("/api/v1/auth/google/start", h.googleStart)
	router.Get("/api/v1/auth/google/callback", h.googleCallback)
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

func (h *Handler) requestOTP(w http.ResponseWriter, r *http.Request) {
	var req OTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	resp, err := h.service.RequestOTP(r.Context(), req)
	if err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusCreated, resp)
}

func (h *Handler) verifyOTP(w http.ResponseWriter, r *http.Request) {
	var req OTPVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	resp, err := h.service.VerifyOTP(r.Context(), req)
	if err != nil {
		respond(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, resp)
}

func (h *Handler) googleStart(w http.ResponseWriter, r *http.Request) {
	authURL, err := h.service.GoogleAuthURL(r.Context(), OAuthStartRequest{
		RedirectURI: r.URL.Query().Get("redirect_uri"),
		Role:        r.URL.Query().Get("role"),
		Mode:        r.URL.Query().Get("mode"),
	})
	if err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	http.Redirect(w, r, authURL, http.StatusFound)
}

func (h *Handler) googleCallback(w http.ResponseWriter, r *http.Request) {
	if oauthErr := r.URL.Query().Get("error"); oauthErr != "" {
		respond(w, http.StatusUnauthorized, map[string]string{"error": oauthErr})
		return
	}
	resp, err := h.service.HandleGoogleCallback(r.Context(), r.URL.Query().Get("code"), r.URL.Query().Get("state"))
	if err != nil {
		if resp != nil && resp.RedirectURI != "" {
			redirectURL, parseErr := url.Parse(resp.RedirectURI)
			if parseErr == nil {
				fragment := url.Values{}
				fragment.Set("error", err.Error())
				redirectURL.Fragment = fragment.Encode()
				http.Redirect(w, r, redirectURL.String(), http.StatusFound)
				return
			}
		}
		respond(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}
	redirectURL, err := url.Parse(resp.RedirectURI)
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": "invalid frontend redirect URI"})
		return
	}
	fragment := url.Values{}
	fragment.Set("token", resp.Token)
	fragment.Set("token_type", resp.TokenType)
	redirectURL.Fragment = fragment.Encode()
	http.Redirect(w, r, redirectURL.String(), http.StatusFound)
}

func respond(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}
