package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type Payment struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	OrderID       uuid.UUID `gorm:"type:uuid" json:"order_id"`
	UserID        uuid.UUID `gorm:"type:uuid" json:"user_id"`
	AmountCents   int64     `json:"amount_cents"`
	Currency      string    `json:"currency"`
	Provider      string    `json:"provider"`
	Status        string    `json:"status"`
	FailureReason string    `json:"failure_reason,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (Payment) TableName() string { return "payments" }

type Outbox struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	EventType   string
	Payload     datatypes.JSON
	CreatedAt   time.Time
	PublishedAt *time.Time
}

func (Outbox) TableName() string { return "outbox_events" }

type Inbox struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	ProcessedAt time.Time
}

func (Inbox) TableName() string { return "inbox_events" }
