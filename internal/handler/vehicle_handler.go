package handler

import (
	"autoshop/internal/dto"
	"autoshop/internal/usecase"
	pkgerrors "autoshop/pkg/errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type VehicleHandler struct {
	usecase *usecase.VehicleUseCase
}

func NewVehicleHandler(u *usecase.VehicleUseCase) *VehicleHandler {
	return &VehicleHandler{usecase: u}
}

// @Summary     Criar veículo
// @Description Cadastra um novo veículo vinculado a um cliente
// @Tags        vehicles
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       request body dto.CreateVehicleRequest true "Dados do veículo"
// @Success     201 {object} domain.Vehicle
// @Failure     400 {object} map[string]interface{}
// @Failure     401 {object} map[string]interface{}
// @Failure     422 {object} map[string]interface{}
// @Router      /vehicles [post]
func (h *VehicleHandler) Create(c *gin.Context) {
	var req dto.CreateVehicleRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"errors": pkgerrors.ParseValidationErrors(err),
		})
		return
	}

	if req.CustomerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"errors": []pkgerrors.ValidationError{
				{Field: "customer_id", Message: "cliente é obrigatório"},
			},
		})
		return
	}

	vehicle, err := h.usecase.Create(req.CustomerID, req.Plate, req.Brand, req.Model, req.Year)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"errors": []pkgerrors.ValidationError{
				{Field: "vehicle", Message: err.Error()},
			},
		})
		return
	}

	c.JSON(http.StatusCreated, vehicle)
}

// @Summary     Listar veículos
// @Description Retorna todos os veículos cadastrados
// @Tags        vehicles
// @Produce     json
// @Security    BearerAuth
// @Success     200 {array} domain.Vehicle
// @Failure     401 {object} map[string]interface{}
// @Router      /vehicles [get]
func (h *VehicleHandler) GetAll(c *gin.Context) {
	vehicles, err := h.usecase.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, vehicles)
}

// @Summary     Buscar veículo
// @Description Retorna um veículo pelo ID
// @Tags        vehicles
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "ID do veículo"
// @Success     200 {object} domain.Vehicle
// @Failure     401 {object} map[string]interface{}
// @Failure     404 {object} map[string]interface{}
// @Router      /vehicles/{id} [get]
func (h *VehicleHandler) GetByID(c *gin.Context) {
	id := c.Param("id")

	vehicle, err := h.usecase.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"errors": []pkgerrors.ValidationError{
				{Field: "id", Message: "veículo não encontrado"},
			},
		})
		return
	}

	c.JSON(http.StatusOK, vehicle)
}

// @Summary     Veículos por cliente
// @Description Retorna todos os veículos de um cliente
// @Tags        vehicles
// @Produce     json
// @Security    BearerAuth
// @Param       customer_id path string true "ID do cliente"
// @Success     200 {array} domain.Vehicle
// @Failure     401 {object} map[string]interface{}
// @Router      /vehicles/customer/{customer_id} [get]
func (h *VehicleHandler) GetByCustomerID(c *gin.Context) {
	customerID := c.Param("customer_id")

	vehicles, err := h.usecase.GetByCustomerID(customerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, vehicles)
}

// @Summary     Atualizar veículo
// @Description Atualiza os dados de um veículo existente
// @Tags        vehicles
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id      path string                    true "ID do veículo"
// @Param       request body dto.UpdateVehicleRequest  true "Dados atualizados"
// @Success     200 {object} domain.Vehicle
// @Failure     400 {object} map[string]interface{}
// @Failure     401 {object} map[string]interface{}
// @Failure     422 {object} map[string]interface{}
// @Router      /vehicles/{id} [put]
func (h *VehicleHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var req dto.UpdateVehicleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"errors": pkgerrors.ParseValidationErrors(err),
		})
		return
	}

	vehicle, err := h.usecase.Update(id, req.Plate, req.Brand, req.Model, req.Year)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"errors": []pkgerrors.ValidationError{
				{Field: "vehicle", Message: err.Error()},
			},
		})
		return
	}

	c.JSON(http.StatusOK, vehicle)
}

// @Summary     Deletar veículo
// @Description Remove um veículo do sistema
// @Tags        vehicles
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "ID do veículo"
// @Success     200 {object} map[string]interface{}
// @Failure     401 {object} map[string]interface{}
// @Failure     404 {object} map[string]interface{}
// @Router      /vehicles/{id} [delete]
func (h *VehicleHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	if err := h.usecase.Delete(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"errors": []pkgerrors.ValidationError{
				{Field: "id", Message: "veículo não encontrado"},
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "veículo deletado com sucesso"})
}
