package httpapi

import (
	app "myproject/payment/internal/application"

	"github.com/gin-gonic/gin"
)

func (h *Handler) list(c *gin.Context) {
	payments, err := h.payments.List(c, scope(c))
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"items": payments})
}

func (h *Handler) get(c *gin.Context) {
	payment, err := h.payments.Get(c, c.Param("id"), scope(c))
	if err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}
	c.JSON(200, payment)
}

func (h *Handler) succeed(c *gin.Context) {
	h.decide(c, true)
}

func (h *Handler) fail(c *gin.Context) {
	h.decide(c, false)
}

func (h *Handler) decide(c *gin.Context, success bool) {
	var payment app.Payment
	var err error
	if success {
		payment, err = h.payments.Succeed(c, c.Param("id"), scope(c))
	} else {
		var in struct {
			Reason string `json:"reason"`
		}
		_ = c.ShouldBindJSON(&in)
		payment, err = h.payments.Fail(c, c.Param("id"), scope(c), in.Reason)
	}
	if err != nil {
		writePaymentError(c, err)
		return
	}
	c.JSON(200, payment)
}
