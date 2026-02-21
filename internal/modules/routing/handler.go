package routing

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// Handler exposes routing HTTP endpoints.
type Handler struct{ service Service }

func NewHandler(service Service) *Handler { return &Handler{service: service} }

func (h *Handler) RegisterRoutes(r *chi.Mux) {
	r.Route("/api/v1/routing", func(r chi.Router) {
		// Order routing
		r.Post("/route", h.routeOrder)                              // POST   /api/v1/routing/route
		r.Get("/decisions/order/{order_id}", h.getDecision)         // GET    /api/v1/routing/decisions/order/{id}
		r.Post("/decisions/order/{order_id}/override", h.override)  // POST   /api/v1/routing/decisions/order/{id}/override
		r.Get("/decisions/store/{store_id}", h.listStoreDecisions)  // GET    /api/v1/routing/decisions/store/{id}

		// Rules management
		r.Post("/rules", h.createRule)          // POST   /api/v1/routing/rules
		r.Get("/rules", h.listRules)            // GET    /api/v1/routing/rules
		r.Put("/rules/{id}", h.updateRule)      // PUT    /api/v1/routing/rules/{id}
		r.Delete("/rules/{id}", h.deleteRule)   // DELETE /api/v1/routing/rules/{id}
	})
}

func (h *Handler) routeOrder(w http.ResponseWriter, r *http.Request) {
	var req RouteOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	decision, err := h.service.RouteOrder(r.Context(), req)
	if err != nil {
		code := http.StatusInternalServerError
		msg := err.Error()
		if strings.Contains(msg, "no eligible stores") {
			code = http.StatusUnprocessableEntity
		} else if strings.Contains(msg, "required") || strings.Contains(msg, "invalid") {
			code = http.StatusBadRequest
		}
		respond(w, code, map[string]string{"error": msg})
		return
	}
	respond(w, http.StatusCreated, decision)
}

func (h *Handler) getDecision(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "order_id")
	d, err := h.service.GetDecision(r.Context(), orderID)
	if err != nil {
		respond(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, d)
}

func (h *Handler) override(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "order_id")
	var req OverrideRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	d, err := h.service.OverrideRoute(r.Context(), orderID, req)
	if err != nil {
		code := http.StatusInternalServerError
		if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "invalid") {
			code = http.StatusBadRequest
		}
		respond(w, code, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusCreated, d)
}

func (h *Handler) listStoreDecisions(w http.ResponseWriter, r *http.Request) {
	storeID := chi.URLParam(r, "store_id")
	decisions, err := h.service.ListStoreDecisions(r.Context(), storeID)
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, decisions)
}

func (h *Handler) createRule(w http.ResponseWriter, r *http.Request) {
	var req CreateRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	rule, err := h.service.CreateRule(r.Context(), req)
	if err != nil {
		code := http.StatusInternalServerError
		if strings.Contains(err.Error(), "required") {
			code = http.StatusBadRequest
		}
		respond(w, code, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusCreated, rule)
}

func (h *Handler) listRules(w http.ResponseWriter, r *http.Request) {
	rules, err := h.service.ListRules(r.Context())
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, rules)
}

func (h *Handler) updateRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req CreateRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	rule, err := h.service.UpdateRule(r.Context(), id, req)
	if err != nil {
		code := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			code = http.StatusNotFound
		}
		respond(w, code, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, rule)
}

func (h *Handler) deleteRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.DeleteRule(r.Context(), id); err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func respond(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}
