package config

import (
	"context"
	"fmt"

	"github.com/sethvargo/go-envconfig"
)

// Config holds all application configuration
type Config struct {
	// Server configuration
	Server ServerConfig `env:",prefix=SERVER_"`

	// Database configuration
	Database DatabaseConfig `env:",prefix=DB_"`

	// Application configuration
	App AppConfig `env:",prefix=APP_"`
}

// ServerConfig holds server-related configuration
type ServerConfig struct {
	Port         string `env:"PORT,default=8080"`
	Host         string `env:"HOST,default=0.0.0.0"`
	ReadTimeout  int    `env:"READ_TIMEOUT,default=30"`  // seconds
	WriteTimeout int    `env:"WRITE_TIMEOUT,default=30"` // seconds
}

// DatabaseConfig holds PostgreSQL configuration
type DatabaseConfig struct {
	Host     string `env:"HOST,default=localhost"`
	Port     string `env:"PORT,default=5432"`
	User     string `env:"USER,default=postgres"`
	Password string `env:"PASSWORD,default=postgres"`
	Name     string `env:"NAME,default=coupon_system"`
	SSLMode  string `env:"SSL_MODE,default=disable"`
	MaxConns int    `env:"MAX_CONNS,default=25"`
	MinConns int    `env:"MIN_CONNS,default=5"`
}

// AppConfig holds application-specific configuration
type AppConfig struct {
	Environment string `env:"ENVIRONMENT,default=development"`
	LogLevel    string `env:"LOG_LEVEL,default=info"`
	Debug       bool   `env:"DEBUG,default=false"`
}

// Load loads configuration from environment variables
func Load(ctx context.Context) (*Config, error) {
	var cfg Config
	if err := envconfig.Process(ctx, &cfg); err != nil {
		return nil, fmt.Errorf("failed to process environment config: %w", err)
	}
	return &cfg, nil
}

// GetDatabaseURL returns the PostgreSQL connection URL
func (c *DatabaseConfig) GetDatabaseURL() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode)
}

// GetServerAddr returns the server address
func (c *ServerConfig) GetServerAddr() string {
	return fmt.Sprintf("%s:%s", c.Host, c.Port)
}

// IsDevelopment returns true if running in development environment
func (c *AppConfig) IsDevelopment() bool {
	return c.Environment == "development"
}

// IsProduction returns true if running in production environment
func (c *AppConfig) IsProduction() bool {
	return c.Environment == "production"
}
