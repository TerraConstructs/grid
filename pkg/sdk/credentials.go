package sdk

import "time"

// Credentials represents the authentication credentials.
type Credentials struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	ExpiresAt    time.Time `json:"expires_at"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	PrincipalID  string    `json:"principal_id,omitempty"` // Casbin principal ID (e.g., "sa:{clientID}" or "user:{subject}")
}

func (c *Credentials) IsExpired() bool {
	return time.Now().After(c.ExpiresAt)
}

// CredentialStore abstracts credential persistence.
// Implementations MUST be provided by SDK consumers (e.g., gridctl, web apps).
// This interface enables the SDK to remain agnostic to storage mechanisms
// while allowing consumers to implement platform-specific persistence
// (filesystem, browser localStorage, secure enclaves, etc.).
type CredentialStore interface {
	// SaveCredentials persists the given credentials.
	// Implementations SHOULD ensure credentials are stored securely (e.g., file permissions 0600).
	SaveCredentials(credentials *Credentials) error

	// LoadCredentials retrieves previously saved credentials.
	// Returns an error if credentials don't exist or cannot be loaded.
	LoadCredentials() (*Credentials, error)

	// DeleteCredentials removes stored credentials.
	// Should be idempotent (no error if credentials don't exist).
	DeleteCredentials() error
}
