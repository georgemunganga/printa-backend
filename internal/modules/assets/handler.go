package assets

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

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

const guestAssetLifetime = 24 * time.Hour

func (h *Handler) RegisterPublicRoutes(r chi.Router) {
	r.Post("/api/v1/assets/guest-upload", h.guestUpload)
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/v1/assets", func(r chi.Router) {
		r.With(middleware.RequireRole(middleware.RoleCustomer, middleware.RoleVendor, middleware.RoleStaff, middleware.RoleCashier)).Post("/upload", h.upload)
		r.With(middleware.RequireRole(middleware.RoleCustomer)).Post("/claim", h.claim)
		r.With(middleware.RequireRole(middleware.RoleCustomer, middleware.RoleVendor, middleware.RoleStaff, middleware.RoleCashier, middleware.RoleAdmin)).Get("/{asset_id}", h.get)
	})
}

func (h *Handler) guestUpload(w http.ResponseWriter, r *http.Request) {
	// Bound temporary database storage without placing cleanup on the request's
	// critical path. Claimed assets have no expiry and are never selected here.
	_, _ = h.storage.CleanupExpiredGuests(r.Context(), 100)
	file, header, contentType, data, ok := readUpload(w, r)
	if !ok {
		return
	}
	defer file.Close()
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": "could not secure guest upload"})
		return
	}
	claimToken := base64.RawURLEncoding.EncodeToString(tokenBytes)
	tokenHash := sha256.Sum256([]byte(claimToken))
	expiresAt := time.Now().UTC().Add(guestAssetLifetime)
	a, err := h.storage.UploadGuest(r.Context(), header.Filename, contentType, data, hex.EncodeToString(tokenHash[:]), expiresAt)
	if err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusCreated, map[string]interface{}{"asset_id": a.ID, "name": a.Name, "content_type": a.ContentType, "size_bytes": a.Size, "storage_provider": a.Provider, "claim_token": claimToken, "expires_at": expiresAt})
}

func (h *Handler) upload(w http.ResponseWriter, r *http.Request) {
	file, header, contentType, data, ok := readUpload(w, r)
	if !ok {
		return
	}
	defer file.Close()
	a, err := h.storage.Upload(r.Context(), middleware.GetUserID(r), header.Filename, contentType, data)
	if err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusCreated, map[string]interface{}{"asset_id": a.ID, "name": a.Name, "content_type": a.ContentType, "size_bytes": a.Size, "storage_provider": a.Provider, "url": "/api/v1/assets/" + a.ID})
}

func readUpload(w http.ResponseWriter, r *http.Request) (io.ReadCloser, *multipart.FileHeader, string, []byte, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, assetstore.MaxSize+1024*1024)
	if err := r.ParseMultipartForm(assetstore.MaxSize); err != nil {
		respond(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "file exceeds 20 MB limit"})
		return nil, nil, "", nil, false
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": "multipart field file is required"})
		return nil, nil, "", nil, false
	}
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if !allowedContentType(contentType) {
		respond(w, http.StatusUnsupportedMediaType, map[string]string{"error": "only PDF, PNG, JPEG, SVG, TIFF, and WebP design files are accepted"})
		file.Close()
		return nil, nil, "", nil, false
	}
	data, err := io.ReadAll(io.LimitReader(file, assetstore.MaxSize+1))
	if err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": "could not read file"})
		file.Close()
		return nil, nil, "", nil, false
	}
	return file, header, contentType, data, true
}

func (h *Handler) claim(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var req struct {
		AssetID    string `json:"asset_id"`
		ClaimToken string `json:"claim_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": "asset_id and claim_token are required"})
		return
	}
	req.AssetID = strings.TrimSpace(req.AssetID)
	req.ClaimToken = strings.TrimSpace(req.ClaimToken)
	if req.AssetID == "" || req.ClaimToken == "" {
		respond(w, http.StatusBadRequest, map[string]string{"error": "asset_id and claim_token are required"})
		return
	}
	tokenHash := sha256.Sum256([]byte(req.ClaimToken))
	userID := middleware.GetUserID(r)
	var name, contentType, provider string
	var size int64
	err := h.db.QueryRowContext(r.Context(), `
		UPDATE design_assets
		SET owner_id=$1, guest_token_hash=NULL, expires_at=NULL
		WHERE id=$2
		  AND owner_id IS NULL
		  AND guest_token_hash=$3
		  AND expires_at > NOW()
		  AND deleted_at IS NULL
		RETURNING original_name, content_type, size_bytes, storage_provider`,
		userID, req.AssetID, hex.EncodeToString(tokenHash[:]),
	).Scan(&name, &contentType, &size, &provider)
	if errors.Is(err, sql.ErrNoRows) {
		// A successful claim is idempotent. This covers a retry after the first
		// response was lost without allowing one customer to claim another's file.
		err = h.db.QueryRowContext(r.Context(), `
			SELECT original_name, content_type, size_bytes, storage_provider
			FROM design_assets
			WHERE id=$1 AND owner_id=$2 AND deleted_at IS NULL`, req.AssetID, userID,
		).Scan(&name, &contentType, &size, &provider)
	}
	if errors.Is(err, sql.ErrNoRows) {
		respond(w, http.StatusGone, map[string]string{"error": "guest upload is invalid, expired, or already claimed"})
		return
	}
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": "could not claim guest upload"})
		return
	}
	respond(w, http.StatusOK, map[string]interface{}{"asset_id": req.AssetID, "name": name, "content_type": contentType, "size_bytes": size, "storage_provider": provider, "url": "/api/v1/assets/" + req.AssetID})
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
	if err := h.db.QueryRowContext(r.Context(), `SELECT owner_id FROM design_assets WHERE id=$1 AND owner_id IS NOT NULL AND deleted_at IS NULL`, assetID).Scan(&ownerID); err != nil {
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
