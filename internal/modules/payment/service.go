package payment

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"

	"github.com/google/uuid"
)

// Service defines payment business logic.
type Service interface {
	ListMethods() []PaymentMethod
	Initiate(ctx context.Context, req InitiatePaymentRequest) (*PaymentTransaction, error)
	GetByID(ctx context.Context, id string) (*PaymentTransaction, error)
	Verify(ctx context.Context, id string) (*PaymentTransaction, error)
	VerifyByProviderRef(ctx context.Context, reference string) (*PaymentTransaction, error)
	HandleWebhook(ctx context.Context, payload WebhookPayload) (*PaymentTransaction, error)
	Refund(ctx context.Context, id string) (*PaymentTransaction, error)
	ListByReference(ctx context.Context, refType ReferenceType, refID string) ([]*PaymentTransaction, error)
	ListByVendor(ctx context.Context, vendorID string) ([]*PaymentTransaction, error)
}

type service struct {
	repo     Repository
	gateways GatewayRegistry
}

func NewService(repo Repository, gateways GatewayRegistry) Service {
	return &service{repo: repo, gateways: gateways}
}

func (s *service) ListMethods() []PaymentMethod {
	methods := []PaymentMethod{{Provider: ProviderCash, Label: "Cash", Enabled: true, Message: "Pay when you collect or receive your order."}}
	for _, definition := range []PaymentMethod{
		{Provider: ProviderMTNMomo, Label: "MTN MoMo", RequiresPhone: true, Message: "Coming soon"},
		{Provider: ProviderAirtel, Label: "Airtel Money", RequiresPhone: true, Message: "Coming soon"},
		{Provider: ProviderZamtel, Label: "Zamtel Money", RequiresPhone: true, Message: "Coming soon"},
	} {
		gateway, exists := s.gateways[definition.Provider]
		if available, ok := gateway.(availabilityAwareGateway); exists && ok && available.Available() {
			definition.Enabled = true
			definition.Message = "A payment prompt will be sent to your phone."
		}
		methods = append(methods, definition)
	}
	return methods
}

func (s *service) methodEnabled(provider Provider) bool {
	for _, method := range s.ListMethods() {
		if method.Provider == provider {
			return method.Enabled
		}
	}
	return false
}

func (s *service) Initiate(ctx context.Context, req InitiatePaymentRequest) (*PaymentTransaction, error) {
	// Validate provider
	provider := Provider(strings.ToUpper(req.Provider))
	if provider == "" {
		return nil, fmt.Errorf("provider is required")
	}
	if !s.methodEnabled(provider) {
		return nil, fmt.Errorf("payment method %s is not currently available", provider)
	}

	// Validate reference
	if req.ReferenceID == "" || req.ReferenceType == "" {
		return nil, fmt.Errorf("reference_type and reference_id are required")
	}
	if req.Amount <= 0 {
		return nil, fmt.Errorf("amount must be greater than 0")
	}

	currency := req.Currency
	if currency == "" {
		currency = "ZMW"
	}

	// Idempotency: return existing transaction if key already used
	if req.IdempotencyKey != "" {
		existing, err := s.repo.GetByIdempotencyKey(ctx, req.IdempotencyKey)
		if err == nil && existing != nil {
			return existing, nil
		}
	}

	// Build transaction record
	var vendorID *uuid.UUID
	if req.VendorID != "" {
		id, err := uuid.Parse(req.VendorID)
		if err != nil {
			return nil, fmt.Errorf("invalid vendor_id: %w", err)
		}
		vendorID = &id
	}

	refID, err := uuid.Parse(req.ReferenceID)
	if err != nil {
		return nil, fmt.Errorf("invalid reference_id: %w", err)
	}

	tx := &PaymentTransaction{
		ID:             uuid.New(),
		ReferenceType:  ReferenceType(strings.ToUpper(req.ReferenceType)),
		ReferenceID:    refID,
		VendorID:       vendorID,
		Provider:       provider,
		Status:         TxPending,
		Amount:         req.Amount,
		Currency:       currency,
		PhoneNumber:    req.PhoneNumber,
		Description:    req.Description,
		IdempotencyKey: req.IdempotencyKey,
	}

	// Cash is due at handoff. Recording the choice must not mark an unpaid online order as paid.
	if provider == ProviderCash {
		tx.Status = TxPending
		tx.ProviderStatus = "AWAITING_CASH"
		if err := s.repo.Create(ctx, tx); err != nil {
			return nil, err
		}
		return s.repo.GetByID(ctx, tx.ID.String())
	}

	// Persist as PENDING first (before gateway call to avoid lost records)
	if err := s.repo.Create(ctx, tx); err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, fmt.Errorf("duplicate payment request (idempotency key already used)")
		}
		return nil, err
	}

	// Call the gateway
	gw, ok := s.gateways[provider]
	if !ok {
		_ = s.repo.UpdateStatus(ctx, tx.ID.String(), TxFailed, "NO_GATEWAY", "no gateway registered for provider: "+string(provider))
		return nil, fmt.Errorf("no gateway registered for provider: %s", provider)
	}

	resp, err := gw.Initiate(ctx, &req)
	if err != nil {
		_ = s.repo.UpdateStatus(ctx, tx.ID.String(), TxFailed, "GATEWAY_ERROR", err.Error())
		return nil, fmt.Errorf("gateway initiation failed: %w", err)
	}

	// Update with provider reference
	_ = s.repo.UpdateProviderRef(ctx, tx.ID.String(), resp.ProviderRef, resp.ProviderStatus)
	_ = s.repo.UpdateStatus(ctx, tx.ID.String(), TxProcessing, resp.ProviderStatus, "")

	return s.repo.GetByID(ctx, tx.ID.String())
}

