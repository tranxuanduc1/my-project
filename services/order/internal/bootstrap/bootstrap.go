package bootstrap

import (
	"context"
	"net/http"
	"time"

	"myproject/order/internal/application"
	"myproject/order/internal/infrastructure/broker"
	"myproject/order/internal/infrastructure/cache"
	"myproject/order/internal/infrastructure/config"
	"myproject/order/internal/infrastructure/postgres"
	"myproject/order/internal/infrastructure/search"
	"myproject/order/internal/infrastructure/storage"
	"myproject/order/internal/transport/httpapi"

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
	objectStorage, err := storage.NewMinIOStorage(
		context.Background(),
		env("MINIO_ENDPOINT", "localhost:9000"),
		env("MINIO_PUBLIC_ENDPOINT", "localhost:9000"),
		env("MINIO_ACCESS_KEY", "minioadmin"),
		env("MINIO_SECRET_KEY", "minioadmin"),
		"products",
	)
	if err != nil {
		return err
	}
	httpClient := &http.Client{Timeout: 3 * time.Second}
	productService := application.NewProductService(
		store,
		cache.NewRedisProductCache(env("REDIS_ADDR", "localhost:6379"), time.Minute),
		search.NewMeiliSearch(env("MEILISEARCH_URL", "http://localhost:7700"), env("MEILISEARCH_API_KEY", "local-development-master-key"), httpClient),
		objectStorage,
	)
	orderService := application.NewOrderService(store)
	paymentEvents := application.NewPaymentEventService(store)
	rabbit := broker.NewRabbitMQ(env("RABBITMQ_URL", "amqp://app:app@localhost:5672/"))
	go rabbit.StartOutboxPublisher(context.Background(), store)
	go rabbit.StartPaymentConsumer(context.Background(), paymentEvents)
	r := gin.Default()
	httpapi.NewHandler(store, productService, orderService, []byte(env("JWT_SECRET", "dev-secret")), env("INTERNAL_API_KEY", "internal-key")).RegisterRoutes(r)
	return r.Run(":" + env("PORT", "8082"))
}
