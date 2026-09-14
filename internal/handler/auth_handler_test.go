package handler_test

import (
	"autoshop/internal/dto"
	"autoshop/internal/handler"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupAuthRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := handler.NewAuthHandler()
	router.POST("/auth/login", h.Login)
	return router
}

func TestAuthHandler_Login_Success(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret")
	router := setupAuthRouter()

	body, _ := json.Marshal(dto.LoginRequest{Email: "admin@autoshop.com", Password: "admin123"})
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp dto.LoginResponse
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.Token)
	assert.Equal(t, "admin", resp.Role)
}

func TestAuthHandler_Login_WrongPassword(t *testing.T) {
	router := setupAuthRouter()

	body, _ := json.Marshal(dto.LoginRequest{Email: "admin@autoshop.com", Password: "senha-errada"})
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuthHandler_Login_UnknownEmail(t *testing.T) {
	router := setupAuthRouter()

	body, _ := json.Marshal(dto.LoginRequest{Email: "outro@email.com", Password: "admin123"})
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuthHandler_Login_InvalidJSON(t *testing.T) {
	router := setupAuthRouter()

	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString("{invalido"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
