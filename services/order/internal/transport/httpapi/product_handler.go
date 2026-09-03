package httpapi

import (
	"errors"

	"myproject/order/internal/application/apperrors"
	"myproject/order/internal/domain"

	"github.com/gin-gonic/gin"
)

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
		if errors.Is(err, apperrors.ErrInvalidInput) {
			c.JSON(400, gin.H{"error": "image content_type required"})
			return
		}
		writeProductError(c, err)
		return
	}
	c.JSON(200, gin.H{"upload_url": url, "method": "PUT", "object_key": key, "expires_in": 900})
}
