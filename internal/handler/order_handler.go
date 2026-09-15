package handler

import (
	"autoshop/internal/domain"
	"autoshop/internal/dto"
	"autoshop/internal/middleware"
	"autoshop/internal/usecase"
	pkgerrors "autoshop/pkg/errors"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

const msgOrderNotFound = "ordem de serviço não encontrada"

type OrderHandler struct {
	usecase *usecase.OrderUseCase
}

func NewOrderHandler(u *usecase.OrderUseCase) *OrderHandler {
	return &OrderHandler{usecase: u}
}

// @Summary     Criar OS
// @Description Cria uma nova ordem de serviço com orçamento automático
// @Tags        orders
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       request body dto.CreateOrderRequest true "Dados da OS"
// @Success     201 {object} domain.Order
// @Failure     400 {object} map[string]interface{}
// @Failure     401 {object} map[string]interface{}
// @Failure     422 {object} map[string]interface{}
// @Router      /orders [post]
func (h *OrderHandler) Create(c *gin.Context) {
	var req dto.CreateOrderRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"errors": pkgerrors.ParseValidationErrors(err),
		})
		return
	}

	// Monta serviceIDs
	var serviceIDs []string
	for _, s := range req.Services {
		serviceIDs = append(serviceIDs, s.ServiceID)
	}

	// Monta partRequests
	var partRequests []struct {
		PartID   string
		Quantity int
	}
	for _, p := range req.Parts {
		partRequests = append(partRequests, struct {
			PartID   string
			Quantity int
		}{PartID: p.PartID, Quantity: p.Quantity})
	}

	order, err := h.usecase.Create(req.CustomerID, req.VehicleID, req.Notes, serviceIDs, partRequests, middleware.CorrelationID(c))
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"errors": []pkgerrors.ValidationError{
				{Field: "order", Message: err.Error()},
			},
		})
		return
	}

	c.JSON(http.StatusCreated, order)
}

// @Summary     Listar OS
// @Description Retorna as ordens de serviço. Sem o filtro "status", aplica a
// @Description regra padrão de listagem: exclui logicamente as OS finalizadas
// @Description e entregues, e ordena por Em Execução > Aguardando Aprovação >
// @Description Diagnóstico > Recebida (mais antigas primeiro). Com o filtro
// @Description "status", retorna apenas as OS daquele status, sem essa regra
// @Tags        orders
// @Produce     json
// @Security    BearerAuth
// @Param       status query string false "Filtrar por status (ignora a regra de ordenação padrão)"
// @Success     200 {array} domain.Order
// @Failure     401 {object} map[string]interface{}
// @Router      /orders [get]
func (h *OrderHandler) GetAll(c *gin.Context) {
	status := c.Query("status")

	if status != "" {
		orders, err := h.usecase.GetByStatus(status)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, orders)
		return
	}

	orders, err := h.usecase.GetAllActive()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, orders)
}

// @Summary     Buscar OS
// @Description Retorna uma ordem de serviço pelo ID
// @Tags        orders
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "ID da OS"
// @Success     200 {object} domain.Order
// @Failure     401 {object} map[string]interface{}
// @Failure     404 {object} map[string]interface{}
// @Router      /orders/{id} [get]
func (h *OrderHandler) GetByID(c *gin.Context) {
	id := c.Param("id")

	order, err := h.usecase.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"errors": []pkgerrors.ValidationError{
				{Field: "id", Message: msgOrderNotFound},
			},
		})
		return
	}

	c.JSON(http.StatusOK, order)
}

// @Summary     Consultar status da OS
// @Description Retorna a situação atual da ordem de serviço (Recebida, Diagnóstico,
// @Description Aguardando Aprovação, Execução, Finalizada, Entregue ou Recusada) —
// @Description rota pública para acompanhamento (ex: link enviado por e-mail),
// @Description mantida do desenho da Fase 2
// @Tags        orders
// @Produce     json
// @Param       id path string true "ID da OS"
// @Success     200 {object} dto.OrderStatusResponse
// @Failure     404 {object} map[string]interface{}
// @Router      /orders/{id}/status [get]
func (h *OrderHandler) GetStatus(c *gin.Context) {
	id := c.Param("id")

	order, err := h.usecase.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"errors": []pkgerrors.ValidationError{
				{Field: "id", Message: msgOrderNotFound},
			},
		})
		return
	}

	c.JSON(http.StatusOK, dto.OrderStatusResponse{
		ID:          order.ID,
		Status:      string(order.Status),
		StatusLabel: domain.StatusLabel(order.Status),
		UpdatedAt:   order.UpdatedAt,
	})
}

