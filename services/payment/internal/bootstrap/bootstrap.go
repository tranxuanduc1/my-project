package bootstrap

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"myproject/payment/internal/application"
	"myproject/payment/internal/infrastructure/broker"
	"myproject/payment/internal/infrastructure/config"
	"myproject/payment/internal/infrastructure/orderclient"
	"myproject/payment/internal/infrastructure/postgres"
	"myproject/payment/internal/transport/httpapi"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func env(k, d string) string { return config.Env(k, d) }

func Migrate() error {
	dsn := env("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/payments?sslmode=disable")
	return postgres.Migrate(env("MIGRATIONS_PATH", "file://migrations"), dsn)
}

func Run(ctx context.Context) error {
	if err := Migrate(); err != nil {
		return err
	}
	slog.Info("payment service starting", "port", env("PORT", "8083"))
	store, err := postgres.Open(env("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/payments?sslmode=disable"))
	if err != nil {
		return err
	}
	httpClient := &http.Client{Timeout: 3 * time.Second, Transport: otelhttp.NewTransport(http.DefaultTransport)}
	payments := application.NewPaymentService(store, orderclient.New(env("ORDER_URL", "http://localhost:8082"), env("INTERNAL_API_KEY", "internal-key"), httpClient))
	rabbit := broker.NewRabbitMQ(env("RABBITMQ_URL", "amqp://app:app@localhost:5672/"))
	workerCtx, stopWorkers := context.WithCancel(ctx)
	defer stopWorkers()
	go rabbit.StartOrderConsumer(workerCtx, application.NewOrderCreatedService(store))
	go rabbit.StartOutboxPublisher(workerCtx, store)
	r := gin.New()
	r.ContextWithFallback = true
	r.Use(
		otelgin.Middleware("payment"),
		gin.Logger(),
		gin.Recovery(),
	)
	httpapi.NewHandler(store, payments, []byte(env("JWT_SECRET", "dev-secret"))).RegisterRoutes(r)
	server := &http.Server{
		Addr:    ":" + env("PORT", "8083"),
		Handler: r,
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()
	slog.Info("payment service started", "addr", server.Addr)

	select {
	case <-ctx.Done():
		slog.Info("payment service shutting down", "reason", ctx.Err().Error())
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		stopWorkers()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		slog.Info("payment service stopped")
		return nil
	case err := <-errCh:
		stopWorkers()
		if errors.Is(err, http.ErrServerClosed) {
			slog.Info("payment service stopped")
			return nil
		}
		return err
	}
}
