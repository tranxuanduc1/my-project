package httpapi

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	httpRequests metric.Int64Counter
	httpDuration metric.Float64Histogram
	httpInflight metric.Int64UpDownCounter
)

func init() {
	meter := otel.Meter("myproject/order/http")
	var err error
	httpRequests, err = meter.Int64Counter(
		"http.server.request.count",
		metric.WithDescription("Inbound HTTP requests by route, method, and status."),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		slog.Warn("http request metric initialization failed", "error", err)
	}
	httpDuration, err = meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithDescription("Inbound HTTP request duration."),
		metric.WithUnit("s"),
	)
	if err != nil {
		slog.Warn("http duration metric initialization failed", "error", err)
	}
	httpInflight, err = meter.Int64UpDownCounter(
		"http.server.active_requests",
		metric.WithDescription("Inbound HTTP requests currently in flight."),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		slog.Warn("http in-flight metric initialization failed", "error", err)
	}
}

func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		attrs := []attribute.KeyValue{
			attribute.String("http.request.method", c.Request.Method),
			attribute.String("http.route", routeTemplate(c)),
		}
		if httpInflight != nil {
			httpInflight.Add(c.Request.Context(), 1, metric.WithAttributes(attrs...))
			defer httpInflight.Add(c.Request.Context(), -1, metric.WithAttributes(attrs...))
		}

		startedAt := time.Now()
		c.Next()

		attrs = append(attrs, attribute.Int("http.response.status_code", c.Writer.Status()))
		if httpRequests != nil {
			httpRequests.Add(c.Request.Context(), 1, metric.WithAttributes(attrs...))
		}
		if httpDuration != nil {
			httpDuration.Record(c.Request.Context(), time.Since(startedAt).Seconds(), metric.WithAttributes(attrs...))
		}
	}
}
