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

// CredentialStore is an interface for storing and retrieving credentials.
type CredentialStore interface {
	SaveCredentials(credentials *Credentials) error
	LoadCredentials() (*Credentials, error)
	DeleteCredentials() error
	IsValid() (bool, error)
}
