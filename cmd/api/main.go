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
	"autoshop/internal/handler"
	"autoshop/internal/middleware"
	"autoshop/internal/repository"
	"autoshop/internal/usecase"
	"autoshop/pkg/config"
	"autoshop/pkg/observability"

	_ "autoshop/docs" // swagger docs gerados automaticamente pelo swag

	"github.com/gin-gonic/gin"
	"github.com/newrelic/go-agent/v3/integrations/nrgin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func main() {
	// Inicializa o agente do New Relic (observabilidade — Fase 3). App fica
	// nil se NEW_RELIC_LICENSE_KEY não estiver configurada, e todo o resto do
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

	partRepo := repository.NewPartPostgresRepository(db)
	partUC := usecase.NewPartUseCase(partRepo)
	partHandler := handler.NewPartHandler(partUC)

	orderRepo := repository.NewOrderPostgresRepository(db)
	orderUC := usecase.NewOrderUseCase(orderRepo, customerRepo, vehicleRepo, serviceRepo, partRepo)
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

		parts := admin.Group("/parts")
		{
			parts.POST("/", partHandler.Create)
			parts.GET("/", partHandler.GetAll)
			parts.GET("/low-stock", partHandler.GetLowStock)
			parts.GET("/:id", partHandler.GetByID)
			parts.PUT("/:id", partHandler.Update)
			parts.PATCH("/:id/stock", partHandler.AdjustStock)
			parts.DELETE("/:id", partHandler.Delete)
		}

		orders := admin.Group("/orders")
		{
			orders.POST("/", orderHandler.Create)
			orders.GET("/", orderHandler.GetAll)
			orders.GET("/metrics/average-time", orderHandler.GetAverageServiceTime)
			orders.GET("/:id", orderHandler.GetByID)
			orders.PATCH("/:id/status", orderHandler.UpdateStatus)
			orders.DELETE("/:id", orderHandler.Delete)
		}
	}

	r.Run(":8080")
}
