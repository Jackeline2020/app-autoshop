// Package client contém os clientes HTTP para outros microsserviços.
package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"autoshop/pkg/auth"
)

// ErrPartNotFound indica que a peça não existe no Execution Service.
var ErrPartNotFound = errors.New("peça não encontrada")

// CatalogPart é a cópia dos dados da peça guardada na OS.
type CatalogPart struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

// ExecutionClient consulta o catálogo de peças do Execution Service. É a
// única chamada síncrona do OS Service para outro serviço (RFC-005): uma
// leitura feita na abertura da OS.
type ExecutionClient struct {
	baseURL string
	http    *http.Client
}

func NewExecutionClient(baseURL string) *ExecutionClient {
	return &ExecutionClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *ExecutionClient) FindPart(ctx context.Context, id string) (CatalogPart, error) {
	// Token de serviço, assinado com o mesmo segredo dos demais serviços
	token, err := auth.GenerateToken("os-service", "service")
	if err != nil {
		return CatalogPart{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/parts/"+url.PathEscape(id), nil)
	if err != nil {
		return CatalogPart{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.http.Do(req)
	if err != nil {
		return CatalogPart{}, fmt.Errorf("Execution Service indisponível: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return CatalogPart{}, ErrPartNotFound
	case resp.StatusCode != http.StatusOK:
		return CatalogPart{}, fmt.Errorf("Execution Service respondeu %d ao consultar a peça", resp.StatusCode)
	}

	var part CatalogPart
	if err := json.NewDecoder(resp.Body).Decode(&part); err != nil {
		return CatalogPart{}, fmt.Errorf("resposta inválida do Execution Service: %w", err)
	}
	return part, nil
}
