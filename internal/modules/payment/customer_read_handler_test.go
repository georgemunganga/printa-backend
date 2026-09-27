package payment

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/georgemunganga/printa-backend/internal/middleware"
	"github.com/georgemunganga/printa-backend/internal/modules/order"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type customerReadPayments struct {
	Service
	transaction *PaymentTransaction
	listed      int
}

func (s *customerReadPayments) GetByID(_ context.Context, _ string) (*PaymentTransaction, error) {
	return s.transaction, nil
}
func (s *customerReadPayments) ListByReference(_ context.Context, _ ReferenceType, _ string) ([]*PaymentTransaction, error) {
	s.listed++
	return []*PaymentTransaction{s.transaction}, nil
}

type customerReadOrders struct {
	order.Service
	owned *order.Order
}

func (s *customerReadOrders) GetOrder(_ context.Context, _ string) (*order.Order, error) {
	return s.owned, nil
}

func TestCustomerPaymentReadRequiresOrderOwnership(t *testing.T) {
	ownerID := uuid.New()
	otherID := uuid.New()
	orderID := uuid.New()
	transactionID := uuid.New()
	payments := &customerReadPayments{transaction: &PaymentTransaction{ID: transactionID, ReferenceType: RefOrder, ReferenceID: orderID, Status: TxPending}}
	orders := &customerReadOrders{owned: &order.Order{ID: orderID, CustomerID: &ownerID}}
	handler := NewHandler(payments, nil, orders)
	router := chi.NewRouter()
	handler.RegisterProtectedRoutes(router)

	request := func(userID uuid.UUID, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		ctx := context.WithValue(req.Context(), middleware.ContextKeyUserID, userID.String())
		ctx = context.WithValue(ctx, middleware.ContextKeyRole, string(middleware.RoleCustomer))
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req.WithContext(ctx))
		return response
	}
	for _, path := range []string{"/api/v1/payments/order/" + orderID.String(), "/api/v1/payments/" + transactionID.String()} {
		if got := request(ownerID, path).Code; got != http.StatusOK {
			t.Errorf("owner GET %s = %d", path, got)
		}
		if got := request(otherID, path).Code; got == http.StatusOK {
			t.Errorf("other user read %s", path)
		}
	}
	if payments.listed != 1 {
		t.Errorf("listed %d times; want owner only", payments.listed)
	}
}
