package payment

import (
	"context"
	"fmt"
	"strings"

	"github.com/georgemunganga/printa-backend/internal/modules/billing"
	"github.com/google/uuid"
)

// lencoMobileMoneyGateway uses the same production collection client as vendor subscriptions.
type lencoMobileMoneyGateway struct {
	client   *billing.LencoCollectionClient
	operator string
	ready    bool
}

func NewLencoMobileMoneyGateway(baseURL, secret, operator string) Gateway {
	return &lencoMobileMoneyGateway{
		client:   billing.NewLencoCollectionClient(baseURL, secret),
		operator: strings.ToLower(strings.TrimSpace(operator)),
		ready:    strings.TrimSpace(baseURL) != "" && strings.TrimSpace(secret) != "",
	}
}

func (g *lencoMobileMoneyGateway) Available() bool { return g.ready }

func (g *lencoMobileMoneyGateway) Initiate(ctx context.Context, req *InitiatePaymentRequest) (*ProviderInitResponse, error) {
	phone := strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(strings.TrimSpace(req.PhoneNumber))
	phone = strings.TrimPrefix(phone, "+")
	if len(phone) < 9 || len(phone) > 15 || strings.Trim(phone, "0123456789") != "" {
		return nil, fmt.Errorf("phone_number is required for mobile money")
	}
	reference := "ORDER-" + uuid.NewString()
	collection, err := g.client.InitiateMobileMoneyCollection(ctx, billing.MobileMoneyCollectionRequest{
		Amount: req.Amount, Currency: req.Currency, Reference: reference,
		Phone: phone, Operator: g.operator, Country: "zm", Bearer: "merchant",
	})
	if err != nil {
		return nil, err
	}
	if collection.Reference != reference {
		return nil, fmt.Errorf("collection reference did not match payment request")
	}
	return &ProviderInitResponse{ProviderRef: reference, ProviderStatus: collection.Status, Message: "Approve the payment prompt on your phone."}, nil
}

func (g *lencoMobileMoneyGateway) Verify(ctx context.Context, reference string) (*ProviderInitResponse, error) {
	collection, err := g.client.VerifyCollection(ctx, reference)
	if err != nil {
		return nil, err
	}
	return &ProviderInitResponse{ProviderRef: reference, ProviderStatus: collection.Status}, nil
}

func (g *lencoMobileMoneyGateway) Refund(context.Context, string, float64) (*ProviderInitResponse, error) {
	return nil, fmt.Errorf("mobile money refunds require manual review")
}
