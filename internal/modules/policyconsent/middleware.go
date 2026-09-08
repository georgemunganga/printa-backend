package policyconsent

import (
	"net/http"
	"strings"

	"github.com/georgemunganga/printa-backend/internal/middleware"
)

func RequireCurrentVendorConsent(service Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if middleware.GetRole(r) != middleware.RoleVendor || consentExemptRequest(r) {
				next.ServeHTTP(w, r)
				return
			}
			accepted, err := service.HasRequiredAcceptance(r.Context(), middleware.GetUserID(r))
			if err != nil {
				respondError(w, http.StatusInternalServerError, "unable to verify required vendor policy acceptance")
				return
			}
			if !accepted {
				respondError(w, http.StatusPreconditionRequired, "acceptance of the current Vendor Terms and Privacy Notice is required before using vendor operations")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func consentExemptRequest(r *http.Request) bool {
	if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/orders/") {
		return true
	}
	path := r.URL.Path
	return strings.HasPrefix(path, "/api/v1/vendor/policies/") || strings.HasPrefix(path, "/api/v1/vendor/operating-status") || path == "/api/v1/vendor/onboard" || path == "/api/v1/vendor/profile" || strings.HasPrefix(path, "/api/v1/users/") || strings.HasPrefix(path, "/api/v1/orders/customer/") || path == "/api/v1/orders" || strings.HasPrefix(path, "/api/v1/delivery/locations") || path == "/api/v1/assets/claim" || path == "/api/v1/assets/upload"
}
