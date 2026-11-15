package client_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"autoshop/internal/client"
	"autoshop/pkg/auth"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func executionServer(t *testing.T) *httptest.Server {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		claims, err := auth.ValidateToken(token)
		if err != nil || claims.Role != "service" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/parts/p1":
			_, _ = w.Write([]byte(`{"id":"p1","name":"Filtro de óleo","price":35.5,"stock":10}`))
		case "/parts/invalida":
			_, _ = w.Write([]byte(`não é json`))
		case "/parts/erro":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(ts.Close)
	return ts
}

func TestFindPart(t *testing.T) {
	t.Setenv("JWT_SECRET", "segredo-teste")
	c := client.NewExecutionClient(executionServer(t).URL + "/")

	part, err := c.FindPart(context.Background(), "p1")
	require.NoError(t, err)
	assert.Equal(t, client.CatalogPart{ID: "p1", Name: "Filtro de óleo", Price: 35.5}, part)
}

func TestFindPart_Errors(t *testing.T) {
	t.Setenv("JWT_SECRET", "segredo-teste")
	c := client.NewExecutionClient(executionServer(t).URL)

	_, err := c.FindPart(context.Background(), "nao-existe")
	assert.ErrorIs(t, err, client.ErrPartNotFound)

	_, err = c.FindPart(context.Background(), "erro")
	assert.ErrorContains(t, err, "500")

	_, err = c.FindPart(context.Background(), "invalida")
	assert.ErrorContains(t, err, "resposta inválida")
}

func TestFindPart_Unreachable(t *testing.T) {
	t.Setenv("JWT_SECRET", "segredo-teste")
	_, err := client.NewExecutionClient("http://127.0.0.1:1").FindPart(context.Background(), "p1")
	assert.ErrorContains(t, err, "indisponível")
}

func TestFindPart_WithoutSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	_, err := client.NewExecutionClient("http://127.0.0.1:1").FindPart(context.Background(), "p1")
	assert.Error(t, err)
}
