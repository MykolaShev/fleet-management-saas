// Package main is the entrypoint for the Fleet Management SaaS API server.
package main

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/MykolaShev/fleet-management-saas/internal/auth"
	"github.com/MykolaShev/fleet-management-saas/internal/config"
	"github.com/MykolaShev/fleet-management-saas/internal/handler"
	"github.com/MykolaShev/fleet-management-saas/internal/llm"
	"github.com/MykolaShev/fleet-management-saas/internal/platform/postgres"
	"github.com/MykolaShev/fleet-management-saas/internal/repository"
	"github.com/MykolaShev/fleet-management-saas/internal/service"
)

// sessionTokenTTL is how long a session JWT stays valid after login.
const sessionTokenTTL = 24 * time.Hour

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

	featureFlagRepo := repository.NewFeatureFlagRepository(db)
	featureFlagService := service.NewFeatureFlagService(featureFlagRepo)
	featureFlagHandler := handler.NewFeatureFlagHandler(featureFlagService)

	jwtIssuer := auth.NewJWTIssuer(cfg.JWTSecret, sessionTokenTTL)
	oauthConfig := auth.NewOAuth2Config(auth.GoogleConfig{
		ClientID:     cfg.OAuthClientID,
		ClientSecret: cfg.OAuthClientSecret,
		RedirectURL:  cfg.OAuthRedirectURL,
	})
	authService := service.NewAuthService(tenantRepo, userRepo, jwtIssuer)
	authHandler := handler.NewAuthHandler(oauthConfig, authService, jwtIssuer, tenantRepo)

	groqClient := llm.NewGroqClient(cfg.LLMAPIKey, cfg.LLMModel)
	agentService := service.NewAgentService(groqClient, vehicleService, deliveryService)
	agentHandler := handler.NewAgentHandler(agentService)

	router := gin.Default()
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	api := router.Group("/api/v1")

	// Public: this is how a client gets a token in the first place, so it
	// must sit outside the Authenticate middleware below.
	authHandler.RegisterRoutes(api)
	if cfg.AppEnv == "local" {
		// Dev-only shortcut so Postman/CI can get a valid session token
		// without a real Google account. Never registered outside local
		// development — see AuthHandler.RegisterDevRoutes.
		authHandler.RegisterDevRoutes(api)
		log.Println("APP_ENV=local: /api/v1/auth/dev-token is enabled")
	}

	protected := api.Group("", handler.Authenticate(jwtIssuer))
	vehicleHandler.RegisterRoutes(protected)
	userHandler.RegisterRoutes(protected)
	deliveryHandler.RegisterRoutes(protected)
	featureFlagHandler.RegisterRoutes(protected)
	agentHandler.RegisterRoutes(protected)

	log.Printf("starting server on :%s", cfg.AppPort)
	if err := router.Run(":" + cfg.AppPort); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
