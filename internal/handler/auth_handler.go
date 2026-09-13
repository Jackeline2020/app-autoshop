package handler

import (
	"autoshop/internal/dto"
	"autoshop/pkg/auth"
	pkgerrors "autoshop/pkg/errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Usuário fixo para MVP — depois migra para banco
var adminUser = struct {
	ID       string
	Name     string
	Email    string
	Password string
	Role     string
}{
	ID:       "1",
	Name:     "Administrador",
	Email:    "admin@autoshop.com",
	Password: "admin123",
	Role:     "admin",
}

type AuthHandler struct{}

func NewAuthHandler() *AuthHandler {
	return &AuthHandler{}
}

// @Summary     Login
// @Description Autentica o usuário e retorna o token JWT
// @Tags        auth
// @Accept      json
// @Produce     json
// @Param       request body dto.LoginRequest true "Credenciais"
// @Success     200 {object} dto.LoginResponse
// @Failure     401 {object} map[string]interface{}
// @Router      /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"errors": pkgerrors.ParseValidationErrors(err),
		})
		return
	}

	if req.Email != adminUser.Email || req.Password != adminUser.Password {
		c.JSON(http.StatusUnauthorized, gin.H{
			"errors": []pkgerrors.ValidationError{
				{Field: "credentials", Message: "email ou senha inválidos"},
			},
		})
		return
	}

	token, err := auth.GenerateToken(adminUser.ID, adminUser.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro ao gerar token"})
		return
	}

	c.JSON(http.StatusOK, dto.LoginResponse{
		Token: token,
		Name:  adminUser.Name,
		Role:  adminUser.Role,
	})
}
