package application

import (
	"net/http/httptest"
	"testing"

	"myproject/iam/internal/transport/httpauth"

	"github.com/gin-gonic/gin"
)

func TestRequireRoleRejectsCustomer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("claims", &claims{Roles: []string{"customer"}})
	httpauth.RequireRole("admin")(c)
	if w.Code != 403 || !c.IsAborted() {
		t.Fatalf("expected forbidden, got %d", w.Code)
	}
}
