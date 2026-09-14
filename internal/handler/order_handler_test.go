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

func setupOrderRouter(uc *usecase.OrderUseCase) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := handler.NewOrderHandler(uc)
	router.POST("/orders", h.Create)
	router.GET("/orders", h.GetAll)
	router.GET("/orders/:id", h.GetByID)
	router.GET("/orders/metrics/average-time", h.GetAverageServiceTime)
	router.PATCH("/orders/:id/status", h.UpdateStatus)
	router.PATCH("/orders/:id/approve", h.ApproveOrder)
	router.DELETE("/orders/:id", h.Delete)
	return router
}

// setupOrderCustomerRouter simula o que o middleware.AuthMiddleware() faria
// depois de validar um JWT: injeta role/user_id no contexto antes do handler,
// igual à rota real /orders/customer/:customer_id em cmd/api/main.go.
func setupOrderCustomerRouter(uc *usecase.OrderUseCase, role, userID string, setContext bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if setContext {
			c.Set("role", role)
			c.Set("user_id", userID)
		}
		c.Next()
	})
	h := handler.NewOrderHandler(uc)
	router.GET("/orders/customer/:customer_id", h.GetByCustomerID)
	return router
}

func TestOrderHandler_Create_Success(t *testing.T) {
	customerRepo := &mocks.CustomerRepositoryMock{}
	customerRepo.On("FindByID", "cust-1").Return(domain.Customer{ID: "cust-1", Name: "João"}, nil)
	vehicleRepo := &mocks.VehicleRepositoryMock{}
	vehicleRepo.On("FindByID", "veh-1").Return(domain.Vehicle{ID: "veh-1", Plate: "ABC1234"}, nil)
	serviceRepo := &mocks.ServiceRepositoryMock{}
	serviceRepo.On("FindByID", "svc-1").Return(domain.Service{ID: "svc-1", Name: "Troca de óleo", Price: 100}, nil)
	partRepo := &mocks.PartRepositoryMock{}
	orderRepo := &mocks.OrderRepositoryMock{}
	orderRepo.On("Create", mock.AnythingOfType("domain.Order")).
		Return(domain.Order{ID: "os-1", CustomerID: "cust-1"}, nil)

	uc := usecase.NewOrderUseCase(orderRepo, customerRepo, vehicleRepo, serviceRepo, partRepo)
	router := setupOrderRouter(uc)

	body, _ := json.Marshal(map[string]any{
		"customer_id": "cust-1", "vehicle_id": "veh-1",
		"services": []map[string]string{{"service_id": "svc-1"}},
	})
	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestOrderHandler_Create_CustomerNotFound(t *testing.T) {
	customerRepo := &mocks.CustomerRepositoryMock{}
	customerRepo.On("FindByID", "inexistente").Return(domain.Customer{}, errors.New("não encontrado"))

	uc := usecase.NewOrderUseCase(&mocks.OrderRepositoryMock{}, customerRepo, &mocks.VehicleRepositoryMock{}, &mocks.ServiceRepositoryMock{}, &mocks.PartRepositoryMock{})
	router := setupOrderRouter(uc)

	body, _ := json.Marshal(map[string]any{
		"customer_id": "inexistente", "vehicle_id": "veh-1",
		"services": []map[string]string{{"service_id": "svc-1"}},
	})
	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestOrderHandler_GetAll_Default(t *testing.T) {
	repo := &mocks.OrderRepositoryMock{}
	repo.On("FindAll").Return([]domain.Order{{ID: "os-1", Status: domain.StatusReceived}}, nil)

	uc := usecase.NewOrderUseCase(repo, nil, nil, nil, nil)
	router := setupOrderRouter(uc)

	req := httptest.NewRequest(http.MethodGet, "/orders", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestOrderHandler_GetAll_ByStatus(t *testing.T) {
	repo := &mocks.OrderRepositoryMock{}
	repo.On("FindByStatus", domain.StatusFinished).Return([]domain.Order{{ID: "os-1", Status: domain.StatusFinished}}, nil)

	uc := usecase.NewOrderUseCase(repo, nil, nil, nil, nil)
	router := setupOrderRouter(uc)

	req := httptest.NewRequest(http.MethodGet, "/orders?status=finalizada", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestOrderHandler_GetByID_NotFound(t *testing.T) {
	repo := &mocks.OrderRepositoryMock{}
	repo.On("FindByID", "inexistente").Return(domain.Order{}, errors.New("não encontrada"))

	uc := usecase.NewOrderUseCase(repo, nil, nil, nil, nil)
	router := setupOrderRouter(uc)

	req := httptest.NewRequest(http.MethodGet, "/orders/inexistente", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestOrderHandler_GetByCustomerID_AdminSeesAny(t *testing.T) {
	repo := &mocks.OrderRepositoryMock{}
	repo.On("FindByCustomerID", "cust-2").Return([]domain.Order{{ID: "os-1", CustomerID: "cust-2"}}, nil)

	uc := usecase.NewOrderUseCase(repo, nil, nil, nil, nil)
	router := setupOrderCustomerRouter(uc, "admin", "func-1", true)

	req := httptest.NewRequest(http.MethodGet, "/orders/customer/cust-2", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestOrderHandler_GetByCustomerID_CustomerSeesOwn(t *testing.T) {
	repo := &mocks.OrderRepositoryMock{}
	repo.On("FindByCustomerID", "cust-1").Return([]domain.Order{{ID: "os-1", CustomerID: "cust-1"}}, nil)
	customerRepo := &mocks.CustomerRepositoryMock{}
	customerRepo.On("FindByID", "cust-1").Return(domain.Customer{ID: "cust-1", Status: domain.CustomerStatusActive}, nil)

	uc := usecase.NewOrderUseCase(repo, customerRepo, nil, nil, nil)
	router := setupOrderCustomerRouter(uc, "customer", "cust-1", true)

	req := httptest.NewRequest(http.MethodGet, "/orders/customer/cust-1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestOrderHandler_GetByCustomerID_InactiveCustomerBlocked(t *testing.T) {
	repo := &mocks.OrderRepositoryMock{}
	customerRepo := &mocks.CustomerRepositoryMock{}
	customerRepo.On("FindByID", "cust-1").Return(domain.Customer{ID: "cust-1", Status: domain.CustomerStatusInactive}, nil)

	uc := usecase.NewOrderUseCase(repo, customerRepo, nil, nil, nil)
	router := setupOrderCustomerRouter(uc, "customer", "cust-1", true)

	req := httptest.NewRequest(http.MethodGet, "/orders/customer/cust-1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "cliente inativo")
	repo.AssertNotCalled(t, "FindByCustomerID")
}

func TestOrderHandler_GetByCustomerID_AdminSeesInactiveCustomerToo(t *testing.T) {
	// Funcionário precisa continuar enxergando OS de clientes inativos
	// (suporte/histórico) — só o auto-atendimento do próprio cliente é bloqueado.
	repo := &mocks.OrderRepositoryMock{}
	repo.On("FindByCustomerID", "cust-9").Return([]domain.Order{{ID: "os-9", CustomerID: "cust-9"}}, nil)

	uc := usecase.NewOrderUseCase(repo, nil, nil, nil, nil)
	router := setupOrderCustomerRouter(uc, "admin", "func-1", true)

	req := httptest.NewRequest(http.MethodGet, "/orders/customer/cust-9", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestOrderHandler_GetByCustomerID_CustomerBlockedFromOthers(t *testing.T) {
	repo := &mocks.OrderRepositoryMock{}

	uc := usecase.NewOrderUseCase(repo, nil, nil, nil, nil)
	router := setupOrderCustomerRouter(uc, "customer", "cust-1", true)

	req := httptest.NewRequest(http.MethodGet, "/orders/customer/cust-2", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	repo.AssertNotCalled(t, "FindByCustomerID")
}

func TestOrderHandler_UpdateStatus_Success(t *testing.T) {
	repo := &mocks.OrderRepositoryMock{}
	repo.On("FindByID", "os-1").Return(domain.Order{ID: "os-1", Status: domain.StatusReceived}, nil)
	repo.On("Update", mock.AnythingOfType("domain.Order")).Return(nil)

	uc := usecase.NewOrderUseCase(repo, nil, nil, nil, nil)
	router := setupOrderRouter(uc)

	body, _ := json.Marshal(map[string]string{"status": "em_diagnostico"})
	req := httptest.NewRequest(http.MethodPatch, "/orders/os-1/status", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestOrderHandler_UpdateStatus_InvalidTransition(t *testing.T) {
	repo := &mocks.OrderRepositoryMock{}
	repo.On("FindByID", "os-1").Return(domain.Order{ID: "os-1", Status: domain.StatusReceived}, nil)

	uc := usecase.NewOrderUseCase(repo, nil, nil, nil, nil)
	router := setupOrderRouter(uc)

	body, _ := json.Marshal(map[string]string{"status": "entregue"})
	req := httptest.NewRequest(http.MethodPatch, "/orders/os-1/status", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestOrderHandler_GetAverageServiceTime_Success(t *testing.T) {
	repo := &mocks.OrderRepositoryMock{}
	repo.On("FindAll").Return([]domain.Order{
		{ID: "os-1", StartedAt: "2026-01-01T10:00:00Z", FinishedAt: "2026-01-01T10:30:00Z",
			Services: []domain.OrderService{{ServiceName: "Troca de óleo"}}},
	}, nil)

	uc := usecase.NewOrderUseCase(repo, nil, nil, nil, nil)
	router := setupOrderRouter(uc)

	req := httptest.NewRequest(http.MethodGet, "/orders/metrics/average-time", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestOrderHandler_ApproveOrder_Success(t *testing.T) {
	repo := &mocks.OrderRepositoryMock{}
	repo.On("FindByID", "os-1").Return(domain.Order{ID: "os-1", Status: domain.StatusWaitingApproval}, nil)
	repo.On("Update", mock.AnythingOfType("domain.Order")).Return(nil)

	uc := usecase.NewOrderUseCase(repo, nil, nil, nil, &mocks.PartRepositoryMock{})
	router := setupOrderRouter(uc)

	approved := true
	body, _ := json.Marshal(map[string]any{"approved": &approved})
	req := httptest.NewRequest(http.MethodPatch, "/orders/os-1/approve", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestOrderHandler_ApproveOrder_NotWaitingApproval(t *testing.T) {
	repo := &mocks.OrderRepositoryMock{}
	repo.On("FindByID", "os-1").Return(domain.Order{ID: "os-1", Status: domain.StatusReceived}, nil)

	uc := usecase.NewOrderUseCase(repo, nil, nil, nil, nil)
	router := setupOrderRouter(uc)

	approved := true
	body, _ := json.Marshal(map[string]any{"approved": &approved})
	req := httptest.NewRequest(http.MethodPatch, "/orders/os-1/approve", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestOrderHandler_Delete_Success(t *testing.T) {
	repo := &mocks.OrderRepositoryMock{}
	repo.On("FindByID", "os-1").Return(domain.Order{ID: "os-1"}, nil)
	repo.On("Delete", "os-1").Return(nil)

	uc := usecase.NewOrderUseCase(repo, nil, nil, nil, nil)
	router := setupOrderRouter(uc)

	req := httptest.NewRequest(http.MethodDelete, "/orders/os-1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestOrderHandler_Delete_NotFound(t *testing.T) {
	repo := &mocks.OrderRepositoryMock{}
	repo.On("FindByID", "inexistente").Return(domain.Order{}, errors.New("não encontrada"))

	uc := usecase.NewOrderUseCase(repo, nil, nil, nil, nil)
	router := setupOrderRouter(uc)

	req := httptest.NewRequest(http.MethodDelete, "/orders/inexistente", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}
