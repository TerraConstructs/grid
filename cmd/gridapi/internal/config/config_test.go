package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoad_WithEnvironmentVariables tests that GRID_ prefixed environment variables work
func TestLoad_WithEnvironmentVariables(t *testing.T) {
	defer func() {
		os.Unsetenv("GRID_DATABASE_URL")
		os.Unsetenv("GRID_SERVER_URL")
		os.Unsetenv("GRID_SERVER_ADDR")
		os.Unsetenv("GRID_DEBUG")
		os.Unsetenv("GRID_MAX_DB_CONNECTIONS")
	}()

	os.Setenv("GRID_DATABASE_URL", "postgres://env:env@localhost:5432/env")
	os.Setenv("GRID_SERVER_URL", "http://env:9090")
	os.Setenv("GRID_SERVER_ADDR", "env:9090")
	os.Setenv("GRID_DEBUG", "true")
	os.Setenv("GRID_MAX_DB_CONNECTIONS", "50")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "postgres://env:env@localhost:5432/env", cfg.DatabaseURL)
	assert.Equal(t, "http://env:9090", cfg.ServerURL)
	assert.Equal(t, "env:9090", cfg.ServerAddr)
	assert.True(t, cfg.Debug)
	assert.Equal(t, 50, cfg.MaxDBConnections)
}

// TestLoad_WithConfigFile tests config file loading
func TestLoad_WithConfigFile(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "gridapi.yaml")

	// Write config file
	configContent := `
database_url: "postgres://file:file@localhost/file"
server_url: "http://file:8888"
server_addr: "127.0.0.1:8888"
debug: true
max_db_connections: 30
oidc:
  issuer: "https://file.example.com"
  client_id: "file-client"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Configure Viper to load from the file
	// We must reset Viper to ensure clean state
	viper.Reset()
	viper.SetConfigFile(configPath)
	err = viper.ReadInConfig()
	require.NoError(t, err)

	// Clean up env vars to ensure no interference
	os.Unsetenv("GRID_DATABASE_URL")
	os.Unsetenv("GRID_SERVER_URL")
	os.Unsetenv("GRID_SERVER_ADDR")
	os.Unsetenv("GRID_DEBUG")
	os.Unsetenv("GRID_MAX_DB_CONNECTIONS")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "postgres://file:file@localhost/file", cfg.DatabaseURL)
	assert.Equal(t, "http://file:8888", cfg.ServerURL)
	assert.Equal(t, "127.0.0.1:8888", cfg.ServerAddr)
	assert.True(t, cfg.Debug)
	assert.Equal(t, 30, cfg.MaxDBConnections)
	assert.Equal(t, "https://file.example.com", cfg.OIDC.Issuer)
	assert.Equal(t, "file-client", cfg.OIDC.ClientID)
}

// TestLoad_EnvironmentVariablePrecedence tests that env vars have precedence over config file
func TestLoad_EnvironmentVariablePrecedence(t *testing.T) {
	defer func() {
		os.Unsetenv("GRID_DATABASE_URL")
		os.Unsetenv("GRID_SERVER_URL")
		os.Unsetenv("GRID_OIDC_ISSUER")
	}()

	// Create config file with one set of values
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "gridapi.yaml")
	configContent := `
database_url: "postgres://file/file"
server_url: "http://file"
oidc:
  issuer: "https://file.example.com"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	viper.Reset()
	viper.SetConfigFile(configPath)
	err = viper.ReadInConfig()
	require.NoError(t, err)

	// Set environment variables with different values
	os.Setenv("GRID_DATABASE_URL", "postgres://env/env")
	os.Setenv("GRID_SERVER_URL", "http://env")
	os.Setenv("GRID_OIDC_ISSUER", "https://env.example.com")

	cfg, err := Load()
	require.NoError(t, err)

	// Environment variables should take precedence
	assert.Equal(t, "postgres://env/env", cfg.DatabaseURL)
	assert.Equal(t, "http://env", cfg.ServerURL)
	assert.Equal(t, "https://env.example.com", cfg.OIDC.Issuer)
}

