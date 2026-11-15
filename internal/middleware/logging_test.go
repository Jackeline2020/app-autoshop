package middleware_test

import (
	"autoshop/internal/middleware"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupLoggingRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.Logger())
	router.GET("/ok", func(c *gin.Context) {
		c.String(http.StatusOK, middleware.CorrelationID(c))
	})
	router.GET("/bad", func(c *gin.Context) { c.Status(http.StatusBadRequest) })
	router.GET("/fail", func(c *gin.Context) {
		_ = c.Error(assert.AnError)
		c.Status(http.StatusInternalServerError)
	})
	return router
}

func TestLogger_KeepsCorrelationIDFromHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	req.Header.Set(middleware.CorrelationIDHeader, "corr-123")
	rec := httptest.NewRecorder()
	setupLoggingRouter().ServeHTTP(rec, req)

	assert.Equal(t, "corr-123", rec.Header().Get(middleware.CorrelationIDHeader))
	assert.Equal(t, "corr-123", rec.Body.String(), "o handler enxerga o mesmo ID")
}

func TestLogger_GeneratesCorrelationID(t *testing.T) {
	router := setupLoggingRouter()
	for _, path := range []string{"/ok", "/bad", "/fail"} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		assert.NotEmpty(t, rec.Header().Get(middleware.CorrelationIDHeader), path)
	}
}

func TestCorrelationID_WithoutLogger(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	assert.Empty(t, middleware.CorrelationID(c))
	c.Set("correlation_id", 10)
	assert.Empty(t, middleware.CorrelationID(c))
}

func TestLogEvent(t *testing.T) {
	middleware.LogEvent(slog.LevelInfo, "evento_de_teste", "chave", "valor")
}
