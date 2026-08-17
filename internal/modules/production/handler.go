package production

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/georgemunganga/printa-backend/internal/middleware"
	"github.com/georgemunganga/printa-backend/internal/modules/inventory"
	"github.com/georgemunganga/printa-backend/internal/modules/vendor"
	"github.com/go-chi/chi/v5"
)

// Handler exposes production job HTTP endpoints.
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
	r.Route("/api/v1/production", func(r chi.Router) {
		r.Post("/jobs", h.createJob)
		r.Get("/jobs/{id}", h.getJob)
		r.Get("/jobs/order/{order_id}", h.getJobByOrder)
		r.Get("/stores/{store_id}/jobs", h.listStoreJobs)
		r.Get("/stores/{store_id}/queue-depth", h.queueDepth)
		r.Get("/staff/{user_id}/jobs", h.listMyJobs)
		r.Patch("/jobs/{id}/status", h.updateStatus)
		r.Patch("/jobs/{id}/assign", h.assignJob)
	})
}

func (h *Handler) createJob(w http.ResponseWriter, r *http.Request) {
	var req CreateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if _, ok := h.requireStoreAccess(w, r, req.StoreID, false); !ok {
		return
	}
	job, err := h.service.CreateJob(r.Context(), req)
	if err != nil {
		code := http.StatusInternalServerError
		if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "invalid") {
			code = http.StatusBadRequest
		}
		respond(w, code, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusCreated, job)
}

func (h *Handler) getJob(w http.ResponseWriter, r *http.Request) {
	job, ok := h.requireJobAccess(w, r, chi.URLParam(r, "id"), true)
	if !ok {
		return
	}
	respond(w, http.StatusOK, job)
}

func (h *Handler) getJobByOrder(w http.ResponseWriter, r *http.Request) {
	job, err := h.service.GetJobByOrder(r.Context(), chi.URLParam(r, "order_id"))
	if err != nil {
		respond(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	if _, ok := h.requireStoreAccess(w, r, job.StoreID.String(), true); !ok {
		return
	}
	respond(w, http.StatusOK, job)
}

func (h *Handler) listStoreJobs(w http.ResponseWriter, r *http.Request) {
	storeID := chi.URLParam(r, "store_id")
	if _, ok := h.requireStoreAccess(w, r, storeID, true); !ok {
		return
	}
	jobs, err := h.service.ListStoreJobs(r.Context(), storeID, r.URL.Query().Get("status"))
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if jobs == nil {
		jobs = make([]*ProductionJob, 0)
	}
	respond(w, http.StatusOK, jobs)
}

func (h *Handler) listMyJobs(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "user_id")
	if middleware.GetRole(r) != middleware.RoleAdmin && userID != middleware.GetUserID(r) {
		respond(w, http.StatusForbidden, map[string]string{"error": "insufficient permissions"})
		return
	}
	jobs, err := h.service.ListMyJobs(r.Context(), userID)
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if jobs == nil {
		jobs = make([]*ProductionJob, 0)
	}
	respond(w, http.StatusOK, jobs)
}

func (h *Handler) updateStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, ok := h.requireJobAccess(w, r, id, true); !ok {
		return
	}
	var req UpdateStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	job, err := h.service.UpdateStatus(r.Context(), id, req)
	if err != nil {
		code := http.StatusInternalServerError
		msg := err.Error()
		if strings.Contains(msg, "cannot transition") {
			code = http.StatusUnprocessableEntity
		} else if strings.Contains(msg, "not found") {
			code = http.StatusNotFound
		} else if strings.Contains(msg, "required") {
			code = http.StatusBadRequest
		}
		respond(w, code, map[string]string{"error": msg})
		return
	}
	respond(w, http.StatusOK, job)
}

func (h *Handler) assignJob(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, ok := h.requireJobAccess(w, r, id, false); !ok {
		return
	}
	var req AssignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	job, err := h.service.AssignJob(r.Context(), id, req)
	if err != nil {
		code := http.StatusInternalServerError
		if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "invalid") {
			code = http.StatusBadRequest
		} else if strings.Contains(err.Error(), "not found") {
			code = http.StatusNotFound
		}
		respond(w, code, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, job)
}

func (h *Handler) queueDepth(w http.ResponseWriter, r *http.Request) {
	storeID := chi.URLParam(r, "store_id")
	if _, ok := h.requireStoreAccess(w, r, storeID, true); !ok {
		return
	}
	count, err := h.service.QueueDepth(r.Context(), storeID)
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, map[string]int{"active_jobs": count})
}

func (h *Handler) requireJobAccess(w http.ResponseWriter, r *http.Request, jobID string, allowStaff bool) (*ProductionJob, bool) {
	job, err := h.service.GetJob(r.Context(), jobID)
	if err != nil {
		respond(w, http.StatusNotFound, map[string]string{"error": "production job not found"})
		return nil, false
	}
	if _, ok := h.requireStoreAccess(w, r, job.StoreID.String(), allowStaff); !ok {
		return nil, false
	}
	return job, true
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
