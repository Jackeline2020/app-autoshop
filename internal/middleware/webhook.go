package middleware

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// protege os endpoints de integração por email com um token secreto compartilhado, configurado via variável de ambiente EMAIL_WEBHOOK_SECRET
func EmailWebhookMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		secret := os.Getenv("EMAIL_WEBHOOK_SECRET")
		token := c.GetHeader("X-Webhook-Token")

		if secret == "" || token == "" || token != secret {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "token do webhook inválido ou ausente"})
			c.Abort()
			return
		}

		c.Next()
	}
}
