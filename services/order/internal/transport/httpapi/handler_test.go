package httpapi

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestAccessLogUsesRouteTemplate(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AccessLogMiddleware())
	r.GET("/orders/:id", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/orders/123", nil))

	logLine := logs.String()
	if !strings.Contains(logLine, `"route":"/orders/:id"`) {
		t.Fatalf("expected route template in access log: %s", logLine)
	}
	if strings.Contains(logLine, "/orders/123") {
		t.Fatalf("access log should not contain concrete path: %s", logLine)
	}
}

func TestRecoveryMiddlewareReturnsInternalServerError(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RecoveryMiddleware())
	r.GET("/panic", func(c *gin.Context) {
		panic("boom")
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/panic", nil))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(logs.String(), `"route":"/panic"`) {
		t.Fatalf("expected recovery log with route template: %s", logs.String())
	}
}
