package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"myproject/order/internal/bootstrap"
	"myproject/order/internal/infrastructure/observability"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		if err := bootstrap.Migrate(); err != nil {
			log.Fatal(err)
		}
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	obs, err := observability.Setup(ctx, observability.LoadConfig(), os.Stdout)
	if err != nil {
		log.Fatal(err)
	}
	slog.SetDefault(obs.Logger)
	defer func() {
		if err := obs.Shutdown(context.Background()); err != nil {
			obs.Logger.Error("telemetry shutdown failed", "error", err)
		}
	}()

	if err := bootstrap.Run(ctx); err != nil {
		obs.Logger.Error("order service failed", "error", err)
		if shutdownErr := obs.Shutdown(context.Background()); shutdownErr != nil {
			obs.Logger.Error("telemetry shutdown failed", "error", shutdownErr)
		}
		os.Exit(1)
	}
}
