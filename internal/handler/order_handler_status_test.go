package handler_test

import (
	"autoshop/internal/domain"
	"autoshop/internal/dto"
	"autoshop/internal/handler"
	"autoshop/internal/mocks"
	"autoshop/internal/usecase"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupOrderStatusRouter(uc *usecase.OrderUseCase) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := handler.NewOrderHandler(uc)
	router.GET("/orders/:id/status", h.GetStatus)
	return router
}

func TestOrderHandler_GetStatus_Success(t *testing.T) {
	repoMock := &mocks.OrderRepositoryMock{}
	repoMock.On("FindByID", "os-1").Return(domain.Order{
		ID:        "os-1",
		Status:    domain.StatusInProgress,
		UpdatedAt: "2026-07-01T00:00:00Z",
	}, nil)

	uc := usecase.NewOrderUseCase(repoMock, nil, nil, nil, nil)
	router := setupOrderStatusRouter(uc)

	req := httptest.NewRequest(http.MethodGet, "/orders/os-1/status", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp dto.OrderStatusResponse
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "os-1", resp.ID)
	assert.Equal(t, "em_execucao", resp.Status)
	assert.Equal(t, "Execução", resp.StatusLabel)
}

func TestOrderHandler_GetStatus_NotFound(t *testing.T) {
	repoMock := &mocks.OrderRepositoryMock{}
	repoMock.On("FindByID", "os-inexistente").Return(domain.Order{}, errors.New("não encontrada"))

	uc := usecase.NewOrderUseCase(repoMock, nil, nil, nil, nil)
	router := setupOrderStatusRouter(uc)

	req := httptest.NewRequest(http.MethodGet, "/orders/os-inexistente/status", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}
