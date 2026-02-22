package payment

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// Gateway is the provider-agnostic interface every payment adapter must implement.
// To add a new provider (e.g., Visa, Stripe), implement this interface.
type Gateway interface {
	// Initiate sends a payment request to the provider and returns the provider reference.
	Initiate(ctx context.Context, req *InitiatePaymentRequest) (*ProviderInitResponse, error)
	// Verify queries the provider for the current status of a transaction.
	Verify(ctx context.Context, providerRef string) (*ProviderInitResponse, error)
	// Refund requests a refund for a completed transaction.
	Refund(ctx context.Context, providerRef string, amount float64) (*ProviderInitResponse, error)
}

// GatewayRegistry maps provider names to their Gateway implementations.
type GatewayRegistry map[Provider]Gateway

// ── MTN Mobile Money Adapter ──────────────────────────────────────────────────
// In production, replace the stub methods with actual MTN MoMo API calls.
// MTN MoMo API docs: https://momodeveloper.mtn.com/

type mtnMomoGateway struct {
	apiKey    string
	apiSecret string
	baseURL   string
	env       string // sandbox | production
}

func NewMTNMomoGateway(apiKey, apiSecret, baseURL, env string) Gateway {
	return &mtnMomoGateway{apiKey: apiKey, apiSecret: apiSecret, baseURL: baseURL, env: env}
}

func (g *mtnMomoGateway) Initiate(ctx context.Context, req *InitiatePaymentRequest) (*ProviderInitResponse, error) {
	if req.PhoneNumber == "" {
		return nil, fmt.Errorf("phone_number is required for MTN Mobile Money")
	}
	if req.Amount <= 0 {
		return nil, fmt.Errorf("amount must be greater than 0")
	}

	// ── PRODUCTION INTEGRATION POINT ──────────────────────────────────────────
	// Replace this block with actual MTN MoMo Collections API call:
	//
	// 1. POST /collection/token/ — get OAuth bearer token
	// 2. POST /collection/v1_0/requesttopay — initiate payment
	//    Headers: X-Reference-Id (UUID), X-Target-Environment, Ocp-Apim-Subscription-Key
	//    Body: { amount, currency, externalId, payer: { partyIdType: "MSISDN", partyId: phone }, payerMessage, payeeNote }
	// 3. Store the X-Reference-Id as provider_ref
	// ──────────────────────────────────────────────────────────────────────────

	// Sandbox stub: simulate async acceptance
	ref := fmt.Sprintf("MTN-%s-%04d", time.Now().Format("20060102150405"), rand.Intn(10000))
	return &ProviderInitResponse{
		ProviderRef:    ref,
		ProviderStatus: "PENDING",
		Message:        fmt.Sprintf("Payment request sent to %s. Awaiting customer approval.", req.PhoneNumber),
	}, nil
}

func (g *mtnMomoGateway) Verify(ctx context.Context, providerRef string) (*ProviderInitResponse, error) {
	// ── PRODUCTION INTEGRATION POINT ──────────────────────────────────────────
	// GET /collection/v1_0/requesttopay/{referenceId}
	// Map response status: SUCCESSFUL -> COMPLETED, FAILED -> FAILED, PENDING -> PROCESSING
	// ──────────────────────────────────────────────────────────────────────────

	// Sandbox stub: simulate successful completion
	return &ProviderInitResponse{
		ProviderRef:    providerRef,
		ProviderStatus: "SUCCESSFUL",
		Message:        "Transaction completed successfully",
	}, nil
}

func (g *mtnMomoGateway) Refund(ctx context.Context, providerRef string, amount float64) (*ProviderInitResponse, error) {
	// ── PRODUCTION INTEGRATION POINT ──────────────────────────────────────────
	// POST /disbursement/v1_0/refund
	// ──────────────────────────────────────────────────────────────────────────

	ref := fmt.Sprintf("MTN-REF-%s-%04d", time.Now().Format("20060102"), rand.Intn(10000))
	return &ProviderInitResponse{
		ProviderRef:    ref,
		ProviderStatus: "SUCCESSFUL",
		Message:        fmt.Sprintf("Refund of %.2f ZMW initiated for %s", amount, providerRef),
	}, nil
}

