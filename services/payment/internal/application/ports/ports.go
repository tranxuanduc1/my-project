package ports

import (
	"context"
	"time"

	"myproject/payment/internal/domain"

	"github.com/google/uuid"
)

type HealthChecker interface {
	Ping(ctx context.Context) error
}

type PaymentRepository interface {
	ListPayments(ctx context.Context, userID *uuid.UUID) ([]domain.Payment, error)
	GetPayment(ctx context.Context, id string, userID *uuid.UUID) (domain.Payment, error)
	FinalizePayment(ctx context.Context, id uuid.UUID, status, reason string) (domain.Payment, error)
	CreateFromOrder(ctx context.Context, eventID, orderID, userID uuid.UUID, amount int64, currency string) error
}

type OrderVerifier interface {
	CheckOrder(ctx context.Context, payment domain.Payment) error
}

type OrderCreatedHandler interface {
	CreateFromOrder(ctx context.Context, eventID, orderID, userID uuid.UUID, amount int64, currency string) error
}

type OutboxStore interface {
	PendingOutbox(ctx context.Context, limit int) ([]domain.Outbox, error)
	OutboxStats(ctx context.Context) (pending int64, oldestAge time.Duration, err error)
	MarkOutboxPublished(ctx context.Context, id uuid.UUID) error
}
