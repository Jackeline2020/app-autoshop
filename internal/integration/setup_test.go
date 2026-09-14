package integration_test

import (
	"autoshop/internal/repository"
	"autoshop/internal/usecase"
	"autoshop/pkg/config"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sort"
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
// aplica o schema a partir das migrations versionadas, em ordem — mesma
// fonte usada em dev/CI/produção, sem duplicar SQL nos testes.
func resetSchema() {
	ctx := context.Background()

	if _, err := pool.Exec(ctx, `DROP SCHEMA public CASCADE`); err != nil {
		panic("erro ao resetar schema de teste: " + err.Error())
	}
	if _, err := pool.Exec(ctx, `CREATE SCHEMA public`); err != nil {
		panic("erro ao recriar schema de teste: " + err.Error())
	}

	for _, path := range migrationPaths() {
		migrationSQL, err := os.ReadFile(path)
		if err != nil {
			panic("erro ao ler migration " + path + ": " + err.Error())
		}

		// pgx prepara statements individualmente (protocolo estendido), então
		// cada migration (um arquivo .sql com várias instruções DDL simples,
		// sem ";" dentro de strings/blocos) é aplicada instrução por instrução.
		for _, stmt := range strings.Split(string(migrationSQL), ";") {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := pool.Exec(ctx, stmt); err != nil {
				panic("erro ao aplicar migration " + path + " no banco de teste: " + err.Error())
			}
		}
	}
}

// migrationPaths lista os arquivos *.up.sql da pasta migrations/ em ordem
// alfabética (mesmo critério usado pelo Job de migration no kind/CI —
// ver infra-k8s-autoshop) — assim uma nova migration nunca precisa de
// mudança aqui, só o arquivo novo precisa existir.
func migrationPaths() []string {
	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(thisFile), "..", "..", "migrations")

	matches, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		panic("erro ao listar migrations: " + err.Error())
	}
	sort.Strings(matches)
	return matches
}

func teardown() {
	if pool != nil {
		pool.Close()
	}
}
