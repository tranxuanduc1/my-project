package application

import (
	"context"
	"fmt"

	"myproject/payment/internal/application/ports"
	"myproject/payment/internal/domain"

	"github.com/google/uuid"
)

type Payment = domain.Payment
type Outbox = domain.Outbox
type Inbox = domain.Inbox

type PaymentService struct {
	payments ports.PaymentRepository
	orders   ports.OrderVerifier
}

func NewPaymentService(payments ports.PaymentRepository, orders ports.OrderVerifier) *PaymentService {
	return &PaymentService{payments: payments, orders: orders}
}

func (s *PaymentService) List(ctx context.Context, userID *uuid.UUID) ([]domain.Payment, error) {
	return s.payments.ListPayments(ctx, userID)
}

func (s *PaymentService) Get(ctx context.Context, id string, userID *uuid.UUID) (domain.Payment, error) {
	return s.payments.GetPayment(ctx, id, userID)
}

func (s *PaymentService) Succeed(ctx context.Context, id string, userID *uuid.UUID) (domain.Payment, error) {
	return s.decide(ctx, id, userID, "succeeded", "")
}

func (s *PaymentService) Fail(ctx context.Context, id string, userID *uuid.UUID, reason string) (domain.Payment, error) {
	if reason == "" {
		reason = "mock payment declined"
	}
	return s.decide(ctx, id, userID, "failed", reason)
}

func (s *PaymentService) decide(ctx context.Context, id string, userID *uuid.UUID, status, reason string) (domain.Payment, error) {
	payment, err := s.payments.GetPayment(ctx, id, userID)
	if err != nil {
		return domain.Payment{}, err
	}
	if payment.Status == status {
		return payment, nil
	}
	if payment.Status != "pending" {
		return domain.Payment{}, ErrConflict
	}
	if err := s.orders.CheckOrder(ctx, payment); err != nil {
		return domain.Payment{}, fmt.Errorf("%w: %v", ErrReconcileOrder, err)
	}
	return s.payments.FinalizePayment(ctx, payment.ID, status, reason)
}
