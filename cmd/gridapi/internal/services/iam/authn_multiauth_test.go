package iam

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

// mockAuthenticator for testing
type mockAuthenticator struct {
	name      string
	principal *Principal
	err       error
}

func (m *mockAuthenticator) Authenticate(ctx context.Context, req AuthRequest) (*Principal, error) {
	return m.principal, m.err
}

// TestIAMService_AuthenticateRequest_NoAuthenticators tests with no authenticators
func TestIAMService_AuthenticateRequest_NoAuthenticators(t *testing.T) {
	svc := &iamService{
		authenticators: []Authenticator{},
	}

	ctx := context.Background()
	req := AuthRequest{
		Headers: http.Header{},
		Cookies: []*http.Cookie{},
	}

	principal, err := svc.AuthenticateRequest(ctx, req)
	if err != nil {
		t.Fatalf("Expected no error with no authenticators, got: %v", err)
	}

	if principal != nil {
		t.Error("Expected nil principal with no authenticators")
	}
}

// TestIAMService_AuthenticateRequest_FirstAuthenticatorSucceeds tests authenticator priority
func TestIAMService_AuthenticateRequest_FirstAuthenticatorSucceeds(t *testing.T) {
	expectedPrincipal := &Principal{
		Subject:     "alice@example.com",
		PrincipalID: "user:alice@example.com",
		InternalID:  "user-123",
		Type:        PrincipalTypeUser,
	}

	svc := &iamService{
		authenticators: []Authenticator{
			&mockAuthenticator{name: "session", principal: expectedPrincipal, err: nil},
			&mockAuthenticator{name: "jwt", principal: nil, err: nil}, // Should not be called
		},
	}

	ctx := context.Background()
	req := AuthRequest{
		Headers: http.Header{},
		Cookies: []*http.Cookie{},
	}

	principal, err := svc.AuthenticateRequest(ctx, req)
	if err != nil {
		t.Fatalf("Expected successful authentication, got error: %v", err)
	}

	if principal == nil {
		t.Fatal("Expected principal to be non-nil")
	}

	if principal.Subject != expectedPrincipal.Subject {
		t.Errorf("Expected subject %s, got %s", expectedPrincipal.Subject, principal.Subject)
	}
}

// TestIAMService_AuthenticateRequest_FirstReturnsNilSecondSucceeds tests fallback
func TestIAMService_AuthenticateRequest_FirstReturnsNilSecondSucceeds(t *testing.T) {
	expectedPrincipal := &Principal{
		Subject:     "bob@example.com",
		PrincipalID: "user:bob@example.com",
		InternalID:  "user-456",
		Type:        PrincipalTypeUser,
	}

	svc := &iamService{
		authenticators: []Authenticator{
			&mockAuthenticator{name: "session", principal: nil, err: nil}, // No credentials
			&mockAuthenticator{name: "jwt", principal: expectedPrincipal, err: nil}, // Success
		},
	}

	ctx := context.Background()
	req := AuthRequest{
		Headers: http.Header{},
		Cookies: []*http.Cookie{},
	}

	principal, err := svc.AuthenticateRequest(ctx, req)
	if err != nil {
		t.Fatalf("Expected successful authentication, got error: %v", err)
	}

	if principal == nil {
		t.Fatal("Expected principal to be non-nil")
	}

	if principal.Subject != expectedPrincipal.Subject {
		t.Errorf("Expected subject %s, got %s", expectedPrincipal.Subject, principal.Subject)
	}
}

// TestIAMService_AuthenticateRequest_AllReturnNil tests unauthenticated request
func TestIAMService_AuthenticateRequest_AllReturnNil(t *testing.T) {
	svc := &iamService{
		authenticators: []Authenticator{
			&mockAuthenticator{name: "session", principal: nil, err: nil}, // No credentials
			&mockAuthenticator{name: "jwt", principal: nil, err: nil},     // No credentials
		},
	}

	ctx := context.Background()
	req := AuthRequest{
		Headers: http.Header{},
		Cookies: []*http.Cookie{},
	}

	principal, err := svc.AuthenticateRequest(ctx, req)
	if err != nil {
		t.Fatalf("Expected no error for unauthenticated request, got: %v", err)
	}

	if principal != nil {
		t.Error("Expected nil principal for unauthenticated request")
	}
}

