package assets

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	assetstore "github.com/georgemunganga/printa-backend/internal/assets"
	"github.com/georgemunganga/printa-backend/internal/middleware"
	"github.com/go-chi/chi/v5"
)

type Handler struct{ storage assetstore.Storage }

func NewHandler(db *sql.DB) (*Handler, error) {
	storage, err := assetstore.NewStorage(db)
	if err != nil {
		return nil, err
	}
	return &Handler{storage: storage}, nil
}
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/v1/assets", func(r chi.Router) {
		r.With(middleware.RequireRole(middleware.RoleCustomer)).Post("/upload", h.upload)
		r.With(middleware.RequireRole(middleware.RoleCustomer)).Get("/{asset_id}", h.get)
	})
}

func (h *Handler) upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, assetstore.MaxSize+1024*1024)
	if err := r.ParseMultipartForm(assetstore.MaxSize); err != nil {
		respond(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "file exceeds 20 MB limit"})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": "multipart field file is required"})
		return
	}
	defer file.Close()
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if !allowedContentType(contentType) {
		respond(w, http.StatusUnsupportedMediaType, map[string]string{"error": "only PDF, PNG, JPEG, SVG, TIFF, and WebP design files are accepted"})
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, assetstore.MaxSize+1))
	if err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": "could not read file"})
		return
	}
	a, err := h.storage.Upload(r.Context(), middleware.GetUserID(r), header.Filename, contentType, data)
	if err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusCreated, map[string]interface{}{"asset_id": a.ID, "name": a.Name, "content_type": a.ContentType, "size_bytes": a.Size, "storage_provider": a.Provider, "url": "/api/v1/assets/" + a.ID})
}
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	a, err := h.storage.Open(r.Context(), chi.URLParam(r, "asset_id"), middleware.GetUserID(r))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respond(w, http.StatusNotFound, map[string]string{"error": "asset not found"})
		} else {
			respond(w, http.StatusInternalServerError, map[string]string{"error": "could not load asset"})
		}
		return
	}
	w.Header().Set("Content-Type", a.ContentType)
	w.Header().Set("Content-Disposition", "inline; filename=\""+strings.ReplaceAll(a.Name, "\"", "")+"\"")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(a.Content)
}
func allowedContentType(v string) bool {
	switch strings.ToLower(strings.TrimSpace(strings.Split(v, ";")[0])) {
	case "application/pdf", "image/png", "image/jpeg", "image/svg+xml", "image/tiff", "image/webp":
		return true
	default:
		return false
	}
}
func respond(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
