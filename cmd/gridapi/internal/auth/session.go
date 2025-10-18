package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

const (
	// SessionDuration is the default session lifetime (12 hours)
	SessionDuration = 12 * time.Hour

	// TokenLength is the length of generated bearer tokens in bytes
	TokenLength = 32
)

// SessionInfo represents session metadata for creation
type SessionInfo struct {
	UserID           *string // Set for human users
	ServiceAccountID *string // Set for service accounts
	IDToken          string  // OIDC ID token (JWT) for human sessions
	RefreshToken     string  // OIDC refresh token (optional)
	UserAgent        string  // Browser/CLI user agent
	IPAddress        string  // Client IP address
}

// GenerateBearerToken generates a cryptographically secure random bearer token
// Returns: token (hex string), token hash (SHA256 hex), error
//
// Reference: FR-007 (session management), data-model.md §294-335 (Session entity)
func GenerateBearerToken() (string, string, error) {
	// Generate random bytes
	tokenBytes := make([]byte, TokenLength)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", "", fmt.Errorf("generate random token: %w", err)
	}

	// Convert to hex string
	token := hex.EncodeToString(tokenBytes)

	// Hash for storage
	hash := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(hash[:])

	return token, tokenHash, nil
}

// HashBearerToken hashes a bearer token for storage/lookup
// Returns SHA256 hex hash
func HashBearerToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// ValidateSessionInfo validates session info before creation
// Ensures exactly one identity type is set
func ValidateSessionInfo(info *SessionInfo) error {
	// Check exactly one identity type
	userSet := info.UserID != nil
	saSet := info.ServiceAccountID != nil

	if !userSet && !saSet {
		return fmt.Errorf("session must reference either user_id or service_account_id")
	}

	if userSet && saSet {
		return fmt.Errorf("session cannot reference both user_id and service_account_id")
	}

	// For human users, ID token is required
	if userSet && info.IDToken == "" {
		return fmt.Errorf("id_token required for user sessions")
	}

	// For service accounts, ID token should not be set
	if saSet && info.IDToken != "" {
		return fmt.Errorf("id_token should not be set for service account sessions")
	}

	return nil
}

// CalculateExpiry calculates session expiry time from creation
// Returns current time + SessionDuration (12 hours)
func CalculateExpiry(createdAt time.Time) time.Time {
	return createdAt.Add(SessionDuration)
}

// IsSessionExpired checks if a session has expired
func IsSessionExpired(expiresAt time.Time) bool {
	return time.Now().After(expiresAt)
}

// IsSessionRevoked checks session revocation flag
func IsSessionRevoked(revoked bool) bool {
	return revoked
}

// ValidateSessionToken performs comprehensive session validation
// Checks expiration, revocation, and identity status
//
// Reference: FR-007 (session revocation), FR-070b (cascade revocation)
func ValidateSessionToken(expiresAt time.Time, revoked bool, identityDisabled bool) error {
	// Check expiration
	if IsSessionExpired(expiresAt) {
		return fmt.Errorf("session expired")
	}

	// Check revocation
	if IsSessionRevoked(revoked) {
		return fmt.Errorf("session revoked")
	}

	// Check identity disabled (user or service account)
	if identityDisabled {
		return fmt.Errorf("identity disabled")
	}

	return nil
}
