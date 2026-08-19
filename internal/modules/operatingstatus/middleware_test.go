package operatingstatus

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOperatingStatusExemptRequestAllowsOnlyReadOnlyBillingRecovery(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		want   bool
	}{
		{name: "tier catalogue read", method: http.MethodGet, path: "/api/v1/billing/tiers", want: true},
		{name: "subscription read", method: http.MethodGet, path: "/api/v1/billing/subscriptions/vendor/vendor-1", want: true},
		{name: "invoice history read", method: http.MethodGet, path: "/api/v1/billing/invoices/vendor/vendor-1", want: true},
		{name: "subscription creation remains locked", method: http.MethodPost, path: "/api/v1/billing/subscriptions", want: false},
		{name: "tier change remains locked", method: http.MethodPatch, path: "/api/v1/billing/subscriptions/vendor/vendor-1/tier", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(test.method, test.path, nil)
			if got := operatingStatusExemptRequest(req); got != test.want {
				t.Fatalf("operatingStatusExemptRequest(%s %s) = %t, want %t", test.method, test.path, got, test.want)
			}
		})
	}
}
