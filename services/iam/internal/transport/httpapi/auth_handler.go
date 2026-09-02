package httpapi

import (
	"errors"

	app "myproject/iam/internal/application"
	"myproject/iam/internal/transport/httpauth"

	"github.com/gin-gonic/gin"
)

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
