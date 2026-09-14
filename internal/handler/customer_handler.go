package handler

import (
	"autoshop/internal/domain"
	"autoshop/internal/dto"
	"autoshop/internal/usecase"
	pkgerrors "autoshop/pkg/errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type CustomerHandler struct {
	usecase *usecase.CustomerUseCase
}

func NewCustomerHandler(u *usecase.CustomerUseCase) *CustomerHandler {
	return &CustomerHandler{usecase: u}
}

// @Summary     Criar cliente
// @Description Cria um novo cliente no sistema
// @Tags        customers
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       request body dto.CreateCustomerRequest true "Dados do cliente"
// @Success     201 {object} domain.Customer
// @Failure     400 {object} map[string]interface{}
// @Failure     401 {object} map[string]interface{}
// @Failure     422 {object} map[string]interface{}
// @Router      /customers [post]
func (h *CustomerHandler) Create(c *gin.Context) {
	var req dto.CreateCustomerRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"errors": pkgerrors.ParseValidationErrors(err),
		})
		return
	}

	address := domain.Address{
		Street:     req.Address.Street,
		Number:     req.Address.Number,
		Complement: req.Address.Complement,
		City:       req.Address.City,
		State:      req.Address.State,
		ZipCode:    req.Address.ZipCode,
	}

	customer, err := h.usecase.Create(req.Name, req.CPF, req.CNPJ, req.Email, req.Phone, address)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, customer)
}

// @Summary     Listar clientes
// @Description Retorna todos os clientes cadastrados
// @Tags        customers
// @Produce     json
// @Security    BearerAuth
// @Success     200 {array} domain.Customer
// @Failure     401 {object} map[string]interface{}
// @Router      /customers [get]
func (h *CustomerHandler) GetAll(c *gin.Context) {
	customers, err := h.usecase.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, customers)
}

// @Summary     Buscar cliente
// @Description Retorna um cliente pelo ID
// @Tags        customers
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "ID do cliente"
// @Success     200 {object} domain.Customer
// @Failure     401 {object} map[string]interface{}
// @Failure     404 {object} map[string]interface{}
// @Router      /customers/{id} [get]
func (h *CustomerHandler) GetByID(c *gin.Context) {
	id := c.Param("id")

	customer, err := h.usecase.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"errors": []pkgerrors.ValidationError{
				{Field: "id", Message: "cliente não encontrado"},
			},
		})
		return
	}

	c.JSON(http.StatusOK, customer)
}

// @Summary     Atualizar cliente
// @Description Atualiza os dados de um cliente existente
// @Tags        customers
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "ID do cliente"
// @Param       request body dto.UpdateCustomerRequest true "Dados atualizados do cliente"
// @Success     200 {object} domain.Customer
// @Failure     400 {object} map[string]interface{}
// @Failure     401 {object} map[string]interface{}
// @Failure     404 {object} map[string]interface{}
// @Router      /customers/{id} [put]
func (h *CustomerHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var req dto.UpdateCustomerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"errors": pkgerrors.ParseValidationErrors(err),
		})
		return
	}

	address := domain.Address{
		Street:     req.Address.Street,
		Number:     req.Address.Number,
		Complement: req.Address.Complement,
		City:       req.Address.City,
		State:      req.Address.State,
		ZipCode:    req.Address.ZipCode,
	}

	customer, err := h.usecase.Update(id, req.Name, req.CPF, req.CNPJ, req.Email, req.Phone, address)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, customer)
}

// @Summary     Ativar/inativar cliente
// @Description Muda o status do cliente (ativo/inativo). Um cliente inativo
// @Description continua existindo na base, mas o fluxo de autenticação por
// @Description CPF (lambda-auth-autoshop) passa a recusar a emissão de token
// @Description pra ele — é a checagem de "status do cliente" exigida na Fase 3.
// @Tags        customers
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id      path string                        true "ID do cliente"
// @Param       request body dto.UpdateCustomerStatusRequest true "Novo status"
// @Success     200 {object} domain.Customer
// @Failure     400 {object} map[string]interface{}
// @Failure     401 {object} map[string]interface{}
// @Failure     404 {object} map[string]interface{}
// @Router      /customers/{id}/status [patch]
func (h *CustomerHandler) UpdateStatus(c *gin.Context) {
	id := c.Param("id")

	var req dto.UpdateCustomerStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"errors": pkgerrors.ParseValidationErrors(err),
		})
		return
	}

	customer, err := h.usecase.UpdateStatus(id, req.Status)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"errors": []pkgerrors.ValidationError{
				{Field: "id", Message: "cliente não encontrado"},
			},
		})
		return
	}

	c.JSON(http.StatusOK, customer)
}

// @Summary     Deletar cliente
// @Description Remove um cliente do sistema
// @Tags        customers
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "ID do cliente"
// @Success     200 {object} map[string]interface{}
// @Failure     401 {object} map[string]interface{}
// @Failure     404 {object} map[string]interface{}
// @Router      /customers/{id} [delete]
func (h *CustomerHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	if err := h.usecase.Delete(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"errors": []pkgerrors.ValidationError{
				{Field: "id", Message: "cliente não encontrado"},
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "cliente deletado com sucesso"})
}
