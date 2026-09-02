package httpapi

import (
	"myproject/iam/internal/domain"

	"github.com/gin-gonic/gin"
)

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
