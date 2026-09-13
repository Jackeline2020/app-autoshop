package handler

import (
	"autoshop/internal/dto"
	"autoshop/internal/usecase"
	pkgerrors "autoshop/pkg/errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type ServiceHandler struct {
	usecase *usecase.ServiceUseCase
}

func NewServiceHandler(u *usecase.ServiceUseCase) *ServiceHandler {
	return &ServiceHandler{usecase: u}
}

// @Summary     Criar serviço
// @Description Cadastra um novo serviço disponível na oficina
// @Tags        services
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       request body dto.CreateServiceRequest true "Dados do serviço"
// @Success     201 {object} domain.Service
// @Failure     400 {object} map[string]interface{}
// @Failure     401 {object} map[string]interface{}
// @Failure     422 {object} map[string]interface{}
// @Router      /services [post]
func (h *ServiceHandler) Create(c *gin.Context) {
	var req dto.CreateServiceRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"errors": pkgerrors.ParseValidationErrors(err),
		})
		return
	}

	service, err := h.usecase.Create(req.Name, req.Description, req.Price, req.EstimatedTime)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"errors": []pkgerrors.ValidationError{
				{Field: "service", Message: err.Error()},
			},
		})
		return
	}

	c.JSON(http.StatusCreated, service)
}

// @Summary     Listar serviços
// @Description Retorna todos os serviços cadastrados
// @Tags        services
// @Produce     json
// @Security    BearerAuth
// @Success     200 {array} domain.Service
// @Failure     401 {object} map[string]interface{}
// @Router      /services [get]
func (h *ServiceHandler) GetAll(c *gin.Context) {
	services, err := h.usecase.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, services)
}

// @Summary     Buscar serviço
// @Description Retorna um serviço pelo ID
// @Tags        services
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "ID do serviço"
// @Success     200 {object} domain.Service
// @Failure     401 {object} map[string]interface{}
// @Failure     404 {object} map[string]interface{}
// @Router      /services/{id} [get]
func (h *ServiceHandler) GetByID(c *gin.Context) {
	id := c.Param("id")

	service, err := h.usecase.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"errors": []pkgerrors.ValidationError{
				{Field: "id", Message: "serviço não encontrado"},
			},
		})
		return
	}

	c.JSON(http.StatusOK, service)
}

// @Summary     Atualizar serviço
// @Description Atualiza os dados de um serviço existente
// @Tags        services
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id      path string                    true "ID do serviço"
// @Param       request body dto.UpdateServiceRequest  true "Dados atualizados"
// @Success     200 {object} domain.Service
// @Failure     400 {object} map[string]interface{}
// @Failure     401 {object} map[string]interface{}
// @Failure     422 {object} map[string]interface{}
// @Router      /services/{id} [put]
func (h *ServiceHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var req dto.UpdateServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"errors": pkgerrors.ParseValidationErrors(err),
		})
		return
	}

	service, err := h.usecase.Update(id, req.Name, req.Description, req.Price, req.EstimatedTime)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"errors": []pkgerrors.ValidationError{
				{Field: "service", Message: err.Error()},
			},
		})
		return
	}

	c.JSON(http.StatusOK, service)
}

// @Summary     Deletar serviço
// @Description Remove um serviço do sistema
// @Tags        services
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "ID do serviço"
// @Success     200 {object} map[string]interface{}
// @Failure     401 {object} map[string]interface{}
// @Failure     404 {object} map[string]interface{}
// @Router      /services/{id} [delete]
func (h *ServiceHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	if err := h.usecase.Delete(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"errors": []pkgerrors.ValidationError{
				{Field: "id", Message: "serviço não encontrado"},
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "serviço deletado com sucesso"})
}
