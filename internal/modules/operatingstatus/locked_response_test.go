package operatingstatus

import (
	"context"
	"github.com/georgemunganga/printa-backend/internal/middleware"
	"github.com/georgemunganga/printa-backend/internal/modules/vendor"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type pausedStatus struct{ Service }

func (pausedStatus) GetStatus(context.Context, string) (*OperatingStatus, error) {
	return &OperatingStatus{Operational: false, BlockingReasons: []BlockReason{BlockSubscriptionDue}}, nil
}

type pausedVendor struct{ vendor.Service }

func (pausedVendor) GetVendor(context.Context, string) (*vendor.Vendor, error) {
	return &vendor.Vendor{ID: uuid.New()}, nil
}
func TestVendorFinancialOperationsRemainLocked(t *testing.T) {
	h := RequireOperationalVendor(pausedStatus{}, pausedVendor{})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("paused vendor operation was allowed") }))
	for _, path := range []string{"/api/v1/payments", "/api/v1/billing/subscriptions", "/api/v1/pos/sales"} {
		r := httptest.NewRequest("POST", path, nil)
		ctx := context.WithValue(r.Context(), middleware.ContextKeyRole, string(middleware.RoleVendor))
		ctx = context.WithValue(ctx, middleware.ContextKeyUserID, uuid.New().String())
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r.WithContext(ctx))
		if w.Code != 423 || !strings.Contains(w.Body.String(), "VENDOR_OPERATIONS_LOCKED") || !strings.Contains(w.Body.String(), "blocking_reasons") {
			t.Fatalf("unexpected locked response: %d %s", w.Code, w.Body.String())
		}
	}
}
