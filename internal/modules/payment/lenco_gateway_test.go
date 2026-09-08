package payment

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLencoMobileMoneyGatewayInitiatesRealCollectionRequest(t *testing.T) {
	var operator, phone string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/collections/mobile-money" || r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("unexpected request %s authorization=%q", r.URL.Path, r.Header.Get("Authorization"))
		}
		var body struct {
			Amount                               float64 `json:"amount"`
			Reference, Phone, Operator, Currency string
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		operator, phone = body.Operator, body.Phone
		_ = json.NewEncoder(w).Encode(map[string]any{"status": true, "data": map[string]any{
			"id": "collection-1", "amount": "88.00", "currency": "ZMW", "reference": body.Reference, "status": "pay-offline",
		}})
	}))
	defer server.Close()

	gateway := NewLencoMobileMoneyGateway(server.URL, "secret", "mtn")
	available, ok := gateway.(availabilityAwareGateway)
	if !ok || !available.Available() {
		t.Fatal("configured Lenco gateway should be available")
	}
	result, err := gateway.Initiate(context.Background(), &InitiatePaymentRequest{Amount: 88, Currency: "ZMW", PhoneNumber: "0977000000"})
	if err != nil {
		t.Fatalf("Initiate() error = %v", err)
	}
	if operator != "mtn" || phone != "0977000000" || result.ProviderRef == "" || result.ProviderStatus != "pay-offline" {
		t.Fatalf("request operator=%q phone=%q result=%+v", operator, phone, result)
	}
}
