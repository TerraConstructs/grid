package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
)

// Config holds the application configuration
type Config struct {
	// Database connection string (DSN)
	DatabaseURL string `mapstructure:"database_url"`

	// Server bind address (host:port)
	ServerAddr string `mapstructure:"server_addr"`

	// Base URL for backend config generation
	ServerURL string `mapstructure:"server_url"`

	// Maximum database connection pool size
	MaxDBConnections int `mapstructure:"max_db_connections"`

	// Enable debug logging
	Debug bool `mapstructure:"debug"`

	// IAM cache refresh interval (default: 5m)
	CacheRefreshInterval time.Duration `mapstructure:"cache_refresh_interval"`

	// OIDC authentication configuration
	OIDC OIDCConfig `mapstructure:"oidc"`
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
	Issuer string `mapstructure:"issuer"`

	// ClientID is the audience claim for tokens issued by the Internal IdP (Mode 2)
	// It represents the Grid API itself.
	ClientID string `mapstructure:"client_id"`

	// SigningKeyPath is the path where the OIDC provider's signing key and kid is stored (Mode 2 only)
	// If empty, defaults to a system temp directory
	// Key is persisted to disk to ensure tokens remain valid across server restarts
	SigningKeyPath string `mapstructure:"signing_key_path"`

	// External IdP Configuration (Mode 1)
	// When configured, Grid acts as Resource Server validating external tokens
	// Leave nil for Mode 2 (Internal IdP Only)
	ExternalIdP *ExternalIdPConfig `mapstructure:"external_idp"`

	// JWT claim extraction configuration (applies to both modes)
	GroupsClaimField string `mapstructure:"groups_claim_field"` // Default: "groups"
	GroupsClaimPath  string `mapstructure:"groups_claim_path"`  // Optional: for nested extraction (e.g., "name" for [{name:"dev"}])
	UserIDClaimField string `mapstructure:"user_id_claim_field"` // Default: "sub"
	EmailClaimField  string `mapstructure:"email_claim_field"`  // Default: "email"
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
	Issuer       string   `mapstructure:"issuer"`       // External IdP's issuer URL (e.g., "https://login.microsoftonline.com/tenant-id/v2.0")
	ClientID     string   `mapstructure:"client_id"`    // Grid's confidential client ID for server-side operations (SSO callback)
	CLIClientID  string   `mapstructure:"cli_client_id"` // Public client ID for CLI device flow (e.g., "gridctl")
	ClientSecret string   `mapstructure:"client_secret"` // Grid's client secret with external IdP (for confidential client only)
	RedirectURI  string   `mapstructure:"redirect_uri"`  // Grid's SSO callback URL (e.g., "https://grid.example.com/auth/sso/callback")
	Scopes       []string `mapstructure:"scopes"`       // Optional: Additional OIDC scopes beyond default ["openid", "profile", "email"]
}

// Load reads configuration from Viper with support for:
// - Config files (YAML/JSON/TOML)
// - Environment variables (prefixed with GRID_)
// - Defaults
func Load() (*Config, error) {
	// Use the global viper instance (configured by root command's initConfig)
	v := viper.GetViper()

	// Set defaults
	setDefaults(v)

	// Enable automatic environment variable reading with GRID_ prefix
	// Order matters: SetEnvKeyReplacer must be called before AutomaticEnv
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.SetEnvPrefix("GRID")
	v.AutomaticEnv()

	// NOTE: Nested struct fields are populated from env vars because setDefaults()
	// already registered all keys with v.SetDefault(). This allows AutomaticEnv() to work.

	// Unmarshal into Config struct
	cfg := &Config{}

	decoderConfig := &mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.StringToSliceHookFunc(","),
		),
		Result:           cfg,
		WeaklyTypedInput: true,
		TagName:          "mapstructure",
	}

	decoder, err := mapstructure.NewDecoder(decoderConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create decoder: %w", err)
	}

	if err := decoder.Decode(v.AllSettings()); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Special handling for ExternalIdP: Check if it's effectively unconfigured
	// Since we set defaults, the struct is always created. We consider it "unconfigured"
	// only if all critical fields are empty.
	if cfg.OIDC.ExternalIdP != nil {
		ext := cfg.OIDC.ExternalIdP
		// Check if any critical field is set (ignore defaults like CLIClientID)
		isConfigured := ext.Issuer != "" || ext.ClientID != "" || ext.ClientSecret != "" || ext.RedirectURI != ""

		if !isConfigured {
			cfg.OIDC.ExternalIdP = nil
		}
	}

	// Validate configuration
	if err := validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// setDefaults configures default values for all configuration options
