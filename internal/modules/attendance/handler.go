package attendance

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/georgemunganga/printa-backend/internal/middleware"
	"github.com/georgemunganga/printa-backend/internal/modules/inventory"
	"github.com/georgemunganga/printa-backend/internal/modules/vendor"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service       Service
	inventory     inventory.Service
	vendorService vendor.Service
}

func NewHandler(service Service, inventoryService inventory.Service, vendorService vendor.Service) *Handler {
	return &Handler{service: service, inventory: inventoryService, vendorService: vendorService}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/v1/attendance", func(r chi.Router) {
		r.Put("/stores/{store_id}/staff/{user_id}/pin", h.setPIN)
		r.Post("/stores/{store_id}/clock", h.clock)
		r.Get("/stores/{store_id}/events", h.listRecent)
	})
}

func (h *Handler) setPIN(w http.ResponseWriter, r *http.Request) {
	storeID := chi.URLParam(r, "store_id")
	if !h.canManageAttendance(w, r, storeID) {
		return
	}
	var req SetPINRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if err := h.service.SetPIN(r.Context(), storeID, chi.URLParam(r, "user_id"), req.PIN); err != nil {
		handleServiceError(w, err)
		return
	}
	respond(w, http.StatusNoContent, nil)
}

func (h *Handler) clock(w http.ResponseWriter, r *http.Request) {
	storeID := chi.URLParam(r, "store_id")
	var req ClockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if !h.canClock(w, r, storeID, req.UserID) {
		return
	}
	result, err := h.service.Clock(r.Context(), storeID, req.UserID, req.PIN, middleware.GetUserID(r))
	if err != nil {
		handleServiceError(w, err)
		return
	}
	respond(w, http.StatusCreated, result)
}

func (h *Handler) listRecent(w http.ResponseWriter, r *http.Request) {
	storeID := chi.URLParam(r, "store_id")
	if !h.canReadAttendance(w, r, storeID) {
		return
	}
	events, err := h.service.ListRecent(r.Context(), storeID, 50)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	if events == nil {
		events = make([]*AttendanceEvent, 0)
	}
	respond(w, http.StatusOK, events)
}

func (h *Handler) canClock(w http.ResponseWriter, r *http.Request, storeID, subjectUserID string) bool {
	if h.canManageAttendance(w, r, storeID) {
		return true
	}
	if middleware.GetUserID(r) != subjectUserID {
		respond(w, http.StatusForbidden, map[string]string{"error": "staff can clock only their own attendance"})
		return false
	}
	return h.isAssignedStaff(w, r, storeID)
}

func (h *Handler) canReadAttendance(w http.ResponseWriter, r *http.Request, storeID string) bool {
	if h.canManageAttendance(w, r, storeID) {
		return true
	}
	return h.isAssignedStaff(w, r, storeID)
}

func (h *Handler) canManageAttendance(w http.ResponseWriter, r *http.Request, storeID string) bool {
	store, err := h.inventory.GetStore(r.Context(), storeID)
	if err != nil {
		respond(w, http.StatusNotFound, map[string]string{"error": "store not found"})
		return false
	}
	switch middleware.GetRole(r) {
	case middleware.RoleAdmin:
		return true
	case middleware.RoleVendor:
		vendorProfile, err := h.vendorService.GetVendor(r.Context(), middleware.GetUserID(r))
		if err == nil && vendorProfile.ID == store.VendorID {
			return true
		}
	case middleware.RoleStaff, middleware.RoleCashier:
		staff, err := h.inventory.ListStaff(r.Context(), storeID)
		if err == nil {
			for _, member := range staff {
				if member.UserID.String() == middleware.GetUserID(r) && strings.EqualFold(member.Role, "MANAGER") {
					return true
				}
			}
		}
	}
	return false
}

func (h *Handler) isAssignedStaff(w http.ResponseWriter, r *http.Request, storeID string) bool {
	staff, err := h.inventory.ListStaff(r.Context(), storeID)
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": "unable to verify store staff"})
		return false
	}
	for _, member := range staff {
		if member.UserID.String() == middleware.GetUserID(r) && member.IsActive {
			return true
		}
	}
	respond(w, http.StatusForbidden, map[string]string{"error": "staff assignment is required"})
	return false
}

func handleServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotAssigned):
		respond(w, http.StatusNotFound, map[string]string{"error": "staff member is not assigned to this store"})
	case errors.Is(err, ErrPINNotConfigured):
		respond(w, http.StatusConflict, map[string]string{"error": "staff PIN has not been configured"})
	case strings.Contains(err.Error(), "invalid staff PIN"):
		respond(w, http.StatusUnauthorized, map[string]string{"error": "invalid staff PIN"})
	default:
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
}

func respond(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if body != nil {
		_ = json.NewEncoder(w).Encode(body)
	}
}
