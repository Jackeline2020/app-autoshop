package integration_test

import (
	"autoshop/internal/repository"
	"autoshop/internal/usecase"
	"autoshop/pkg/config"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	pool            *pgxpool.Pool
	customerUseCase *usecase.CustomerUseCase
	vehicleUseCase  *usecase.VehicleUseCase
	serviceUseCase  *usecase.ServiceUseCase
	partUseCase     *usecase.PartUseCase
	orderUseCase    *usecase.OrderUseCase
)

func TestMain(m *testing.M) {
	setup()
	code := m.Run()
	teardown()
	os.Exit(code)
}

func setup() {
	pool = config.NewPostgresPool()
	resetSchema()
	setupUseCases()
}

func setupUseCases() {
	customerRepo := repository.NewCustomerPostgresRepository(pool)
	vehicleRepo := repository.NewVehiclePostgresRepository(pool)
	serviceRepo := repository.NewServicePostgresRepository(pool)
	partRepo := repository.NewPartPostgresRepository(pool)
	orderRepo := repository.NewOrderPostgresRepository(pool)

	customerUseCase = usecase.NewCustomerUseCase(customerRepo)
	vehicleUseCase = usecase.NewVehicleUseCase(vehicleRepo, customerRepo)
	serviceUseCase = usecase.NewServiceUseCase(serviceRepo)
	partUseCase = usecase.NewPartUseCase(partRepo)
	orderUseCase = usecase.NewOrderUseCase(orderRepo, customerRepo, vehicleRepo, serviceRepo, partRepo)
}

// resetSchema garante um banco limpo a cada execução (equivalente ao
// delete+create de tabelas que a Fase 2 fazia contra o DynamoDB Local) e
// aplica o schema a partir da migration versionada — mesma fonte usada em
// dev/CI/produção, sem duplicar SQL nos testes.
func resetSchema() {
	ctx := context.Background()

	if _, err := pool.Exec(ctx, `DROP SCHEMA public CASCADE`); err != nil {
		panic("erro ao resetar schema de teste: " + err.Error())
	}
	if _, err := pool.Exec(ctx, `CREATE SCHEMA public`); err != nil {
		panic("erro ao recriar schema de teste: " + err.Error())
	}

	migrationSQL, err := os.ReadFile(migrationPath())
	if err != nil {
		panic("erro ao ler migration: " + err.Error())
	}

	// pgx prepara statements individualmente (protocolo estendido), então a
	// migration (um arquivo .sql com várias instruções DDL simples, sem
	// ";" dentro de strings/blocos) é aplicada instrução por instrução.
	for _, stmt := range strings.Split(string(migrationSQL), ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := pool.Exec(ctx, stmt); err != nil {
			panic("erro ao aplicar migration no banco de teste: " + err.Error())
		}
	}
}

func migrationPath() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "migrations", "000001_init_schema.up.sql")
}

func teardown() {
	if pool != nil {
		pool.Close()
	}
}
