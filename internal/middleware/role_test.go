package middleware_test

import (
	"autoshop/internal/middleware"
	"autoshop/pkg/auth"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequireRole(t *testing.T) {
	t.Setenv("JWT_SECRET", "segredo-de-teste")
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.AuthMiddleware(), middleware.RequireRole("service"))
	router.GET("/internal", func(c *gin.Context) { c.Status(http.StatusOK) })

	call := func(role string) int {
		token, err := auth.GenerateToken("quem-chama", role)
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodGet, "/internal", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec.Code
	}

	assert.Equal(t, http.StatusOK, call("service"))
	assert.Equal(t, http.StatusForbidden, call("customer"))
	assert.Equal(t, http.StatusForbidden, call("admin"))
}
