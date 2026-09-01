package ports

import (
	"context"
	"time"

	"myproject/order/internal/domain"

	"github.com/google/uuid"
)

type HealthChecker interface {
	Ping(ctx context.Context) error
}

type ProductRepository interface {
	ListProducts(ctx context.Context, query string, limit, offset int) ([]domain.Product, error)
	GetProduct(ctx context.Context, id string) (domain.Product, error)
	CreateProduct(ctx context.Context, product domain.Product) error
	UpdateProduct(ctx context.Context, id string, product domain.Product) (domain.Product, error)
	DeleteProduct(ctx context.Context, id string) error
	SetProductImage(ctx context.Context, id, objectKey string) error
}

type ProductCache interface {
	GetProduct(ctx context.Context, id string) (domain.Product, bool)
	SetProduct(ctx context.Context, product domain.Product)
	DeleteProduct(ctx context.Context, id string)
}

type ProductSearch interface {
	SearchProducts(ctx context.Context, query string, limit, offset int) ([]domain.Product, error)
	IndexProduct(ctx context.Context, product domain.Product) error
	DeleteProduct(ctx context.Context, id uuid.UUID) error
}

type ProductStorage interface {
	PresignProductImage(ctx context.Context, productID uuid.UUID, ttl time.Duration) (uploadURL, objectKey string, err error)
}

type OrderItemInput struct {
	ProductID uuid.UUID `json:"product_id"`
	Quantity  int       `json:"quantity"`
}

type OrderRepository interface {
	CreateOrder(ctx context.Context, userID uuid.UUID, idempotencyKey string, items []OrderItemInput) (domain.Order, bool, error)
	ListOrders(ctx context.Context, userID *uuid.UUID, limit, offset int) ([]domain.Order, error)
	GetOrder(ctx context.Context, id string, userID *uuid.UUID) (domain.Order, error)
	CancelOrder(ctx context.Context, id string, userID *uuid.UUID) error
	ApplyPayment(ctx context.Context, eventID, orderID uuid.UUID, eventType string) error
}

type OutboxStore interface {
	PendingOutbox(ctx context.Context, limit int) ([]domain.Outbox, error)
	MarkOutboxPublished(ctx context.Context, id uuid.UUID) error
}

type PaymentEventHandler interface {
	ApplyPayment(ctx context.Context, eventID, orderID uuid.UUID, eventType string) error
}
