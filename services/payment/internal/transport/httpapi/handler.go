package httpapi

import (
	"errors"

	app "myproject/payment/internal/application"
	"myproject/payment/internal/application/ports"
	"myproject/payment/internal/transport/httpauth"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	health   ports.HealthChecker
	payments *app.PaymentService
	secret   []byte
}

func NewHandler(health ports.HealthChecker, payments *app.PaymentService, secret []byte) *Handler {
	return &Handler{health: health, payments: payments, secret: secret}
}

func (h *Handler) healthz(c *gin.Context) {
	if err := h.health.Ping(c); err != nil {
		c.JSON(503, gin.H{"status": "down"})
		return
	}
	c.JSON(200, gin.H{"status": "ok"})
}

func scope(c *gin.Context) *uuid.UUID {
	if httpauth.HasRole(c, "admin") {
		return nil
	}
	userID := httpauth.UserID(c)
	return &userID
}

func writePaymentError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, app.ErrNotFound):
		c.JSON(404, gin.H{"error": "not found"})
	case errors.Is(err, app.ErrReconcileOrder):
		c.JSON(503, gin.H{"error": "order reconciliation failed: " + err.Error()})
	default:
		c.JSON(409, gin.H{"error": err.Error()})
	}
}
