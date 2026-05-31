package admin

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	appMiddleware "github.com/georgemunganga/printa-backend/internal/middleware"
)

type Handler struct{ service Service }

func NewHandler(service Service) *Handler { return &Handler{service: service} }

// RegisterRoutes registers all admin endpoints under /api/v1/admin — ADMIN role required.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/v1/admin", func(r chi.Router) {
		r.Use(appMiddleware.RequireRole(appMiddleware.RoleAdmin))

		// Platform stats
		r.Get("/stats", h.getStats)

		// User management
		r.Get("/users", h.listUsers)
		r.Get("/users/{id}", h.getUser)
		r.Patch("/users/{id}", h.updateUser)
		r.Delete("/users/{id}", h.deactivateUser)

		// Vendor management
		r.Get("/vendors", h.listVendors)
		r.Get("/vendors/{id}", h.getVendor)
		r.Patch("/vendors/{id}/status", h.updateVendorStatus)

		// Order management
		r.Get("/orders", h.listOrders)
		r.Get("/orders/{id}", h.getOrder)

		// Subscription management
		r.Get("/subscriptions", h.listSubscriptions)

		// Audit log
		r.Get("/audit-logs", h.listAuditLogs)
	})
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func respond(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

func pageParams(r *http.Request) (int, int) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	return page, pageSize
}

// ── Stats ─────────────────────────────────────────────────────────────────────

func (h *Handler) getStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.service.GetPlatformStats(r.Context())
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, stats)
}

// ── Users ─────────────────────────────────────────────────────────────────────

func (h *Handler) listUsers(w http.ResponseWriter, r *http.Request) {
	page, pageSize := pageParams(r)
	role := r.URL.Query().Get("role")
	search := r.URL.Query().Get("search")

	users, total, err := h.service.ListUsers(r.Context(), role, search, page, pageSize)
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, map[string]interface{}{
		"data":      users,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (h *Handler) getUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user, err := h.service.GetUser(r.Context(), id)
	if err != nil {
		code := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			code = http.StatusNotFound
		}
		respond(w, code, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, user)
}

func (h *Handler) updateUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	adminID := appMiddleware.GetUserID(r)
	user, err := h.service.UpdateUser(r.Context(), adminID, id, req)
	if err != nil {
		code := http.StatusInternalServerError
		if strings.Contains(err.Error(), "invalid") {
			code = http.StatusBadRequest
		}
		respond(w, code, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, user)
}

func (h *Handler) deactivateUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	adminID := appMiddleware.GetUserID(r)
	if err := h.service.DeactivateUser(r.Context(), adminID, id); err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, map[string]string{"message": "user deactivated"})
}

// ── Vendors ───────────────────────────────────────────────────────────────────

func (h *Handler) listVendors(w http.ResponseWriter, r *http.Request) {
	page, pageSize := pageParams(r)
	status := r.URL.Query().Get("status")
	search := r.URL.Query().Get("search")

	vendors, total, err := h.service.ListVendors(r.Context(), status, search, page, pageSize)
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, map[string]interface{}{
		"data":      vendors,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (h *Handler) getVendor(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	vendor, err := h.service.GetVendor(r.Context(), id)
	if err != nil {
		code := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			code = http.StatusNotFound
		}
		respond(w, code, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, vendor)
}

func (h *Handler) updateVendorStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req UpdateVendorStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	adminID := appMiddleware.GetUserID(r)
	vendor, err := h.service.UpdateVendorStatus(r.Context(), adminID, id, req.Status)
	if err != nil {
		code := http.StatusInternalServerError
		if strings.Contains(err.Error(), "invalid") {
			code = http.StatusBadRequest
		}
		respond(w, code, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, vendor)
}

// ── Orders ────────────────────────────────────────────────────────────────────

func (h *Handler) listOrders(w http.ResponseWriter, r *http.Request) {
	page, pageSize := pageParams(r)
	status := r.URL.Query().Get("status")

	orders, total, err := h.service.ListOrders(r.Context(), status, page, pageSize)
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, map[string]interface{}{
		"data":      orders,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (h *Handler) getOrder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	o, err := h.service.GetOrder(r.Context(), id)
	if err != nil {
		code := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			code = http.StatusNotFound
		}
		respond(w, code, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, o)
}

// ── Subscriptions ─────────────────────────────────────────────────────────────

func (h *Handler) listSubscriptions(w http.ResponseWriter, r *http.Request) {
	page, pageSize := pageParams(r)
	status := r.URL.Query().Get("status")

	subs, total, err := h.service.ListSubscriptions(r.Context(), status, page, pageSize)
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, map[string]interface{}{
		"data":      subs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// ── Audit Logs ────────────────────────────────────────────────────────────────

func (h *Handler) listAuditLogs(w http.ResponseWriter, r *http.Request) {
	page, pageSize := pageParams(r)
	adminID := r.URL.Query().Get("admin_id")
	targetType := r.URL.Query().Get("target_type")

	logs, total, err := h.service.ListAuditLogs(r.Context(), adminID, targetType, page, pageSize)
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, map[string]interface{}{
		"data":      logs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}
