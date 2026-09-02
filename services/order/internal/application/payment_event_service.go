package application

import (
	"context"

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
	return s.orders.ApplyPayment(ctx, eventID, orderID, eventType)
}
