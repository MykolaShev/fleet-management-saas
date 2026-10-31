// Package main is the entrypoint for the Fleet Management SaaS API server.
package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/MykolaShev/fleet-management-saas/internal/auth"
	"github.com/MykolaShev/fleet-management-saas/internal/cache"
	"github.com/MykolaShev/fleet-management-saas/internal/config"
	"github.com/MykolaShev/fleet-management-saas/internal/handler"
	"github.com/MykolaShev/fleet-management-saas/internal/llm"
	"github.com/MykolaShev/fleet-management-saas/internal/mcp"
	"github.com/MykolaShev/fleet-management-saas/internal/platform/postgres"
	"github.com/MykolaShev/fleet-management-saas/internal/repository"
	"github.com/MykolaShev/fleet-management-saas/internal/service"
	"github.com/MykolaShev/fleet-management-saas/internal/tracking"
)

// positionSimulatorInterval is how often C10's simulator nudges active
// vehicles — this project's stand-in for a real GPS feed.
const positionSimulatorInterval = 2 * time.Second

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

	// C22: minimal MCP server exposing the fleet tools, plus its own
	// HTTP endpoint (so it's testable/demoable independently of the LLM
	// agent) and the AI agent, which calls it in-process for tool-calling.
	mcpServer := mcp.NewServer()
	mcp.RegisterFleetTools(mcpServer, vehicleService, deliveryService, deliveryService, userService)
	mcpHandler := handler.NewMCPHandler(mcpServer)

	groqClient := llm.NewGroqClient(cfg.LLMAPIKey, cfg.LLMModel)
	agentService := service.NewAgentService(groqClient, vehicleService, deliveryService, mcpServer)
	agentHandler := handler.NewAgentHandler(agentService)

	// C10: WS/long-poll/adaptive-poll all serve the same live positions.
	// The simulator runs for the life of the process — no graceful
	// shutdown plumbing exists yet (see B7/deploy stages), so
	// context.Background() is fine here for now.
	trackingHub := tracking.NewHub()
	simulator := tracking.NewSimulator(vehicleRepo, trackingHub, positionSimulatorInterval)
	go simulator.Run(context.Background())
	trackingHandler := handler.NewTrackingHandler(trackingHub)

	// B9: pick the cache backend by config, not by what's "nicer" — both
	// implement cache.Cache identically from the middleware's point of
	// view. Fails fast on a configured-but-unreachable Redis rather than
	// silently degrading, same as the DB connection above.
	var httpCache cache.Cache
	switch cfg.CacheBackend {
	case "redis":
		redisCache := cache.NewRedis(cfg.RedisAddr, cfg.RedisPassword)
		pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := redisCache.Ping(pingCtx); err != nil {
			log.Fatalf("CACHE_BACKEND=redis but Redis at %s is unreachable: %v", cfg.RedisAddr, err)
		}
		httpCache = redisCache
		log.Printf("cache backend: redis (%s)", cfg.RedisAddr)
	default:
		httpCache = cache.NewMemory()
		log.Println("cache backend: memory")
	}

	router := gin.Default()
	router.Use(handler.CORS(cfg.CORSAllowedOrigins))
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	// C8 demo page — not F5 (the real SPA), just enough to visually prove
	// the WS feed drives a real map. See web/map.html's own comments.
	router.StaticFile("/map", "./web/map.html")

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

	// B9's cache sits only on plain CRUD routes — never on agentHandler
	// (GET /agent/chat is SSE; caching a chat reply by its question text
	// would return a stale answer to a repeated question, not a fresh
	// LLM call) or trackingHandler (GET /vehicles/ws is a long-lived
	// WebSocket upgrade, not a request with a cacheable body).
	cached := protected.Group("", handler.CacheMiddleware(httpCache, cfg.CacheTTL))
	vehicleHandler.RegisterRoutes(cached)
	userHandler.RegisterRoutes(cached)
	deliveryHandler.RegisterRoutes(cached)
	featureFlagHandler.RegisterRoutes(cached)

	agentHandler.RegisterRoutes(protected)
	mcpHandler.RegisterRoutes(protected)
	trackingHandler.RegisterRoutes(protected)

	log.Printf("starting server on :%s", cfg.AppPort)
	if err := router.Run(":" + cfg.AppPort); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}