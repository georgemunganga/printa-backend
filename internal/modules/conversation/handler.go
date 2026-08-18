package conversation

import (
	"encoding/json"
	"net/http"

	"github.com/georgemunganga/printa-backend/internal/middleware"
	"github.com/georgemunganga/printa-backend/internal/modules/inventory"
	"github.com/georgemunganga/printa-backend/internal/modules/order"
	"github.com/georgemunganga/printa-backend/internal/modules/vendor"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service          Service
	orderService     order.Service
	inventoryService inventory.Service
	vendorService    vendor.Service
}

func NewHandler(service Service, orderService order.Service, inventoryService inventory.Service, vendorService vendor.Service) *Handler {
	return &Handler{
		service:          service,
		orderService:     orderService,
		inventoryService: inventoryService,
		vendorService:    vendorService,
	}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/v1/conversations", func(r chi.Router) {
		r.Get("/orders/{order_id}/messages", h.listMessages)
		r.Post("/orders/{order_id}/messages", h.sendMessage)
	})
}

func (h *Handler) listMessages(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "order_id")
	if !h.requireOrderAccess(w, r, orderID) {
		return
	}
	messages, err := h.service.List(r.Context(), orderID, middleware.GetUserID(r))
	if err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, messages)
}

func (h *Handler) sendMessage(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "order_id")
	if !h.requireOrderAccess(w, r, orderID) {
		return
	}
	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	message, err := h.service.Send(r.Context(), orderID, middleware.GetUserID(r), req.Body)
	if err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusCreated, message)
}

func (h *Handler) requireOrderAccess(w http.ResponseWriter, r *http.Request, orderID string) bool {
	purchase, err := h.orderService.GetOrder(r.Context(), orderID)
	if err != nil {
		respond(w, http.StatusNotFound, map[string]string{"error": "order not found"})
		return false
	}

	switch middleware.GetRole(r) {
	case middleware.RoleAdmin:
		return true
	case middleware.RoleCustomer:
		if purchase.CustomerID != nil && purchase.CustomerID.String() == middleware.GetUserID(r) {
			return true
		}
	case middleware.RoleVendor:
		vendorProfile, err := h.vendorService.GetVendor(r.Context(), middleware.GetUserID(r))
		if err == nil {
			store, storeErr := h.inventoryService.GetStore(r.Context(), purchase.StoreID.String())
			if storeErr == nil && store.VendorID == vendorProfile.ID {
				return true
			}
		}
	case middleware.RoleStaff, middleware.RoleCashier:
		staff, err := h.inventoryService.ListStaff(r.Context(), purchase.StoreID.String())
		if err == nil {
			for _, member := range staff {
				if member.UserID.String() == middleware.GetUserID(r) && member.IsActive {
					return true
				}
			}
		}
	}

	respond(w, http.StatusForbidden, map[string]string{"error": "not authorized to access this order conversation"})
	return false
}

func respond(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if body != nil {
		_ = json.NewEncoder(w).Encode(body)
	}
}
