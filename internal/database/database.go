package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"github.com/kkkkikiki/coupon/internal/config"
)

// DB holds database connections
type DB struct {
	Postgres *sqlx.DB
}

// NewDB creates new database connections using config
func NewDB(ctx context.Context, cfg *config.Config) (*DB, error) {
	// Connect to PostgreSQL
	postgres, err := sqlx.Connect("postgres", cfg.Database.GetDatabaseURL())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	// Configure connection pool
	postgres.SetMaxOpenConns(cfg.Database.MaxConns)
	postgres.SetMaxIdleConns(cfg.Database.MinConns)
	postgres.SetConnMaxLifetime(time.Hour)

	// Test PostgreSQL connection
	if err := postgres.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}

	log.Println("Successfully connected to PostgreSQL")

	return &DB{
		Postgres: postgres,
	}, nil
}

// Close closes all database connections
func (db *DB) Close() error {
	if err := db.Postgres.Close(); err != nil {
		return fmt.Errorf("failed to close PostgreSQL: %w", err)
	}

	return nil
}
