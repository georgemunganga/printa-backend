package delivery

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/georgemunganga/printa-backend/internal/middleware"
	"github.com/georgemunganga/printa-backend/internal/modules/inventory"
	"github.com/georgemunganga/printa-backend/internal/modules/vendor"
	"github.com/go-chi/chi/v5"
)

// ZoneHandler exposes delivery-zone management to store owners and a public coverage lookup to storefront clients.
type ZoneHandler struct {
	service          ZoneService
	inventoryService inventory.Service
	vendorService    vendor.Service
}

func NewZoneHandler(service ZoneService, inventoryService inventory.Service, vendorService vendor.Service) *ZoneHandler {
	return &ZoneHandler{service: service, inventoryService: inventoryService, vendorService: vendorService}
}

func (h *ZoneHandler) RegisterRoutes(r chi.Router) {
	r.Get("/api/v1/delivery/stores/{store_id}/zones", h.listZones)
	r.Post("/api/v1/delivery/stores/{store_id}/zones", h.createZone)
	r.Patch("/api/v1/delivery/stores/{store_id}/zones/{id}", h.updateZone)
	r.Delete("/api/v1/delivery/stores/{store_id}/zones/{id}", h.deleteZone)
}

func (h *ZoneHandler) RegisterStorefrontRoutes(r chi.Router) {
	r.Post("/api/v1/storefront/stores/{store_id}/delivery-eligibility", h.checkEligibility)
}

func (h *ZoneHandler) listZones(w http.ResponseWriter, r *http.Request) {
	storeID, ok := h.requireStoreOwner(w, r)
	if !ok {
		return
	}
	zones, err := h.service.ListZones(r.Context(), storeID)
	if err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, zones)
}

func (h *ZoneHandler) createZone(w http.ResponseWriter, r *http.Request) {
	storeID, ok := h.requireStoreOwner(w, r)
	if !ok {
		return
	}
	var req UpsertZoneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	zone, err := h.service.CreateZone(r.Context(), storeID, req)
	if err != nil {
		if isDuplicateZone(err) {
			respond(w, http.StatusConflict, map[string]string{"error": "this store already has a zone for that city and country"})
			return
		}
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusCreated, zone)
}

func (h *ZoneHandler) updateZone(w http.ResponseWriter, r *http.Request) {
	storeID, ok := h.requireStoreOwner(w, r)
	if !ok {
		return
	}
	var req UpsertZoneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	zone, err := h.service.UpdateZone(r.Context(), chi.URLParam(r, "id"), storeID, req)
	if err != nil {
		respondZoneError(w, err)
		return
	}
	respond(w, http.StatusOK, zone)
}

func (h *ZoneHandler) deleteZone(w http.ResponseWriter, r *http.Request) {
	storeID, ok := h.requireStoreOwner(w, r)
	if !ok {
		return
	}
	if err := h.service.DeleteZone(r.Context(), chi.URLParam(r, "id"), storeID); err != nil {
		respondZoneError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ZoneHandler) checkEligibility(w http.ResponseWriter, r *http.Request) {
	storeID := chi.URLParam(r, "store_id")
	store, err := h.inventoryService.GetStore(r.Context(), storeID)
	if err != nil || !store.IsActive {
		respond(w, http.StatusNotFound, map[string]string{"error": "store not found"})
		return
	}
	var req EligibilityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	result, err := h.service.CheckEligibility(r.Context(), storeID, req)
	if err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, result)
}

func (h *ZoneHandler) requireStoreOwner(w http.ResponseWriter, r *http.Request) (string, bool) {
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

func isDuplicateZone(err error) bool {
	return strings.Contains(err.Error(), "23505") || strings.Contains(err.Error(), "duplicate key")
}

func respondZoneError(w http.ResponseWriter, err error) {
	if err == sql.ErrNoRows {
		respond(w, http.StatusNotFound, map[string]string{"error": "delivery zone not found"})
		return
	}
	if isDuplicateZone(err) {
		respond(w, http.StatusConflict, map[string]string{"error": "this store already has a zone for that city and country"})
		return
	}
	respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
}
