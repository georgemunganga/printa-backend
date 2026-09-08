package order

import (
	"context"
	"database/sql"
	"testing"
)

type deliveryTotalRepository struct{ created *Order }

func (r *deliveryTotalRepository) CreateOrder(_ context.Context, order *Order) error {
	r.created = order
	return nil
}
func (*deliveryTotalRepository) GetProductPrice(context.Context, string, string) (float64, bool, error) {
	return 100, true, nil
}
func (*deliveryTotalRepository) GetOrderByID(context.Context, string) (*Order, error) {
	return nil, sql.ErrNoRows
}
func (*deliveryTotalRepository) GetOrderByNumber(context.Context, string) (*Order, error) {
	return nil, sql.ErrNoRows
}
func (*deliveryTotalRepository) GetByIdempotencyKey(context.Context, string) (*Order, error) {
	return nil, sql.ErrNoRows
}
func (*deliveryTotalRepository) ListOrdersByStore(context.Context, string, string) ([]*Order, error) {
	return nil, nil
}
func (*deliveryTotalRepository) ListOrdersByCustomer(context.Context, string) ([]*Order, error) {
	return nil, nil
}
func (*deliveryTotalRepository) UpdateStatus(context.Context, string, OrderStatus) error { return nil }

func TestPlaceOrderIncludesServerDeliveryFeeInTotal(t *testing.T) {
	repo := &deliveryTotalRepository{}
	service := NewService(repo)
	order, err := service.PlaceOrder(context.Background(), PlaceOrderRequest{
		StoreID:            "7e6ed121-374a-4da0-a4a6-2a2cbddaa721",
		Channel:            "ONLINE",
		DeliveryFee:        50,
		DeliveryDistanceKM: 8.4,
		Items: []CartItem{{
			VendorStoreProductID: "e428134c-a9c7-49e6-9ea7-562ef166924c",
			Quantity:             2,
		}},
	})
	if err != nil {
		t.Fatalf("PlaceOrder() error = %v", err)
	}
	if order.Subtotal != 200 || order.Tax != 32 || order.DeliveryFee != 50 || order.Total != 282 {
		t.Fatalf("unexpected totals: subtotal %.2f tax %.2f delivery %.2f total %.2f", order.Subtotal, order.Tax, order.DeliveryFee, order.Total)
	}
	if repo.created != order || order.DeliveryDistanceKM != 8.4 {
		t.Fatalf("delivery quote was not persisted on the created order")
	}
}

func TestQuoteReturnsSameServerTotalsWithoutCreatingOrder(t *testing.T) {
	repo := &deliveryTotalRepository{}
	service := NewService(repo)
	quote, err := service.Quote(context.Background(), "7e6ed121-374a-4da0-a4a6-2a2cbddaa721", []CartItem{{
		VendorStoreProductID: "e428134c-a9c7-49e6-9ea7-562ef166924c", Quantity: 3,
	}}, 0, 40, 4.8)
	if err != nil {
		t.Fatalf("Quote() error = %v", err)
	}
	if quote.Subtotal != 300 || quote.Tax != 48 || quote.DeliveryFee != 40 || quote.Total != 388 || quote.DeliveryDistanceKM != 4.8 {
		t.Fatalf("unexpected quote: %+v", quote)
	}
	if repo.created != nil {
		t.Fatal("Quote() persisted an order")
	}
}
