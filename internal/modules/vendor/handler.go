package vendor

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/georgemunganga/printa-backend/internal/middleware"
	"github.com/georgemunganga/printa-backend/internal/modules/policyconsent"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service        Service
	consentService policyconsent.Service
}

func NewHandler(service Service, consentService policyconsent.Service) *Handler {
	return &Handler{service: service, consentService: consentService}
}

func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Route("/api/v1/vendor", func(r chi.Router) {
		r.Post("/onboard", h.onboardVendor)
		r.Get("/profile", h.getVendor)
	})
}

type onboardVendorRequest struct {
	OwnerID      string `json:"owner_id"`
	BusinessName string `json:"business_name"`
	TaxID        string `json:"tax_id"`

	// The wizard supplies these fields when it is completing initial vendor
	// onboarding. They are intentionally optional so the established profile-only
	// administrative flow remains backward compatible.
	StoreName      string   `json:"store_name"`
	StoreAddress   string   `json:"store_address"`
	StoreCity      string   `json:"store_city"`
	StoreCountry   string   `json:"store_country"`
	StoreLatitude  *float64 `json:"store_latitude"`
	StoreLongitude *float64 `json:"store_longitude"`
	StaffPIN       string   `json:"staff_pin"`
}

func (h *Handler) onboardVendor(w http.ResponseWriter, r *http.Request) {
	var req onboardVendorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if middleware.GetRole(r) != middleware.RoleAdmin {
		if middleware.GetRole(r) != middleware.RoleVendor {
			http.Error(w, "insufficient permissions", http.StatusForbidden)
			return
		}
		req.OwnerID = middleware.GetUserID(r)
	}

	accepted, err := h.consentService.HasRequiredAcceptance(r.Context(), req.OwnerID)
	if err != nil {
		http.Error(w, "unable to verify required vendor policy acceptance", http.StatusInternalServerError)
		return
	}
	if !accepted {
		http.Error(w, "acceptance of the current Vendor Terms and Privacy Notice is required before onboarding", http.StatusPreconditionRequired)
		return
	}

	var vendorRecord *Vendor
	if strings.TrimSpace(req.StoreName) != "" {
		vendorRecord, err = h.service.OnboardVendorWithFirstStore(r.Context(), req.OwnerID, req.BusinessName, req.TaxID, FirstStoreInput{
			Name:      req.StoreName,
			Address:   req.StoreAddress,
			City:      req.StoreCity,
			Country:   req.StoreCountry,
			Latitude:  req.StoreLatitude,
			Longitude: req.StoreLongitude,
			OwnerPIN:  req.StaffPIN,
		})
	} else {
		vendorRecord, err = h.service.OnboardVendor(r.Context(), req.OwnerID, req.BusinessName, req.TaxID)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.consentService.AttachAcceptedPoliciesToVendor(r.Context(), req.OwnerID, vendorRecord.ID.String(), policyconsent.RequestIP(r), r.UserAgent()); err != nil {
		http.Error(w, "vendor onboarding could not record policy evidence; retry the request", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(vendorRecord)
}

func (h *Handler) getVendor(w http.ResponseWriter, r *http.Request) {
	ownerID := r.URL.Query().Get("owner_id")
	if middleware.GetRole(r) != middleware.RoleAdmin {
		if middleware.GetRole(r) != middleware.RoleVendor {
			http.Error(w, "insufficient permissions", http.StatusForbidden)
			return
		}
		ownerID = middleware.GetUserID(r)
	}
	if ownerID == "" {
		http.Error(w, "owner_id is required", http.StatusBadRequest)
		return
	}

	vendorRecord, err := h.service.GetVendor(r.Context(), ownerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(vendorRecord)
}
