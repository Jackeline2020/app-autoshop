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

func setupServiceRouter(uc *usecase.ServiceUseCase) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := handler.NewServiceHandler(uc)
	router.POST("/services", h.Create)
	router.GET("/services", h.GetAll)
	router.GET("/services/:id", h.GetByID)
	router.PUT("/services/:id", h.Update)
	router.DELETE("/services/:id", h.Delete)
	return router
}

func TestServiceHandler_Create_Success(t *testing.T) {
	repo := &mocks.ServiceRepositoryMock{}
	repo.On("Create", mock.AnythingOfType("domain.Service")).
		Return(domain.Service{ID: "s1", Name: "Troca de óleo"}, nil)

	router := setupServiceRouter(usecase.NewServiceUseCase(repo))
	body, _ := json.Marshal(map[string]any{"name": "Troca de óleo", "price": 100.0, "estimated_time": 30})
	req := httptest.NewRequest(http.MethodPost, "/services", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestServiceHandler_Create_InvalidPrice(t *testing.T) {
	repo := &mocks.ServiceRepositoryMock{}
	router := setupServiceRouter(usecase.NewServiceUseCase(repo))

	body, _ := json.Marshal(map[string]any{"name": "Troca de óleo", "price": -10.0, "estimated_time": 30})
	req := httptest.NewRequest(http.MethodPost, "/services", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	repo.AssertNotCalled(t, "Create")
}

func TestServiceHandler_Create_MissingRequiredField(t *testing.T) {
	router := setupServiceRouter(usecase.NewServiceUseCase(&mocks.ServiceRepositoryMock{}))

	body, _ := json.Marshal(map[string]any{"price": 100.0, "estimated_time": 30})
	req := httptest.NewRequest(http.MethodPost, "/services", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestServiceHandler_GetAll_Success(t *testing.T) {
	repo := &mocks.ServiceRepositoryMock{}
	repo.On("FindAll").Return([]domain.Service{{ID: "s1"}}, nil)

	router := setupServiceRouter(usecase.NewServiceUseCase(repo))
	req := httptest.NewRequest(http.MethodGet, "/services", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestServiceHandler_GetByID_NotFound(t *testing.T) {
	repo := &mocks.ServiceRepositoryMock{}
	repo.On("FindByID", "inexistente").Return(domain.Service{}, errors.New("não encontrado"))

	router := setupServiceRouter(usecase.NewServiceUseCase(repo))
	req := httptest.NewRequest(http.MethodGet, "/services/inexistente", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestServiceHandler_Update_Success(t *testing.T) {
	repo := &mocks.ServiceRepositoryMock{}
	repo.On("FindByID", "s1").Return(domain.Service{ID: "s1"}, nil)
	repo.On("Update", mock.AnythingOfType("domain.Service")).Return(nil)

	router := setupServiceRouter(usecase.NewServiceUseCase(repo))
	body, _ := json.Marshal(map[string]any{"name": "Troca de óleo", "price": 120.0, "estimated_time": 40})
	req := httptest.NewRequest(http.MethodPut, "/services/s1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestServiceHandler_Delete_Success(t *testing.T) {
	repo := &mocks.ServiceRepositoryMock{}
	repo.On("FindByID", "s1").Return(domain.Service{ID: "s1"}, nil)
	repo.On("Delete", "s1").Return(nil)

	router := setupServiceRouter(usecase.NewServiceUseCase(repo))
	req := httptest.NewRequest(http.MethodDelete, "/services/s1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestServiceHandler_Delete_NotFound(t *testing.T) {
	repo := &mocks.ServiceRepositoryMock{}
	repo.On("FindByID", "inexistente").Return(domain.Service{}, errors.New("não encontrado"))

	router := setupServiceRouter(usecase.NewServiceUseCase(repo))
	req := httptest.NewRequest(http.MethodDelete, "/services/inexistente", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}
