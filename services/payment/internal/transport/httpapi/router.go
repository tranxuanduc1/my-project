package httpapi

import (
	"myproject/payment/internal/transport/httpauth"

	"github.com/gin-gonic/gin"
)

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	r.GET("/health", h.healthz)

	v := r.Group("/api/v1")
	v.Use(httpauth.Middleware(h.secret))
	v.GET("/payments", h.list)
	v.GET("/payments/:id", h.get)
	v.POST("/payments/:id/succeed", h.succeed)
	v.POST("/payments/:id/fail", h.fail)
}