// TestLoad_WithOIDCExternalIdP tests External IdP configuration via Env Vars
func TestLoad_WithOIDCExternalIdP(t *testing.T) {
	defer func() {
		os.Unsetenv("GRID_DATABASE_URL")
		os.Unsetenv("GRID_SERVER_URL")
		os.Unsetenv("GRID_OIDC_EXTERNAL_IDP_ISSUER")
		os.Unsetenv("GRID_OIDC_EXTERNAL_IDP_CLIENT_ID")
		os.Unsetenv("GRID_OIDC_EXTERNAL_IDP_CLIENT_SECRET")
		os.Unsetenv("GRID_OIDC_EXTERNAL_IDP_REDIRECT_URI")
	}()

	// Reset Viper
	viper.Reset()

	// Set required fields
	os.Setenv("GRID_DATABASE_URL", "postgres://test/test")
	os.Setenv("GRID_SERVER_URL", "http://test")
	os.Setenv("GRID_OIDC_EXTERNAL_IDP_ISSUER", "https://idp.example.com")
	os.Setenv("GRID_OIDC_EXTERNAL_IDP_CLIENT_ID", "client-id")
	os.Setenv("GRID_OIDC_EXTERNAL_IDP_CLIENT_SECRET", "secret")
	os.Setenv("GRID_OIDC_EXTERNAL_IDP_REDIRECT_URI", "http://callback")

	cfg, err := Load()
	require.NoError(t, err)

	assert.NotNil(t, cfg.OIDC.ExternalIdP)
	assert.Equal(t, "https://idp.example.com", cfg.OIDC.ExternalIdP.Issuer)
	assert.Equal(t, "client-id", cfg.OIDC.ExternalIdP.ClientID)
	assert.Equal(t, "secret", cfg.OIDC.ExternalIdP.ClientSecret)
	assert.Equal(t, "http://callback", cfg.OIDC.ExternalIdP.RedirectURI)
	assert.Equal(t, "gridctl", cfg.OIDC.ExternalIdP.CLIClientID) // Default
	assert.Empty(t, cfg.OIDC.Issuer)                             // Internal IdP not set
}

// TestLoad_WithInternalIdP tests Internal IdP configuration via Env Vars
func TestLoad_WithInternalIdP(t *testing.T) {
	defer func() {
		os.Unsetenv("GRID_DATABASE_URL")
		os.Unsetenv("GRID_SERVER_URL")
		os.Unsetenv("GRID_OIDC_ISSUER")
		os.Unsetenv("GRID_OIDC_CLIENT_ID")
	}()

	viper.Reset()

	os.Setenv("GRID_DATABASE_URL", "postgres://test/test")
	os.Setenv("GRID_SERVER_URL", "http://test")
	os.Setenv("GRID_OIDC_ISSUER", "https://grid.example.com")
	os.Setenv("GRID_OIDC_CLIENT_ID", "grid-api")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "https://grid.example.com", cfg.OIDC.Issuer)
	assert.Equal(t, "grid-api", cfg.OIDC.ClientID)
	assert.Nil(t, cfg.OIDC.ExternalIdP) // External IdP not set
}

// TestLoad_OIDCModeConflict tests that both modes cannot be enabled simultaneously
func TestLoad_OIDCModeConflict(t *testing.T) {
	defer func() {
		os.Unsetenv("GRID_DATABASE_URL")
		os.Unsetenv("GRID_SERVER_URL")
		os.Unsetenv("GRID_OIDC_ISSUER")
		os.Unsetenv("GRID_OIDC_EXTERNAL_IDP_ISSUER")
		os.Unsetenv("GRID_OIDC_EXTERNAL_IDP_CLIENT_ID")
		os.Unsetenv("GRID_OIDC_EXTERNAL_IDP_CLIENT_SECRET")
		os.Unsetenv("GRID_OIDC_EXTERNAL_IDP_REDIRECT_URI")
	}()

	viper.Reset()

	os.Setenv("GRID_DATABASE_URL", "postgres://test/test")
	os.Setenv("GRID_SERVER_URL", "http://test")
	os.Setenv("GRID_OIDC_ISSUER", "https://grid.example.com")
	os.Setenv("GRID_OIDC_EXTERNAL_IDP_ISSUER", "https://idp.example.com")
	os.Setenv("GRID_OIDC_EXTERNAL_IDP_CLIENT_ID", "client-id")
	os.Setenv("GRID_OIDC_EXTERNAL_IDP_CLIENT_SECRET", "secret")
	os.Setenv("GRID_OIDC_EXTERNAL_IDP_REDIRECT_URI", "http://callback")

	cfg, err := Load()
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "cannot enable both")
}