func setDefaults(v *viper.Viper) {
	// Database defaults
	v.SetDefault("database_url", "") // Register key for Env var lookup, but force explicit value
	v.SetDefault("server_addr", "localhost:8080")
	v.SetDefault("server_url", "")   // Register key for Env var lookup, but force explicit value
	v.SetDefault("max_db_connections", 25)
	v.SetDefault("debug", false)
	v.SetDefault("cache_refresh_interval", "5m")

	// OIDC defaults
	v.SetDefault("oidc.groups_claim_field", "groups")
	v.SetDefault("oidc.groups_claim_path", "")
	v.SetDefault("oidc.user_id_claim_field", "sub")
	v.SetDefault("oidc.email_claim_field", "email")

	// Explicitly set defaults for nested OIDC keys so Viper knows they exist
	// and can populate them from environment variables during Unmarshal.
	v.SetDefault("oidc.issuer", "")
	v.SetDefault("oidc.client_id", "")
	v.SetDefault("oidc.signing_key_path", "")
	v.SetDefault("oidc.external_idp.issuer", "")
	v.SetDefault("oidc.external_idp.client_id", "")
	v.SetDefault("oidc.external_idp.client_secret", "")
	v.SetDefault("oidc.external_idp.redirect_uri", "")

	// Default CLI client ID for external IdP
	v.SetDefault("oidc.external_idp.cli_client_id", "gridctl")

	// Default OIDC scopes for external IdP (matches pre-Viper behavior)
	// Without "openid" scope, IdP will treat request as OAuth2-only and won't return id_token
	v.SetDefault("oidc.external_idp.scopes", []string{"openid", "profile", "email"})
}

// validate performs configuration validation
func validate(cfg *Config) error {
	// Validate required fields
	if cfg.DatabaseURL == "" {
		return fmt.Errorf("database_url is required (set GRID_DATABASE_URL or add to config file)")
	}

	if cfg.ServerURL == "" {
		return fmt.Errorf("server_url is required (set GRID_SERVER_URL or add to config file)")
	}

	// OIDC mode validation
	modeExternal := cfg.OIDC.ExternalIdP != nil
	modeInternal := cfg.OIDC.Issuer != ""

	// Enforce mutually exclusive modes
	if modeExternal && modeInternal {
		return fmt.Errorf("OIDC config error: cannot enable both External IdP mode (GRID_OIDC_EXTERNAL_IDP_*) and Internal IdP mode (GRID_OIDC_ISSUER). Choose exactly one authentication mode.")
	}

	// Validate Mode 1: External IdP Only
	if modeExternal {
		if cfg.OIDC.ExternalIdP.Issuer == "" {
			return fmt.Errorf("GRID_OIDC_EXTERNAL_IDP_ISSUER is required for External IdP mode")
		}
		if cfg.OIDC.ExternalIdP.ClientID == "" {
			return fmt.Errorf("GRID_OIDC_EXTERNAL_IDP_CLIENT_ID is required for External IdP mode")
		}
		if cfg.OIDC.ExternalIdP.ClientSecret == "" {
			return fmt.Errorf("GRID_OIDC_EXTERNAL_IDP_CLIENT_SECRET is required for External IdP mode")
		}
		if cfg.OIDC.ExternalIdP.RedirectURI == "" {
			return fmt.Errorf("GRID_OIDC_EXTERNAL_IDP_REDIRECT_URI is required for External IdP mode")
		}
	}

	// Mode 2: Internal IdP Only - no additional validation needed here
	// Provider initialization in oidc.go will validate Issuer is set

	return nil
}