// ── Airtel Money Adapter ──────────────────────────────────────────────────────
// In production, replace the stub methods with actual Airtel Money API calls.
// Airtel Money API docs: https://developers.airtel.africa/

type airtelMoneyGateway struct {
	clientID     string
	clientSecret string
	baseURL      string
	env          string // sandbox | production
}

func NewAirtelMoneyGateway(clientID, clientSecret, baseURL, env string) Gateway {
	return &airtelMoneyGateway{clientID: clientID, clientSecret: clientSecret, baseURL: baseURL, env: env}
}

func (g *airtelMoneyGateway) Initiate(ctx context.Context, req *InitiatePaymentRequest) (*ProviderInitResponse, error) {
	if req.PhoneNumber == "" {
		return nil, fmt.Errorf("phone_number is required for Airtel Money")
	}
	if req.Amount <= 0 {
		return nil, fmt.Errorf("amount must be greater than 0")
	}

	// ── PRODUCTION INTEGRATION POINT ──────────────────────────────────────────
	// Replace this block with actual Airtel Money Collections API call:
	//
	// 1. POST /auth/oauth2/token — get access token
	//    Body: { client_id, client_secret, grant_type: "client_credentials" }
	// 2. POST /merchant/v2/payments/ — initiate payment
	//    Headers: Authorization: Bearer <token>, X-Country: ZM, X-Currency: ZMW
	//    Body: { reference, subscriber: { country, currency, msisdn }, transaction: { amount, country, currency, id } }
	// 3. Store the transaction.id as provider_ref
	// ──────────────────────────────────────────────────────────────────────────

	// Sandbox stub
	ref := fmt.Sprintf("ATL-%s-%04d", time.Now().Format("20060102150405"), rand.Intn(10000))
	return &ProviderInitResponse{
		ProviderRef:    ref,
		ProviderStatus: "DP",
		// DP = "Debit Pending" in Airtel terminology
		Message: fmt.Sprintf("Airtel Money request sent to %s. Awaiting PIN confirmation.", req.PhoneNumber),
	}, nil
}

func (g *airtelMoneyGateway) Verify(ctx context.Context, providerRef string) (*ProviderInitResponse, error) {
	// ── PRODUCTION INTEGRATION POINT ──────────────────────────────────────────
	// GET /standard/v1/payments/{id}
	// Map response status: TS -> COMPLETED, TF -> FAILED, DP -> PROCESSING
	// ──────────────────────────────────────────────────────────────────────────

	return &ProviderInitResponse{
		ProviderRef:    providerRef,
		ProviderStatus: "TS",
		// TS = "Transaction Successful" in Airtel terminology
		Message: "Transaction successful",
	}, nil
}

func (g *airtelMoneyGateway) Refund(ctx context.Context, providerRef string, amount float64) (*ProviderInitResponse, error) {
	// ── PRODUCTION INTEGRATION POINT ──────────────────────────────────────────
	// POST /standard/v1/payments/refund
	// ──────────────────────────────────────────────────────────────────────────

	ref := fmt.Sprintf("ATL-REF-%s-%04d", time.Now().Format("20060102"), rand.Intn(10000))
	return &ProviderInitResponse{
		ProviderRef:    ref,
		ProviderStatus: "TS",
		Message:        fmt.Sprintf("Refund of %.2f ZMW initiated for %s", amount, providerRef),
	}, nil
}

// ── Status Normaliser ─────────────────────────────────────────────────────────
// Maps provider-specific status strings to our internal TxStatus.

func NormaliseStatus(provider Provider, providerStatus string) TxStatus {
	s := strings.ToUpper(providerStatus)
	switch provider {
	case ProviderMTNMomo:
		switch s {
		case "SUCCESSFUL":
			return TxCompleted
		case "FAILED":
			return TxFailed
		case "PENDING":
			return TxPending
		default:
			return TxProcessing
		}
	case ProviderAirtel:
		switch s {
		case "TS": // Transaction Successful
			return TxCompleted
		case "TF": // Transaction Failed
			return TxFailed
		case "DP": // Debit Pending
			return TxProcessing
		default:
			return TxProcessing
		}
	default:
		return TxProcessing
	}
}
