package operatingstatus

import (
	"net/http"
	"strings"

	"github.com/georgemunganga/printa-backend/internal/middleware"
	"github.com/georgemunganga/printa-backend/internal/modules/vendor"
)

func RequireOperationalVendor(service Service, vendorService vendor.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if middleware.GetRole(r) != middleware.RoleVendor || operatingStatusExemptRequest(r) {
				next.ServeHTTP(w, r)
				return
			}
			vendorProfile, err := vendorService.GetVendor(r.Context(), middleware.GetUserID(r))
			if err != nil {
				respondJSON(w, http.StatusForbidden, map[string]string{"error": "authenticated vendor profile is required"})
				return
			}
			status, err := service.GetStatus(r.Context(), vendorProfile.ID.String())
			if err != nil {
				respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "unable to verify vendor operating status"})
				return
			}
			if !status.Operational {
				respondJSON(w, http.StatusLocked, status)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func operatingStatusExemptRequest(r *http.Request) bool {
	if operatingStatusExemptPath(r.URL.Path) {
		return true
	}

	// Subscription, invoice, and wallet overview reads are recovery information, not operational
	// actions. Writes remain gated so a paused vendor cannot use recovery endpoints to bypass controls.
	return r.Method == http.MethodGet && (strings.HasPrefix(r.URL.Path, "/api/v1/billing/") || r.URL.Path == "/api/v1/vendor/wallet")
}

func operatingStatusExemptPath(path string) bool {
	return strings.HasPrefix(path, "/api/v1/vendor/operating-status") ||
		strings.HasPrefix(path, "/api/v1/vendor/policies/") ||
		path == "/api/v1/vendor/onboard" ||
		path == "/api/v1/vendor/profile" ||
		strings.HasPrefix(path, "/api/v1/users/") ||
		strings.HasPrefix(path, "/api/v1/submissions/")
}
