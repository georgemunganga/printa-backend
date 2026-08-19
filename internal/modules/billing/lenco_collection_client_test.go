package billing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLencoCollectionClientInitiatesMobileMoneyWithServerRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/collections/mobile-money" {
			t.Fatalf("request = %s %s, want POST /collections/mobile-money", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret-test" {
			t.Fatalf("Authorization = %q", got)
		}
		if got := r.Header.Get("User-Agent"); got != "Printa-Subscription-Service/1.0" {
			t.Fatalf("User-Agent = %q", got)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload["amount"] != float64(500) || payload["reference"] != "SUB-locked-reference" || payload["phone"] != "0977433571" || payload["operator"] != "mtn" || payload["country"] != "zm" || payload["bearer"] != "merchant" {
			t.Fatalf("payload = %#v", payload)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":true,"message":"","data":{"id":"collection-1","amount":"500.00","currency":"ZMW","reference":"SUB-locked-reference","status":"pay-offline","reasonForFailure":null}}`))
	}))
	defer server.Close()

	client := NewLencoCollectionClient(server.URL, "secret-test")
	collection, err := client.InitiateMobileMoneyCollection(context.Background(), MobileMoneyCollectionRequest{
		Amount: 500, Currency: "ZMW", Reference: "SUB-locked-reference", Phone: "0977433571", Operator: "mtn", Country: "zm", Bearer: "merchant",
	})
	if err != nil {
		t.Fatalf("InitiateMobileMoneyCollection() error = %v", err)
	}
	if collection.ID != "collection-1" || collection.Status != "pay-offline" || collection.Amount != 500 || collection.Reference != "SUB-locked-reference" {
		t.Fatalf("collection = %#v", collection)
	}
}
