package order

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Service defines the order management business logic.
type Service interface {
	// PlaceOrder validates the cart, calculates totals, and persists the order atomically.
	PlaceOrder(ctx context.Context, req PlaceOrderRequest) (*Order, error)

	// GetOrder retrieves a full order with its items by UUID.
	GetOrder(ctx context.Context, id string) (*Order, error)

	// GetOrderByNumber retrieves an order by its human-readable number.
	GetOrderByNumber(ctx context.Context, orderNumber string) (*Order, error)

	// ListStoreOrders returns all orders for a store, optionally filtered by status.
	ListStoreOrders(ctx context.Context, storeID string, status string) ([]*Order, error)

	// ListCustomerOrders returns all orders placed by a customer.
	ListCustomerOrders(ctx context.Context, customerID string) ([]*Order, error)

	// UpdateStatus advances an order to a new lifecycle status.
	UpdateStatus(ctx context.Context, id string, req UpdateStatusRequest) (*Order, error)

	// CancelOrder cancels a PENDING or CONFIRMED order.
	CancelOrder(ctx context.Context, id string) error
}

type service struct {
	repo Repository
}

// NewService creates a new order service.
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

// validTransitions defines the allowed status state machine.
var validTransitions = map[OrderStatus][]OrderStatus{
	StatusPending:      {StatusConfirmed, StatusCancelled},
	StatusConfirmed:    {StatusInProduction, StatusCancelled},
	StatusInProduction: {StatusReady},
	StatusReady:        {StatusDelivered},
	StatusDelivered:    {},
	StatusCancelled:    {},
}

func (s *service) PlaceOrder(ctx context.Context, req PlaceOrderRequest) (*Order, error) {
	if len(req.Items) == 0 {
		return nil, fmt.Errorf("order must contain at least one item")
	}
	if req.StoreID == "" {
		return nil, fmt.Errorf("store_id is required")
	}

	storeID, err := uuid.Parse(req.StoreID)
	if err != nil {
		return nil, fmt.Errorf("invalid store_id: %w", err)
	}

	channel := OrderChannel(strings.ToUpper(req.Channel))
	if channel == "" {
		channel = ChannelOnline
	}

	// ── Build order items, validate stock & availability ──────────────────────
	var items []*OrderItem
	var subtotal float64

	for _, ci := range req.Items {
		if ci.Quantity <= 0 {
			return nil, fmt.Errorf("quantity must be > 0 for product %s", ci.VendorStoreProductID)
		}
		price, available, err := s.repo.GetProductPrice(ctx, ci.VendorStoreProductID)
		if err != nil {
			return nil, fmt.Errorf("product %s not found in this store", ci.VendorStoreProductID)
		}
		if !available {
			return nil, fmt.Errorf("product %s is currently unavailable", ci.VendorStoreProductID)
		}

		pid, err := uuid.Parse(ci.VendorStoreProductID)
		if err != nil {
			return nil, fmt.Errorf("invalid vendor_store_product_id: %w", err)
		}

		lineTotal := price * float64(ci.Quantity)
		subtotal += lineTotal

		items = append(items, &OrderItem{
			ID:                   uuid.New(),
			VendorStoreProductID: pid,
			Quantity:             ci.Quantity,
			UnitPrice:            price,
			LineTotal:            lineTotal,
			Customisation:        ci.Customisation,
		})
	}

	// ── Calculate totals ──────────────────────────────────────────────────────
	discount := req.Discount
	if discount < 0 {
		discount = 0
	}
	taxRate := 0.16 // 16% VAT — Zambia standard rate
	taxable := subtotal - discount
	if taxable < 0 {
		taxable = 0
	}
	tax := taxable * taxRate
	total := taxable + tax

	// ── Build order ───────────────────────────────────────────────────────────
	o := &Order{
		ID:              uuid.New(),
		StoreID:         storeID,
		OrderNumber:     generateOrderNumber(),
		Status:          StatusPending,
		Channel:         channel,
		Subtotal:        round2(subtotal),
		Discount:        round2(discount),
		Tax:             round2(tax),
		Total:           round2(total),
		Currency:        "ZMW",
		Notes:           req.Notes,
		DeliveryAddress: req.DeliveryAddress,
		Items:           items,
	}

	if req.CustomerID != "" {
		uid, err := uuid.Parse(req.CustomerID)
		if err != nil {
			return nil, fmt.Errorf("invalid customer_id: %w", err)
		}
		o.CustomerID = &uid
	}

	if err := s.repo.CreateOrder(ctx, o); err != nil {
		return nil, fmt.Errorf("failed to persist order: %w", err)
	}
	return o, nil
}

func (s *service) GetOrder(ctx context.Context, id string) (*Order, error) {
	return s.repo.GetOrderByID(ctx, id)
}

func (s *service) GetOrderByNumber(ctx context.Context, orderNumber string) (*Order, error) {
	return s.repo.GetOrderByNumber(ctx, orderNumber)
}

func (s *service) ListStoreOrders(ctx context.Context, storeID string, status string) ([]*Order, error) {
	return s.repo.ListOrdersByStore(ctx, storeID, status)
}

func (s *service) ListCustomerOrders(ctx context.Context, customerID string) ([]*Order, error) {
	return s.repo.ListOrdersByCustomer(ctx, customerID)
}

func (s *service) UpdateStatus(ctx context.Context, id string, req UpdateStatusRequest) (*Order, error) {
	o, err := s.repo.GetOrderByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("order not found: %w", err)
	}

	newStatus := OrderStatus(strings.ToUpper(req.Status))
	allowed := validTransitions[o.Status]
	valid := false
	for _, s := range allowed {
		if s == newStatus {
			valid = true
			break
		}
	}
	if !valid {
		return nil, fmt.Errorf("cannot transition order from %s to %s", o.Status, newStatus)
	}

	if err := s.repo.UpdateStatus(ctx, id, newStatus); err != nil {
		return nil, err
	}
	o.Status = newStatus
	return o, nil
}

func (s *service) CancelOrder(ctx context.Context, id string) error {
	o, err := s.repo.GetOrderByID(ctx, id)
	if err != nil {
		return fmt.Errorf("order not found: %w", err)
	}
	if o.Status != StatusPending && o.Status != StatusConfirmed {
		return fmt.Errorf("only PENDING or CONFIRMED orders can be cancelled (current: %s)", o.Status)
	}
	return s.repo.UpdateStatus(ctx, id, StatusCancelled)
}

// ── helpers ───────────────────────────────────────────────────────────────────

// generateOrderNumber creates a human-readable order number: ORD-YYYYMMDD-XXXX
func generateOrderNumber() string {
	date := time.Now().UTC().Format("20060102")
	suffix := strings.ToUpper(uuid.New().String()[:4])
	return fmt.Sprintf("ORD-%s-%s", date, suffix)
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
