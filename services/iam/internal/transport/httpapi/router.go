package httpapi

import (
	"myproject/iam/internal/transport/httpauth"

	"github.com/gin-gonic/gin"
)

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	r.GET("/health", h.healthz)

	v1 := r.Group("/api/v1")
	v1.POST("/auth/register", h.register)
	v1.POST("/auth/login", h.login)

	auth := v1.Group("")
	auth.Use(httpauth.Middleware(h.secret))
	auth.GET("/users/me", h.me)

	admin := auth.Group("")
	admin.Use(httpauth.RequireRole("admin"))
	admin.GET("/users", h.listUsers)
	admin.GET("/users/:id", h.getUser)
	admin.PATCH("/users/:id/status", h.setStatus)
	admin.PUT("/users/:id/roles", h.setRoles)
	admin.GET("/roles", h.listRoles)
	admin.POST("/roles", h.createRole)
	admin.PUT("/roles/:id", h.updateRole)
	admin.DELETE("/roles/:id", h.deleteRole)
}
