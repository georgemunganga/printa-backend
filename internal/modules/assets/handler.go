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

type Handler struct {
	storage assetstore.Storage
	db      *sql.DB
}

func NewHandler(db *sql.DB) (*Handler, error) {
	storage, err := assetstore.NewStorage(db)
	if err != nil {
		return nil, err
	}
	return &Handler{storage: storage, db: db}, nil
}

// Storage returns the configured asset storage for modules that add their own authorization boundary.
func (h *Handler) Storage() assetstore.Storage { return h.storage }

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/v1/assets", func(r chi.Router) {
		r.With(middleware.RequireRole(middleware.RoleCustomer, middleware.RoleVendor, middleware.RoleStaff, middleware.RoleCashier)).Post("/upload", h.upload)
		r.With(middleware.RequireRole(middleware.RoleCustomer, middleware.RoleVendor, middleware.RoleStaff, middleware.RoleCashier, middleware.RoleAdmin)).Get("/{asset_id}", h.get)
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
	assetID := chi.URLParam(r, "asset_id")
	ownerID, err := h.authorizedAssetOwner(r, assetID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respond(w, http.StatusNotFound, map[string]string{"error": "asset not found"})
		} else {
			respond(w, http.StatusInternalServerError, map[string]string{"error": "could not authorize asset"})
		}
		return
	}
	a, err := h.storage.Open(r.Context(), assetID, ownerID)
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

// authorizedAssetOwner allows the uploader, an administrator, or an active member of the
// store receiving an order that explicitly references this asset. Unrelated vendors and
// staff receive the same not-found response as a missing asset to avoid leaking its existence.
func (h *Handler) authorizedAssetOwner(r *http.Request, assetID string) (string, error) {
	var ownerID string
	if err := h.db.QueryRowContext(r.Context(), `SELECT owner_id FROM design_assets WHERE id=$1 AND deleted_at IS NULL`, assetID).Scan(&ownerID); err != nil {
		return "", err
	}
	userID := middleware.GetUserID(r)
	if ownerID == userID || middleware.GetRole(r) == middleware.RoleAdmin {
		return ownerID, nil
	}

	var allowed bool
	switch middleware.GetRole(r) {
	case middleware.RoleVendor:
		err := h.db.QueryRowContext(r.Context(), `
			SELECT EXISTS(
				SELECT 1
				FROM order_items oi
				JOIN orders o ON o.id = oi.order_id
				JOIN stores s ON s.id = o.store_id
				JOIN vendors v ON v.id = s.vendor_id
				WHERE (
					oi.customisation->>'asset_id' = $1
					OR EXISTS (
						SELECT 1 FROM jsonb_array_elements(COALESCE(oi.customisation->'uploaded_assets', '[]'::jsonb)) linked_asset
						WHERE linked_asset->>'asset_id' = $1
					)
				) AND v.owner_id = $2
			)`, assetID, userID).Scan(&allowed)
		if err != nil {
			return "", err
		}
	case middleware.RoleStaff, middleware.RoleCashier:
		err := h.db.QueryRowContext(r.Context(), `
			SELECT EXISTS(
				SELECT 1
				FROM order_items oi
				JOIN orders o ON o.id = oi.order_id
				JOIN store_staff ss ON ss.store_id = o.store_id
				WHERE (
					oi.customisation->>'asset_id' = $1
					OR EXISTS (
						SELECT 1 FROM jsonb_array_elements(COALESCE(oi.customisation->'uploaded_assets', '[]'::jsonb)) linked_asset
						WHERE linked_asset->>'asset_id' = $1
					)
				)
				  AND ss.user_id = $2
				  AND COALESCE(ss.is_active, TRUE)
			)`, assetID, userID).Scan(&allowed)
		if err != nil {
			return "", err
		}
	}
	if !allowed {
		return "", sql.ErrNoRows
	}
	return ownerID, nil
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
