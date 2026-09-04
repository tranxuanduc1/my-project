package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"myproject/order/internal/application/apperrors"
	"myproject/order/internal/application/ports"
	"myproject/order/internal/domain"

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
	m, err := migrate.New(path, dsn+"&x-migrations-table=orders_schema_migrations")
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

func (s *Store) ListProducts(ctx context.Context, query string, limit, offset int) ([]domain.Product, error) {
	var products []domain.Product
	db := s.db.WithContext(ctx).Where("active = true")
	if query = strings.TrimSpace(query); query != "" {
		db = db.Where("name ILIKE ? OR sku ILIKE ?", "%"+query+"%", "%"+query+"%")
	}
	err := db.Limit(limit).Offset(offset).Order("created_at desc").Find(&products).Error
	return products, err
}

func (s *Store) GetProduct(ctx context.Context, id string) (domain.Product, error) {
	var product domain.Product
	err := s.db.WithContext(ctx).First(&product, "id = ? AND active = true", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Product{}, apperrors.ErrNotFound
	}
	return product, err
}

func (s *Store) CreateProduct(ctx context.Context, product domain.Product) error {
	if err := s.db.WithContext(ctx).Create(&product).Error; err != nil {
		return apperrors.ErrConflict
	}
	return nil
}

func (s *Store) UpdateProduct(ctx context.Context, id string, in domain.Product) (domain.Product, error) {
	var product domain.Product
	if err := s.db.WithContext(ctx).First(&product, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Product{}, apperrors.ErrNotFound
		}
		return domain.Product{}, err
	}
	product.SKU = in.SKU
	product.Name = in.Name
	product.Description = in.Description
	product.PriceCents = in.PriceCents
	product.Currency = in.Currency
	product.Stock = in.Stock
	product.Active = in.Active
	if err := s.db.WithContext(ctx).Save(&product).Error; err != nil {
		return domain.Product{}, apperrors.ErrConflict
	}
	return product, nil
}

func (s *Store) DeleteProduct(ctx context.Context, id string) error {
	res := s.db.WithContext(ctx).Model(&domain.Product{}).Where("id = ?", id).Update("active", false)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (s *Store) SetProductImage(ctx context.Context, id, objectKey string) error {
	res := s.db.WithContext(ctx).Model(&domain.Product{}).Where("id = ?", id).Update("image_object_key", objectKey)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (s *Store) CreateOrder(ctx context.Context, userID uuid.UUID, idempotencyKey string, items []ports.OrderItemInput) (domain.Order, bool, error) {
	var existing domain.Order
	if err := s.db.WithContext(ctx).Preload("Items").Where("user_id = ? AND idempotency_key = ?", userID, idempotencyKey).First(&existing).Error; err == nil {
		return existing, true, nil
	}
	var result domain.Order
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		order := domain.Order{ID: uuid.New(), UserID: userID, Status: "pending_payment", IdempotencyKey: idempotencyKey}
		for _, item := range items {
			if item.Quantity < 1 {
				return errors.New("quantity must be positive")
			}
			var product domain.Product
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&product, "id = ? AND active = true", item.ProductID).Error; err != nil {
				return errors.New("product not found")
			}
			if product.Stock < item.Quantity {
				return fmt.Errorf("insufficient stock for %s", product.SKU)
			}
			if order.Currency == "" {
				order.Currency = product.Currency
			} else if order.Currency != product.Currency {
				return errors.New("mixed currencies are not supported")
			}
			line := product.PriceCents * int64(item.Quantity)
			order.AmountCents += line
			order.Items = append(order.Items, domain.OrderItem{ID: uuid.New(), OrderID: order.ID, ProductID: product.ID, SKU: product.SKU, Name: product.Name, UnitPriceCents: product.PriceCents, Quantity: item.Quantity, LineTotalCents: line})
			if err := tx.Model(&product).Update("stock", gorm.Expr("stock - ?", item.Quantity)).Error; err != nil {
				return err
			}
		}
		if err := tx.Omit("Items").Create(&order).Error; err != nil {
			return err
		}
		if err := tx.Create(&order.Items).Error; err != nil {
			return err
		}
		eventID := uuid.New()
		payload, _ := json.Marshal(map[string]any{"event_id": eventID, "event_type": "order.created", "version": 1, "occurred_at": time.Now().UTC(), "order_id": order.ID, "user_id": order.UserID, "amount_cents": order.AmountCents, "currency": order.Currency})
		if err := tx.Create(&domain.Outbox{ID: eventID, EventType: "order.created", Payload: datatypes.JSON(payload), Headers: propagationHeaders(ctx)}).Error; err != nil {
			return err
		}
		result = order
		return nil
	})
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			if e := s.db.WithContext(ctx).Preload("Items").Where("user_id = ? AND idempotency_key = ?", userID, idempotencyKey).First(&existing).Error; e == nil {
				return existing, true, nil
			}
		}
		return domain.Order{}, false, err
	}
	return result, false, nil
}

func (s *Store) ListOrders(ctx context.Context, userID *uuid.UUID, limit, offset int) ([]domain.Order, error) {
	var orders []domain.Order
	db := s.db.WithContext(ctx).Preload("Items")
	if userID != nil {
		db = db.Where("user_id = ?", *userID)
	}
	err := db.Limit(limit).Offset(offset).Order("created_at desc").Find(&orders).Error
	return orders, err
}

func (s *Store) GetOrder(ctx context.Context, id string, userID *uuid.UUID) (domain.Order, error) {
	var order domain.Order
	db := s.db.WithContext(ctx).Preload("Items").Where("id = ?", id)
	if userID != nil {
		db = db.Where("user_id = ?", *userID)
	}
	err := db.First(&order).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Order{}, apperrors.ErrNotFound
	}
	return order, err
}

func (s *Store) CancelOrder(ctx context.Context, id string, userID *uuid.UUID) error {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var order domain.Order
		db := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Items").Where("id = ?", id)
		if userID != nil {
			db = db.Where("user_id = ?", *userID)
		}
		if err := db.First(&order).Error; err != nil {
			return err
		}
		if order.Status != "pending_payment" {
			return errors.New("order cannot be cancelled")
		}
		for _, item := range order.Items {
			if err := tx.Model(&domain.Product{}).Where("id = ?", item.ProductID).Update("stock", gorm.Expr("stock + ?", item.Quantity)).Error; err != nil {
				return err
			}
		}
		return tx.Model(&order).Update("status", "cancelled").Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return apperrors.ErrNotFound
	}
	return err
}

func (s *Store) ApplyPayment(ctx context.Context, eventID, orderID uuid.UUID, eventType string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		tx.Model(&domain.Inbox{}).Where("id = ?", eventID).Count(&count)
		if count > 0 {
			return nil
		}
		var order domain.Order
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Items").First(&order, "id = ?", orderID).Error; err != nil {
			return err
		}
		if order.Status != "pending_payment" {
			return tx.Create(&domain.Inbox{ID: eventID}).Error
		}
		status := "confirmed"
		if eventType == "payment.failed" {
			status = "payment_failed"
			for _, item := range order.Items {
				if err := tx.Model(&domain.Product{}).Where("id = ?", item.ProductID).Update("stock", gorm.Expr("stock + ?", item.Quantity)).Error; err != nil {
					return err
				}
			}
		}
		if err := tx.Model(&order).Update("status", status).Error; err != nil {
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
