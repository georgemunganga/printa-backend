package pos

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/georgemunganga/printa-backend/internal/middleware"
	"github.com/georgemunganga/printa-backend/internal/modules/inventory"
	"github.com/georgemunganga/printa-backend/internal/modules/vendor"
	"github.com/go-chi/chi/v5"
)

// Handler exposes POS HTTP endpoints.
type Handler struct {
	service          Service
	inventoryService inventory.Service
	vendorService    vendor.Service
}

func NewHandler(service Service, inventoryService inventory.Service, vendorService vendor.Service) *Handler {
	return &Handler{
		service:          service,
		inventoryService: inventoryService,
		vendorService:    vendorService,
	}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/v1/pos", func(r chi.Router) {
		r.Post("/transactions", h.recordPayment)
		r.Get("/transactions/{id}", h.getTransaction)
		r.Get("/transactions/order/{order_id}", h.getByOrder)
		r.Get("/stores/{store_id}/transactions", h.listStoreTransactions)
		r.Post("/transactions/{id}/refund", h.refund)
	})
}

func (h *Handler) recordPayment(w http.ResponseWriter, r *http.Request) {
	var req CreateTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if _, ok := h.requireStoreAccess(w, r, req.StoreID, true); !ok {
		return
	}
	if middleware.GetRole(r) == middleware.RoleStaff || middleware.GetRole(r) == middleware.RoleCashier {
		if req.CashierID != "" && req.CashierID != middleware.GetUserID(r) {
			respond(w, http.StatusForbidden, map[string]string{"error": "cashier_id must match authenticated user"})
			return
		}
		req.CashierID = middleware.GetUserID(r)
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
	tx, ok := h.requireTransactionAccess(w, r, chi.URLParam(r, "id"), true)
	if !ok {
		return
	}
	respond(w, http.StatusOK, tx)
}

func (h *Handler) getByOrder(w http.ResponseWriter, r *http.Request) {
	tx, err := h.service.GetTransactionByOrder(r.Context(), chi.URLParam(r, "order_id"))
	if err != nil {
		respond(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	if _, ok := h.requireStoreAccess(w, r, tx.StoreID.String(), true); !ok {
		return
	}
	respond(w, http.StatusOK, tx)
}

func (h *Handler) listStoreTransactions(w http.ResponseWriter, r *http.Request) {
	storeID := chi.URLParam(r, "store_id")
	if _, ok := h.requireStoreAccess(w, r, storeID, true); !ok {
		return
	}
	txs, err := h.service.ListStoreTransactions(r.Context(), storeID)
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if txs == nil {
		txs = make([]*POSTransaction, 0)
	}
	respond(w, http.StatusOK, txs)
}

func (h *Handler) refund(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, ok := h.requireTransactionAccess(w, r, id, false); !ok {
		return
	}
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

func (h *Handler) requireTransactionAccess(w http.ResponseWriter, r *http.Request, transactionID string, allowStaff bool) (*POSTransaction, bool) {
	tx, err := h.service.GetTransaction(r.Context(), transactionID)
	if err != nil {
		respond(w, http.StatusNotFound, map[string]string{"error": "POS transaction not found"})
		return nil, false
	}
	if _, ok := h.requireStoreAccess(w, r, tx.StoreID.String(), allowStaff); !ok {
		return nil, false
	}
	return tx, true
}

func (h *Handler) requireStoreAccess(w http.ResponseWriter, r *http.Request, storeID string, allowStaff bool) (*inventory.Store, bool) {
	store, err := h.inventoryService.GetStore(r.Context(), storeID)
	if err != nil {
		respond(w, http.StatusNotFound, map[string]string{"error": "store not found"})
		return nil, false
	}

	switch middleware.GetRole(r) {
	case middleware.RoleAdmin:
		return store, true
	case middleware.RoleVendor:
		currentVendor, err := h.vendorService.GetVendor(r.Context(), middleware.GetUserID(r))
		if err != nil {
			respond(w, http.StatusForbidden, map[string]string{"error": "authenticated vendor profile is required"})
			return nil, false
		}
		if store.VendorID == currentVendor.ID {
			return store, true
		}
	case middleware.RoleStaff, middleware.RoleCashier:
		if allowStaff {
			staff, err := h.inventoryService.ListStaff(r.Context(), storeID)
			if err == nil {
				for _, member := range staff {
					if member.UserID.String() == middleware.GetUserID(r) {
						return store, true
					}
				}
			}
		}
	}

	respond(w, http.StatusForbidden, map[string]string{"error": "insufficient permissions"})
	return nil, false
}

func respond(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
