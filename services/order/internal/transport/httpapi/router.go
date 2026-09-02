package httpapi

import (
	"myproject/order/internal/transport/httpauth"

	"github.com/gin-gonic/gin"
)

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
