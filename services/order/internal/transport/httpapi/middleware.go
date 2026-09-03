package httpapi

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func AccessLogMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		startedAt := time.Now()
		c.Next()

		status := c.Writer.Status()
		level := slog.LevelInfo
		if c.FullPath() == "/health" && status < http.StatusInternalServerError {
			level = slog.LevelDebug
		} else if status >= http.StatusInternalServerError {
			level = slog.LevelError
		} else if status >= http.StatusBadRequest {
			level = slog.LevelWarn
		}

		slog.LogAttrs(
			c.Request.Context(),
			level,
			"http request completed",
			slog.String("http_method", c.Request.Method),
			slog.String("route", routeTemplate(c)),
			slog.Int("status", status),
			slog.Duration("duration", time.Since(startedAt)),
			slog.Int("response_size", responseSize(c)),
		)
	}
}

func RecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				err := fmt.Errorf("panic: %v", recovered)
				span := trace.SpanFromContext(c.Request.Context())
				if span.SpanContext().IsValid() {
					span.RecordError(err)
					span.SetStatus(codes.Error, "panic")
				}
				_ = c.Error(err)
				slog.ErrorContext(c.Request.Context(), "http request panicked", "error", err, "route", routeTemplate(c))
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			}
		}()
		c.Next()
	}
}

func routeTemplate(c *gin.Context) string {
	if route := c.FullPath(); route != "" {
		return route
	}
	return "unmatched"
}

func responseSize(c *gin.Context) int {
	size := c.Writer.Size()
	if size < 0 {
		return 0
	}
	return size
}
