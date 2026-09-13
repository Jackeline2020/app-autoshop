package handler_test

import (
	"autoshop/internal/domain"
	"autoshop/internal/handler"
	"autoshop/internal/mocks"
	"autoshop/internal/usecase"
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupEmailRouter(uc *usecase.OrderUseCase) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := handler.NewEmailHandler(uc)
	router.POST("/integrations/email/orders/status", h.UpdateStatus)
	return router
}

func TestEmailHandler_UpdateStatus_InvalidJSON(t *testing.T) {
	uc := usecase.NewOrderUseCase(&mocks.OrderRepositoryMock{}, nil, nil, nil, nil)
	router := setupEmailRouter(uc)

	req := httptest.NewRequest(http.MethodPost, "/integrations/email/orders/status", bytes.NewBufferString("{invalido"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestEmailHandler_UpdateStatus_UsecaseError(t *testing.T) {
	repoMock := &mocks.OrderRepositoryMock{}
	repoMock.On("FindByID", "os-inexistente").Return(domain.Order{}, errors.New("ordem de serviço não encontrada"))

	uc := usecase.NewOrderUseCase(repoMock, nil, nil, nil, nil)
	router := setupEmailRouter(uc)

	body, _ := json.Marshal(map[string]string{
		"order_id": "os-inexistente",
		"status":   "em_diagnostico",
	})
	req := httptest.NewRequest(http.MethodPost, "/integrations/email/orders/status", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestEmailHandler_UpdateStatus_Success(t *testing.T) {
	existing := domain.Order{ID: "os-1", Status: domain.StatusReceived}

	repoMock := &mocks.OrderRepositoryMock{}
	repoMock.On("FindByID", "os-1").Return(existing, nil)
	repoMock.On("Update", mock.AnythingOfType("domain.Order")).Return(nil)

	uc := usecase.NewOrderUseCase(repoMock, nil, nil, nil, nil)
	router := setupEmailRouter(uc)

	body, _ := json.Marshal(map[string]string{
		"order_id": "os-1",
		"status":   "em_diagnostico",
	})
	req := httptest.NewRequest(http.MethodPost, "/integrations/email/orders/status", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp domain.Order
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, domain.StatusDiagnosis, resp.Status)
}
