package notification

import (
	"encoding/json"
	"net/http"
	"strconv"

	appMiddleware "github.com/georgemunganga/printa-backend/internal/middleware"
	"github.com/go-chi/chi/v5"
)

type Handler struct{ svc Service }

func NewHandler(svc Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/v1/notifications", func(r chi.Router) {
		// Recipient-scoped — any authenticated user can access their own notifications
		r.Get("/", h.listNotifications)
		r.Get("/unread-count", h.getUnreadCount)
		r.Post("/mark-all-read", h.markAllRead)
		r.Get("/{id}", h.getNotification)
		r.Post("/{id}/read", h.markRead)
		r.Post("/{id}/dismiss", h.dismiss)
		r.Delete("/{id}", h.deleteNotification)

		// Admin-only: create and broadcast notifications
		r.Group(func(r chi.Router) {
			r.Use(appMiddleware.RequireRole(appMiddleware.RoleAdmin))
			r.Post("/", h.createNotification)
			r.Post("/bulk", h.bulkCreate)
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

// GET /api/v1/notifications
func (h *Handler) listNotifications(w http.ResponseWriter, r *http.Request) {
	recipientID := appMiddleware.GetUserID(r)
	if recipientID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	filter := ListFilter{
		RecipientID: recipientID,
		Limit:       20,
		Offset:      0,
	}
	if v := r.URL.Query().Get("status"); v != "" {
		filter.Status = Status(v)
	}
	if v := r.URL.Query().Get("type"); v != "" {
		filter.Type = Type(v)
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

	notifications, total, err := h.svc.List(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if notifications == nil {
		notifications = make([]*Notification, 0)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"notifications": notifications,
		"total":         total,
		"limit":         filter.Limit,
		"offset":        filter.Offset,
	})
}

// GET /api/v1/notifications/unread-count
func (h *Handler) getUnreadCount(w http.ResponseWriter, r *http.Request) {
	recipientID := appMiddleware.GetUserID(r)
	if recipientID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	count, err := h.svc.GetUnreadCount(r.Context(), recipientID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, UnreadCount{RecipientID: recipientID, Count: count})
}

// GET /api/v1/notifications/{id}
func (h *Handler) getNotification(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	n, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	// Ensure users can only read their own notifications
	recipientID := appMiddleware.GetUserID(r)
	role := appMiddleware.GetRole(r)
	if n.RecipientID != recipientID && role != appMiddleware.RoleAdmin {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	writeJSON(w, http.StatusOK, n)
}

// POST /api/v1/notifications/{id}/read
func (h *Handler) markRead(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	recipientID := appMiddleware.GetUserID(r)
	if err := h.svc.MarkRead(r.Context(), id, recipientID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "read"})
}

// POST /api/v1/notifications/mark-all-read
func (h *Handler) markAllRead(w http.ResponseWriter, r *http.Request) {
	recipientID := appMiddleware.GetUserID(r)
	if err := h.svc.MarkAllRead(r.Context(), recipientID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "all_read"})
}

// POST /api/v1/notifications/{id}/dismiss
func (h *Handler) dismiss(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	recipientID := appMiddleware.GetUserID(r)
	if err := h.svc.Dismiss(r.Context(), id, recipientID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "dismissed"})
}

// DELETE /api/v1/notifications/{id}
func (h *Handler) deleteNotification(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	recipientID := appMiddleware.GetUserID(r)
	if err := h.svc.Delete(r.Context(), id, recipientID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/v1/notifications (ADMIN only)
func (h *Handler) createNotification(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	n, err := h.svc.Create(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, n)
}

// POST /api/v1/notifications/bulk (ADMIN only)
func (h *Handler) bulkCreate(w http.ResponseWriter, r *http.Request) {
	var req BulkCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.svc.BulkCreate(r.Context(), req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"status":     "created",
		"recipients": len(req.RecipientIDs),
	})
}
