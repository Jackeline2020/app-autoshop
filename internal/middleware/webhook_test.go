package middleware_test

import (
	"autoshop/internal/middleware"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupWebhookRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.EmailWebhookMiddleware())
	router.POST("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return router
}

func TestEmailWebhookMiddleware_ValidToken(t *testing.T) {
	t.Setenv("EMAIL_WEBHOOK_SECRET", "meu-segredo-de-teste")
	router := setupWebhookRouter()

	req := httptest.NewRequest(http.MethodPost, "/protected", nil)
	req.Header.Set("X-Webhook-Token", "meu-segredo-de-teste")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestEmailWebhookMiddleware_InvalidToken(t *testing.T) {
	t.Setenv("EMAIL_WEBHOOK_SECRET", "meu-segredo-de-teste")
	router := setupWebhookRouter()

	req := httptest.NewRequest(http.MethodPost, "/protected", nil)
	req.Header.Set("X-Webhook-Token", "token-errado")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestEmailWebhookMiddleware_MissingToken(t *testing.T) {
	t.Setenv("EMAIL_WEBHOOK_SECRET", "meu-segredo-de-teste")
	router := setupWebhookRouter()

	req := httptest.NewRequest(http.MethodPost, "/protected", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// Garante que, sem a variável de ambiente configurada, nenhuma requisição passa
func TestEmailWebhookMiddleware_SecretNotConfigured_RejectsEvenOldDefault(t *testing.T) {
	t.Setenv("EMAIL_WEBHOOK_SECRET", "")
	router := setupWebhookRouter()

	req := httptest.NewRequest(http.MethodPost, "/protected", nil)
	req.Header.Set("X-Webhook-Token", "autoshop-webhook-dev")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