// @Summary     OS por cliente
// @Description Retorna todas as OS de um cliente — rota sensível, exige o JWT
// @Description emitido pela Function Serverless de autenticação por CPF
// @Description (lambda-auth-autoshop). Um token de cliente (role "customer")
// @Description só pode consultar o próprio customer_id; um token de
// @Description funcionário (role "admin") pode consultar qualquer um.
// @Tags        orders
// @Produce     json
// @Security    BearerAuth
// @Param       customer_id path string true "ID do cliente"
// @Success     200 {array} domain.Order
// @Failure     401 {object} map[string]interface{}
// @Failure     403 {object} map[string]interface{}
// @Failure     404 {object} map[string]interface{}
// @Router      /orders/customer/{customer_id} [get]
func (h *OrderHandler) GetByCustomerID(c *gin.Context) {
	customerID := c.Param("customer_id")

	// Token de cliente (emitido via CPF pela Lambda) só enxerga as próprias
	// OS — impede que um cliente troque o customer_id na URL e veja OS de
	// outra pessoa. Token de funcionário (login fixo /auth/login) não tem
	// essa restrição, porque a oficina precisa consultar qualquer cliente.
	isCustomer := false
	if role, exists := c.Get("role"); exists && role == "customer" {
		isCustomer = true
		userID, _ := c.Get("user_id")
		if userID != customerID {
			c.JSON(http.StatusForbidden, gin.H{
				"errors": []pkgerrors.ValidationError{
					{Field: "customer_id", Message: "você só pode consultar as próprias ordens de serviço"},
				},
			})
			return
		}
	}

	var orders []domain.Order
	var err error

	if isCustomer {
		// Revalida o status do cliente a cada chamada — um token de cliente
		// continua tecnicamente válido depois que o cliente é inativado (o
		// JWT não é revogado), então sem essa checagem aqui o cliente
		// inativado continuaria vendo as próprias OS normalmente. Mesma
		// mensagem usada quando ele tenta pegar um token novo pela lambda.
		orders, err = h.usecase.GetByCustomerIDChecked(customerID)
		if errors.Is(err, usecase.ErrCustomerInactive) {
			c.JSON(http.StatusForbidden, gin.H{
				"errors": []pkgerrors.ValidationError{
					{Field: "customer_id", Message: err.Error()},
				},
			})
			return
		}
	} else {
		orders, err = h.usecase.GetByCustomerID(customerID)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, orders)
}

// @Summary     Atualizar status da OS
// @Description Avança o status da OS conforme fluxo permitido
// @Tags        orders
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id      path string                        true "ID da OS"
// @Param       request body dto.UpdateOrderStatusRequest  true "Novo status"
// @Success     200 {object} domain.Order
// @Failure     400 {object} map[string]interface{}
// @Failure     401 {object} map[string]interface{}
// @Failure     422 {object} map[string]interface{}
// @Router      /orders/{id}/status [patch]
func (h *OrderHandler) UpdateStatus(c *gin.Context) {
	id := c.Param("id")

	var req dto.UpdateOrderStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"errors": pkgerrors.ParseValidationErrors(err),
		})
		return
	}

	order, err := h.usecase.UpdateStatus(id, req.Status, middleware.CorrelationID(c))
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"errors": []pkgerrors.ValidationError{
				{Field: "status", Message: err.Error()},
			},
		})
		return
	}

	c.JSON(http.StatusOK, order)
}

// @Summary     Tempo médio por serviço
// @Description Retorna o tempo médio de execução por tipo de serviço em minutos
// @Tags        orders
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} map[string]float64
// @Failure     401 {object} map[string]interface{}
// @Router      /orders/metrics/average-time [get]
func (h *OrderHandler) GetAverageServiceTime(c *gin.Context) {
	averages, err := h.usecase.GetAverageServiceTime()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, averages)
}

// @Summary     Aprovar ou recusar orçamento
// @Description Rota pública para o cliente aprovar ou recusar o orçamento da
// @Description OS (ex: link enviado por e-mail, sem exigir login) — mantida
// @Description do desenho da Fase 2
// @Tags        orders
// @Accept      json
// @Produce     json
// @Param       id      path string                   true "ID da OS"
// @Param       request body dto.ApproveOrderRequest  true "Aprovação"
// @Success     200 {object} domain.Order
// @Failure     400 {object} map[string]interface{}
// @Failure     422 {object} map[string]interface{}
// @Router      /orders/{id}/approve [patch]
func (h *OrderHandler) ApproveOrder(c *gin.Context) {
	id := c.Param("id")

	var req dto.ApproveOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"errors": pkgerrors.ParseValidationErrors(err),
		})
		return
	}

	order, err := h.usecase.ApproveOrder(id, *req.Approved, req.Reason, middleware.CorrelationID(c))
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"errors": []pkgerrors.ValidationError{
				{Field: "order", Message: err.Error()},
			},
		})
		return
	}

	c.JSON(http.StatusOK, order)
}

// @Summary     Deletar OS
// @Description Remove uma ordem de serviço do sistema
// @Tags        orders
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "ID da OS"
// @Success     200 {object} map[string]interface{}
// @Failure     401 {object} map[string]interface{}
// @Failure     404 {object} map[string]interface{}
// @Router      /orders/{id} [delete]
func (h *OrderHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	if err := h.usecase.Delete(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"errors": []pkgerrors.ValidationError{
				{Field: "id", Message: msgOrderNotFound},
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "ordem de serviço deletada com sucesso"})
}
