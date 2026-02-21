package inventory

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// Handler exposes inventory HTTP endpoints.
type Handler struct{ service Service }

func NewHandler(service Service) *Handler { return &Handler{service: service} }

func (h *Handler) RegisterRoutes(r *chi.Mux) {
	r.Route("/api/v1/inventory", func(r chi.Router) {
		// Store endpoints
		r.Post("/stores", h.createStore)
		r.Get("/stores/{id}", h.getStore)
		r.Get("/stores", h.listStores) // ?vendor_id=...

		// Staff endpoints
		r.Post("/stores/{store_id}/staff", h.addStaff)
		r.Get("/stores/{store_id}/staff", h.listStaff)
		r.Delete("/stores/{store_id}/staff/{user_id}", h.removeStaff)

		// Product listing endpoints
		r.Post("/stores/{store_id}/products", h.addProduct)
		r.Get("/stores/{store_id}/products", h.listProducts)
		r.Patch("/products/{id}/stock", h.updateStock)
		r.Patch("/products/{id}/availability", h.setAvailability)
	})
}

// isDuplicateKey returns true when the error is a PostgreSQL unique constraint violation (code 23505).
func isDuplicateKey(err error) bool {
	return strings.Contains(err.Error(), "23505") || strings.Contains(err.Error(), "duplicate key")
}

func (h *Handler) createStore(w http.ResponseWriter, r *http.Request) {
	var req CreateStoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	store, err := h.service.CreateStore(r.Context(), req)
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusCreated, store)
}

func (h *Handler) getStore(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	store, err := h.service.GetStore(r.Context(), id)
	if err != nil {
		respond(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, store)
}

func (h *Handler) listStores(w http.ResponseWriter, r *http.Request) {
	vendorID := r.URL.Query().Get("vendor_id")
	if vendorID == "" {
		respond(w, http.StatusBadRequest, map[string]string{"error": "vendor_id is required"})
		return
	}
	stores, err := h.service.ListStores(r.Context(), vendorID)
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, stores)
}

func (h *Handler) addStaff(w http.ResponseWriter, r *http.Request) {
	storeID := chi.URLParam(r, "store_id")
	var body struct {
		UserID string `json:"user_id"`
		Role   string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	staff, err := h.service.AddStaff(r.Context(), storeID, body.UserID, body.Role)
	if err != nil {
		if isDuplicateKey(err) {
			respond(w, http.StatusConflict, map[string]string{
				"error": "this user is already a staff member of this store",
			})
			return
		}
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusCreated, staff)
}

func (h *Handler) listStaff(w http.ResponseWriter, r *http.Request) {
	storeID := chi.URLParam(r, "store_id")
	staff, err := h.service.ListStaff(r.Context(), storeID)
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, staff)
}

func (h *Handler) removeStaff(w http.ResponseWriter, r *http.Request) {
	storeID := chi.URLParam(r, "store_id")
	userID := chi.URLParam(r, "user_id")
	if err := h.service.RemoveStaff(r.Context(), storeID, userID); err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) addProduct(w http.ResponseWriter, r *http.Request) {
	storeID := chi.URLParam(r, "store_id")
	var req AddProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	req.StoreID = storeID
	p, err := h.service.AddProduct(r.Context(), req)
	if err != nil {
		if isDuplicateKey(err) {
			respond(w, http.StatusConflict, map[string]string{
				"error": "this product is already listed in this store",
			})
			return
		}
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusCreated, p)
}

func (h *Handler) listProducts(w http.ResponseWriter, r *http.Request) {
	storeID := chi.URLParam(r, "store_id")
	products, err := h.service.ListProducts(r.Context(), storeID)
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, products)
}

func (h *Handler) updateStock(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		Quantity int `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := h.service.UpdateStock(r.Context(), id, body.Quantity); err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, map[string]string{"status": "stock updated"})
}

func (h *Handler) setAvailability(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		Available bool `json:"available"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := h.service.SetAvailability(r.Context(), id, body.Available); err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, map[string]string{"status": "availability updated"})
}

func respond(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}
