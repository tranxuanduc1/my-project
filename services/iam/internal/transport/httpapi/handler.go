package httpapi

import (
	"errors"

	app "myproject/iam/internal/application"
	"myproject/iam/internal/application/ports"
	"myproject/iam/internal/domain"
	"myproject/iam/internal/transport/httpauth"

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

func (h *Handler) healthz(c *gin.Context) {
	if err := h.health.Ping(c); err != nil {
		c.JSON(503, gin.H{"status": "down"})
		return
	}
	c.JSON(200, gin.H{"status": "ok"})
}

func (h *Handler) register(c *gin.Context) {
	var in struct{ Email, Password string }
	if c.ShouldBindJSON(&in) != nil {
		c.JSON(400, gin.H{"error": "email hợp lệ và password tối thiểu 8 ký tự là bắt buộc"})
		return
	}
	user, err := h.auth.Register(c, in.Email, in.Password)
	if err != nil {
		if errors.Is(err, app.ErrInvalidInput) {
			c.JSON(400, gin.H{"error": "email hợp lệ và password tối thiểu 8 ký tự là bắt buộc"})
			return
		}
		c.JSON(409, gin.H{"error": "email đã tồn tại"})
		return
	}
	c.JSON(201, user)
}

func (h *Handler) login(c *gin.Context) {
	var in struct{ Email, Password string }
	if c.ShouldBindJSON(&in) != nil {
		c.JSON(400, gin.H{"error": "invalid body"})
		return
	}
	token, err := h.auth.Login(c, in.Email, in.Password)
	if err != nil {
		c.JSON(401, gin.H{"error": "invalid credentials"})
		return
	}
	c.JSON(200, token)
}

func (h *Handler) me(c *gin.Context) {
	claims := c.MustGet("claims").(*httpauth.Claims)
	user, err := h.auth.Me(c, claims.Subject)
	if err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}
	c.JSON(200, user)
}

func (h *Handler) listUsers(c *gin.Context) {
	users, err := h.users.List(c)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"items": users})
}

func (h *Handler) getUser(c *gin.Context) {
	user, err := h.users.Get(c, c.Param("id"))
	if err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}
	c.JSON(200, user)
}

func (h *Handler) setStatus(c *gin.Context) {
	var in struct{ Status string }
	if c.ShouldBindJSON(&in) != nil {
		c.JSON(400, gin.H{"error": "status must be active or disabled"})
		return
	}
	if err := h.users.SetStatus(c, c.Param("id"), in.Status); err != nil {
		writeUserError(c, err)
		return
	}
	c.Status(204)
}

func (h *Handler) setRoles(c *gin.Context) {
	var in struct{ Roles []string }
	if c.ShouldBindJSON(&in) != nil {
		c.JSON(400, gin.H{"error": "roles required"})
		return
	}
	if err := h.users.SetRoles(c, c.Param("id"), in.Roles); err != nil {
		writeUserError(c, err)
		return
	}
	c.Status(204)
}

func (h *Handler) listRoles(c *gin.Context) {
	roles, err := h.roles.List(c)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"items": roles})
}

func (h *Handler) createRole(c *gin.Context) {
	var role domain.Role
	if c.ShouldBindJSON(&role) != nil {
		c.JSON(400, gin.H{"error": "name required"})
		return
	}
	role, err := h.roles.Create(c, role)
	if err != nil {
		writeRoleError(c, err)
		return
	}
	c.JSON(201, role)
}

func (h *Handler) updateRole(c *gin.Context) {
	var role domain.Role
	if c.ShouldBindJSON(&role) != nil {
		c.JSON(400, gin.H{"error": "name required"})
		return
	}
	if err := h.roles.Update(c, c.Param("id"), role); err != nil {
		writeRoleError(c, err)
		return
	}
	c.Status(204)
}

func (h *Handler) deleteRole(c *gin.Context) {
	if err := h.roles.Delete(c, c.Param("id")); err != nil {
		writeRoleError(c, err)
		return
	}
	c.Status(204)
}

func writeUserError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, app.ErrInvalidInput):
		c.JSON(400, gin.H{"error": "roles required"})
	case errors.Is(err, app.ErrUnknownRole):
		c.JSON(400, gin.H{"error": "unknown role"})
	case errors.Is(err, app.ErrNotFound):
		c.JSON(404, gin.H{"error": "not found"})
	default:
		c.JSON(500, gin.H{"error": err.Error()})
	}
}

func writeRoleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, app.ErrInvalidInput):
		c.JSON(400, gin.H{"error": "name required"})
	case errors.Is(err, app.ErrConflict):
		c.JSON(409, gin.H{"error": "role exists"})
	case errors.Is(err, app.ErrRoleInUse):
		c.JSON(409, gin.H{"error": "role is in use"})
	case errors.Is(err, app.ErrNotFound):
		c.JSON(404, gin.H{"error": "not found"})
	default:
		c.JSON(500, gin.H{"error": err.Error()})
	}
}
