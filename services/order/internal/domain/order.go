package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type Product struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	SKU            string    `json:"sku"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	PriceCents     int64     `json:"price_cents"`
	Currency       string    `json:"currency"`
	Stock          int       `json:"stock"`
	ImageObjectKey string    `json:"image_object_key,omitempty"`
	Active         bool      `json:"active"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (Product) TableName() string { return "orders.products" }

type Order struct {
	ID             uuid.UUID   `gorm:"type:uuid;primaryKey" json:"id"`
	UserID         uuid.UUID   `gorm:"type:uuid" json:"user_id"`
	Status         string      `json:"status"`
	AmountCents    int64       `json:"amount_cents"`
	Currency       string      `json:"currency"`
	IdempotencyKey string      `json:"-"`
	Items          []OrderItem `json:"items"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

func (Order) TableName() string { return "orders.orders" }

type OrderItem struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	OrderID        uuid.UUID `gorm:"type:uuid" json:"-"`
	ProductID      uuid.UUID `gorm:"type:uuid" json:"product_id"`
	SKU            string    `json:"sku"`
	Name           string    `json:"name"`
	UnitPriceCents int64     `json:"unit_price_cents"`
	Quantity       int       `json:"quantity"`
	LineTotalCents int64     `json:"line_total_cents"`
}

func (OrderItem) TableName() string { return "orders.order_items" }

type Outbox struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	EventType   string
	Payload     datatypes.JSON
	CreatedAt   time.Time
	PublishedAt *time.Time
}

func (Outbox) TableName() string { return "orders.outbox_events" }

type Inbox struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	ProcessedAt time.Time
}

func (Inbox) TableName() string { return "orders.inbox_events" }
