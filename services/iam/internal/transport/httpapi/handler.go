package httpapi

import (
	"errors"

	app "myproject/iam/internal/application"
	"myproject/iam/internal/application/apperrors"
	"myproject/iam/internal/application/ports"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	health ports.HealthChecker
	auth   *app.AuthService
	users  *app.UserService
	roles  *app.RoleService
	secret []byte
}

func NewHandler(health ports.HealthChecker, auth *app.AuthService, users *app.UserService, roles *app.RoleService, secret []byte) *Handler {
	return &Handler{health: health, auth: auth, users: users, roles: roles, secret: secret}
}

func (h *Handler) healthz(c *gin.Context) {
	if err := h.health.Ping(c); err != nil {
		c.JSON(503, gin.H{"status": "down"})
		return
	}
	c.JSON(200, gin.H{"status": "ok"})
}

func writeUserError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, apperrors.ErrInvalidInput):
		c.JSON(400, gin.H{"error": "roles required"})
	case errors.Is(err, apperrors.ErrUnknownRole):
		c.JSON(400, gin.H{"error": "unknown role"})
	case errors.Is(err, apperrors.ErrNotFound):
		c.JSON(404, gin.H{"error": "not found"})
	default:
		c.JSON(500, gin.H{"error": err.Error()})
	}
}

func writeRoleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, apperrors.ErrInvalidInput):
		c.JSON(400, gin.H{"error": "name required"})
	case errors.Is(err, apperrors.ErrConflict):
		c.JSON(409, gin.H{"error": "role exists"})
	case errors.Is(err, apperrors.ErrRoleInUse):
		c.JSON(409, gin.H{"error": "role is in use"})
	case errors.Is(err, apperrors.ErrNotFound):
		c.JSON(404, gin.H{"error": "not found"})
	default:
		c.JSON(500, gin.H{"error": err.Error()})
	}
}
