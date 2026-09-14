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

func setupVehicleRouter(uc *usecase.VehicleUseCase) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := handler.NewVehicleHandler(uc)
	router.POST("/vehicles", h.Create)
	router.GET("/vehicles", h.GetAll)
	router.GET("/vehicles/:id", h.GetByID)
	router.GET("/vehicles/customer/:customer_id", h.GetByCustomerID)
	router.PUT("/vehicles/:id", h.Update)
	router.DELETE("/vehicles/:id", h.Delete)
	return router
}

func TestVehicleHandler_Create_Success(t *testing.T) {
	customerRepo := &mocks.CustomerRepositoryMock{}
	customerRepo.On("FindByID", "cust-1").Return(domain.Customer{ID: "cust-1"}, nil)

	vehicleRepo := &mocks.VehicleRepositoryMock{}
	vehicleRepo.On("Create", mock.AnythingOfType("domain.Vehicle")).
		Return(domain.Vehicle{ID: "v1", CustomerID: "cust-1"}, nil)

	router := setupVehicleRouter(usecase.NewVehicleUseCase(vehicleRepo, customerRepo))
	body, _ := json.Marshal(map[string]any{
		"customer_id": "cust-1", "plate": "ABC1234", "brand": "Fiat", "model": "Uno", "year": 2020,
	})
	req := httptest.NewRequest(http.MethodPost, "/vehicles", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestVehicleHandler_Create_MissingCustomerID(t *testing.T) {
	router := setupVehicleRouter(usecase.NewVehicleUseCase(&mocks.VehicleRepositoryMock{}, &mocks.CustomerRepositoryMock{}))
	body, _ := json.Marshal(map[string]any{
		"customer_id": "", "plate": "ABC1234", "brand": "Fiat", "model": "Uno", "year": 2020,
	})
	req := httptest.NewRequest(http.MethodPost, "/vehicles", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestVehicleHandler_Create_CustomerNotFound(t *testing.T) {
	customerRepo := &mocks.CustomerRepositoryMock{}
	customerRepo.On("FindByID", "inexistente").Return(domain.Customer{}, errors.New("não encontrado"))

	router := setupVehicleRouter(usecase.NewVehicleUseCase(&mocks.VehicleRepositoryMock{}, customerRepo))
	body, _ := json.Marshal(map[string]any{
		"customer_id": "inexistente", "plate": "ABC1234", "brand": "Fiat", "model": "Uno", "year": 2020,
	})
	req := httptest.NewRequest(http.MethodPost, "/vehicles", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestVehicleHandler_GetAll_Success(t *testing.T) {
	repo := &mocks.VehicleRepositoryMock{}
	repo.On("FindAll").Return([]domain.Vehicle{{ID: "v1"}}, nil)

	router := setupVehicleRouter(usecase.NewVehicleUseCase(repo, &mocks.CustomerRepositoryMock{}))
	req := httptest.NewRequest(http.MethodGet, "/vehicles", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestVehicleHandler_GetByID_NotFound(t *testing.T) {
	repo := &mocks.VehicleRepositoryMock{}
	repo.On("FindByID", "inexistente").Return(domain.Vehicle{}, errors.New("não encontrado"))

	router := setupVehicleRouter(usecase.NewVehicleUseCase(repo, &mocks.CustomerRepositoryMock{}))
	req := httptest.NewRequest(http.MethodGet, "/vehicles/inexistente", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestVehicleHandler_GetByCustomerID_Success(t *testing.T) {
	repo := &mocks.VehicleRepositoryMock{}
	repo.On("FindByCustomerID", "cust-1").Return([]domain.Vehicle{{ID: "v1", CustomerID: "cust-1"}}, nil)

	router := setupVehicleRouter(usecase.NewVehicleUseCase(repo, &mocks.CustomerRepositoryMock{}))
	req := httptest.NewRequest(http.MethodGet, "/vehicles/customer/cust-1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestVehicleHandler_Update_Success(t *testing.T) {
	repo := &mocks.VehicleRepositoryMock{}
	repo.On("FindByID", "v1").Return(domain.Vehicle{ID: "v1", CustomerID: "cust-1"}, nil)
	repo.On("Update", mock.AnythingOfType("domain.Vehicle")).Return(nil)

	router := setupVehicleRouter(usecase.NewVehicleUseCase(repo, &mocks.CustomerRepositoryMock{}))
	body, _ := json.Marshal(map[string]any{"plate": "XYZ5678", "brand": "Fiat", "model": "Uno", "year": 2021})
	req := httptest.NewRequest(http.MethodPut, "/vehicles/v1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestVehicleHandler_Delete_Success(t *testing.T) {
	repo := &mocks.VehicleRepositoryMock{}
	repo.On("FindByID", "v1").Return(domain.Vehicle{ID: "v1"}, nil)
	repo.On("Delete", "v1").Return(nil)

	router := setupVehicleRouter(usecase.NewVehicleUseCase(repo, &mocks.CustomerRepositoryMock{}))
	req := httptest.NewRequest(http.MethodDelete, "/vehicles/v1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestVehicleHandler_Delete_NotFound(t *testing.T) {
	repo := &mocks.VehicleRepositoryMock{}
	repo.On("FindByID", "inexistente").Return(domain.Vehicle{}, errors.New("não encontrado"))

	router := setupVehicleRouter(usecase.NewVehicleUseCase(repo, &mocks.CustomerRepositoryMock{}))
	req := httptest.NewRequest(http.MethodDelete, "/vehicles/inexistente", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}
