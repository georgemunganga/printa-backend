package production

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// Handler exposes production job HTTP endpoints.
type Handler struct{ service Service }

func NewHandler(service Service) *Handler { return &Handler{service: service} }

func (h *Handler) RegisterRoutes(r *chi.Mux) {
	r.Route("/api/v1/production", func(r chi.Router) {
		r.Post("/jobs", h.createJob)                                  // POST   /api/v1/production/jobs
		r.Get("/jobs/{id}", h.getJob)                                 // GET    /api/v1/production/jobs/{id}
		r.Get("/jobs/order/{order_id}", h.getJobByOrder)              // GET    /api/v1/production/jobs/order/{id}
		r.Get("/stores/{store_id}/jobs", h.listStoreJobs)             // GET    /api/v1/production/stores/{id}/jobs
		r.Get("/stores/{store_id}/queue-depth", h.queueDepth)         // GET    /api/v1/production/stores/{id}/queue-depth
		r.Get("/staff/{user_id}/jobs", h.listMyJobs)                  // GET    /api/v1/production/staff/{id}/jobs
		r.Patch("/jobs/{id}/status", h.updateStatus)                  // PATCH  /api/v1/production/jobs/{id}/status
		r.Patch("/jobs/{id}/assign", h.assignJob)                     // PATCH  /api/v1/production/jobs/{id}/assign
	})
}

func (h *Handler) createJob(w http.ResponseWriter, r *http.Request) {
	var req CreateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
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
	id := chi.URLParam(r, "id")
	job, err := h.service.GetJob(r.Context(), id)
	if err != nil {
		respond(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, job)
}

func (h *Handler) getJobByOrder(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "order_id")
	job, err := h.service.GetJobByOrder(r.Context(), orderID)
	if err != nil {
		respond(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, job)
}

func (h *Handler) listStoreJobs(w http.ResponseWriter, r *http.Request) {
	storeID := chi.URLParam(r, "store_id")
	status := r.URL.Query().Get("status")
	jobs, err := h.service.ListStoreJobs(r.Context(), storeID, status)
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, jobs)
}

func (h *Handler) listMyJobs(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "user_id")
	jobs, err := h.service.ListMyJobs(r.Context(), userID)
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, jobs)
}

func (h *Handler) updateStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
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
	count, err := h.service.QueueDepth(r.Context(), storeID)
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, map[string]int{"active_jobs": count})
}

func respond(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}