func (s *service) GetByID(ctx context.Context, id string) (*PaymentTransaction, error) {
	tx, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("payment transaction not found: %w", err)
	}
	return tx, nil
}

func (s *service) Verify(ctx context.Context, id string) (*PaymentTransaction, error) {
	tx, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("payment transaction not found: %w", err)
	}

	if tx.Status == TxCompleted || tx.Status == TxFailed || tx.Status == TxRefunded {
		return tx, nil // already in terminal state
	}

	gw, ok := s.gateways[tx.Provider]
	if !ok {
		return nil, fmt.Errorf("no gateway registered for provider: %s", tx.Provider)
	}

	resp, err := gw.Verify(ctx, tx.ProviderRef)
	if err != nil {
		_ = s.repo.IncrementRetry(ctx, id, err.Error())
		return nil, fmt.Errorf("gateway verification failed: %w", err)
	}

	internalStatus := NormaliseStatus(tx.Provider, resp.ProviderStatus)
	_ = s.repo.UpdateStatus(ctx, id, internalStatus, resp.ProviderStatus, "")

	return s.repo.GetByID(ctx, id)
}

func (s *service) VerifyByProviderRef(ctx context.Context, reference string) (*PaymentTransaction, error) {
	for _, provider := range []Provider{ProviderMTNMomo, ProviderAirtel, ProviderZamtel} {
		tx, err := s.repo.GetByProviderRef(ctx, provider, reference)
		if err == nil {
			return s.Verify(ctx, tx.ID.String())
		}
		if err != sql.ErrNoRows {
			return nil, err
		}
	}
	return nil, sql.ErrNoRows
}

func (s *service) HandleWebhook(ctx context.Context, payload WebhookPayload) (*PaymentTransaction, error) {
	provider := Provider(strings.ToUpper(payload.Provider))
	if provider != ProviderMTNMomo && provider != ProviderAirtel {
		return nil, fmt.Errorf("unsupported webhook provider: %s", provider)
	}
	if payload.ExternalRef == "" || payload.Status == "" {
		return nil, fmt.Errorf("external_ref and status are required")
	}

	// Find the transaction by provider reference.
	tx, err := s.repo.GetByProviderRef(ctx, provider, payload.ExternalRef)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no transaction found for provider_ref: %s", payload.ExternalRef)
		}
		return nil, err
	}
	if payload.Amount > 0 && math.Abs(tx.Amount-payload.Amount) > 0.005 {
		return nil, fmt.Errorf("webhook amount does not match payment transaction")
	}
	if payload.Currency != "" && !strings.EqualFold(tx.Currency, payload.Currency) {
		return nil, fmt.Errorf("webhook currency does not match payment transaction")
	}

	// Persist received data for auditability even when the status is already terminal.
	if err := s.repo.RecordWebhook(ctx, tx.ID.String(), payload.RawPayload); err != nil {
		return nil, err
	}
	if tx.Status == TxCompleted || tx.Status == TxFailed || tx.Status == TxRefunded || tx.Status == TxCancelled {
		return tx, nil
	}

	internalStatus := NormaliseStatus(provider, payload.Status)
	if err := s.repo.UpdateStatus(ctx, tx.ID.String(), internalStatus, payload.Status, ""); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, tx.ID.String())
}

func (s *service) Refund(ctx context.Context, id string) (*PaymentTransaction, error) {
	tx, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("payment transaction not found: %w", err)
	}

	if tx.Status != TxCompleted {
		return nil, fmt.Errorf("only COMPLETED transactions can be refunded (current status: %s)", tx.Status)
	}

	// CASH refunds are handled manually
	if tx.Provider == ProviderCash {
		_ = s.repo.UpdateStatus(ctx, id, TxRefunded, "REFUNDED", "")
		return s.repo.GetByID(ctx, id)
	}

	gw, ok := s.gateways[tx.Provider]
	if !ok {
		return nil, fmt.Errorf("no gateway registered for provider: %s", tx.Provider)
	}

	resp, err := gw.Refund(ctx, tx.ProviderRef, tx.Amount)
	if err != nil {
		return nil, fmt.Errorf("gateway refund failed: %w", err)
	}

	_ = s.repo.UpdateStatus(ctx, id, TxRefunded, resp.ProviderStatus, "")

	return s.repo.GetByID(ctx, id)
}

func (s *service) ListByReference(ctx context.Context, refType ReferenceType, refID string) ([]*PaymentTransaction, error) {
	return s.repo.ListByReference(ctx, refType, refID)
}

func (s *service) ListByVendor(ctx context.Context, vendorID string) ([]*PaymentTransaction, error) {
	return s.repo.ListByVendor(ctx, vendorID)
}
