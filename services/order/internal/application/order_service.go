package application

import (
	"context"
	"log/slog"
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
	key = strings.TrimSpace(key)
	if key == "" || len(items) == 0 {
		slog.WarnContext(ctx, "order create rejected", "user_id", userID, "item_count", len(items), "reason", "invalid input")
		return domain.Order{}, false, apperrors.ErrInvalidInput
	}
	order, existing, err := s.orders.CreateOrder(ctx, userID, key, items)
	if err != nil {
		slog.ErrorContext(ctx, "order create failed", "user_id", userID, "item_count", len(items), "error", err)
		return domain.Order{}, false, err
	}
	slog.InfoContext(ctx, "order create completed", "order_id", order.ID, "user_id", userID, "item_count", len(items), "amount_cents", order.AmountCents, "currency", order.Currency, "existing", existing)
	return order, existing, nil
}

func (s *OrderService) List(ctx context.Context, userID *uuid.UUID, limit, offset int) ([]domain.Order, error) {
	orders, err := s.orders.ListOrders(ctx, userID, limit, offset)
	if err != nil {
		slog.ErrorContext(ctx, "order list failed", "scoped", userID != nil, "limit", limit, "offset", offset, "error", err)
		return nil, err
	}
	slog.DebugContext(ctx, "order list completed", "scoped", userID != nil, "limit", limit, "offset", offset, "count", len(orders))
	return orders, nil
}

func (s *OrderService) Get(ctx context.Context, id string, userID *uuid.UUID) (domain.Order, error) {
	order, err := s.orders.GetOrder(ctx, id, userID)
	if err != nil {
		slog.WarnContext(ctx, "order get failed", "order_id", id, "scoped", userID != nil, "error", err)
		return domain.Order{}, err
	}
	slog.DebugContext(ctx, "order get completed", "order_id", order.ID, "status", order.Status, "scoped", userID != nil)
	return order, nil
}

func (s *OrderService) Cancel(ctx context.Context, id string, userID *uuid.UUID) error {
	if err := s.orders.CancelOrder(ctx, id, userID); err != nil {
		slog.WarnContext(ctx, "order cancel failed", "order_id", id, "scoped", userID != nil, "error", err)
		return err
	}
	slog.InfoContext(ctx, "order cancelled", "order_id", id, "scoped", userID != nil)
	return nil
}
