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
	// Quote validates current catalogue prices and returns totals without creating an order.
	Quote(ctx context.Context, storeID string, items []CartItem, discount, deliveryFee, deliveryDistanceKM float64) (*OrderQuote, error)
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

func (s *service) Quote(ctx context.Context, storeID string, cartItems []CartItem, discount, deliveryFee, deliveryDistanceKM float64) (*OrderQuote, error) {
	quote, _, err := s.calculate(ctx, storeID, cartItems, discount, deliveryFee, deliveryDistanceKM)
	return quote, err
}

func (s *service) calculate(ctx context.Context, storeID string, cartItems []CartItem, discount, deliveryFee, deliveryDistanceKM float64) (*OrderQuote, []*OrderItem, error) {
	if len(cartItems) == 0 {
		return nil, nil, fmt.Errorf("order must contain at least one item")
	}
	if storeID == "" {
		return nil, nil, fmt.Errorf("store_id is required")
	}
	if _, err := uuid.Parse(storeID); err != nil {
		return nil, nil, fmt.Errorf("invalid store_id: %w", err)
	}
	var subtotal float64
	items := make([]*OrderItem, 0, len(cartItems))
	for _, item := range cartItems {
		if item.Quantity <= 0 {
			return nil, nil, fmt.Errorf("quantity must be > 0 for product %s", item.VendorStoreProductID)
		}
		productID, err := uuid.Parse(item.VendorStoreProductID)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid vendor_store_product_id: %w", err)
		}
		price, available, err := s.repo.GetProductPrice(ctx, storeID, item.VendorStoreProductID)
		if err != nil {
			return nil, nil, fmt.Errorf("product %s not found in this store", item.VendorStoreProductID)
		}
		if !available {
			return nil, nil, fmt.Errorf("product %s is currently unavailable", item.VendorStoreProductID)
		}
		lineTotal := price * float64(item.Quantity)
		subtotal += lineTotal
		items = append(items, &OrderItem{ID: uuid.New(), VendorStoreProductID: productID, Quantity: item.Quantity, UnitPrice: price, LineTotal: lineTotal, Customisation: item.Customisation})
	}
	if discount < 0 {
		discount = 0
	}
	if deliveryFee < 0 {
		deliveryFee = 0
	}
	taxable := subtotal - discount
	if taxable < 0 {
		taxable = 0
	}
	tax := taxable * 0.16
	return &OrderQuote{
		Subtotal: round2(subtotal), Discount: round2(discount), Tax: round2(tax),
		DeliveryFee: round2(deliveryFee), DeliveryDistanceKM: round2(deliveryDistanceKM),
		Total: round2(taxable + tax + deliveryFee), Currency: "ZMW",
	}, items, nil
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
	if req.IdempotencyKey != "" {
		existing, err := s.repo.GetByIdempotencyKey(ctx, req.IdempotencyKey)
		if err == nil {
			if req.CustomerID != "" && (existing.CustomerID == nil || existing.CustomerID.String() != req.CustomerID) {
				return nil, fmt.Errorf("idempotency key is already associated with another customer")
			}
			return existing, nil
		}
		if !strings.Contains(err.Error(), "no rows") && !strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("lookup idempotent order: %w", err)
		}
	}

	storeID, err := uuid.Parse(req.StoreID)
	if err != nil {
		return nil, fmt.Errorf("invalid store_id: %w", err)
	}

	channel := OrderChannel(strings.ToUpper(req.Channel))
	if channel == "" {
		channel = ChannelOnline
	}

	quote, items, err := s.calculate(ctx, req.StoreID, req.Items, req.Discount, req.DeliveryFee, req.DeliveryDistanceKM)
	if err != nil {
		return nil, err
	}

	// ── Build order ───────────────────────────────────────────────────────────
	o := &Order{
		ID:                 uuid.New(),
		StoreID:            storeID,
		OrderNumber:        generateOrderNumber(),
		Status:             StatusPending,
		Channel:            channel,
		Subtotal:           quote.Subtotal,
		Discount:           quote.Discount,
		Tax:                quote.Tax,
		DeliveryFee:        quote.DeliveryFee,
		DeliveryDistanceKM: quote.DeliveryDistanceKM,
		Total:              quote.Total,
		Currency:           quote.Currency,
		Notes:              req.Notes,
		DeliveryAddress:    req.DeliveryAddress,
		IdempotencyKey:     req.IdempotencyKey,
		Items:              items,
	}

	if req.CustomerID != "" {
		uid, err := uuid.Parse(req.CustomerID)
		if err != nil {
			return nil, fmt.Errorf("invalid customer_id: %w", err)
		}
		o.CustomerID = &uid
	}

	if err := s.repo.CreateOrder(ctx, o); err != nil {
		if req.IdempotencyKey != "" && strings.Contains(err.Error(), "duplicate key") {
			if existing, lookupErr := s.repo.GetByIdempotencyKey(ctx, req.IdempotencyKey); lookupErr == nil {
				if req.CustomerID != "" && (existing.CustomerID == nil || existing.CustomerID.String() != req.CustomerID) {
					return nil, fmt.Errorf("idempotency key is already associated with another customer")
				}
				return existing, nil
			}
		}
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
