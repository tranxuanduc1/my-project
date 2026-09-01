package httpapi

import (
	"errors"
	"strconv"

	app "myproject/order/internal/application"
	"myproject/order/internal/application/ports"
	"myproject/order/internal/domain"
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

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	r.GET("/health", h.healthz)
	internal := r.Group("/internal/v1")
	internal.Use(h.internalAuth())
	internal.GET("/orders/:id", h.internalOrder)
	v := r.Group("/api/v1")
	v.Use(httpauth.Middleware(h.secret))
	v.GET("/products", h.listProducts)
	v.GET("/products/:id", h.getProduct)
	v.GET("/orders", h.listOrders)
	v.GET("/orders/:id", h.getOrder)
	v.POST("/orders", h.createOrder)
	v.POST("/orders/:id/cancel", h.cancelOrder)
	admin := v.Group("")
	admin.Use(httpauth.RequireRole("admin"))
	admin.POST("/products", h.createProduct)
	admin.PUT("/products/:id", h.updateProduct)
	admin.DELETE("/products/:id", h.deleteProduct)
	admin.POST("/products/:id/image/presign", h.presignImage)
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

func (h *Handler) listProducts(c *gin.Context) {
	limit, offset := page(c)
	products, err := h.products.List(c, c.Query("q"), limit, offset)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"items": products, "limit": limit, "offset": offset})
}

func (h *Handler) getProduct(c *gin.Context) {
	product, err := h.products.Get(c, c.Param("id"))
	if err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}
	c.JSON(200, product)
}

func (h *Handler) createProduct(c *gin.Context) {
	var product domain.Product
	if c.ShouldBindJSON(&product) != nil {
		c.JSON(400, gin.H{"error": "invalid product"})
		return
	}
	product, err := h.products.Create(c, product)
	if err != nil {
		writeProductError(c, err)
		return
	}
	c.JSON(201, product)
}

func (h *Handler) updateProduct(c *gin.Context) {
	var product domain.Product
	if c.ShouldBindJSON(&product) != nil {
		c.JSON(400, gin.H{"error": "invalid product"})
		return
	}
	product, err := h.products.Update(c, c.Param("id"), product)
	if err != nil {
		writeProductError(c, err)
		return
	}
	c.JSON(200, product)
}

func (h *Handler) deleteProduct(c *gin.Context) {
	if err := h.products.Delete(c, c.Param("id")); err != nil {
		writeProductError(c, err)
		return
	}
	c.Status(204)
}

func (h *Handler) presignImage(c *gin.Context) {
	var in struct {
		ContentType string `json:"content_type"`
	}
	if c.ShouldBindJSON(&in) != nil {
		c.JSON(400, gin.H{"error": "image content_type required"})
		return
	}
	url, key, err := h.products.PresignImage(c, c.Param("id"), in.ContentType)
	if err != nil {
		if errors.Is(err, app.ErrInvalidInput) {
			c.JSON(400, gin.H{"error": "image content_type required"})
			return
		}
		writeProductError(c, err)
		return
	}
	c.JSON(200, gin.H{"upload_url": url, "method": "PUT", "object_key": key, "expires_in": 900})
}

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
		if errors.Is(err, app.ErrInvalidInput) {
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

func writeProductError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, app.ErrInvalidInput):
		c.JSON(400, gin.H{"error": "invalid product"})
	case errors.Is(err, app.ErrNotFound):
		c.JSON(404, gin.H{"error": "not found"})
	case errors.Is(err, app.ErrConflict):
		c.JSON(409, gin.H{"error": "conflict"})
	default:
		c.JSON(500, gin.H{"error": err.Error()})
	}
}