// TestLoad_ExternalIdPMissingRequiredFields tests External IdP validation
func TestLoad_ExternalIdPMissingRequiredFields(t *testing.T) {
	tests := []struct {
		name        string
		unsetEnvVar string
		expectedErr string
	}{
		{
			name:        "Missing ISSUER",
			unsetEnvVar: "GRID_OIDC_EXTERNAL_IDP_ISSUER",
			expectedErr: "GRID_OIDC_EXTERNAL_IDP_ISSUER is required",
		},
		{
			name:        "Missing CLIENT_ID",
			unsetEnvVar: "GRID_OIDC_EXTERNAL_IDP_CLIENT_ID",
			expectedErr: "GRID_OIDC_EXTERNAL_IDP_CLIENT_ID is required",
		},
		{
			name:        "Missing CLIENT_SECRET",
			unsetEnvVar: "GRID_OIDC_EXTERNAL_IDP_CLIENT_SECRET",
			expectedErr: "GRID_OIDC_EXTERNAL_IDP_CLIENT_SECRET is required",
		},
		{
			name:        "Missing REDIRECT_URI",
			unsetEnvVar: "GRID_OIDC_EXTERNAL_IDP_REDIRECT_URI",
			expectedErr: "GRID_OIDC_EXTERNAL_IDP_REDIRECT_URI is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				os.Unsetenv("GRID_DATABASE_URL")
				os.Unsetenv("GRID_SERVER_URL")
				os.Unsetenv("GRID_OIDC_EXTERNAL_IDP_ISSUER")
				os.Unsetenv("GRID_OIDC_EXTERNAL_IDP_CLIENT_ID")
				os.Unsetenv("GRID_OIDC_EXTERNAL_IDP_CLIENT_SECRET")
				os.Unsetenv("GRID_OIDC_EXTERNAL_IDP_REDIRECT_URI")
			}()

			viper.Reset()

			// Set all required fields
			os.Setenv("GRID_DATABASE_URL", "postgres://test/test")
			os.Setenv("GRID_SERVER_URL", "http://test")
			os.Setenv("GRID_OIDC_EXTERNAL_IDP_ISSUER", "https://idp.example.com")
			os.Setenv("GRID_OIDC_EXTERNAL_IDP_CLIENT_ID", "client-id")
			os.Setenv("GRID_OIDC_EXTERNAL_IDP_CLIENT_SECRET", "secret")
			os.Setenv("GRID_OIDC_EXTERNAL_IDP_REDIRECT_URI", "http://callback")

			// Unset one field
			os.Unsetenv(tt.unsetEnvVar)

			cfg, err := Load()
			require.Error(t, err)
			assert.Nil(t, cfg)
			assert.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}

// TestLoad_WithDefaults tests that defaults are applied when no env vars are set
func TestLoad_WithDefaults(t *testing.T) {
	defer func() {
		os.Unsetenv("GRID_DATABASE_URL")
		os.Unsetenv("GRID_SERVER_URL")
		os.Unsetenv("GRID_SERVER_ADDR")
		os.Unsetenv("GRID_DEBUG")
		os.Unsetenv("GRID_MAX_DB_CONNECTIONS")
	}()

	// Unset optional env vars to use defaults
	os.Unsetenv("GRID_SERVER_ADDR")
	os.Unsetenv("GRID_DEBUG")
	os.Unsetenv("GRID_MAX_DB_CONNECTIONS")

	// Set required fields (since they no longer have defaults)
	os.Setenv("GRID_DATABASE_URL", "postgres://required:required@localhost:5432/grid")
	os.Setenv("GRID_SERVER_URL", "http://required:8080")

	viper.Reset()

	cfg, err := Load()
	require.NoError(t, err)

	// Required fields should match env vars
	assert.Equal(t, "postgres://required:required@localhost:5432/grid", cfg.DatabaseURL)
	assert.Equal(t, "http://required:8080", cfg.ServerURL)

	// Defaults should be applied for optional fields
	assert.Equal(t, "localhost:8080", cfg.ServerAddr)
	assert.False(t, cfg.Debug)
	assert.Equal(t, 25, cfg.MaxDBConnections)
}

// TestLoad_MissingRequiredDatabaseURL tests validation of required fields
func TestLoad_MissingRequiredDatabaseURL(t *testing.T) {
	defer func() {
		os.Unsetenv("GRID_DATABASE_URL")
		os.Unsetenv("GRID_SERVER_URL")
	}()

	viper.Reset()

	os.Unsetenv("GRID_DATABASE_URL")
	os.Setenv("GRID_SERVER_URL", "http://test")

	cfg, err := Load()
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "database_url is required")
}

