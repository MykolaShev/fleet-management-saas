// Package main is the entrypoint for the Fleet Management SaaS API server.
//
// At this stage it wires up config + database (schema via AutoMigrate) and
// exposes a /health endpoint. CRUD routes are added as part of the REST API
// stage.
package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/MykolaShev/fleet-management-saas/internal/config"
	"github.com/MykolaShev/fleet-management-saas/internal/handler"
	"github.com/MykolaShev/fleet-management-saas/internal/platform/postgres"
	"github.com/MykolaShev/fleet-management-saas/internal/repository"
	"github.com/MykolaShev/fleet-management-saas/internal/service"
)

func main() {
	cfg := config.Load()

	db, err := postgres.Connect(postgres.Config{
		Host:     cfg.DBHost,
		Port:     cfg.DBPort,
		User:     cfg.DBUser,
		Password: cfg.DBPassword,
		DBName:   cfg.DBName,
		SSLMode:  cfg.DBSSLMode,
	})
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}

	if err := postgres.AutoMigrate(db); err != nil {
		log.Fatalf("automigrate failed: %v", err)
	}
	log.Println("database schema is up to date")

	tenantRepo := repository.NewTenantRepository(db)

	vehicleRepo := repository.NewVehicleRepository(db)
	vehicleService := service.NewVehicleService(vehicleRepo)
	vehicleHandler := handler.NewVehicleHandler(vehicleService)

	userRepo := repository.NewUserRepository(db)
	userService := service.NewUserService(userRepo)
	userHandler := handler.NewUserHandler(userService)

	deliveryRepo := repository.NewDeliveryRepository(db)
	deliveryService := service.NewDeliveryService(deliveryRepo, vehicleRepo, userRepo)
	deliveryHandler := handler.NewDeliveryHandler(deliveryService)

	router := gin.Default()
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	api := router.Group("/api/v1", handler.EnsureTenant(tenantRepo))
	vehicleHandler.RegisterRoutes(api)
	userHandler.RegisterRoutes(api)
	deliveryHandler.RegisterRoutes(api)

	log.Printf("starting server on :%s", cfg.AppPort)
	if err := router.Run(":" + cfg.AppPort); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
