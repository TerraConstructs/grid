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

	// OIDC authentication configuration
	OIDC OIDCConfig

	// Casbin model file path
	CasbinModelPath string
}

// OIDCConfig holds OIDC configuration for Grid's authentication.
// Grid supports two mutually exclusive deployment modes:
//
// Mode 1: External IdP Only (Resource Server)
//   - Grid validates tokens issued by external IdP (Keycloak, Entra ID, Okta)
//   - Service accounts are IdP clients (use client credentials against IdP)
//   - Config: ExternalIdP != nil, Issuer = ""
//
// Mode 2: Internal IdP Only (Self-Contained Provider)
//   - Grid issues and validates its own tokens
//   - Service accounts managed in Grid (use client credentials against Grid)
//   - Config: Issuer != "", ExternalIdP = nil
//
// A deployment must choose exactly ONE mode. Hybrid mode is not supported.
type OIDCConfig struct {
	// Grid as Internal OIDC Provider (Mode 2)
	// Issuer is Grid's own issuer URL (e.g., "https://grid.example.com")
	// Leave empty for Mode 1 (External IdP Only)
	Issuer string

	// ClientID is the audience claim for tokens issued by the Internal IdP (Mode 2)
	// It represents the Grid API itself.
	ClientID string

	// SigningKeyPath is the path where the OIDC provider's signing key is stored (Mode 2 only)
	// If empty, defaults to a system temp directory
	// Key is persisted to disk to ensure tokens remain valid across server restarts
	SigningKeyPath string

	// External IdP Configuration (Mode 1)
	// When configured, Grid acts as Resource Server validating external tokens
	// Leave nil for Mode 2 (Internal IdP Only)
	ExternalIdP *ExternalIdPConfig

	// JWT claim extraction configuration (applies to both modes)
	GroupsClaimField string // Default: "groups"
	GroupsClaimPath  string // Optional: for nested extraction (e.g., "name" for [{name:"dev"}])
	UserIDClaimField string // Default: "sub"
	EmailClaimField  string // Default: "email"
}

// IsInternalIdPMode returns true if Grid is configured as an Internal IdP (Mode 2)
func (c *OIDCConfig) IsInternalIdPMode() bool {
	return c.Issuer != "" && c.ExternalIdP == nil
}

// ExternalIdPConfig holds configuration for external identity providers (Keycloak, Azure Entra ID, Okta, etc.)
// This enables Mode 1: External IdP Only - Grid acts as Resource Server validating external tokens.
//
// In this mode:
// - Human users authenticate via SSO (web or CLI device flow proxied to IdP)
// - Service accounts are IdP clients created in the external IdP (not in Grid)
// - Grid validates tokens but never issues them
type ExternalIdPConfig struct {
	Issuer       string   // External IdP's issuer URL (e.g., "https://login.microsoftonline.com/tenant-id/v2.0")
	ClientID     string   // Grid's client ID registered with external IdP
	ClientSecret string   // Grid's client secret with external IdP
	RedirectURI  string   // Grid's SSO callback URL (e.g., "https://grid.example.com/auth/sso/callback")
	Scopes       []string // Optional: Additional OIDC scopes beyond default ["openid", "profile", "email"]
}

// Load reads configuration from environment variables with fallback defaults
func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:      getEnv("DATABASE_URL", "postgres://grid:gridpass@localhost:5432/grid?sslmode=disable"),
		ServerAddr:       getEnv("SERVER_ADDR", "localhost:8080"),
		ServerURL:        getEnv("SERVER_URL", "http://localhost:8080"),
		MaxDBConnections: getEnvInt("MAX_DB_CONNECTIONS", 25),
		Debug:            getEnvBool("DEBUG", false),
		CasbinModelPath:  getEnv("CASBIN_MODEL_PATH", "cmd/gridapi/casbin/model.conf"),
		OIDC: OIDCConfig{
			Issuer:           getEnv("OIDC_ISSUER", ""),
			ClientID:         getEnv("OIDC_CLIENT_ID", ""),
			SigningKeyPath:   getEnv("OIDC_SIGNING_KEY_PATH", ""),
			ExternalIdP:      loadExternalIdPConfig(),
			GroupsClaimField: getEnv("OIDC_GROUPS_CLAIM", "groups"),
			GroupsClaimPath:  getEnv("OIDC_GROUPS_PATH", ""),
			UserIDClaimField: getEnv("OIDC_USER_ID_CLAIM", "sub"),
			EmailClaimField:  getEnv("OIDC_EMAIL_CLAIM", "email"),
		},
	}

	// Validate required fields
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	if cfg.ServerURL == "" {
		return nil, fmt.Errorf("SERVER_URL is required")
	}

	// OIDC configuration is optional (can run without auth for development)
	// Grid supports two mutually exclusive authentication modes:
	// - Mode 1: External IdP Only (ExternalIdP != nil, Issuer = "")
	// - Mode 2: Internal IdP Only (Issuer != "", ExternalIdP = nil)
	// Hybrid mode is NOT supported.

	modeExternal := cfg.OIDC.ExternalIdP != nil
	modeInternal := cfg.OIDC.Issuer != ""

	// Enforce mutually exclusive modes
	if modeExternal && modeInternal {
		return nil, fmt.Errorf("OIDC config error: cannot enable both External IdP mode (EXTERNAL_IDP_*) and Internal IdP mode (OIDC_ISSUER). Choose exactly one authentication mode.")
	}

	// Validate Mode 1: External IdP Only
	if modeExternal {
		if cfg.OIDC.ExternalIdP.Issuer == "" {
			return nil, fmt.Errorf("EXTERNAL_IDP_ISSUER is required for External IdP mode")
		}
		if cfg.OIDC.ExternalIdP.ClientID == "" {
			return nil, fmt.Errorf("EXTERNAL_IDP_CLIENT_ID is required for External IdP mode")
		}
		if cfg.OIDC.ExternalIdP.ClientSecret == "" {
			return nil, fmt.Errorf("EXTERNAL_IDP_CLIENT_SECRET is required for External IdP mode")
		}
		if cfg.OIDC.ExternalIdP.RedirectURI == "" {
			return nil, fmt.Errorf("EXTERNAL_IDP_REDIRECT_URI is required for External IdP mode")
		}
	}

	// Mode 2: Internal IdP Only - no additional validation needed here
	// Provider initialization in oidc.go will validate Issuer is set

	return cfg, nil
}

// loadExternalIdPConfig loads external IdP configuration from environment variables
// Returns nil if external IdP is not configured
func loadExternalIdPConfig() *ExternalIdPConfig {
	issuer := getEnv("EXTERNAL_IDP_ISSUER", "")
	if issuer == "" {
		return nil // External IdP not configured
	}

	scopes := []string{"openid", "profile", "email"} // Default scopes
	// Could parse EXTERNAL_IDP_SCOPES for custom scopes if needed

	return &ExternalIdPConfig{
		Issuer:       issuer,
		ClientID:     getEnv("EXTERNAL_IDP_CLIENT_ID", ""),
		ClientSecret: getEnv("EXTERNAL_IDP_CLIENT_SECRET", ""),
		RedirectURI:  getEnv("EXTERNAL_IDP_REDIRECT_URI", ""),
		Scopes:       scopes,
	}
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
