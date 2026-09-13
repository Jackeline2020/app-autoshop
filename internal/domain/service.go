package domain

import (
	"errors"
	"time"
)

type Service struct {
	ID            string  `json:"id"             dynamodbav:"id"`
	Name          string  `json:"name"           dynamodbav:"name"`
	Description   string  `json:"description"    dynamodbav:"description"`
	Price         float64 `json:"price"          dynamodbav:"price"`
	EstimatedTime int     `json:"estimated_time" dynamodbav:"estimated_time"` // em minutos
	CreatedAt     string  `json:"created_at"     dynamodbav:"created_at"`
}

func (s *Service) Validate() error {
	if s.Name == "" {
		return errors.New("nome do serviço é obrigatório")
	}

	if s.Price <= 0 {
		return errors.New("preço do serviço deve ser maior que zero")
	}

	if s.EstimatedTime <= 0 {
		return errors.New("tempo estimado deve ser maior que zero")
	}

	return nil
}

func NewService(name, description string, price float64, estimatedTime int) Service {
	return Service{
		Name:          name,
		Description:   description,
		Price:         price,
		EstimatedTime: estimatedTime,
		CreatedAt:     time.Now().Format(time.RFC3339),
	}
}
