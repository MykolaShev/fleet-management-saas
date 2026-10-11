package config

import (
	"log"
	"os"

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
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}