package middleware

import (
	"autoshop/pkg/auth"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "token não informado"})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "formato do token inválido. Use: Bearer <token>"})
			c.Abort()
			return
		}

		claims, err := auth.ValidateToken(parts[1])
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "token inválido ou expirado"})
			c.Abort()
			return
		}

		// Injeta os dados do usuário no contexto para uso nos handlers
		c.Set("user_id", claims.UserID)
		c.Set("role", claims.Role)

		c.Next()
	}
}

// RequireRole libera a rota só para os papéis informados. Deve vir depois
// do AuthMiddleware, que coloca o papel do token no contexto.
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := c.GetString("role")
		for _, r := range roles {
			if role == r {
				c.Next()
				return
			}
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "acesso não permitido para este perfil"})
		c.Abort()
	}
}
