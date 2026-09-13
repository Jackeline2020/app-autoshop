package repository

import (
	"fmt"
	"time"
)

// O domínio (internal/domain) representa datas como string RFC3339, herdado
// da Fase 2 (DynamoDB). Mantemos esse contrato para não alterar domain/usecase
// e convertemos aqui, na borda com o Postgres (colunas TIMESTAMPTZ).

func parseTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, s)
}

func parseOptionalTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	return &t
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func formatOptionalTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return formatTime(*t)
}

// timeScanner implementa sql.Scanner para ler uma coluna TIMESTAMPTZ
// diretamente como o formato string (RFC3339) usado pelo domínio.
// Trata NULL como string vazia (campo opcional: started_at, finished_at, delivered_at).
type timeScanner time.Time

func (ts *timeScanner) Scan(src any) error {
	if src == nil {
		*ts = timeScanner(time.Time{})
		return nil
	}
	switch v := src.(type) {
	case time.Time:
		*ts = timeScanner(v)
		return nil
	default:
		return fmt.Errorf("timeScanner: tipo inesperado %T", src)
	}
}

func (ts timeScanner) String() string {
	return formatTime(time.Time(ts))
}
