package domain

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

type Vehicle struct {
	ID         string `json:"id"          dynamodbav:"id"`
	CustomerID string `json:"customer_id" dynamodbav:"customer_id"`
	Plate      string `json:"plate"       dynamodbav:"plate"`
	Brand      string `json:"brand"       dynamodbav:"brand"`
	Model      string `json:"model"       dynamodbav:"model"`
	Year       int    `json:"year"        dynamodbav:"year"`
}

func (v *Vehicle) Validate() error {
	if v.CustomerID == "" {
		return errors.New("cliente é obrigatório")
	}

	if v.Plate == "" {
		return errors.New("placa é obrigatória")
	}

	if !isValidPlate(v.Plate) {
		return errors.New("placa inválida. Use o formato antigo (ABC-1234) ou Mercosul (ABC1D23)")
	}

	if v.Brand == "" {
		return errors.New("marca é obrigatória")
	}

	if v.Model == "" {
		return errors.New("modelo é obrigatório")
	}

	currentYear := time.Now().Year()
	if v.Year < 1900 || v.Year > currentYear+1 {
		return errors.New("ano do veículo inválido")
	}

	return nil
}

func isValidPlate(plate string) bool {
	plate = strings.ToUpper(strings.ReplaceAll(plate, "-", ""))

	// Formato antigo: ABC1234
	oldFormat := regexp.MustCompile(`^[A-Z]{3}[0-9]{4}$`)
	// Formato Mercosul: ABC1D23
	mercosulFormat := regexp.MustCompile(`^[A-Z]{3}[0-9][A-Z][0-9]{2}$`)

	return oldFormat.MatchString(plate) || mercosulFormat.MatchString(plate)
}
