package billing

import "context"

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

	// Tier lookup (needed for invoice generation)
	GetTierByID(ctx context.Context, tierID string) (name string, price float64, err error)
}
