package httpapi

import "github.com/gin-gonic/gin"

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
