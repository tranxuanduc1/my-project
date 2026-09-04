package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"myproject/payment/internal/application/apperrors"
	"myproject/payment/internal/domain"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	otelgorm "gorm.io/plugin/opentelemetry/tracing"
)

type Store struct{ db *gorm.DB }

func Open(dsn string) (*Store, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := db.Use(otelgorm.NewPlugin(
		otelgorm.WithDBSystem("postgresql"),
		otelgorm.WithoutQueryVariables(),
	)); err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func Migrate(path, dsn string) error {
	m, err := migrate.New(path, dsn+"&x-migrations-table=payments_schema_migrations")
	if err != nil {
		return err
	}
	defer m.Close()
	err = m.Up()
	if errors.Is(err, migrate.ErrNoChange) {
		return nil
	}
	return err
}

func (s *Store) Ping(ctx context.Context) error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

func (s *Store) ListPayments(ctx context.Context, userID *uuid.UUID) ([]domain.Payment, error) {
	var payments []domain.Payment
	db := s.db.WithContext(ctx)
	if userID != nil {
		db = db.Where("user_id = ?", *userID)
	}
	err := db.Order("created_at desc").Find(&payments).Error
	return payments, err
}

func (s *Store) GetPayment(ctx context.Context, id string, userID *uuid.UUID) (domain.Payment, error) {
	var payment domain.Payment
	db := s.db.WithContext(ctx).Where("id = ?", id)
	if userID != nil {
		db = db.Where("user_id = ?", *userID)
	}
	err := db.First(&payment).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Payment{}, apperrors.ErrNotFound
	}
	return payment, err
}

func (s *Store) FinalizePayment(ctx context.Context, id uuid.UUID, status, reason string) (domain.Payment, error) {
	var payment domain.Payment
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&payment, "id = ?", id).Error; err != nil {
			return err
		}
		if payment.Status == status {
			return nil
		}
		if payment.Status != "pending" {
			return apperrors.ErrConflict
		}
		payment.Status = status
		payment.FailureReason = reason
		if err := tx.Save(&payment).Error; err != nil {
			return err
		}
		eventType := "payment." + status
		eventID := uuid.New()
		body, _ := json.Marshal(map[string]any{"event_id": eventID, "event_type": eventType, "version": 1, "occurred_at": time.Now().UTC(), "payment_id": payment.ID, "order_id": payment.OrderID, "user_id": payment.UserID, "failure_reason": reason})
		return tx.Create(&domain.Outbox{ID: eventID, EventType: eventType, Payload: datatypes.JSON(body), Headers: propagationHeaders(ctx)}).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Payment{}, apperrors.ErrNotFound
	}
	return payment, err
}

func (s *Store) CreateFromOrder(ctx context.Context, eventID, orderID, userID uuid.UUID, amount int64, currency string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		tx.Model(&domain.Inbox{}).Where("id = ?", eventID).Count(&count)
		if count > 0 {
			return nil
		}
		payment := domain.Payment{ID: uuid.New(), OrderID: orderID, UserID: userID, AmountCents: amount, Currency: currency, Provider: "mock", Status: "pending"}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&payment).Error; err != nil {
			return err
		}
		return tx.Create(&domain.Inbox{ID: eventID}).Error
	})
}

func (s *Store) PendingOutbox(ctx context.Context, limit int) ([]domain.Outbox, error) {
	var events []domain.Outbox
	err := s.db.WithContext(ctx).Where("published_at IS NULL").Order("created_at").Limit(limit).Find(&events).Error
	return events, err
}

func (s *Store) OutboxStats(ctx context.Context) (int64, time.Duration, error) {
	var pending int64
	if err := s.db.WithContext(ctx).Model(&domain.Outbox{}).Where("published_at IS NULL").Count(&pending).Error; err != nil {
		return 0, 0, err
	}
	if pending == 0 {
		return 0, 0, nil
	}
	var oldest domain.Outbox
	if err := s.db.WithContext(ctx).Where("published_at IS NULL").Order("created_at").First(&oldest).Error; err != nil {
		return 0, 0, err
	}
	return pending, time.Since(oldest.CreatedAt), nil
}

func (s *Store) MarkOutboxPublished(ctx context.Context, id uuid.UUID) error {
	return s.db.WithContext(ctx).Model(&domain.Outbox{}).Where("id = ?", id).Update("published_at", time.Now()).Error
}

func propagationHeaders(ctx context.Context) datatypes.JSONMap {
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	headers := datatypes.JSONMap{}
	for key, value := range carrier {
		if value != "" {
			headers[key] = value
		}
	}
	return headers
}
