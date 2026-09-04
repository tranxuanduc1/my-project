package bootstrap

import (
	"context"
	"errors"
	"log/slog"
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
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

func env(k, d string) string { return config.Env(k, d) }

func Migrate() error {
	dsn := env("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/orders?sslmode=disable")
	return postgres.Migrate(env("MIGRATIONS_PATH", "file://migrations"), dsn)
}

func Run(ctx context.Context) error {
	if err := Migrate(); err != nil {
		return err
	}
	slog.Info("order service starting", "port", env("PORT", "8082"))
	store, err := postgres.Open(env("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/orders?sslmode=disable"))
	if err != nil {
		return err
	}
	objectStorage, err := storage.NewMinIOStorage(
		ctx,
		env("MINIO_ENDPOINT", "localhost:9000"),
		env("MINIO_PUBLIC_ENDPOINT", "localhost:9000"),
		env("MINIO_ACCESS_KEY", "minioadmin"),
		env("MINIO_SECRET_KEY", "minioadmin"),
		"products",
	)
	if err != nil {
		return err
	}
	productService := application.NewProductService(
		store,
		cache.NewRedisProductCache(env("REDIS_ADDR", "localhost:6379"), time.Minute),
		search.NewMeiliSearch(env("MEILISEARCH_URL", "http://localhost:7700"), env("MEILISEARCH_API_KEY", "local-development-master-key"), search.NewHTTPClient(3*time.Second)),
		objectStorage,
	)
	orderService := application.NewOrderService(store)
	paymentEvents := application.NewPaymentEventService(store)
	rabbit := broker.NewRabbitMQ(env("RABBITMQ_URL", "amqp://app:app@localhost:5672/"))
	workerCtx, stopWorkers := context.WithCancel(ctx)
	defer stopWorkers()
	go rabbit.StartOutboxPublisher(workerCtx, store)
	go rabbit.StartPaymentConsumer(workerCtx, paymentEvents)
	r := gin.New()
	r.ContextWithFallback = true
	r.Use(
		otelgin.Middleware("order"),
		httpapi.MetricsMiddleware(),
		httpapi.AccessLogMiddleware(),
		httpapi.RecoveryMiddleware(),
	)
	httpapi.NewHandler(store, productService, orderService, []byte(env("JWT_SECRET", "dev-secret")), env("INTERNAL_API_KEY", "internal-key")).RegisterRoutes(r)
	server := &http.Server{
		Addr:    ":" + env("PORT", "8082"),
		Handler: r,
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()
	slog.Info("order service started", "addr", server.Addr)

	select {
	case <-ctx.Done():
		slog.Info("order service shutting down", "reason", ctx.Err().Error())
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		stopWorkers()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		slog.Info("order service stopped")
		return nil
	case err := <-errCh:
		stopWorkers()
		if errors.Is(err, http.ErrServerClosed) {
			slog.Info("order service stopped")
			return nil
		}
		return err
	}
}
