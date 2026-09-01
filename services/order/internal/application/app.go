// Package application contains the order use cases.
package application

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

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
