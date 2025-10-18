package auth

import (
	"fmt"
	"strings"
	"time"
)

// ValidateClaims performs comprehensive validation of JWT claims
// Checks required fields, expiration, and claim formats
//
// Reference: FR-006, FR-006a (token validation requirements)
func ValidateClaims(claims map[string]interface{}, requiredClaims []string) error {
	// Check required claims are present
	for _, required := range requiredClaims {
		if _, ok := claims[required]; !ok {
			return fmt.Errorf("missing required claim: %s", required)
		}
	}

	// Validate expiration (exp claim)
	if exp, ok := claims["exp"]; ok {
		var expTime int64
		switch v := exp.(type) {
		case float64:
			expTime = int64(v)
		case int64:
			expTime = v
		default:
			return fmt.Errorf("invalid exp claim type: %T", exp)
		}

		if time.Now().Unix() > expTime {
			return fmt.Errorf("token expired")
		}
	}

	// Validate issued at (iat claim)
	if iat, ok := claims["iat"]; ok {
		var iatTime int64
		switch v := iat.(type) {
		case float64:
			iatTime = int64(v)
		case int64:
			iatTime = v
		default:
			return fmt.Errorf("invalid iat claim type: %T", iat)
		}

		// Reject tokens issued in the future (clock skew tolerance: 5 minutes)
		if time.Now().Unix() < iatTime-300 {
			return fmt.Errorf("token issued in the future")
		}
	}

	// Validate not before (nbf claim)
	if nbf, ok := claims["nbf"]; ok {
		var nbfTime int64
		switch v := nbf.(type) {
		case float64:
			nbfTime = int64(v)
		case int64:
			nbfTime = v
		default:
			return fmt.Errorf("invalid nbf claim type: %T", nbf)
		}

		if time.Now().Unix() < nbfTime {
			return fmt.Errorf("token not yet valid")
		}
	}

	return nil
}

// ValidateSubject validates the subject claim format
// Subject must be in format: provider|provider_user_id (e.g., "keycloak|123")
//
// Reference: data-model.md §56 (User.Subject validation)
func ValidateSubject(subject string) error {
	if subject == "" {
		return fmt.Errorf("subject is empty")
	}

	parts := strings.Split(subject, "|")
	if len(parts) != 2 {
		return fmt.Errorf("invalid subject format: expected 'provider|id', got '%s'", subject)
	}

	provider := parts[0]
	userID := parts[1]

	if provider == "" {
		return fmt.Errorf("subject provider is empty")
	}

	if userID == "" {
		return fmt.Errorf("subject user ID is empty")
	}

	return nil
}

// ValidateEmail validates email format (basic RFC 5322 check)
func ValidateEmail(email string) error {
	if email == "" {
		return fmt.Errorf("email is empty")
	}

	// Basic validation: contains @ and domain
	if !strings.Contains(email, "@") {
		return fmt.Errorf("invalid email format: missing @")
	}

	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return fmt.Errorf("invalid email format: multiple @ symbols")
	}

	local := parts[0]
	domain := parts[1]

	if local == "" {
		return fmt.Errorf("invalid email format: empty local part")
	}

	if domain == "" {
		return fmt.Errorf("invalid email format: empty domain")
	}

	if !strings.Contains(domain, ".") {
		return fmt.Errorf("invalid email format: domain missing TLD")
	}

	return nil
}
