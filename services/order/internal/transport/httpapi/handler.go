package httpapi

import (
	"errors"
	"strconv"

	app "myproject/order/internal/application"
	"myproject/order/internal/application/apperrors"
	"myproject/order/internal/application/ports"
	"myproject/order/internal/transport/httpauth"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	health      ports.HealthChecker
	products    *app.ProductService
	orders      *app.OrderService
	secret      []byte
	internalKey string
}

func NewHandler(health ports.HealthChecker, products *app.ProductService, orders *app.OrderService, secret []byte, internalKey string) *Handler {
	return &Handler{health: health, products: products, orders: orders, secret: secret, internalKey: internalKey}
}

func (h *Handler) healthz(c *gin.Context) {
	if err := h.health.Ping(c); err != nil {
		c.JSON(503, gin.H{"status": "down"})
		return
	}
	c.JSON(200, gin.H{"status": "ok"})
}

func (h *Handler) internalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetHeader("X-Internal-API-Key") != h.internalKey {
			c.AbortWithStatus(401)
			return
		}
		c.Next()
	}
}

func page(c *gin.Context) (int, int) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit < 1 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func userScope(c *gin.Context) *uuid.UUID {
	if httpauth.HasRole(c, "admin") {
		return nil
	}
	userID := httpauth.UserID(c)
	return &userID
}

func writeProductError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, apperrors.ErrInvalidInput):
		c.JSON(400, gin.H{"error": "invalid product"})
	case errors.Is(err, apperrors.ErrNotFound):
		c.JSON(404, gin.H{"error": "not found"})
	case errors.Is(err, apperrors.ErrConflict):
		c.JSON(409, gin.H{"error": "conflict"})
	default:
		c.JSON(500, gin.H{"error": err.Error()})
	}
}
