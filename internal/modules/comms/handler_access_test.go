package comms

import (
	"context"
	"github.com/georgemunganga/printa-backend/internal/middleware"
	"github.com/go-chi/chi/v5"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCustomerCannotSendArbitraryProviderMessages(t *testing.T) {
	r := chi.NewRouter()
	NewHandler(nil).RegisterRoutes(r)
	for _, role := range []middleware.Role{middleware.RoleCustomer} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/comms/send", strings.NewReader(`{"channel":"EMAIL","recipient":"external@example.test","body":"must not send"}`))
		ctx := context.WithValue(req.Context(), middleware.ContextKeyRole, string(role))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req.WithContext(ctx))
		if w.Code != http.StatusForbidden {
			t.Fatalf("role %s got %d", role, w.Code)
		}
	}
}

func TestPOSStaffCanReachReceiptValidation(t *testing.T) {
	r := chi.NewRouter()
	NewHandler(nil).RegisterRoutes(r)
	for _, role := range []middleware.Role{middleware.RoleStaff, middleware.RoleCashier} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/comms/send", strings.NewReader(`{}`))
		ctx := context.WithValue(req.Context(), middleware.ContextKeyRole, string(role))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req.WithContext(ctx))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("POS role %s cannot reach receipt validation: %d", role, w.Code)
		}
	}
}
