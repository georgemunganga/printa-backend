package payment

import (
	"context"
	"strings"
	"testing"
)

func TestListMethodsDoesNotExposeStubMobileMoneyGateways(t *testing.T) {
	service := NewService(nil, GatewayRegistry{
		ProviderMTNMomo: NewMTNMomoGateway("key", "secret", "https://example.com", "production"),
		ProviderAirtel:  NewAirtelMoneyGateway("key", "secret", "https://example.com", "production"),
	})
	methods := service.ListMethods()
	if len(methods) != 4 || !methods[0].Enabled || methods[0].Provider != ProviderCash {
		t.Fatalf("unexpected methods: %+v", methods)
	}
	if methods[1].Enabled || methods[2].Enabled || methods[3].Enabled {
		t.Fatalf("stub gateways must not be advertised as available: %+v", methods)
	}
}

func TestInitiateRejectsUnavailableMobileMoneyBeforePersisting(t *testing.T) {
	service := NewService(nil, GatewayRegistry{ProviderMTNMomo: NewMTNMomoGateway("", "", "", "production")})
	_, err := service.Initiate(context.Background(), InitiatePaymentRequest{Provider: string(ProviderMTNMomo)})
	if err == nil || !strings.Contains(err.Error(), "not currently available") {
		t.Fatalf("expected unavailable method error, got %v", err)
	}
}
