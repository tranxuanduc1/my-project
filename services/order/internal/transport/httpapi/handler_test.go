package httpapi

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestPageNormalizesValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/?limit=500&offset=-2", nil)
	limit, offset := page(c)
	if limit != 20 || offset != 0 {
		t.Fatalf("got limit=%d offset=%d", limit, offset)
	}
}
