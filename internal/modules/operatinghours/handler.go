package operatinghours

import (
	"encoding/json"
	"net/http"

	"github.com/georgemunganga/printa-backend/internal/middleware"
	"github.com/georgemunganga/printa-backend/internal/modules/inventory"
	"github.com/georgemunganga/printa-backend/internal/modules/vendor"
	"github.com/go-chi/chi/v5"
)

// Handler exposes store operating hours to the owning vendor and administrators.
type Handler struct {
	service          Service
	inventoryService inventory.Service
	vendorService    vendor.Service
}

func NewHandler(service Service, inventoryService inventory.Service, vendorService vendor.Service) *Handler {
	return &Handler{service: service, inventoryService: inventoryService, vendorService: vendorService}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api/v1/inventory/stores/{store_id}/operating-hours", h.list)
	r.Put("/api/v1/inventory/stores/{store_id}/operating-hours", h.replace)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	storeID, ok := h.requireStoreOwner(w, r)
	if !ok {
		return
	}
	hours, err := h.service.List(r.Context(), storeID)
	if err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, hours)
}

func (h *Handler) replace(w http.ResponseWriter, r *http.Request) {
	storeID, ok := h.requireStoreOwner(w, r)
	if !ok {
		return
	}

	var req ReplaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	hours, err := h.service.Replace(r.Context(), storeID, req)
	if err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, hours)
}

func (h *Handler) requireStoreOwner(w http.ResponseWriter, r *http.Request) (string, bool) {
	storeID := chi.URLParam(r, "store_id")
	store, err := h.inventoryService.GetStore(r.Context(), storeID)
	if err != nil {
		respond(w, http.StatusNotFound, map[string]string{"error": "store not found"})
		return "", false
	}

	switch middleware.GetRole(r) {
	case middleware.RoleAdmin:
		return storeID, true
	case middleware.RoleVendor:
		v, err := h.vendorService.GetVendor(r.Context(), middleware.GetUserID(r))
		if err == nil && v.ID == store.VendorID {
			return storeID, true
		}
	}

	respond(w, http.StatusForbidden, map[string]string{"error": "store-owner or administrator permission is required"})
	return "", false
}

func respond(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
