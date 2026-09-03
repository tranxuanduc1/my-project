package application

import (
	"context"
	"log/slog"

	"myproject/order/internal/application/ports"

	"github.com/google/uuid"
)

type PaymentEventService struct {
	orders ports.OrderRepository
}

func NewPaymentEventService(orders ports.OrderRepository) *PaymentEventService {
	return &PaymentEventService{orders: orders}
}

func (s *PaymentEventService) ApplyPayment(ctx context.Context, eventID, orderID uuid.UUID, eventType string) error {
	if err := s.orders.ApplyPayment(ctx, eventID, orderID, eventType); err != nil {
		slog.ErrorContext(ctx, "payment event apply failed", "event_id", eventID, "order_id", orderID, "event_type", eventType, "error", err)
		return err
	}
	slog.InfoContext(ctx, "payment event applied", "event_id", eventID, "order_id", orderID, "event_type", eventType)
	return nil
}
