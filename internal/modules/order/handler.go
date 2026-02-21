package order

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// Handler exposes order HTTP endpoints.
type Handler struct{ service Service }

func NewHandler(service Service) *Handler { return &Handler{service: service} }

func (h *Handler) RegisterRoutes(r *chi.Mux) {
	r.Route("/api/v1/orders", func(r chi.Router) {
		r.Post("/", h.placeOrder)                         // POST   /api/v1/orders
		r.Get("/{id}", h.getOrder)                        // GET    /api/v1/orders/{id}
		r.Get("/number/{number}", h.getOrderByNumber)     // GET    /api/v1/orders/number/{number}
		r.Patch("/{id}/status", h.updateStatus)           // PATCH  /api/v1/orders/{id}/status
		r.Delete("/{id}", h.cancelOrder)                  // DELETE /api/v1/orders/{id}
		r.Get("/store/{store_id}", h.listStoreOrders)     // GET    /api/v1/orders/store/{store_id}?status=PENDING
		r.Get("/customer/{customer_id}", h.listCustomerOrders) // GET /api/v1/orders/customer/{customer_id}
	})
}

func (h *Handler) placeOrder(w http.ResponseWriter, r *http.Request) {
	var req PlaceOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	o, err := h.service.PlaceOrder(r.Context(), req)
	if err != nil {
		code := http.StatusInternalServerError
		msg := err.Error()
		if strings.Contains(msg, "unavailable") || strings.Contains(msg, "not found in this store") {
			code = http.StatusUnprocessableEntity
		} else if strings.Contains(msg, "required") || strings.Contains(msg, "invalid") || strings.Contains(msg, "at least one") {
			code = http.StatusBadRequest
		}
		respond(w, code, map[string]string{"error": msg})
		return
	}
	respond(w, http.StatusCreated, o)
}

func (h *Handler) getOrder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	o, err := h.service.GetOrder(r.Context(), id)
	if err != nil {
		respond(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, o)
}

func (h *Handler) getOrderByNumber(w http.ResponseWriter, r *http.Request) {
	number := chi.URLParam(r, "number")
	o, err := h.service.GetOrderByNumber(r.Context(), number)
	if err != nil {
		respond(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, o)
}

func (h *Handler) updateStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req UpdateStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	o, err := h.service.UpdateStatus(r.Context(), id, req)
	if err != nil {
		code := http.StatusInternalServerError
		if strings.Contains(err.Error(), "cannot transition") {
			code = http.StatusUnprocessableEntity
		} else if strings.Contains(err.Error(), "not found") {
			code = http.StatusNotFound
		}
		respond(w, code, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, o)
}

func (h *Handler) cancelOrder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.CancelOrder(r.Context(), id); err != nil {
		code := http.StatusInternalServerError
		if strings.Contains(err.Error(), "only PENDING") {
			code = http.StatusUnprocessableEntity
		} else if strings.Contains(err.Error(), "not found") {
			code = http.StatusNotFound
		}
		respond(w, code, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, map[string]string{"status": "order cancelled"})
}

func (h *Handler) listStoreOrders(w http.ResponseWriter, r *http.Request) {
	storeID := chi.URLParam(r, "store_id")
	status := r.URL.Query().Get("status")
	orders, err := h.service.ListStoreOrders(r.Context(), storeID, status)
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, orders)
}

func (h *Handler) listCustomerOrders(w http.ResponseWriter, r *http.Request) {
	customerID := chi.URLParam(r, "customer_id")
	orders, err := h.service.ListCustomerOrders(r.Context(), customerID)
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, orders)
}

func respond(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}
