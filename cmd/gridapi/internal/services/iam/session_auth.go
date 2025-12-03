package iam

import (
	"context"
	"fmt"
	"time"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/auth"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/repository"
)

// SessionAuthenticator authenticates requests using session cookies.
//
// Implementation follows Phase 3 specification:
//  1. Extract "grid.session" cookie
//  2. Return (nil, nil) if not present
//  3. Hash cookie value
//  4. Lookup session in DB
//  5. Validate: not revoked, not expired
//  6. Lookup user
//  7. Validate: not disabled
//  8. Extract groups from session.id_token (stored JWT)
//  9. Call ResolveRoles()
//  10. Construct Principal
//  11. Return Principal
//
// This authenticator is stateless and thread-safe.
type SessionAuthenticator struct {
	users      repository.UserRepository
	sessions   repository.SessionRepository
	iamService Service // Reference to parent IAM service for ResolveRoles
}

// NewSessionAuthenticator creates a new session authenticator.
func NewSessionAuthenticator(
	users repository.UserRepository,
	sessions repository.SessionRepository,
	iamService Service,
) *SessionAuthenticator {
	return &SessionAuthenticator{
		users:      users,
		sessions:   sessions,
		iamService: iamService,
	}
}

// Authenticate extracts and validates session cookies.
//
// Returns:
//   - (nil, nil) if no session cookie present (no credentials for this authenticator)
//   - (nil, error) if authentication fails (invalid session, expired, revoked, etc.)
//   - (*Principal, nil) if authentication succeeds
func (a *SessionAuthenticator) Authenticate(ctx context.Context, req AuthRequest) (*Principal, error) {
	// Step 1: Extract "grid.session" cookie
	var sessionCookie string
	for _, cookie := range req.Cookies {
		if cookie.Name == auth.SessionCookieName {
			sessionCookie = cookie.Value
			break
		}
	}

	if sessionCookie == "" {
		// No credentials for this authenticator, try next
		return nil, nil
	}

	// Step 3: Hash cookie value
	tokenHash := auth.HashToken(sessionCookie)

	// Step 4: Lookup session in DB
	session, err := a.sessions.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}
	if session == nil {
		return nil, fmt.Errorf("session not found")
	}

	// Step 5: Validate session
	if session.Revoked {
		return nil, fmt.Errorf("session has been revoked")
	}

	now := time.Now()
	if session.ExpiresAt.Before(now) {
		return nil, fmt.Errorf("session has expired")
	}

	// Step 6: Lookup user (sessions are only for users, not service accounts)
	var user *models.User
	if session.UserID != nil {
		user, err = a.users.GetByID(ctx, *session.UserID)
		if err != nil {
			return nil, fmt.Errorf("user not found: %w", err)
		}
		if user == nil {
			return nil, fmt.Errorf("user not found")
		}
	} else {
		return nil, fmt.Errorf("session has no associated user")
	}

	// Step 7: Validate user
	if user.DisabledAt != nil {
		return nil, fmt.Errorf("user is disabled")
	}

	// Step 8: Extract groups from session.id_token (stored JWT)
	groups, err := auth.ExtractGroupsFromIDToken(session.IDToken)
	if err != nil {
		// Failed to extract groups, continue with empty groups
		// This is not a fatal error - user may have no groups
		groups = []string{}
	}

	// Step 9: Resolve roles using immutable cache
	roles, err := a.iamService.ResolveRoles(ctx, user.ID, groups, true)
	if err != nil {
		return nil, fmt.Errorf("resolve roles: %w", err)
	}

	// Step 10: Construct Principal
	subject := user.PrincipalSubject() // Use helper method to handle nil Subject
	principal := &Principal{
		Subject:     subject,
		PrincipalID: fmt.Sprintf("user:%s", subject),
		InternalID:  user.ID,
		Email:       user.Email,
		Name:        user.Name,
		SessionID:   session.ID,
		Groups:      groups,
		Roles:       roles,
		Type:        PrincipalTypeUser,
	}

	// Update session last used timestamp (non-blocking)
	go func() {
		// Use background context to avoid request cancellation
		bgCtx := context.Background()
		_ = a.sessions.UpdateLastUsed(bgCtx, session.ID)
	}()

	return principal, nil
}
