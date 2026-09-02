package application

import (
	"context"

	"myproject/payment/internal/application/ports"

	"github.com/google/uuid"
)

type OrderCreatedService struct {
	payments ports.PaymentRepository
}

func NewOrderCreatedService(payments ports.PaymentRepository) *OrderCreatedService {
	return &OrderCreatedService{payments: payments}
}

func (s *OrderCreatedService) CreateFromOrder(ctx context.Context, eventID, orderID, userID uuid.UUID, amount int64, currency string) error {
	return s.payments.CreateFromOrder(ctx, eventID, orderID, userID, amount, currency)
}
