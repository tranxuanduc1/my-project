package httpapi

import (
	"errors"

	"myproject/order/internal/application/apperrors"
	"myproject/order/internal/application/ports"
	"myproject/order/internal/transport/httpauth"

	"github.com/gin-gonic/gin"
)

func (h *Handler) createOrder(c *gin.Context) {
	var in struct {
		Items []ports.OrderItemInput `json:"items"`
	}
	if c.ShouldBindJSON(&in) != nil {
		c.JSON(400, gin.H{"error": "items required"})
		return
	}
	order, existing, err := h.orders.Create(c, httpauth.UserID(c), c.GetHeader("Idempotency-Key"), in.Items)
	if err != nil {
		if errors.Is(err, apperrors.ErrInvalidInput) {
			c.JSON(400, gin.H{"error": "items required"})
			return
		}
		c.JSON(409, gin.H{"error": err.Error()})
		return
	}
	if existing {
		c.JSON(200, order)
		return
	}
	c.JSON(201, order)
}

func (h *Handler) listOrders(c *gin.Context) {
	limit, offset := page(c)
	orders, err := h.orders.List(c, userScope(c), limit, offset)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"items": orders, "limit": limit, "offset": offset})
}

func (h *Handler) getOrder(c *gin.Context) {
	order, err := h.orders.Get(c, c.Param("id"), userScope(c))
	if err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}
	c.JSON(200, order)
}

func (h *Handler) internalOrder(c *gin.Context) {
	order, err := h.orders.Get(c, c.Param("id"), nil)
	if err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}
	c.JSON(200, order)
}

func (h *Handler) cancelOrder(c *gin.Context) {
	if err := h.orders.Cancel(c, c.Param("id"), userScope(c)); err != nil {
		c.JSON(409, gin.H{"error": err.Error()})
		return
	}
	c.Status(204)
}
