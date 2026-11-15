// @title           AutoShop API
// @version         1.0
// @description     API de gestão de ordens de serviço para oficina mecânica
// @termsOfService  http://swagger.io/terms/

// @contact.name   Suporte AutoShop
// @contact.email  suporte@autoshop.com

// @license.name  MIT

// @BasePath  /

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Digite: Bearer {seu_token}

package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"autoshop/internal/client"
	"autoshop/internal/consumer"
	"autoshop/internal/handler"
	"autoshop/internal/messaging"
	"autoshop/internal/middleware"
	"autoshop/internal/repository"
	"autoshop/internal/usecase"
	"autoshop/internal/worker"
	"autoshop/pkg/config"
	"autoshop/pkg/observability"

	_ "autoshop/docs" // swagger docs gerados automaticamente pelo swag

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/gin-gonic/gin"
	"github.com/newrelic/go-agent/v3/integrations/nrgin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Inicializa o agente do New Relic (observabilidade — Fase 3). App fica
	// nil se NEW_RELIC_LICENSE_KEY não estiver configurada, e o restante do
	// código trata isso como "observabilidade desligada" sem quebrar nada.
	nrApp := observability.NewRelicApp()
	observability.App = nrApp

	r := gin.New()
	r.Use(gin.Recovery())
	if nrApp != nil {
		r.Use(nrgin.Middleware(nrApp))
	}
	// Logger estruturado (JSON, com correlation_id) substitui o logger padrão
	// do Gin — ver internal/middleware/logging.go.
	r.Use(middleware.Logger())

	db := config.NewPostgresPool()
	defer db.Close()

	authHandler := handler.NewAuthHandler()

	customerRepo := repository.NewCustomerPostgresRepository(db)
	customerUC := usecase.NewCustomerUseCase(customerRepo)
	customerHandler := handler.NewCustomerHandler(customerUC)

	vehicleRepo := repository.NewVehiclePostgresRepository(db)
	vehicleUC := usecase.NewVehicleUseCase(vehicleRepo, customerRepo)
	vehicleHandler := handler.NewVehicleHandler(vehicleUC)

	serviceRepo := repository.NewServicePostgresRepository(db)
	serviceUC := usecase.NewServiceUseCase(serviceRepo)
	serviceHandler := handler.NewServiceHandler(serviceUC)

	// Catálogo de peças do Execution Service (consulta síncrona na abertura
	// da OS, RFC-005)
	executionClient := client.NewExecutionClient(config.Env("EXECUTION_SERVICE_URL", "http://localhost:8082"))

	orderRepo := repository.NewOrderPostgresRepository(db)
	orderUC := usecase.NewOrderUseCase(orderRepo, customerRepo, vehicleRepo, serviceRepo, executionClient)

	// Saga orquestrada (ADR-005): comandos saem pela outbox para as filas do
	// Execution e do Billing; os resultados chegam pela fila os-events.
	sagaRepo := repository.NewSagaPostgresRepository(db)
	stepTimeout, err := time.ParseDuration(config.Env("SAGA_STEP_TIMEOUT", "5m"))
	if err != nil {
		log.Fatalf("SAGA_STEP_TIMEOUT inválido: %v", err)
	}
	orchestrator := usecase.NewSagaOrchestrator(orderRepo, sagaRepo, usecase.SagaConfig{StepTimeout: stepTimeout})
	orderUC.WithSaga(orchestrator)
	startSagaWorkers(ctx, orchestrator, sagaRepo)
	orderHandler := handler.NewOrderHandler(orderUC)

	emailHandler := handler.NewEmailHandler(orderUC)

	// Rota Swagger
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	//Health check
	r.GET("/health", handler.HealthCheck)

	// Rotas públicas
	// /auth/login é o login de funcionário da oficina (usuário fixo — ver
	// internal/handler/auth_handler.go), diferente do fluxo de autenticação
	// por CPF exigido pela Fase 3 (esse é emitido pela Function Serverless
	// no repositório lambda-auth-autoshop, consumido pelas rotas do cliente
	// abaixo). Os dois emitem o mesmo formato de JWT (pkg/auth/jwt.go), só
	// com roles diferentes ("admin" vs "customer").
	r.POST("/auth/login", authHandler.Login)
	// Rotas de acompanhamento/aprovação por e-mail — sem login, mantidas do
	// desenho da Fase 2 (ver docs/rfc e comentários em order_handler.go).
	r.GET("/orders/:id/status", orderHandler.GetStatus)
	r.PATCH("/orders/:id/approve", orderHandler.ApproveOrder)

	// Integração email, atualização de status via webhook, protegida por secret
	emailIntegration := r.Group("/integrations/email")
	emailIntegration.Use(middleware.EmailWebhookMiddleware())
	{
		emailIntegration.POST("/orders/status", emailHandler.UpdateStatus)
	}

	// Rota sensível do cliente: exige um JWT válido (emitido tanto pelo
	// login de funcionário quanto pela Lambda de CPF) — a própria handler
	// (GetByCustomerID) restringe um token de cliente ao seu próprio
	// customer_id, mas deixa um token de funcionário consultar qualquer um.
	customerFacing := r.Group("/orders")
	customerFacing.Use(middleware.AuthMiddleware())
	{
		customerFacing.GET("/customer/:customer_id", orderHandler.GetByCustomerID)
	}

	// Rotas internas: chamadas por outros serviços com token de serviço
	// (role "service"), nunca expostas no API Gateway. A função de
	// autenticação por CPF consulta o cliente aqui, sem acessar este banco.
	internal := r.Group("/internal")
	internal.Use(middleware.AuthMiddleware(), middleware.RequireRole("service"))
	{
		internal.GET("/customers/by-cpf/:cpf", customerHandler.GetByCPFInternal)
	}

	// Rotas protegidas (funcionário da oficina)
	admin := r.Group("/")
	admin.Use(middleware.AuthMiddleware())
	{
		customers := admin.Group("/customers")
		{
			customers.POST("/", customerHandler.Create)
			customers.GET("/", customerHandler.GetAll)
			customers.GET("/:id", customerHandler.GetByID)
			customers.PUT("/:id", customerHandler.Update)
			customers.PATCH("/:id/status", customerHandler.UpdateStatus)
			customers.DELETE("/:id", customerHandler.Delete)
		}

		vehicles := admin.Group("/vehicles")
		{
			vehicles.POST("/", vehicleHandler.Create)
			vehicles.GET("/", vehicleHandler.GetAll)
			vehicles.GET("/:id", vehicleHandler.GetByID)
			vehicles.GET("/customer/:customer_id", vehicleHandler.GetByCustomerID)
			vehicles.PUT("/:id", vehicleHandler.Update)
			vehicles.DELETE("/:id", vehicleHandler.Delete)
		}

		services := admin.Group("/services")
		{
			services.POST("/", serviceHandler.Create)
			services.GET("/", serviceHandler.GetAll)
			services.GET("/:id", serviceHandler.GetByID)
			services.PUT("/:id", serviceHandler.Update)
			services.DELETE("/:id", serviceHandler.Delete)
		}

		orders := admin.Group("/orders")
		{
			orders.POST("/", orderHandler.Create)
			orders.GET("/", orderHandler.GetAll)
			orders.GET("/metrics/average-time", orderHandler.GetAverageServiceTime)
			orders.GET("/metrics/average-time-by-status", orderHandler.GetAverageTimeByStatus)
			orders.GET("/:id", orderHandler.GetByID)
			orders.PATCH("/:id/status", orderHandler.UpdateStatus)
			orders.DELETE("/:id", orderHandler.Delete)
		}
	}

	srv := &http.Server{Addr: ":8080", Handler: r, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("erro no servidor HTTP: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("erro ao encerrar o servidor: %v", err)
	}
	if nrApp != nil {
		nrApp.Shutdown(5 * time.Second)
	}
}

