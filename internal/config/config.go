package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all runtime configuration read from the environment.
type Config struct {
	AppEnv  string // "local" enables dev-only routes (see handler.RegisterDevRoutes)
	AppPort string

	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	// OAuth2 (Google). See README.md for how to obtain these — a free
	// Google Cloud project, no billing account required.
	OAuthClientID     string
	OAuthClientSecret string
	OAuthRedirectURL  string

	// JWTSecret signs our own session tokens (separate from Google's).
	JWTSecret string

	// LLM (Groq, OpenAI-compatible).
	LLMAPIKey string
	LLMModel  string

	// B9 cache. CacheBackend is "memory" (default, no extra
	// infrastructure) or "redis" (needs REDIS_ADDR reachable — see
	// docker-compose, which already runs Redis for this).
	CacheBackend  string
	CacheTTL      time.Duration
	RedisAddr     string
	RedisPassword string

	CORSAllowedOrigins []string

	// cmd/api publishes, cmd/worker
	// consumes and sends the Telegram message.
	RabbitMQURL      string
	TelegramBotToken string
	TelegramChatID   string
}

// Load reads a .env file if present (local dev convenience) and falls back
// to whatever is already in the process environment (Docker/cloud).
func Load() Config {
	if err := godotenv.Load(); err != nil {
		log.Println("config: no .env file found, relying on process environment")
	}

	return Config{
		AppEnv:  getEnv("APP_ENV", "local"),
		AppPort: getEnv("APP_PORT", "8080"),

		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", "postgres"),
		DBName:     getEnv("DB_NAME", "fleet_management"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),

		OAuthClientID:     getEnv("OAUTH_CLIENT_ID", ""),
		OAuthClientSecret: getEnv("OAUTH_CLIENT_SECRET", ""),
		OAuthRedirectURL:  getEnv("OAUTH_REDIRECT_URL", "http://localhost:8080/api/v1/auth/callback"),

		JWTSecret: getEnv("JWT_SECRET", "change-me"),

		LLMAPIKey: getEnv("LLM_API_KEY", ""),
		LLMModel:  getEnv("LLM_MODEL", "openai/gpt-oss-20b"),

		CacheBackend:  getEnv("CACHE_BACKEND", "memory"),
		CacheTTL:      time.Duration(getEnvInt("CACHE_TTL_SECONDS", 15)) * time.Second,
		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),

		CORSAllowedOrigins: strings.Split(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:5173"), ","),

		RabbitMQURL:      getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		TelegramBotToken: getEnv("TELEGRAM_BOT_TOKEN", ""),
		TelegramChatID:   getEnv("TELEGRAM_CHAT_ID", ""),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		log.Printf("config: invalid %s=%q, using default of %d", key, v, fallback)
		return fallback
	}
	return n
}