package conversation

import (
	"context"
	"github.com/georgemunganga/printa-backend/internal/middleware"
	"github.com/georgemunganga/printa-backend/internal/modules/order"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type customerOrders struct {
	order.Service
	purchase *order.Order
}

func (s customerOrders) GetOrder(context.Context, string) (*order.Order, error) {
	return s.purchase, nil
}

type customerConversation struct {
	Service
	calls  int
	sender string
}

func (s *customerConversation) List(_ context.Context, _, reader string) ([]*Message, error) {
	s.calls++
	s.sender = reader
	return []*Message{}, nil
}
func (s *customerConversation) Send(_ context.Context, _, sender, body string, _ []string) (*Message, error) {
	s.calls++
	s.sender = sender
	return &Message{ID: uuid.New(), Body: body}, nil
}
func TestCustomerConversationOwnershipForEveryAccountRole(t *testing.T) {
	owner, other, purchase := uuid.New(), uuid.New(), uuid.New()
	for _, role := range []middleware.Role{middleware.RoleCustomer, middleware.RoleVendor, middleware.RoleAdmin, middleware.RoleStaff, middleware.RoleCashier} {
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			t.Run(string(role)+method, func(t *testing.T) {
				svc := &customerConversation{}
				h := NewHandler(svc, customerOrders{purchase: &order.Order{ID: purchase, CustomerID: &owner}}, nil, nil, nil)
				r := chi.NewRouter()
				h.RegisterCustomerRoutes(r)
				for _, user := range []uuid.UUID{owner, other} {
					req := httptest.NewRequest(method, "/api/v1/customer/conversations/orders/"+purchase.String()+"/messages", strings.NewReader(`{"body":"My order"}`))
					ctx := context.WithValue(req.Context(), middleware.ContextKeyUserID, user.String())
					ctx = context.WithValue(ctx, middleware.ContextKeyRole, string(role))
					w := httptest.NewRecorder()
					r.ServeHTTP(w, req.WithContext(ctx))
					want := http.StatusNotFound
					if user == owner {
						want = http.StatusOK
						if method == http.MethodPost {
							want = http.StatusCreated
						}
					}
					if w.Code != want {
						t.Fatalf("%s: got %d want %d body=%s", user, w.Code, want, w.Body.String())
					}
				}
				if svc.calls != 1 || svc.sender != owner.String() {
					t.Fatal("non-owner reached conversation service")
				}
			})
		}
	}
}
func TestCustomerAttachmentsKeepCustomerRoute(t *testing.T) {
	h := &Handler{}
	m := &Message{ID: uuid.New(), Attachments: []*Attachment{{AssetID: uuid.New()}}}
	req := httptest.NewRequest("GET", "/api/v1/customer/conversations/orders/order/messages", nil)
	h.withAttachmentURLs(req, "order", m)
	if !strings.HasPrefix(m.Attachments[0].URL, "/api/v1/customer/conversations/orders/") {
		t.Fatal("attachment sent back through vendor gate")
	}
}
