package comms

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	appMiddleware "github.com/georgemunganga/printa-backend/internal/middleware"
)

type Handler struct{ svc Service }

func NewHandler(svc Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/v1/comms", func(r chi.Router) {
		// Send a message — available to ADMIN and VENDOR roles
		r.Post("/send", h.send)

		// Delivery logs — ADMIN only
		r.Group(func(r chi.Router) {
			r.Use(appMiddleware.RequireRole(appMiddleware.RoleAdmin))
			r.Get("/logs", h.listLogs)
			r.Get("/logs/{id}", h.getLog)
		})
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// POST /api/v1/comms/send
func (h *Handler) send(w http.ResponseWriter, r *http.Request) {
	var req SendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Channel == "" {
		writeError(w, http.StatusBadRequest, "channel is required")
		return
	}
	if req.Recipient == "" {
		writeError(w, http.StatusBadRequest, "recipient is required")
		return
	}
	if req.Body == "" {
		writeError(w, http.StatusBadRequest, "body is required")
		return
	}

	// Inject caller's user ID as recipient_id if not set
	if req.RecipientID == "" {
		req.RecipientID = appMiddleware.GetUserID(r)
	}

	// Read idempotency key from header if not in body
	if req.IdempotencyKey == "" {
		req.IdempotencyKey = r.Header.Get("Idempotency-Key")
	}

	result, err := h.svc.Send(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

// GET /api/v1/comms/logs
func (h *Handler) listLogs(w http.ResponseWriter, r *http.Request) {
	filter := ListFilter{Limit: 20}
	if v := r.URL.Query().Get("channel"); v != "" {
		filter.Channel = ChannelType(v)
	}
	if v := r.URL.Query().Get("recipient_id"); v != "" {
		filter.RecipientID = v
	}
	if v := r.URL.Query().Get("status"); v != "" {
		filter.Status = DeliveryStatus(v)
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			filter.Limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			filter.Offset = n
		}
	}

	logs, total, err := h.svc.ListLogs(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"logs":   logs,
		"total":  total,
		"limit":  filter.Limit,
		"offset": filter.Offset,
	})
}

// GET /api/v1/comms/logs/{id}
func (h *Handler) getLog(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	log, err := h.svc.GetLog(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "delivery log not found")
		return
	}
	writeJSON(w, http.StatusOK, log)
}
