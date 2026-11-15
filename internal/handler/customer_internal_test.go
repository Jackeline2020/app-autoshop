package handler_test

import (
	"autoshop/internal/domain"
	"autoshop/internal/handler"
	"autoshop/internal/mocks"
	"autoshop/internal/repository"
	"autoshop/internal/usecase"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestCustomerHandler_GetByCPFInternal(t *testing.T) {
	repo := new(mocks.CustomerRepositoryMock)
	repo.On("FindByCPF", "52998224725").Return(domain.Customer{ID: "c1", Name: "Ana", Email: "ana@email.com", Status: "ativo"}, nil)
	repo.On("FindByCPF", "11144477735").Return(domain.Customer{}, repository.ErrCustomerNotFound)
	repo.On("FindByCPF", "39053344705").Return(domain.Customer{}, errors.New("banco fora"))

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/internal/customers/by-cpf/:cpf", handler.NewCustomerHandler(usecase.NewCustomerUseCase(repo)).GetByCPFInternal)

	get := func(cpf string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/internal/customers/by-cpf/"+cpf, nil))
		return rec
	}

	rec := get("529.982.247-25")
	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	assert.Equal(t, map[string]string{"id": "c1", "name": "Ana", "status": "ativo"}, body, "só o necessário para a autenticação")

	assert.Equal(t, http.StatusNotFound, get("11144477735").Code)
	assert.Equal(t, http.StatusBadRequest, get("123").Code)
	assert.Equal(t, http.StatusInternalServerError, get("39053344705").Code)
}