// TestLoad_MissingRequiredServerURL tests validation of required fields
func TestLoad_MissingRequiredServerURL(t *testing.T) {
	defer func() {
		os.Unsetenv("GRID_DATABASE_URL")
		os.Unsetenv("GRID_SERVER_URL")
	}()

	viper.Reset()

	os.Setenv("GRID_DATABASE_URL", "postgres://test/test")
	os.Unsetenv("GRID_SERVER_URL")

	cfg, err := Load()
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "server_url is required")
}

// TestLoad_OIDCDefaults tests OIDC claim field defaults
func TestLoad_OIDCDefaults(t *testing.T) {
	defer func() {
		os.Unsetenv("GRID_DATABASE_URL")
		os.Unsetenv("GRID_SERVER_URL")
	}()

	viper.Reset()

	os.Setenv("GRID_DATABASE_URL", "postgres://test/test")
	os.Setenv("GRID_SERVER_URL", "http://test")

	cfg, err := Load()
	require.NoError(t, err)

	// Check OIDC claim field defaults
	assert.Equal(t, "groups", cfg.OIDC.GroupsClaimField)
	assert.Equal(t, "sub", cfg.OIDC.UserIDClaimField)
	assert.Equal(t, "email", cfg.OIDC.EmailClaimField)
	assert.Empty(t, cfg.OIDC.GroupsClaimPath)
}

// TestLoad_OIDCNoModeSet tests when neither OIDC mode is configured
func TestLoad_OIDCNoModeSet(t *testing.T) {
	defer func() {
		os.Unsetenv("GRID_DATABASE_URL")
		os.Unsetenv("GRID_SERVER_URL")
	}()

	viper.Reset()

	os.Setenv("GRID_DATABASE_URL", "postgres://test/test")
	os.Setenv("GRID_SERVER_URL", "http://test")

	cfg, err := Load()
	require.NoError(t, err)

	// When no mode is set, ExternalIdP should be nil
	assert.Nil(t, cfg.OIDC.ExternalIdP)
	assert.Empty(t, cfg.OIDC.Issuer)
	assert.False(t, cfg.OIDC.IsInternalIdPMode())
}

