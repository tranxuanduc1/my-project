package application

import (
	"context"
	"strings"

	"myproject/order/internal/application/apperrors"
	"myproject/order/internal/application/ports"
	"myproject/order/internal/domain"

	"github.com/google/uuid"
)

type OrderService struct {
	orders ports.OrderRepository
}

func NewOrderService(orders ports.OrderRepository) *OrderService {
	return &OrderService{orders: orders}
}

func (s *OrderService) Create(ctx context.Context, userID uuid.UUID, key string, items []ports.OrderItemInput) (domain.Order, bool, error) {
	if strings.TrimSpace(key) == "" || len(items) == 0 {
		return domain.Order{}, false, apperrors.ErrInvalidInput
	}
	return s.orders.CreateOrder(ctx, userID, strings.TrimSpace(key), items)
}

func (s *OrderService) List(ctx context.Context, userID *uuid.UUID, limit, offset int) ([]domain.Order, error) {
	return s.orders.ListOrders(ctx, userID, limit, offset)
}

func (s *OrderService) Get(ctx context.Context, id string, userID *uuid.UUID) (domain.Order, error) {
	return s.orders.GetOrder(ctx, id, userID)
}

func (s *OrderService) Cancel(ctx context.Context, id string, userID *uuid.UUID) error {
	return s.orders.CancelOrder(ctx, id, userID)
}
