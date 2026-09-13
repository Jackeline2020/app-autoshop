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

	_ "autoshop/docs" // swagger docs gerados automaticamente pelo swag

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func main() {
	r := gin.Default()

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

	// Rotas publicas
	r.POST("/auth/login", authHandler.Login)
	r.GET("/orders/customer/:customer_id", orderHandler.GetByCustomerID)
	r.GET("/orders/:id/status", orderHandler.GetStatus)
	r.PATCH("/orders/:id/approve", orderHandler.ApproveOrder)

	// Integração email, atualização de status via webhook, protegida por secret
	emailIntegration := r.Group("/integrations/email")
	emailIntegration.Use(middleware.EmailWebhookMiddleware())
	{
		emailIntegration.POST("/orders/status", emailHandler.UpdateStatus)
	}

	// Rotas protegidas
	admin := r.Group("/")
	admin.Use(middleware.AuthMiddleware())
	{
		customers := admin.Group("/customers")
		{
			customers.POST("/", customerHandler.Create)
			customers.GET("/", customerHandler.GetAll)
			customers.GET("/:id", customerHandler.GetByID)
			customers.PUT("/:id", customerHandler.Update)
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
