package lenco

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
)

const maxWebhookBody = 1 << 20

// Handler accepts signed provider events. It intentionally persists events only;
// wallet posting is introduced by the separately reviewed ledger module.
type Handler struct {
	db              *sql.DB
	signatureKey    string
	collectionEvent func(context.Context, string) error
}

func NewHandler(db *sql.DB, signatureKey string, collectionEvent ...func(context.Context, string) error) *Handler {
	h := &Handler{db: db, signatureKey: strings.TrimSpace(signatureKey)}
	if len(collectionEvent) > 0 {
		h.collectionEvent = collectionEvent[0]
	}
	return h
}

func NewHandlerFromEnv(db *sql.DB, collectionEvent ...func(context.Context, string) error) *Handler {
	return NewHandler(db, os.Getenv("LENCO_WEBHOOK_SIGNATURE_KEY"), collectionEvent...)
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/api/v1/webhooks/lenco", h.receive)
}

type eventEnvelope struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

func (h *Handler) receive(w http.ResponseWriter, r *http.Request) {
	if h.signatureKey == "" {
		http.Error(w, `{"error":"webhook receiver is not configured"}`, http.StatusServiceUnavailable)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBody))
	if err != nil {
		http.Error(w, `{"error":"invalid webhook body"}`, http.StatusBadRequest)
		return
	}
	provided := strings.TrimSpace(r.Header.Get("X-Lenco-Signature"))
	mac := hmac.New(sha512.New, []byte(h.signatureKey))
	_, _ = mac.Write(body)
	expected := mac.Sum(nil)
	candidate, err := hex.DecodeString(provided)
	if err != nil || !hmac.Equal(expected, candidate) {
		http.Error(w, `{"error":"unauthorized webhook"}`, http.StatusUnauthorized)
		return
	}
	var envelope eventEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil || strings.TrimSpace(envelope.Event) == "" {
		http.Error(w, `{"error":"invalid webhook payload"}`, http.StatusBadRequest)
		return
	}
	var data map[string]any
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		http.Error(w, `{"error":"invalid webhook data"}`, http.StatusBadRequest)
		return
	}
	externalRef := firstString(data, "id", "reference", "transactionReference", "clientReference")
	if externalRef == "" {
		http.Error(w, `{"error":"webhook event reference is required"}`, http.StatusBadRequest)
		return
	}
	eventKey := envelope.Event + ":" + externalRef
	_, err = h.db.ExecContext(r.Context(), `
		INSERT INTO lenco_webhook_events (event_key, event_type, external_reference, payload)
		VALUES ($1, $2, $3, $4::jsonb)
		ON CONFLICT (event_key) DO NOTHING`, eventKey, envelope.Event, externalRef, string(body))
	if err != nil {
		http.Error(w, `{"error":"unable to record webhook"}`, http.StatusInternalServerError)
		return
	}
	// Transaction callbacks can shorten the confirmation path, but payment value
	// is still granted only after the billing service re-queries the collection by
	// its server-created reference. Collection event aliases remain accepted for
	// compatibility with provider configurations that use collection naming.
	if h.collectionEvent != nil && (envelope.Event == "transaction.successful" || envelope.Event == "transaction.failed" || envelope.Event == "collection.successful" || envelope.Event == "collection.failed") {
		if reference := firstString(data, "reference", "clientReference"); reference != "" {
			callback := h.collectionEvent
			go func() { _ = callback(context.Background(), reference) }()
		}
	}
	w.WriteHeader(http.StatusOK)
}

func firstString(data map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := data[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
