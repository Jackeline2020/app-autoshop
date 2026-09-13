package validator

import (
	"regexp"
	"strings"
)

func IsValidCPF(cpf string) bool {
	// Remove pontos e traço
	re := regexp.MustCompile(`\D`)
	cpf = re.ReplaceAllString(cpf, "")

	if len(cpf) != 11 {
		return false
	}

	// Rejeita CPFs com todos dígitos iguais (ex: 111.111.111-11)
	if strings.Count(cpf, string(cpf[0])) == 11 {
		return false
	}

	// Valida 1º dígito verificador
	sum := 0
	for i := 0; i < 9; i++ {
		sum += int(cpf[i]-'0') * (10 - i)
	}
	remainder := (sum * 10) % 11
	if remainder == 10 || remainder == 11 {
		remainder = 0
	}
	if remainder != int(cpf[9]-'0') {
		return false
	}

	// Valida 2º dígito verificador
	sum = 0
	for i := 0; i < 10; i++ {
		sum += int(cpf[i]-'0') * (11 - i)
	}
	remainder = (sum * 10) % 11
	if remainder == 10 || remainder == 11 {
		remainder = 0
	}

	return remainder == int(cpf[10]-'0')
}

func IsValidCNPJ(cnpj string) bool {
	re := regexp.MustCompile(`\D`)
	cnpj = re.ReplaceAllString(cnpj, "")

	if len(cnpj) != 14 {
		return false
	}

	// Rejeita CNPJs com todos dígitos iguais
	if strings.Count(cnpj, string(cnpj[0])) == 14 {
		return false
	}

	// Pesos para os dois dígitos verificadores
	calcDigit := func(cnpj string, length int) int {
		weights := []int{6, 5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
		sum := 0
		for i := 0; i < length; i++ {
			sum += int(cnpj[i]-'0') * weights[13-length+i]
		}
		remainder := sum % 11
		if remainder < 2 {
			return 0
		}
		return 11 - remainder
	}

	if calcDigit(cnpj, 12) != int(cnpj[12]-'0') {
		return false
	}
	return calcDigit(cnpj, 13) == int(cnpj[13]-'0')
}
