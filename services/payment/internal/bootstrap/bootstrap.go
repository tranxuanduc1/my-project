package bootstrap

import (
	"context"
	"net/http"
	"time"

	"myproject/payment/internal/application"
	"myproject/payment/internal/infrastructure/broker"
	"myproject/payment/internal/infrastructure/config"
	"myproject/payment/internal/infrastructure/orderclient"
	"myproject/payment/internal/infrastructure/postgres"
	"myproject/payment/internal/transport/httpapi"

	"github.com/gin-gonic/gin"
)

func env(k, d string) string { return config.Env(k, d) }

func Migrate() error {
	dsn := env("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/commerce?sslmode=disable")
	return postgres.Migrate(env("MIGRATIONS_PATH", "file://migrations"), dsn)
}

func Run() error {
	if err := Migrate(); err != nil {
		return err
	}
	store, err := postgres.Open(env("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/commerce?sslmode=disable"))
	if err != nil {
		return err
	}
	httpClient := &http.Client{Timeout: 3 * time.Second}
	payments := application.NewPaymentService(store, orderclient.New(env("ORDER_URL", "http://localhost:8082"), env("INTERNAL_API_KEY", "internal-key"), httpClient))
	rabbit := broker.NewRabbitMQ(env("RABBITMQ_URL", "amqp://app:app@localhost:5672/"))
	go rabbit.StartOrderConsumer(context.Background(), application.NewOrderCreatedService(store))
	go rabbit.StartOutboxPublisher(context.Background(), store)
	r := gin.Default()
	httpapi.NewHandler(store, payments, []byte(env("JWT_SECRET", "dev-secret"))).RegisterRoutes(r)
	return r.Run(":" + env("PORT", "8083"))
}