// startSagaWorkers inicia, em segundo plano, o consumidor de eventos, a
// publicação da outbox e a verificação de prazos da Saga.
func startSagaWorkers(ctx context.Context, orchestrator *usecase.SagaOrchestrator, sagaRepo *repository.SagaPostgresRepository) {
	awsCfg, err := config.LoadAWSConfig(ctx)
	if err != nil {
		log.Fatalf("erro ao carregar configuração da AWS: %v", err)
	}
	sqsClient := sqs.NewFromConfig(awsCfg)

	queueURL := func(env, name string) string {
		url, err := messaging.QueueURL(ctx, sqsClient, config.Env(env, name))
		if err != nil {
			log.Fatal(err)
		}
		return url
	}
	sender := messaging.NewSender(sqsClient, map[string]string{
		messaging.QueueExecution: queueURL("EXECUTION_COMMANDS_QUEUE", messaging.QueueExecution),
		messaging.QueueBilling:   queueURL("BILLING_COMMANDS_QUEUE", messaging.QueueBilling),
	})
	eventsURL := queueURL("OS_EVENTS_QUEUE", "os-events")

	go consumer.NewSQSConsumer(sqsClient, eventsURL, orchestrator, sagaRepo).Run(ctx)

	relay := worker.NewOutboxRelay(sagaRepo, sender)
	go worker.Every(ctx, time.Second, func(time.Time) { relay.PublishPending(ctx) })
	go worker.Every(ctx, 30*time.Second, func(now time.Time) { worker.CheckTimeouts(orchestrator, now) })
}
