package handler

import (
	"autoshop/internal/dto"
	"autoshop/internal/usecase"
	pkgerrors "autoshop/pkg/errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type EmailHandler struct {
	usecase *usecase.OrderUseCase
}

func NewEmailHandler(u *usecase.OrderUseCase) *EmailHandler {
	return &EmailHandler{usecase: u}
}

// @Summary     Atualizar status da OS via e-mail
// @Description Webhook para provedores de e-mail notificarem uma mudança de status da OS
// @Tags        integrations
// @Accept      json
// @Produce     json
// @Param       X-Webhook-Token header string true "Token secreto do webhook"
// @Param       request body dto.EmailStatusUpdateRequest true "Conteúdo do e-mail recebido"
// @Success     200 {object} domain.Order
// @Failure     400 {object} map[string]interface{}
// @Failure     401 {object} map[string]interface{}
// @Failure     422 {object} map[string]interface{}
// @Router      /integrations/email/orders/status [post]
func (h *EmailHandler) UpdateStatus(c *gin.Context) {
	var req dto.EmailStatusUpdateRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"errors": pkgerrors.ParseValidationErrors(err),
		})
		return
	}

	order, err := h.usecase.UpdateStatusFromEmail(req.OrderID, req.Status, req.Subject)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"errors": []pkgerrors.ValidationError{
				{Field: "email", Message: err.Error()},
			},
		})
		return
	}

	c.JSON(http.StatusOK, order)
}
