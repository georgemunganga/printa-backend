package payment

import (
	"context"
	"github.com/georgemunganga/printa-backend/internal/middleware"
	"github.com/georgemunganga/printa-backend/internal/modules/order"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"net/http/httptest"
	"strings"
	"testing"
)

type customerWritePayments struct {
	Service
	calls    int
	captured InitiatePaymentRequest
}

func (s *customerWritePayments) Initiate(_ context.Context, req InitiatePaymentRequest) (*PaymentTransaction, error) {
	s.calls++
	s.captured = req
	return &PaymentTransaction{ID: uuid.New(), Status: TxPending}, nil
}
func TestCustomerPaymentCannotBecomeVendorPayment(t *testing.T) {
	owner, other, purchase := uuid.New(), uuid.New(), uuid.New()
	for _, role := range []middleware.Role{middleware.RoleCustomer, middleware.RoleVendor, middleware.RoleAdmin, middleware.RoleStaff, middleware.RoleCashier} {
		t.Run(string(role), func(t *testing.T) {
			svc := &customerWritePayments{}
			h := NewHandler(svc, nil, &customerReadOrders{owned: &order.Order{ID: purchase, CustomerID: &owner, Total: 88, Currency: "ZMW"}})
			r := chi.NewRouter()
			h.RegisterCustomerRoutes(r)
			for _, test := range []struct {
				user uuid.UUID
				ref  string
				want int
			}{{owner, "ORDER", 201}, {other, "ORDER", 403}, {owner, "INVOICE", 403}} {
				req := httptest.NewRequest("POST", "/api/v1/customer/payments", strings.NewReader(`{"reference_type":"`+test.ref+`","reference_id":"`+purchase.String()+`","amount":1,"currency":"USD","vendor_id":"spoofed"}`))
				req.Header.Set("Idempotency-Key", "stable-order-attempt")
				ctx := context.WithValue(req.Context(), middleware.ContextKeyUserID, test.user.String())
				ctx = context.WithValue(ctx, middleware.ContextKeyRole, string(role))
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req.WithContext(ctx))
				if w.Code != test.want {
					t.Fatalf("got %d want %d", w.Code, test.want)
				}
			}
			if svc.calls != 1 || svc.captured.Amount != 88 || svc.captured.Currency != "ZMW" || svc.captured.VendorID != "" || svc.captured.IdempotencyKey != "stable-order-attempt" {
				t.Fatalf("untrusted customer payment: %+v", svc.captured)
			}
		})
	}
}