// TestLoad_ExternalIdPFromEnvVars_ViperWorkaround verifies the Viper workaround
// that ensures nested struct fields are populated from environment variables.
//
// This test ensures the workaround (explicit Get() calls) is working.
// Without the workaround, Viper's AutomaticEnv() + AllSettings() doesn't
// populate nested struct fields from environment variables.
//
// REGRESSION TEST: This test will FAIL if the workaround is removed from Load().
func TestLoad_ExternalIdPFromEnvVars_ViperWorkaround(t *testing.T) {
	defer func() {
		os.Unsetenv("GRID_DATABASE_URL")
		os.Unsetenv("GRID_SERVER_URL")
		os.Unsetenv("GRID_OIDC_EXTERNAL_IDP_ISSUER")
		os.Unsetenv("GRID_OIDC_EXTERNAL_IDP_CLIENT_ID")
		os.Unsetenv("GRID_OIDC_EXTERNAL_IDP_CLIENT_SECRET")
		os.Unsetenv("GRID_OIDC_EXTERNAL_IDP_REDIRECT_URI")
		os.Unsetenv("GRID_OIDC_EXTERNAL_IDP_CLI_CLIENT_ID")
	}()

	// Start with clean Viper state
	viper.Reset()

	// Set ALL external IdP fields via environment variables
	// This simulates the e2e test setup where all config comes from env vars
	os.Setenv("GRID_DATABASE_URL", "postgres://test/test")
	os.Setenv("GRID_SERVER_URL", "http://test")
	os.Setenv("GRID_OIDC_EXTERNAL_IDP_ISSUER", "https://keycloak.test/realms/grid")
	os.Setenv("GRID_OIDC_EXTERNAL_IDP_CLIENT_ID", "grid-api-test")
	os.Setenv("GRID_OIDC_EXTERNAL_IDP_CLIENT_SECRET", "super-secret-123")
	os.Setenv("GRID_OIDC_EXTERNAL_IDP_REDIRECT_URI", "http://localhost:8080/auth/sso/callback")
	os.Setenv("GRID_OIDC_EXTERNAL_IDP_CLI_CLIENT_ID", "gridctl-test")

	cfg, err := Load()
	require.NoError(t, err)

	// Verify ExternalIdP is NOT nil
	require.NotNil(t, cfg.OIDC.ExternalIdP, "ExternalIdP should not be nil when env vars are set")

	// Verify ALL fields are populated from environment variables
	assert.Equal(t, "https://keycloak.test/realms/grid", cfg.OIDC.ExternalIdP.Issuer,
		"Issuer should be populated from GRID_OIDC_EXTERNAL_IDP_ISSUER")
	assert.Equal(t, "grid-api-test", cfg.OIDC.ExternalIdP.ClientID,
		"ClientID should be populated from GRID_OIDC_EXTERNAL_IDP_CLIENT_ID")
	assert.Equal(t, "super-secret-123", cfg.OIDC.ExternalIdP.ClientSecret,
		"ClientSecret should be populated from GRID_OIDC_EXTERNAL_IDP_CLIENT_SECRET")
	assert.Equal(t, "http://localhost:8080/auth/sso/callback", cfg.OIDC.ExternalIdP.RedirectURI,
		"RedirectURI should be populated from GRID_OIDC_EXTERNAL_IDP_REDIRECT_URI")
	assert.Equal(t, "gridctl-test", cfg.OIDC.ExternalIdP.CLIClientID,
		"CLIClientID should be populated from GRID_OIDC_EXTERNAL_IDP_CLI_CLIENT_ID")

	// Verify Internal IdP is not set
	assert.Empty(t, cfg.OIDC.Issuer, "Internal IdP issuer should be empty")
}

// TestLoad_InternalIdPFromEnvVars_ViperWorkaround verifies the Viper workaround
// for Internal IdP configuration from environment variables.
//
// REGRESSION TEST: This test will FAIL if the workaround is removed from Load().
func TestLoad_InternalIdPFromEnvVars_ViperWorkaround(t *testing.T) {
	defer func() {
		os.Unsetenv("GRID_DATABASE_URL")
		os.Unsetenv("GRID_SERVER_URL")
		os.Unsetenv("GRID_OIDC_ISSUER")
		os.Unsetenv("GRID_OIDC_CLIENT_ID")
		os.Unsetenv("GRID_OIDC_SIGNING_KEY_PATH")
	}()

	viper.Reset()

	// Set Internal IdP fields via environment variables
	os.Setenv("GRID_DATABASE_URL", "postgres://test/test")
	os.Setenv("GRID_SERVER_URL", "http://test")
	os.Setenv("GRID_OIDC_ISSUER", "https://grid.test")
	os.Setenv("GRID_OIDC_CLIENT_ID", "grid-internal")
	os.Setenv("GRID_OIDC_SIGNING_KEY_PATH", "/tmp/test-keys")

	cfg, err := Load()
	require.NoError(t, err)

	// Verify Internal IdP fields are populated
	assert.Equal(t, "https://grid.test", cfg.OIDC.Issuer,
		"Issuer should be populated from GRID_OIDC_ISSUER")
	assert.Equal(t, "grid-internal", cfg.OIDC.ClientID,
		"ClientID should be populated from GRID_OIDC_CLIENT_ID")
	assert.Equal(t, "/tmp/test-keys", cfg.OIDC.SigningKeyPath,
		"SigningKeyPath should be populated from GRID_OIDC_SIGNING_KEY_PATH")

	// Verify External IdP is not set
	assert.Nil(t, cfg.OIDC.ExternalIdP, "ExternalIdP should be nil")
}