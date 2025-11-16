package iam

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/auth"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
)

// mockSessionRepository for testing
type mockSessionRepository struct {
	sessions map[string]*models.Session // tokenHash → session
}

func (m *mockSessionRepository) Create(ctx context.Context, session *models.Session) error {
	m.sessions[session.TokenHash] = session
	return nil
}

func (m *mockSessionRepository) GetByID(ctx context.Context, id string) (*models.Session, error) {
	for _, s := range m.sessions {
		if s.ID == id {
			return s, nil
		}
	}
	return nil, fmt.Errorf("session not found")
}

func (m *mockSessionRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*models.Session, error) {
	if s, ok := m.sessions[tokenHash]; ok {
		return s, nil
	}
	return nil, fmt.Errorf("session not found")
}

func (m *mockSessionRepository) GetByUserID(ctx context.Context, userID string) ([]models.Session, error) {
	result := []models.Session{}
	for _, s := range m.sessions {
		if s.UserID != nil && *s.UserID == userID {
			result = append(result, *s)
		}
	}
	return result, nil
}

func (m *mockSessionRepository) GetByServiceAccountID(ctx context.Context, serviceAccountID string) ([]models.Session, error) {
	result := []models.Session{}
	for _, s := range m.sessions {
		if s.ServiceAccountID != nil && *s.ServiceAccountID == serviceAccountID {
			result = append(result, *s)
		}
	}
	return result, nil
}

