package catalog

import (
"encoding/json"
"net/http"

"github.com/go-chi/chi/v5"
)

// Handler exposes catalog HTTP endpoints.
type Handler struct{ service Service }

func NewHandler(service Service) *Handler { return &Handler{service: service} }

func (h *Handler) RegisterRoutes(r *chi.Mux) {
r.Route("/api/v1/catalog", func(r chi.Router) {
r.Get("/products", h.listProducts)
r.Post("/products", h.createProduct)
r.Get("/products/{id}", h.getProduct)
r.Put("/products/{id}", h.updateProduct)
})
}

func (h *Handler) listProducts(w http.ResponseWriter, r *http.Request) {
category := r.URL.Query().Get("category")
activeOnly := r.URL.Query().Get("active") != "false"
products, err := h.service.ListProducts(r.Context(), category, activeOnly)
if err != nil {
http.Error(w, err.Error(), http.StatusInternalServerError)
return
}
respond(w, http.StatusOK, products)
}

func (h *Handler) createProduct(w http.ResponseWriter, r *http.Request) {
var req CreateProductRequest
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
http.Error(w, err.Error(), http.StatusBadRequest)
return
}
p, err := h.service.CreateProduct(r.Context(), req)
if err != nil {
http.Error(w, err.Error(), http.StatusInternalServerError)
return
}
respond(w, http.StatusCreated, p)
}

func (h *Handler) getProduct(w http.ResponseWriter, r *http.Request) {
id := chi.URLParam(r, "id")
p, err := h.service.GetProduct(r.Context(), id)
if err != nil {
http.Error(w, err.Error(), http.StatusNotFound)
return
}
respond(w, http.StatusOK, p)
}

func (h *Handler) updateProduct(w http.ResponseWriter, r *http.Request) {
id := chi.URLParam(r, "id")
var req CreateProductRequest
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
http.Error(w, err.Error(), http.StatusBadRequest)
return
}
p, err := h.service.UpdateProduct(r.Context(), id, req)
if err != nil {
http.Error(w, err.Error(), http.StatusInternalServerError)
return
}
respond(w, http.StatusOK, p)
}

func respond(w http.ResponseWriter, status int, body interface{}) {
w.Header().Set("Content-Type", "application/json")
w.WriteHeader(status)
json.NewEncoder(w).Encode(body)
}
