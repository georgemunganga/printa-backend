package pos

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// Handler exposes POS HTTP endpoints.
type Handler struct{ service Service }

func NewHandler(service Service) *Handler { return &Handler{service: service} }

func (h *Handler) RegisterRoutes(r *chi.Mux) {
	r.Route("/api/v1/pos", func(r chi.Router) {
		r.Post("/transactions", h.recordPayment)                              // POST   /api/v1/pos/transactions
		r.Get("/transactions/{id}", h.getTransaction)                         // GET    /api/v1/pos/transactions/{id}
		r.Get("/transactions/order/{order_id}", h.getByOrder)                 // GET    /api/v1/pos/transactions/order/{id}
		r.Get("/stores/{store_id}/transactions", h.listStoreTransactions)     // GET    /api/v1/pos/stores/{id}/transactions
		r.Post("/transactions/{id}/refund", h.refund)                         // POST   /api/v1/pos/transactions/{id}/refund
	})
}

func (h *Handler) recordPayment(w http.ResponseWriter, r *http.Request) {
	var req CreateTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	tx, err := h.service.RecordPayment(r.Context(), req)
	if err != nil {
		code := http.StatusInternalServerError
		msg := err.Error()
		if strings.Contains(msg, "required") || strings.Contains(msg, "invalid") || strings.Contains(msg, "must be") {
			code = http.StatusBadRequest
		}
		respond(w, code, map[string]string{"error": msg})
		return
	}
	respond(w, http.StatusCreated, tx)
}

func (h *Handler) getTransaction(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	tx, err := h.service.GetTransaction(r.Context(), id)
	if err != nil {
		respond(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, tx)
}

func (h *Handler) getByOrder(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "order_id")
	tx, err := h.service.GetTransactionByOrder(r.Context(), orderID)
	if err != nil {
		respond(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, tx)
}

func (h *Handler) listStoreTransactions(w http.ResponseWriter, r *http.Request) {
	storeID := chi.URLParam(r, "store_id")
	txs, err := h.service.ListStoreTransactions(r.Context(), storeID)
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, txs)
}

func (h *Handler) refund(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req RefundRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	tx, err := h.service.RefundTransaction(r.Context(), id, req)
	if err != nil {
		code := http.StatusInternalServerError
		msg := err.Error()
		if strings.Contains(msg, "not found") {
			code = http.StatusNotFound
		} else if strings.Contains(msg, "only COMPLETED") {
			code = http.StatusUnprocessableEntity
		}
		respond(w, code, map[string]string{"error": msg})
		return
	}
	respond(w, http.StatusOK, tx)
}

func respond(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}
