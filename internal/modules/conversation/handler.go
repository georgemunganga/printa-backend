package conversation

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	assetstore "github.com/georgemunganga/printa-backend/internal/assets"
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
	storage          assetstore.Storage
}

func NewHandler(service Service, orderService order.Service, inventoryService inventory.Service, vendorService vendor.Service, storage assetstore.Storage) *Handler {
	return &Handler{
		service:          service,
		orderService:     orderService,
		inventoryService: inventoryService,
		vendorService:    vendorService,
		storage:          storage,
	}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/v1/conversations", func(r chi.Router) {
		r.Get("/orders/{order_id}/messages", h.listMessages)
		r.Post("/orders/{order_id}/messages", h.sendMessage)
		r.Get("/orders/{order_id}/messages/{message_id}/attachments/{asset_id}", h.getAttachment)
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
	for _, message := range messages {
		h.withAttachmentURLs(orderID, message)
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
	message, err := h.service.Send(r.Context(), orderID, middleware.GetUserID(r), req.Body, req.AssetIDs)
	if err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	for index, attachment := range message.Attachments {
		details, err := h.service.GetAttachment(r.Context(), orderID, message.ID.String(), attachment.AssetID.String())
		if err != nil {
			respond(w, http.StatusInternalServerError, map[string]string{"error": "could not load persisted attachment metadata"})
			return
		}
		message.Attachments[index] = details
	}
	h.withAttachmentURLs(orderID, message)
	respond(w, http.StatusCreated, message)
}

func (h *Handler) withAttachmentURLs(orderID string, message *Message) {
	for _, attachment := range message.Attachments {
		attachment.URL = "/api/v1/conversations/orders/" + orderID + "/messages/" + message.ID.String() + "/attachments/" + attachment.AssetID.String()
	}
}

func (h *Handler) getAttachment(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "order_id")
	if !h.requireOrderAccess(w, r, orderID) {
		return
	}
	attachment, err := h.service.GetAttachment(r.Context(), orderID, chi.URLParam(r, "message_id"), chi.URLParam(r, "asset_id"))
	if err != nil {
		if errors.Is(err, io.EOF) {
			respond(w, http.StatusNotFound, map[string]string{"error": "attachment not found"})
			return
		}
		respond(w, http.StatusNotFound, map[string]string{"error": "attachment not found"})
		return
	}
	asset, err := h.storage.Open(r.Context(), attachment.AssetID.String(), attachment.OwnerID.String())
	if err != nil {
		respond(w, http.StatusNotFound, map[string]string{"error": "attachment not found"})
		return
	}
	w.Header().Set("Content-Type", asset.ContentType)
	w.Header().Set("Content-Disposition", "inline; filename=\""+strings.ReplaceAll(asset.Name, "\"", "")+"\"")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(asset.Content)
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
