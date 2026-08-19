package billing

import (
	"context"
	"time"
)

// Repository defines data access for subscriptions and invoices.
type Repository interface {
	// Subscription
	CreateSubscription(ctx context.Context, sub *VendorSubscription) error
	GetSubscriptionByVendor(ctx context.Context, vendorID string) (*VendorSubscription, error)
	GetSubscriptionByID(ctx context.Context, id string) (*VendorSubscription, error)
	UpdateSubscriptionStatus(ctx context.Context, id string, status SubscriptionStatus, reason string) error
	UpdateSubscriptionTier(ctx context.Context, id string, tierID string) error
	RenewSubscriptionPeriod(ctx context.Context, id string, start, end interface{}) error
	ListExpiredSubscriptions(ctx context.Context) ([]*VendorSubscription, error)

	// Invoice
	CreateInvoice(ctx context.Context, inv *BillingInvoice) error
	GetInvoiceByID(ctx context.Context, id string) (*BillingInvoice, error)
	GetInvoiceByNumber(ctx context.Context, number string) (*BillingInvoice, error)
	GetInvoiceByIdempotencyKey(ctx context.Context, key string) (*BillingInvoice, error)
	ListInvoicesByVendor(ctx context.Context, vendorID string) ([]*BillingInvoice, error)
	ListInvoicesBySubscription(ctx context.Context, subscriptionID string) ([]*BillingInvoice, error)
	MarkInvoicePaid(ctx context.Context, id string, ref string, notes string) error
	VoidInvoice(ctx context.Context, id string) error

	// Tier catalogue
	ListTiers(ctx context.Context) ([]*VendorTier, error)
	GetTierCatalogueEntry(ctx context.Context, tierID string) (*VendorTier, error)

	// Subscription checkout
	CreateCheckout(ctx context.Context, checkout *SubscriptionCheckout) error
	GetCheckoutByID(ctx context.Context, id string) (*SubscriptionCheckout, error)
	GetCheckoutByReference(ctx context.Context, reference string) (*SubscriptionCheckout, error)
	GetReusablePendingCheckout(ctx context.Context, vendorID, tierID string, now time.Time) (*SubscriptionCheckout, error)
	RecordCheckoutCollection(ctx context.Context, id, providerCollectionID, providerStatus string) error
	MarkCheckoutFailed(ctx context.Context, id, providerStatus, reason string) error
	ActivateCheckout(ctx context.Context, checkoutID, providerCollectionID, providerStatus string, completedAt time.Time) (*SubscriptionCheckout, error)

	// Tier lookup (needed for invoice generation)
	GetTierByID(ctx context.Context, tierID string) (name string, price float64, err error)
}