// TestIAMService_AuthenticateRequest_FirstAuthenticatorFails tests authentication failure
func TestIAMService_AuthenticateRequest_FirstAuthenticatorFails(t *testing.T) {
	expectedError := errors.New("invalid credentials")

	svc := &iamService{
		authenticators: []Authenticator{
			&mockAuthenticator{name: "session", principal: nil, err: expectedError}, // Auth failed
			&mockAuthenticator{name: "jwt", principal: nil, err: nil},               // Should not be called
		},
	}

	ctx := context.Background()
	req := AuthRequest{
		Headers: http.Header{},
		Cookies: []*http.Cookie{},
	}

	principal, err := svc.AuthenticateRequest(ctx, req)
	if err == nil {
		t.Fatal("Expected authentication error")
	}

	if principal != nil {
		t.Error("Expected nil principal on authentication failure")
	}

	if err.Error() != expectedError.Error() {
		t.Errorf("Expected error '%v', got '%v'", expectedError, err)
	}
}

// TestIAMService_AuthenticateRequest_Priority tests authenticator priority order
func TestIAMService_AuthenticateRequest_Priority(t *testing.T) {
	// Both authenticators return principals, first one should win
	sessionPrincipal := &Principal{
		Subject:     "session-user",
		PrincipalID: "user:session-user",
		Type:        PrincipalTypeUser,
	}

	jwtPrincipal := &Principal{
		Subject:     "jwt-user",
		PrincipalID: "user:jwt-user",
		Type:        PrincipalTypeUser,
	}

	svc := &iamService{
		authenticators: []Authenticator{
			&mockAuthenticator{name: "session", principal: sessionPrincipal, err: nil},
			&mockAuthenticator{name: "jwt", principal: jwtPrincipal, err: nil},
		},
	}

	ctx := context.Background()
	req := AuthRequest{
		Headers: http.Header{},
		Cookies: []*http.Cookie{},
	}

	principal, err := svc.AuthenticateRequest(ctx, req)
	if err != nil {
		t.Fatalf("Expected successful authentication, got error: %v", err)
	}

	if principal == nil {
		t.Fatal("Expected principal to be non-nil")
	}

	// Should use session authenticator (first in list)
	if principal.Subject != "session-user" {
		t.Errorf("Expected session-user (first authenticator), got %s", principal.Subject)
	}
}

// TestIAMService_AuthenticateRequest_MultipleFallbacks tests multiple fallback attempts
func TestIAMService_AuthenticateRequest_MultipleFallbacks(t *testing.T) {
	expectedPrincipal := &Principal{
		Subject:     "third-auth-user",
		PrincipalID: "user:third-auth-user",
		Type:        PrincipalTypeUser,
	}

	svc := &iamService{
		authenticators: []Authenticator{
			&mockAuthenticator{name: "first", principal: nil, err: nil},            // No credentials
			&mockAuthenticator{name: "second", principal: nil, err: nil},           // No credentials
			&mockAuthenticator{name: "third", principal: expectedPrincipal, err: nil}, // Success
		},
	}

	ctx := context.Background()
	req := AuthRequest{
		Headers: http.Header{},
		Cookies: []*http.Cookie{},
	}

	principal, err := svc.AuthenticateRequest(ctx, req)
	if err != nil {
		t.Fatalf("Expected successful authentication, got error: %v", err)
	}

	if principal == nil {
		t.Fatal("Expected principal to be non-nil")
	}

	if principal.Subject != expectedPrincipal.Subject {
		t.Errorf("Expected subject %s, got %s", expectedPrincipal.Subject, principal.Subject)
	}
}

// TestAuthRequest_Construction tests AuthRequest can be built from HTTP request
func TestAuthRequest_Construction(t *testing.T) {
	headers := http.Header{}
	headers.Set("Authorization", "Bearer test-token")
	headers.Set("User-Agent", "test-client")

	cookies := []*http.Cookie{
		{Name: "grid.session", Value: "session-token"},
		{Name: "other", Value: "other-value"},
	}

	req := AuthRequest{
		Headers: headers,
		Cookies: cookies,
	}

	if req.Headers.Get("Authorization") != "Bearer test-token" {
		t.Errorf("Expected Authorization header, got %s", req.Headers.Get("Authorization"))
	}

	if len(req.Cookies) != 2 {
		t.Errorf("Expected 2 cookies, got %d", len(req.Cookies))
	}

	// Verify we can find session cookie
	var sessionCookie *http.Cookie
	for _, c := range req.Cookies {
		if c.Name == "grid.session" {
			sessionCookie = c
			break
		}
	}

	if sessionCookie == nil {
		t.Fatal("Expected to find grid.session cookie")
	}

	if sessionCookie.Value != "session-token" {
		t.Errorf("Expected session-token value, got %s", sessionCookie.Value)
	}
}