func (m *mockSessionRepository) UpdateLastUsed(ctx context.Context, id string) error {
	for _, s := range m.sessions {
		if s.ID == id {
			s.LastUsedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("session not found")
}

func (m *mockSessionRepository) Revoke(ctx context.Context, id string) error {
	for _, s := range m.sessions {
		if s.ID == id {
			s.Revoked = true
			return nil
		}
	}
	return fmt.Errorf("session not found")
}

func (m *mockSessionRepository) RevokeByUserID(ctx context.Context, userID string) error {
	for _, s := range m.sessions {
		if s.UserID != nil && *s.UserID == userID {
			s.Revoked = true
		}
	}
	return nil
}

func (m *mockSessionRepository) RevokeByServiceAccountID(ctx context.Context, serviceAccountID string) error {
	for _, s := range m.sessions {
		if s.ServiceAccountID != nil && *s.ServiceAccountID == serviceAccountID {
			s.Revoked = true
		}
	}
	return nil
}

func (m *mockSessionRepository) DeleteExpired(ctx context.Context) error {
	now := time.Now()
	for hash, s := range m.sessions {
		if s.ExpiresAt.Before(now) {
			delete(m.sessions, hash)
		}
	}
	return nil
}

func (m *mockSessionRepository) List(ctx context.Context) ([]models.Session, error) {
	result := make([]models.Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		result = append(result, *s)
	}
	return result, nil
}

// TestSessionAuthenticator_NoCookie tests behavior when no session cookie present
func TestSessionAuthenticator_NoCookie(t *testing.T) {
	users := &mockUserRepository{users: make(map[string]*models.User)}
	sessions := &mockSessionRepository{sessions: make(map[string]*models.Session)}
	iamService := &mockIAMService{roles: []string{"platform-engineer"}}

	auth := NewSessionAuthenticator(users, sessions, iamService)

	ctx := context.Background()
	req := AuthRequest{
		Headers: http.Header{},
		Cookies: []*http.Cookie{},
	}

	principal, err := auth.Authenticate(ctx, req)
	if err != nil {
		t.Fatalf("Expected no error when no cookie present, got: %v", err)
	}

	if principal != nil {
		t.Error("Expected nil principal when no cookie present")
	}
}

// TestSessionAuthenticator_ValidSession tests successful session authentication
func TestSessionAuthenticator_ValidSession(t *testing.T) {
	users := &mockUserRepository{users: make(map[string]*models.User)}
	sessions := &mockSessionRepository{sessions: make(map[string]*models.Session)}
	iamService := &mockIAMService{roles: []string{"platform-engineer", "product-engineer"}}

	// Create test user
	userID := "user-123"
	sub := "alice@example.com"
	subPtr := &sub
	user := &models.User{
		ID:         userID,
		Subject:    subPtr,
		Email:      "alice@example.com",
		Name:       "Alice",
		DisabledAt: nil,
	}
	users.users[sub] = user

	// Create valid session
	sessionToken := "valid-session-token"
	tokenHash := auth.HashToken(sessionToken)
	session := &models.Session{
		ID:        "session-123",
		UserID:    &userID,
		TokenHash: tokenHash,
		IDToken:   "", // No groups in this test
		ExpiresAt: time.Now().Add(1 * time.Hour),
		Revoked:   false,
	}
	sessions.sessions[tokenHash] = session

	authHandler := NewSessionAuthenticator(users, sessions, iamService)

	ctx := context.Background()
	req := AuthRequest{
		Headers: http.Header{},
		Cookies: []*http.Cookie{
			{Name: auth.SessionCookieName, Value: sessionToken},
		},
	}

	principal, err := authHandler.Authenticate(ctx, req)
	if err != nil {
		t.Fatalf("Expected successful authentication, got error: %v", err)
	}

	if principal == nil {
		t.Fatal("Expected principal to be non-nil")
	}

	if principal.Subject != sub {
		t.Errorf("Expected subject %s, got %s", sub, principal.Subject)
	}

	if principal.Email != "alice@example.com" {
		t.Errorf("Expected email alice@example.com, got %s", principal.Email)
	}

	if principal.Type != PrincipalTypeUser {
		t.Errorf("Expected type %s, got %s", PrincipalTypeUser, principal.Type)
	}

	if len(principal.Roles) != 2 {
		t.Errorf("Expected 2 roles, got %d", len(principal.Roles))
	}

	if principal.SessionID != "session-123" {
		t.Errorf("Expected session ID session-123, got %s", principal.SessionID)
	}
}

// TestSessionAuthenticator_ExpiredSession tests expired session handling
func TestSessionAuthenticator_ExpiredSession(t *testing.T) {
	users := &mockUserRepository{users: make(map[string]*models.User)}
	sessions := &mockSessionRepository{sessions: make(map[string]*models.Session)}
	iamService := &mockIAMService{roles: []string{}}

	// Create test user
	userID := "user-123"
	sub := "alice@example.com"
	subPtr := &sub
	user := &models.User{
		ID:      userID,
		Subject: subPtr,
		Email:   "alice@example.com",
	}
	users.users[sub] = user

	// Create expired session
	sessionToken := "expired-session-token"
	tokenHash := auth.HashToken(sessionToken)
	session := &models.Session{
		ID:        "session-123",
		UserID:    &userID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(-1 * time.Hour), // Expired 1 hour ago
		Revoked:   false,
	}
	sessions.sessions[tokenHash] = session

	authHandler := NewSessionAuthenticator(users, sessions, iamService)

	ctx := context.Background()
	req := AuthRequest{
		Headers: http.Header{},
		Cookies: []*http.Cookie{
			{Name: auth.SessionCookieName, Value: sessionToken},
		},
	}

	principal, err := authHandler.Authenticate(ctx, req)
	if err == nil {
		t.Fatal("Expected error for expired session")
	}

	if principal != nil {
		t.Error("Expected nil principal for expired session")
	}

	if err.Error() != "session has expired" {
		t.Errorf("Expected 'session has expired' error, got: %v", err)
	}
}

// TestSessionAuthenticator_RevokedSession tests revoked session handling
func TestSessionAuthenticator_RevokedSession(t *testing.T) {
	users := &mockUserRepository{users: make(map[string]*models.User)}
	sessions := &mockSessionRepository{sessions: make(map[string]*models.Session)}
	iamService := &mockIAMService{roles: []string{}}

	// Create test user
	userID := "user-123"
	sub := "alice@example.com"
	subPtr := &sub
	user := &models.User{
		ID:      userID,
		Subject: subPtr,
		Email:   "alice@example.com",
	}
	users.users[sub] = user

	// Create revoked session
	sessionToken := "revoked-session-token"
	tokenHash := auth.HashToken(sessionToken)
	session := &models.Session{
		ID:        "session-123",
		UserID:    &userID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(1 * time.Hour),
		Revoked:   true, // Revoked
	}
	sessions.sessions[tokenHash] = session

	authHandler := NewSessionAuthenticator(users, sessions, iamService)

	ctx := context.Background()
	req := AuthRequest{
		Headers: http.Header{},
		Cookies: []*http.Cookie{
			{Name: auth.SessionCookieName, Value: sessionToken},
		},
	}

	principal, err := authHandler.Authenticate(ctx, req)
	if err == nil {
		t.Fatal("Expected error for revoked session")
	}

	if principal != nil {
		t.Error("Expected nil principal for revoked session")
	}

	if err.Error() != "session has been revoked" {
		t.Errorf("Expected 'session has been revoked' error, got: %v", err)
	}
}

// TestSessionAuthenticator_DisabledUser tests disabled user handling
func TestSessionAuthenticator_DisabledUser(t *testing.T) {
	users := &mockUserRepository{users: make(map[string]*models.User)}
	sessions := &mockSessionRepository{sessions: make(map[string]*models.Session)}
	iamService := &mockIAMService{roles: []string{}}

	// Create disabled user
	userID := "user-123"
	sub := "alice@example.com"
	subPtr := &sub
	now := time.Now()
	user := &models.User{
		ID:         userID,
		Subject:    subPtr,
		Email:      "alice@example.com",
		DisabledAt: &now, // User disabled
	}
	users.users[sub] = user

	// Create valid session
	sessionToken := "valid-session-token"
	tokenHash := auth.HashToken(sessionToken)
	session := &models.Session{
		ID:        "session-123",
		UserID:    &userID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(1 * time.Hour),
		Revoked:   false,
	}
	sessions.sessions[tokenHash] = session

	authHandler := NewSessionAuthenticator(users, sessions, iamService)

	ctx := context.Background()
	req := AuthRequest{
		Headers: http.Header{},
		Cookies: []*http.Cookie{
			{Name: auth.SessionCookieName, Value: sessionToken},
		},
	}

	principal, err := authHandler.Authenticate(ctx, req)
	if err == nil {
		t.Fatal("Expected error for disabled user")
	}

	if principal != nil {
		t.Error("Expected nil principal for disabled user")
	}

	if err.Error() != "user is disabled" {
		t.Errorf("Expected 'user is disabled' error, got: %v", err)
	}
}

// TestSessionAuthenticator_InvalidTokenHash tests session not found
func TestSessionAuthenticator_InvalidTokenHash(t *testing.T) {
	users := &mockUserRepository{users: make(map[string]*models.User)}
	sessions := &mockSessionRepository{sessions: make(map[string]*models.Session)}
	iamService := &mockIAMService{roles: []string{}}

	authHandler := NewSessionAuthenticator(users, sessions, iamService)

	ctx := context.Background()
	req := AuthRequest{
		Headers: http.Header{},
		Cookies: []*http.Cookie{
			{Name: auth.SessionCookieName, Value: "invalid-token"},
		},
	}

	principal, err := authHandler.Authenticate(ctx, req)
	if err == nil {
		t.Fatal("Expected error for invalid session token")
	}

	if principal != nil {
		t.Error("Expected nil principal for invalid session")
	}
}
