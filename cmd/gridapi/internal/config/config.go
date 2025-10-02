package config

import (
	"fmt"
	"os"
)

// Config holds the application configuration
type Config struct {
	// Database connection string (DSN)
	DatabaseURL string

	// Server bind address (host:port)
	ServerAddr string

	// Base URL for backend config generation
	ServerURL string

	// Maximum database connection pool size
	MaxDBConnections int

	// Enable debug logging
	Debug bool
}

// Load reads configuration from environment variables with fallback defaults
func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:      getEnv("DATABASE_URL", "postgres://grid:gridpass@localhost:5432/grid?sslmode=disable"),
		ServerAddr:       getEnv("SERVER_ADDR", "localhost:8080"),
		ServerURL:        getEnv("SERVER_URL", "http://localhost:8080"),
		MaxDBConnections: getEnvInt("MAX_DB_CONNECTIONS", 25),
		Debug:            getEnvBool("DEBUG", false),
	}

	// Validate required fields
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	if cfg.ServerURL == "" {
		return nil, fmt.Errorf("SERVER_URL is required")
	}

	return cfg, nil
}

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt retrieves an integer environment variable or returns a default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var result int
		if _, err := fmt.Sscanf(value, "%d", &result); err == nil {
			return result
		}
	}
	return defaultValue
}

// getEnvBool retrieves a boolean environment variable or returns a default value
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true" || value == "1" || value == "yes"
	}
	return defaultValue
}
