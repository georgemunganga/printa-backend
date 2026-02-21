package pos

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Service defines POS business logic.
type Service interface {
	RecordPayment(ctx context.Context, req CreateTransactionRequest) (*POSTransaction, error)
	GetTransaction(ctx context.Context, id string) (*POSTransaction, error)
	GetTransactionByOrder(ctx context.Context, orderID string) (*POSTransaction, error)
	ListStoreTransactions(ctx context.Context, storeID string) ([]*POSTransaction, error)
	RefundTransaction(ctx context.Context, id string, req RefundRequest) (*POSTransaction, error)
}

type service struct{ repo Repository }

func NewService(repo Repository) Service { return &service{repo: repo} }

func (s *service) RecordPayment(ctx context.Context, req CreateTransactionRequest) (*POSTransaction, error) {
	if req.OrderID == "" {
		return nil, fmt.Errorf("order_id is required")
	}
	if req.StoreID == "" {
		return nil, fmt.Errorf("store_id is required")
	}
	if req.Amount <= 0 {
		return nil, fmt.Errorf("amount must be greater than zero")
	}
	if req.PaymentMethod == "" {
		return nil, fmt.Errorf("payment_method is required")
	}

	method := PaymentMethod(strings.ToUpper(req.PaymentMethod))
	switch method {
	case PaymentCash, PaymentCard, PaymentMobileMoney, PaymentVoucher:
		// valid
	default:
		return nil, fmt.Errorf("invalid payment_method: %s (allowed: CASH, CARD, MOBILE_MONEY, VOUCHER)", req.PaymentMethod)
	}

	tx := &POSTransaction{
		ID:            uuid.New(),
		OrderID:       uuid.MustParse(req.OrderID),
		StoreID:       uuid.MustParse(req.StoreID),
		Amount:        req.Amount,
		Currency:      "ZMW",
		PaymentMethod: method,
		Reference:     req.Reference,
		Status:        TxCompleted,
		ChangeGiven:   req.ChangeGiven,
		Notes:         req.Notes,
	}

	if req.CashierID != "" {
		uid, err := uuid.Parse(req.CashierID)
		if err != nil {
			return nil, fmt.Errorf("invalid cashier_id: %w", err)
		}
		tx.CashierID = &uid
	}

	// For cash payments, validate change calculation
	if method == PaymentCash && req.ChangeGiven < 0 {
		return nil, fmt.Errorf("change_given cannot be negative")
	}

	if err := s.repo.Create(ctx, tx); err != nil {
		return nil, err
	}
	return tx, nil
}

func (s *service) GetTransaction(ctx context.Context, id string) (*POSTransaction, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *service) GetTransactionByOrder(ctx context.Context, orderID string) (*POSTransaction, error) {
	return s.repo.GetByOrderID(ctx, orderID)
}

func (s *service) ListStoreTransactions(ctx context.Context, storeID string) ([]*POSTransaction, error) {
	return s.repo.ListByStore(ctx, storeID)
}

func (s *service) RefundTransaction(ctx context.Context, id string, req RefundRequest) (*POSTransaction, error) {
	tx, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("transaction not found: %w", err)
	}
	if tx.Status != TxCompleted {
		return nil, fmt.Errorf("only COMPLETED transactions can be refunded, current status: %s", tx.Status)
	}
	if err := s.repo.UpdateStatus(ctx, id, TxRefunded); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}
