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

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	r.GET("/health", h.healthz)
	v := r.Group("/api/v1")
	v.Use(httpauth.Middleware(h.secret))
	v.GET("/payments", h.list)
	v.GET("/payments/:id", h.get)
	v.POST("/payments/:id/succeed", h.succeed)
	v.POST("/payments/:id/fail", h.fail)
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

func (h *Handler) list(c *gin.Context) {
	payments, err := h.payments.List(c, scope(c))
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"items": payments})
}

func (h *Handler) get(c *gin.Context) {
	payment, err := h.payments.Get(c, c.Param("id"), scope(c))
	if err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}
	c.JSON(200, payment)
}

func (h *Handler) succeed(c *gin.Context) {
	h.decide(c, true)
}

func (h *Handler) fail(c *gin.Context) {
	h.decide(c, false)
}

func (h *Handler) decide(c *gin.Context, success bool) {
	var payment app.Payment
	var err error
	if success {
		payment, err = h.payments.Succeed(c, c.Param("id"), scope(c))
	} else {
		var in struct {
			Reason string `json:"reason"`
		}
		_ = c.ShouldBindJSON(&in)
		payment, err = h.payments.Fail(c, c.Param("id"), scope(c), in.Reason)
	}
	if err != nil {
		switch {
		case errors.Is(err, app.ErrNotFound):
			c.JSON(404, gin.H{"error": "not found"})
		case errors.Is(err, app.ErrReconcileOrder):
			c.JSON(503, gin.H{"error": "order reconciliation failed: " + err.Error()})
		default:
			c.JSON(409, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(200, payment)
}
