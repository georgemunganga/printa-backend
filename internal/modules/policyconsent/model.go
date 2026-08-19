package policyconsent

import (
	"time"

	"github.com/google/uuid"
)

type PolicyStatus string

const (
	PolicyDraft     PolicyStatus = "DRAFT"
	PolicyPublished PolicyStatus = "PUBLISHED"
	PolicyRetired   PolicyStatus = "RETIRED"
)

type Policy struct {
	ID                uuid.UUID    `json:"id"`
	Slug              string       `json:"slug"`
	Version           string       `json:"version"`
	Title             string       `json:"title"`
	Summary           string       `json:"summary"`
	Status            PolicyStatus `json:"status"`
	RequiredForVendor bool         `json:"required_for_vendor"`
	DocumentURL       string       `json:"document_url,omitempty"`
	EffectiveAt       *time.Time   `json:"effective_at,omitempty"`
	PublishedAt       *time.Time   `json:"published_at,omitempty"`
}

type PolicyAcceptance struct {
	Slug    string `json:"slug"`
	Version string `json:"version"`
}

type ConsentStatus struct {
	RequiredPolicies    []Policy `json:"required_policies"`
	AcceptedPolicySlugs []string `json:"accepted_policy_slugs"`
	AcceptanceRequired  bool     `json:"acceptance_required"`
}
