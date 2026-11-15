package usecase_test

import (
	"autoshop/internal/client"
	"autoshop/internal/domain"
	"autoshop/internal/mocks"
	"autoshop/internal/usecase"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupOrderUseCase() (
	*usecase.OrderUseCase,
	*mocks.OrderRepositoryMock,
	*mocks.CustomerRepositoryMock,
	*mocks.VehicleRepositoryMock,
	*mocks.ServiceRepositoryMock,
	*mocks.PartCatalogMock,
) {
	orderRepo := new(mocks.OrderRepositoryMock)
	customerRepo := new(mocks.CustomerRepositoryMock)
	vehicleRepo := new(mocks.VehicleRepositoryMock)
	serviceRepo := new(mocks.ServiceRepositoryMock)
	partRepo := new(mocks.PartCatalogMock)

	uc := usecase.NewOrderUseCase(orderRepo, customerRepo, vehicleRepo, serviceRepo, partRepo)
	return uc, orderRepo, customerRepo, vehicleRepo, serviceRepo, partRepo
}

func TestOrderUseCase_Create_Success(t *testing.T) {
	uc, orderRepo, customerRepo, vehicleRepo, serviceRepo, partRepo := setupOrderUseCase()

	customerRepo.On("FindByID", "cid").Return(domain.Customer{ID: "cid", Name: "João"}, nil)
	vehicleRepo.On("FindByID", "vid").Return(domain.Vehicle{ID: "vid", Plate: "ABC1234"}, nil)
	serviceRepo.On("FindByID", "sid").Return(domain.Service{ID: "sid", Name: "Troca de óleo", Price: 100.0}, nil)
	partRepo.On("FindPart", "pid").Return(client.CatalogPart{ID: "pid", Name: "Filtro", Price: 50.0}, nil)
	orderRepo.On("Create", mock.MatchedBy(func(o domain.Order) bool {
		return o.Total == 200.0 && len(o.Parts) == 1 && o.Parts[0].PartName == "Filtro" && o.Parts[0].UnitPrice == 50.0
	})).Return(domain.Order{ID: "oid"}, nil)

	order, err := uc.Create("cid", "vid", "obs", []string{"sid"}, []struct {
		PartID   string
		Quantity int
	}{{PartID: "pid", Quantity: 2}})

	assert.NoError(t, err)
	assert.NotEmpty(t, order.ID)
	orderRepo.AssertExpectations(t)
}

func TestOrderUseCase_Create_CustomerNotFound(t *testing.T) {
	uc, _, customerRepo, _, _, _ := setupOrderUseCase()

	customerRepo.On("FindByID", "cid").Return(domain.Customer{}, errors.New("not found"))

	_, err := uc.Create("cid", "vid", "", []string{"sid"}, nil)

	assert.EqualError(t, err, "cliente não encontrado")
}

func TestOrderUseCase_Create_CatalogUnavailable(t *testing.T) {
	uc, orderRepo, customerRepo, vehicleRepo, serviceRepo, partRepo := setupOrderUseCase()

	customerRepo.On("FindByID", "cid").Return(domain.Customer{ID: "cid"}, nil)
	vehicleRepo.On("FindByID", "vid").Return(domain.Vehicle{ID: "vid"}, nil)
	serviceRepo.On("FindByID", "sid").Return(domain.Service{ID: "sid", Name: "Troca", Price: 100.0}, nil)
	partRepo.On("FindPart", "pid").Return(client.CatalogPart{}, errors.New("timeout"))

	_, err := uc.Create("cid", "vid", "", []string{"sid"}, []struct {
		PartID   string
		Quantity int
	}{{PartID: "pid", Quantity: 5}})

	assert.ErrorContains(t, err, "não foi possível consultar a peça")
	orderRepo.AssertNotCalled(t, "Create", mock.Anything)
}

func TestOrderUseCase_UpdateStatus_ManualDelivery(t *testing.T) {
	uc, orderRepo, _, _, _, _ := setupOrderUseCase()

	orderRepo.On("FindByID", "oid").Return(domain.Order{
		ID:     "oid",
		Status: domain.StatusFinished,
	}, nil)
	orderRepo.On("Update", mock.AnythingOfType("domain.Order")).Return(nil)

	order, err := uc.UpdateStatus("oid", string(domain.StatusDelivered))

	assert.NoError(t, err)
	assert.Equal(t, domain.StatusDelivered, order.Status)
	assert.NotEmpty(t, order.DeliveredAt)
	orderRepo.AssertExpectations(t)
}

func TestOrderUseCase_UpdateStatus_SagaStatusIsNotManual(t *testing.T) {
	uc, orderRepo, _, _, _, _ := setupOrderUseCase()

	for _, status := range []domain.OrderStatus{
		domain.StatusDiagnosis, domain.StatusWaitingApproval, domain.StatusWaitingPayment,
		domain.StatusInProgress, domain.StatusFinished, domain.StatusCancelled,
	} {
		_, err := uc.UpdateStatus("oid", string(status))
		assert.ErrorIs(t, err, usecase.ErrManualStatusNotAllowed, status)
	}
	orderRepo.AssertNotCalled(t, "FindByID", mock.Anything)
}

func TestOrderUseCase_UpdateStatus_InvalidTransition(t *testing.T) {
	uc, orderRepo, _, _, _, _ := setupOrderUseCase()

	orderRepo.On("FindByID", "oid").Return(domain.Order{
		ID:     "oid",
		Status: domain.StatusReceived,
	}, nil)

	_, err := uc.UpdateStatus("oid", string(domain.StatusDelivered))

	assert.ErrorContains(t, err, "transição de status inválida")
}

func TestOrderUseCase_Delete_Success(t *testing.T) {
	uc, orderRepo, _, _, _, _ := setupOrderUseCase()

	orderRepo.On("FindByID", "oid").Return(domain.Order{ID: "oid"}, nil)
	orderRepo.On("Delete", "oid").Return(nil)

	err := uc.Delete("oid")

	assert.NoError(t, err)
	orderRepo.AssertExpectations(t)
}

func TestOrderUseCase_GetByStatus_InvalidStatus(t *testing.T) {
	uc, _, _, _, _, _ := setupOrderUseCase()

	_, err := uc.GetByStatus("status_invalido")

	assert.ErrorContains(t, err, "status inválido")
}

func TestOrderUseCase_Create_VehicleNotFound(t *testing.T) {
	uc, _, customerRepo, vehicleRepo, _, _ := setupOrderUseCase()

	customerRepo.On("FindByID", "cid").Return(domain.Customer{ID: "cid"}, nil)
	vehicleRepo.On("FindByID", "vid").Return(domain.Vehicle{}, errors.New("not found"))

	_, err := uc.Create("cid", "vid", "", []string{"sid"}, nil)

	assert.EqualError(t, err, "veículo não encontrado")
}

func TestOrderUseCase_Create_ServiceNotFound(t *testing.T) {
	uc, _, customerRepo, vehicleRepo, serviceRepo, _ := setupOrderUseCase()

	customerRepo.On("FindByID", "cid").Return(domain.Customer{ID: "cid"}, nil)
	vehicleRepo.On("FindByID", "vid").Return(domain.Vehicle{ID: "vid"}, nil)
	serviceRepo.On("FindByID", "sid").Return(domain.Service{}, errors.New("not found"))

	_, err := uc.Create("cid", "vid", "", []string{"sid"}, nil)

	assert.ErrorContains(t, err, "serviço não encontrado")
}

func TestOrderUseCase_Create_PartNotFound(t *testing.T) {
	uc, _, customerRepo, vehicleRepo, serviceRepo, partRepo := setupOrderUseCase()

	customerRepo.On("FindByID", "cid").Return(domain.Customer{ID: "cid"}, nil)
	vehicleRepo.On("FindByID", "vid").Return(domain.Vehicle{ID: "vid"}, nil)
	serviceRepo.On("FindByID", "sid").Return(domain.Service{ID: "sid", Name: "Troca", Price: 100.0}, nil)
	partRepo.On("FindPart", "pid").Return(client.CatalogPart{}, client.ErrPartNotFound)

	_, err := uc.Create("cid", "vid", "", []string{"sid"}, []struct {
		PartID   string
		Quantity int
	}{{PartID: "pid", Quantity: 1}})

	assert.ErrorContains(t, err, "peça não encontrada")
}

func TestOrderUseCase_ApproveOrder_Success(t *testing.T) {
	uc, orderRepo, _, _, _, partRepo := setupOrderUseCase()

	orderRepo.On("FindByID", "oid").Return(domain.Order{
		ID:     "oid",
		Status: domain.StatusWaitingApproval,
		Parts:  []domain.OrderPart{{PartID: "pid", PartName: "Filtro", Quantity: 2}},
	}, nil)
	orderRepo.On("Update", mock.AnythingOfType("domain.Order")).Return(nil)

	order, err := uc.ApproveOrder("oid", true, "")

	assert.NoError(t, err)
	assert.Equal(t, domain.StatusWaitingPayment, order.Status)
	// o estoque é reservado pelo Execution Service, não pelo OS
	partRepo.AssertNotCalled(t, "FindPart", mock.Anything)
}

func TestOrderUseCase_ApproveOrder_Rejected(t *testing.T) {
	uc, orderRepo, _, _, _, _ := setupOrderUseCase()

	orderRepo.On("FindByID", "oid").Return(domain.Order{
		ID:     "oid",
		Status: domain.StatusWaitingApproval,
		Parts:  []domain.OrderPart{},
	}, nil)
	orderRepo.On("Update", mock.AnythingOfType("domain.Order")).Return(nil)

	order, err := uc.ApproveOrder("oid", false, "Valor alto demais")

	assert.NoError(t, err)
	assert.Equal(t, domain.StatusRejected, order.Status)
	assert.Contains(t, order.Notes, "Valor alto demais")
}

func TestOrderUseCase_ApproveOrder_WrongStatus(t *testing.T) {
	uc, orderRepo, _, _, _, _ := setupOrderUseCase()

	orderRepo.On("FindByID", "oid").Return(domain.Order{
		ID:     "oid",
		Status: domain.StatusReceived,
	}, nil)

	_, err := uc.ApproveOrder("oid", true, "")

	assert.ErrorContains(t, err, "não está aguardando aprovação")
}

func TestOrderUseCase_GetAll_Success(t *testing.T) {
	uc, orderRepo, _, _, _, _ := setupOrderUseCase()

	orderRepo.On("FindAll").Return([]domain.Order{
		{ID: "oid1"}, {ID: "oid2"},
	}, nil)

	orders, err := uc.GetAll()

	assert.NoError(t, err)
	assert.Len(t, orders, 2)
}

func TestOrderUseCase_GetByCustomerID_Success(t *testing.T) {
	uc, orderRepo, _, _, _, _ := setupOrderUseCase()

	orderRepo.On("FindByCustomerID", "cid").Return([]domain.Order{
		{ID: "oid1", CustomerID: "cid"},
	}, nil)

	orders, err := uc.GetByCustomerID("cid")

	assert.NoError(t, err)
	assert.Len(t, orders, 1)
}

func TestOrderUseCase_GetByStatus_Valid(t *testing.T) {
	uc, orderRepo, _, _, _, _ := setupOrderUseCase()

	orderRepo.On("FindByStatus", domain.StatusReceived).Return([]domain.Order{
		{ID: "oid1", Status: domain.StatusReceived},
	}, nil)

	orders, err := uc.GetByStatus(string(domain.StatusReceived))

	assert.NoError(t, err)
	assert.Len(t, orders, 1)
}

func TestOrderUseCase_GetAllActive_ExcludesClosedAndSorts(t *testing.T) {
	uc, orderRepo, _, _, _, _ := setupOrderUseCase()

	orderRepo.On("FindAll").Return([]domain.Order{
		{ID: "received-old", Status: domain.StatusReceived, CreatedAt: "2024-01-01T00:00:00Z"},
		{ID: "received-new", Status: domain.StatusReceived, CreatedAt: "2024-01-02T00:00:00Z"},
		{ID: "in-progress", Status: domain.StatusInProgress, CreatedAt: "2024-01-03T00:00:00Z"},
		{ID: "waiting-approval", Status: domain.StatusWaitingApproval, CreatedAt: "2024-01-04T00:00:00Z"},
		{ID: "diagnosis", Status: domain.StatusDiagnosis, CreatedAt: "2024-01-05T00:00:00Z"},
		{ID: "finished", Status: domain.StatusFinished, CreatedAt: "2024-01-06T00:00:00Z"},
		{ID: "delivered", Status: domain.StatusDelivered, CreatedAt: "2024-01-07T00:00:00Z"},
	}, nil)

	orders, err := uc.GetAllActive()

	assert.NoError(t, err)
	assert.Len(t, orders, 5)

	var ids []string
	for _, o := range orders {
		ids = append(ids, o.ID)
	}

	assert.Equal(t, []string{
		"in-progress",
		"waiting-approval",
		"diagnosis",
		"received-old",
		"received-new",
	}, ids)
}

func TestOrderUseCase_UpdateStatusFromEmail_DirectFields(t *testing.T) {
	uc, orderRepo, _, _, _, _ := setupOrderUseCase()

	orderRepo.On("FindByID", "oid").Return(domain.Order{
		ID:     "oid",
		Status: domain.StatusFinished,
	}, nil)
	orderRepo.On("Update", mock.AnythingOfType("domain.Order")).Return(nil)

	order, err := uc.UpdateStatusFromEmail("oid", "entregue", "")

	assert.NoError(t, err)
	assert.Equal(t, domain.StatusDelivered, order.Status)
}

func TestOrderUseCase_UpdateStatusFromEmail_ParsesSubject(t *testing.T) {
	uc, orderRepo, _, _, _, _ := setupOrderUseCase()

	const orderID = "aabbcc11-2233-4455-6677-889900aabbcc"

	orderRepo.On("FindByID", orderID).Return(domain.Order{
		ID:     orderID,
		Status: domain.StatusFinished,
	}, nil)
	orderRepo.On("Update", mock.AnythingOfType("domain.Order")).Return(nil)

	order, err := uc.UpdateStatusFromEmail("", "", "OS "+orderID+" -> entregue")

	assert.NoError(t, err)
	assert.Equal(t, domain.StatusDelivered, order.Status)
}

func TestOrderUseCase_UpdateStatusFromEmail_SagaStatusRejected(t *testing.T) {
	uc, _, _, _, _, _ := setupOrderUseCase()

	_, err := uc.UpdateStatusFromEmail("oid", "em_diagnostico", "")

	assert.ErrorIs(t, err, usecase.ErrManualStatusNotAllowed)
}

func TestOrderUseCase_UpdateStatusFromEmail_NoDataAtAll(t *testing.T) {
	uc, _, _, _, _, _ := setupOrderUseCase()

	_, err := uc.UpdateStatusFromEmail("", "", "")

	assert.ErrorContains(t, err, "sem order_id/status e sem assunto")
}

func TestOrderUseCase_UpdateStatusFromEmail_UnparsableSubject(t *testing.T) {
	uc, _, _, _, _, _ := setupOrderUseCase()

	_, err := uc.UpdateStatusFromEmail("", "", "assunto qualquer sem o formato esperado")

	assert.ErrorContains(t, err, "não foi possível identificar")
}

func TestOrderUseCase_UpdateStatusFromEmail_UnknownStatus(t *testing.T) {
	uc, orderRepo, _, _, _, _ := setupOrderUseCase()

	orderRepo.On("FindByID", "oid").Return(domain.Order{ID: "oid", Status: domain.StatusReceived}, nil).Maybe()

	_, err := uc.UpdateStatusFromEmail("oid", "status_inexistente", "")

	assert.ErrorContains(t, err, "não é reconhecido")
	orderRepo.AssertNotCalled(t, "Update")
}

func TestOrderUseCase_GetAverageServiceTime_Success(t *testing.T) {
	uc, orderRepo, _, _, _, _ := setupOrderUseCase()

	orderRepo.On("FindAll").Return([]domain.Order{
		{
			ID:         "oid1",
			StartedAt:  "2024-01-01T08:00:00Z",
			FinishedAt: "2024-01-01T10:00:00Z",
			Services:   []domain.OrderService{{ServiceName: "Troca de óleo"}},
		},
	}, nil)

	averages, err := uc.GetAverageServiceTime()

	assert.NoError(t, err)
	assert.Contains(t, averages, "Troca de óleo")
	assert.Equal(t, 120.0, averages["Troca de óleo"])
}

func TestOrderUseCase_GetAverageTimeByStatus_Success(t *testing.T) {
	uc, orderRepo, _, _, _, _ := setupOrderUseCase()

	orderRepo.On("FindAll").Return([]domain.Order{
		{
			ID:                "oid1",
			DiagnosisAt:       "2024-01-01T08:00:00Z",
			WaitingApprovalAt: "2024-01-01T08:15:00Z",
			StartedAt:         "2024-01-01T09:00:00Z",
			FinishedAt:        "2024-01-01T09:30:00Z",
			DeliveredAt:       "2024-01-01T09:45:00Z",
		},
	}, nil)

	averages, err := uc.GetAverageTimeByStatus()

	assert.NoError(t, err)
	assert.Equal(t, 15.0, averages["Diagnóstico"])
	assert.Equal(t, 30.0, averages["Execução"])
	assert.Equal(t, 15.0, averages["Finalização"])
}

func TestOrderUseCase_GetAverageTimeByStatus_IgnoresIncompleteTimestamps(t *testing.T) {
	uc, orderRepo, _, _, _, _ := setupOrderUseCase()

	// OS ainda em andamento (sem FinishedAt/DeliveredAt) não deve gerar
	// duração nem quebrar o cálculo dos outros status.
	orderRepo.On("FindAll").Return([]domain.Order{
		{ID: "oid1", DiagnosisAt: "2024-01-01T08:00:00Z"},
	}, nil)

	averages, err := uc.GetAverageTimeByStatus()

	assert.NoError(t, err)
	assert.NotContains(t, averages, "Diagnóstico")
	assert.NotContains(t, averages, "Execução")
	assert.NotContains(t, averages, "Finalização")
}

func TestOrderUseCase_GetAverageTimeByStatus_IgnoresInvalidTimestamps(t *testing.T) {
	uc, orderRepo, _, _, _, _ := setupOrderUseCase()

	// Timestamps corrompidos/malformados (ex: dado legado ou erro de
	// integração) não devem quebrar o cálculo nem entrar na média — apenas
	// são ignorados, igual a um timestamp ausente.
	orderRepo.On("FindAll").Return([]domain.Order{
		{
			ID:                "oid1",
			DiagnosisAt:       "data-invalida",
			WaitingApprovalAt: "2024-01-01T08:15:00Z",
		},
		{
			ID:         "oid2",
			StartedAt:  "2024-01-01T09:00:00Z",
			FinishedAt: "data-invalida",
		},
		{
			ID:          "oid3",
			FinishedAt:  "2024-01-01T09:30:00Z",
			DeliveredAt: "data-invalida",
		},
	}, nil)

	averages, err := uc.GetAverageTimeByStatus()

	assert.NoError(t, err)
	assert.NotContains(t, averages, "Diagnóstico")
	assert.NotContains(t, averages, "Execução")
	assert.NotContains(t, averages, "Finalização")
}
