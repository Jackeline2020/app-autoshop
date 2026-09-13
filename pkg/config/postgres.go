package config

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPostgresPool cria e valida o pool de conexões com o PostgreSQL a partir
// de variáveis de ambiente (DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME,
// DB_SSLMODE). Falha rápido (log.Fatalf) se o banco não responder, do mesmo
// jeito que a inicialização do DynamoDB fazia na Fase 2.
func NewPostgresPool() *pgxpool.Pool {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		getEnv("DB_USER", "autoshop"),
		getEnv("DB_PASSWORD", "autoshop"),
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_NAME", "autoshop"),
		getEnv("DB_SSLMODE", "disable"),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("erro ao criar pool de conexões com o Postgres: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("erro ao conectar ao Postgres: %v", err)
	}

	return pool
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
