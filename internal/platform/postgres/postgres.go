package postgres

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/MykolaShev/fleet-management-saas/internal/domain"
)

// Config holds Postgres connection parameters.
type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// Connect opens a GORM connection to Postgres.
func Connect(cfg Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}

	return db, nil
}

// AutoMigrate creates/updates tables for all domain models. Order matters:
// Tenant has no foreign keys, so it must be migrated first.
//
// Anything GORM can't express (extensions, PostGIS columns, custom indexes)
// belongs in migrations/ as raw SQL instead.
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&domain.Tenant{},
		&domain.User{},
		&domain.Vehicle{},
		&domain.Delivery{},
	)
}
