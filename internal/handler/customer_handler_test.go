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

func setupCustomerRouter(uc *usecase.CustomerUseCase) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := handler.NewCustomerHandler(uc)
	router.POST("/customers", h.Create)
	router.GET("/customers", h.GetAll)
	router.GET("/customers/:id", h.GetByID)
	router.PUT("/customers/:id", h.Update)
	router.PATCH("/customers/:id/status", h.UpdateStatus)
	router.DELETE("/customers/:id", h.Delete)
	return router
}

func validCustomerBody() []byte {
	body, _ := json.Marshal(map[string]any{
		"name":  "João Silva",
		"cpf":   "529.982.247-25",
		"email": "joao@email.com",
		"phone": "11999999999",
		"address": map[string]string{
			"street": "Rua das Flores", "number": "123",
			"city": "São Paulo", "state": "SP", "zip_code": "01310-100",
		},
	})
	return body
}

func TestCustomerHandler_Create_Success(t *testing.T) {
	repo := &mocks.CustomerRepositoryMock{}
	repo.On("Create", mock.AnythingOfType("domain.Customer")).
		Return(domain.Customer{ID: "uuid", Name: "João Silva", Status: domain.CustomerStatusActive}, nil)

	router := setupCustomerRouter(usecase.NewCustomerUseCase(repo))
	req := httptest.NewRequest(http.MethodPost, "/customers", bytes.NewBuffer(validCustomerBody()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestCustomerHandler_Create_InvalidJSON(t *testing.T) {
	router := setupCustomerRouter(usecase.NewCustomerUseCase(&mocks.CustomerRepositoryMock{}))
	req := httptest.NewRequest(http.MethodPost, "/customers", bytes.NewBufferString("{invalido"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCustomerHandler_Create_ValidationError(t *testing.T) {
	repo := &mocks.CustomerRepositoryMock{}
	router := setupCustomerRouter(usecase.NewCustomerUseCase(repo))

	body, _ := json.Marshal(map[string]any{
		"name": "João Silva", "cpf": "111.111.111-11", "email": "joao@email.com", "phone": "11999999999",
		"address": map[string]string{"street": "Rua A", "number": "1", "city": "SP", "state": "SP", "zip_code": "01310-100"},
	})
	req := httptest.NewRequest(http.MethodPost, "/customers", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	repo.AssertNotCalled(t, "Create")
}

func TestCustomerHandler_GetAll_Success(t *testing.T) {
	repo := &mocks.CustomerRepositoryMock{}
	repo.On("FindAll").Return([]domain.Customer{{ID: "1", Name: "João"}}, nil)

	router := setupCustomerRouter(usecase.NewCustomerUseCase(repo))
	req := httptest.NewRequest(http.MethodGet, "/customers", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCustomerHandler_GetByID_Success(t *testing.T) {
	repo := &mocks.CustomerRepositoryMock{}
	repo.On("FindByID", "uuid").Return(domain.Customer{ID: "uuid", Name: "João"}, nil)

	router := setupCustomerRouter(usecase.NewCustomerUseCase(repo))
	req := httptest.NewRequest(http.MethodGet, "/customers/uuid", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCustomerHandler_GetByID_NotFound(t *testing.T) {
	repo := &mocks.CustomerRepositoryMock{}
	repo.On("FindByID", "inexistente").Return(domain.Customer{}, errors.New("cliente não encontrado"))

	router := setupCustomerRouter(usecase.NewCustomerUseCase(repo))
	req := httptest.NewRequest(http.MethodGet, "/customers/inexistente", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestCustomerHandler_Update_Success(t *testing.T) {
	repo := &mocks.CustomerRepositoryMock{}
	repo.On("FindByID", "uuid").Return(domain.Customer{ID: "uuid", Name: "João"}, nil)
	repo.On("Update", mock.AnythingOfType("domain.Customer")).Return(nil)

	router := setupCustomerRouter(usecase.NewCustomerUseCase(repo))
	req := httptest.NewRequest(http.MethodPut, "/customers/uuid", bytes.NewBuffer(validCustomerBody()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCustomerHandler_Update_NotFound(t *testing.T) {
	repo := &mocks.CustomerRepositoryMock{}
	repo.On("FindByID", "inexistente").Return(domain.Customer{}, errors.New("não encontrado"))

	router := setupCustomerRouter(usecase.NewCustomerUseCase(repo))
	req := httptest.NewRequest(http.MethodPut, "/customers/inexistente", bytes.NewBuffer(validCustomerBody()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestCustomerHandler_UpdateStatus_Success(t *testing.T) {
	repo := &mocks.CustomerRepositoryMock{}
	repo.On("FindByID", "uuid").Return(domain.Customer{ID: "uuid", Status: domain.CustomerStatusActive}, nil)
	repo.On("UpdateStatus", "uuid", domain.CustomerStatusInactive).Return(nil)

	router := setupCustomerRouter(usecase.NewCustomerUseCase(repo))
	body, _ := json.Marshal(map[string]string{"status": "inativo"})
	req := httptest.NewRequest(http.MethodPatch, "/customers/uuid/status", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCustomerHandler_UpdateStatus_InvalidValue(t *testing.T) {
	repo := &mocks.CustomerRepositoryMock{}
	router := setupCustomerRouter(usecase.NewCustomerUseCase(repo))

	body, _ := json.Marshal(map[string]string{"status": "bloqueado"})
	req := httptest.NewRequest(http.MethodPatch, "/customers/uuid/status", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// binding:"oneof=ativo inativo" rejeita antes de chamar o usecase
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	repo.AssertNotCalled(t, "FindByID")
}

func TestCustomerHandler_Delete_Success(t *testing.T) {
	repo := &mocks.CustomerRepositoryMock{}
	repo.On("FindByID", "uuid").Return(domain.Customer{ID: "uuid"}, nil)
	repo.On("Delete", "uuid").Return(nil)

	router := setupCustomerRouter(usecase.NewCustomerUseCase(repo))
	req := httptest.NewRequest(http.MethodDelete, "/customers/uuid", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCustomerHandler_Delete_NotFound(t *testing.T) {
	repo := &mocks.CustomerRepositoryMock{}
	repo.On("FindByID", "inexistente").Return(domain.Customer{}, errors.New("não encontrado"))

	router := setupCustomerRouter(usecase.NewCustomerUseCase(repo))
	req := httptest.NewRequest(http.MethodDelete, "/customers/inexistente", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}
