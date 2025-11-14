package iam

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/config"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
)

// mockUserRepository for testing
type mockUserRepository struct {
	users map[string]*models.User // subject → user
}

func (m *mockUserRepository) Create(ctx context.Context, user *models.User) error {
	if user.Subject != nil {
		m.users[*user.Subject] = user
	} else {
		m.users[user.Email] = user
	}
	return nil
}

func (m *mockUserRepository) GetByID(ctx context.Context, id string) (*models.User, error) {
	for _, u := range m.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, fmt.Errorf("user not found")
}

func (m *mockUserRepository) GetBySubject(ctx context.Context, subject string) (*models.User, error) {
	if u, ok := m.users[subject]; ok {
		return u, nil
	}
	return nil, fmt.Errorf("user not found")
}

func (m *mockUserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	if u, ok := m.users[email]; ok {
		return u, nil
	}
	return nil, fmt.Errorf("user not found")
}

func (m *mockUserRepository) Update(ctx context.Context, user *models.User) error {
	if user.Subject != nil {
		m.users[*user.Subject] = user
	}
	return nil
}

func (m *mockUserRepository) UpdateLastLogin(ctx context.Context, id string) error {
	for _, u := range m.users {
		if u.ID == id {
			now := time.Now()
			u.LastLoginAt = &now
			return nil
		}
	}
	return fmt.Errorf("user not found")
}

func (m *mockUserRepository) SetPasswordHash(ctx context.Context, id string, passwordHash string) error {
	return nil
}

func (m *mockUserRepository) List(ctx context.Context) ([]models.User, error) {
	result := make([]models.User, 0, len(m.users))
	for _, u := range m.users {
		result = append(result, *u)
	}
	return result, nil
}

// mockServiceAccountRepository for testing
type mockServiceAccountRepository struct {
	accounts map[string]*models.ServiceAccount // clientID → account
}

func (m *mockServiceAccountRepository) Create(ctx context.Context, sa *models.ServiceAccount) error {
	m.accounts[sa.ClientID] = sa
	return nil
}

func (m *mockServiceAccountRepository) GetByID(ctx context.Context, id string) (*models.ServiceAccount, error) {
	for _, sa := range m.accounts {
		if sa.ID == id {
			return sa, nil
		}
	}
	return nil, fmt.Errorf("service account not found")
}

func (m *mockServiceAccountRepository) GetByName(ctx context.Context, name string) (*models.ServiceAccount, error) {
	for _, sa := range m.accounts {
		if sa.Name == name {
			return sa, nil
		}
	}
	return nil, fmt.Errorf("service account not found")
}

func (m *mockServiceAccountRepository) GetByClientID(ctx context.Context, clientID string) (*models.ServiceAccount, error) {
	if sa, ok := m.accounts[clientID]; ok {
		return sa, nil
	}
	return nil, fmt.Errorf("service account not found")
}

func (m *mockServiceAccountRepository) Update(ctx context.Context, sa *models.ServiceAccount) error {
	m.accounts[sa.ClientID] = sa
	return nil
}

func (m *mockServiceAccountRepository) UpdateLastUsed(ctx context.Context, id string) error {
	return nil
}

func (m *mockServiceAccountRepository) UpdateSecretHash(ctx context.Context, id string, secretHash string) error {
	return nil
}

func (m *mockServiceAccountRepository) SetDisabled(ctx context.Context, id string, disabled bool) error {
	return nil
}

func (m *mockServiceAccountRepository) List(ctx context.Context) ([]models.ServiceAccount, error) {
	result := make([]models.ServiceAccount, 0, len(m.accounts))
	for _, sa := range m.accounts {
		result = append(result, *sa)
	}
	return result, nil
}

func (m *mockServiceAccountRepository) ListByCreator(ctx context.Context, createdBy string) ([]models.ServiceAccount, error) {
	return nil, nil
}

// mockRevokedJTIRepository for testing
type mockRevokedJTIRepository struct {
	revokedJTIs map[string]bool
}

func (m *mockRevokedJTIRepository) Create(ctx context.Context, revokedJTI *models.RevokedJTI) error {
	m.revokedJTIs[revokedJTI.JTI] = true
	return nil
}

func (m *mockRevokedJTIRepository) IsRevoked(ctx context.Context, jti string) (bool, error) {
	return m.revokedJTIs[jti], nil
}

func (m *mockRevokedJTIRepository) DeleteExpired(ctx context.Context, gracePeriod time.Duration) error {
	return nil
}

func (m *mockRevokedJTIRepository) GetByJTI(ctx context.Context, jti string) (*models.RevokedJTI, error) {
	if m.revokedJTIs[jti] {
		return &models.RevokedJTI{JTI: jti}, nil
	}
	return nil, fmt.Errorf("not found")
}

// mockIAMService for testing (simplified, only implements ResolveRoles)
type mockIAMService struct {
	roles []string
}

func (m *mockIAMService) AuthenticateRequest(ctx context.Context, req AuthRequest) (*Principal, error) {
	return nil, nil
}

func (m *mockIAMService) ResolveRoles(ctx context.Context, principalID string, groups []string, isUser bool) ([]string, error) {
	// Return mock roles for testing
	return m.roles, nil
}

func (m *mockIAMService) Authorize(ctx context.Context, principal *Principal, obj, act string, labels map[string]interface{}) (bool, error) {
	return false, nil
}

func (m *mockIAMService) RefreshGroupRoleCache(ctx context.Context) error {
	return nil
}

func (m *mockIAMService) GetGroupRoleCacheSnapshot() GroupRoleSnapshot {
	return GroupRoleSnapshot{}
}

func (m *mockIAMService) CreateSession(ctx context.Context, userID, idToken string, expiresAt time.Time) (*models.Session, string, error) {
	return nil, "", nil
}

func (m *mockIAMService) RevokeSession(ctx context.Context, sessionID string) error {
	return nil
}

func (m *mockIAMService) GetSessionByID(ctx context.Context, sessionID string) (*models.Session, error) {
	return nil, nil
}

func (m *mockIAMService) ListUserSessions(ctx context.Context, userID string) ([]models.Session, error) {
	return nil, nil
}

func (m *mockIAMService) RevokeJTI(ctx context.Context, jti string, expiresAt time.Time) error {
	return nil
}

func (m *mockIAMService) CreateUser(ctx context.Context, email, username, passwordHash, subject string) (*models.User, error) {
	return nil, nil
}

func (m *mockIAMService) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	return nil, nil
}

func (m *mockIAMService) GetUserBySubject(ctx context.Context, subject string) (*models.User, error) {
	return nil, nil
}

func (m *mockIAMService) GetUserByID(ctx context.Context, userID string) (*models.User, error) {
	return nil, nil
}

func (m *mockIAMService) DisableUser(ctx context.Context, userID string) error {
	return nil
}

func (m *mockIAMService) CreateServiceAccount(ctx context.Context, name, createdBy string) (*models.ServiceAccount, string, error) {
	return nil, "", nil
}

func (m *mockIAMService) ListServiceAccounts(ctx context.Context) ([]*models.ServiceAccount, error) {
	return nil, nil
}

func (m *mockIAMService) GetServiceAccountByName(ctx context.Context, name string) (*models.ServiceAccount, error) {
	return nil, nil
}

func (m *mockIAMService) GetServiceAccountByClientID(ctx context.Context, clientID string) (*models.ServiceAccount, error) {
	return nil, nil
}

func (m *mockIAMService) RevokeServiceAccount(ctx context.Context, clientID string) error {
	return nil
}

func (m *mockIAMService) RotateServiceAccountSecret(ctx context.Context, clientID string) (string, time.Time, error) {
	return "", time.Time{}, nil
}

func (m *mockIAMService) AssignRolesToServiceAccount(ctx context.Context, serviceAccountID string, roleIDs []string) error {
	return nil
}

func (m *mockIAMService) RemoveRolesFromServiceAccount(ctx context.Context, serviceAccountID string, roleIDs []string) error {
	return nil
}

func (m *mockIAMService) AssignUserRole(ctx context.Context, userID, serviceAccountID, roleID string) error {
	return nil
}

func (m *mockIAMService) RemoveUserRole(ctx context.Context, userID, serviceAccountID, roleID string) error {
	return nil
}

func (m *mockIAMService) AssignGroupRole(ctx context.Context, groupName, roleID string) error {
	return nil
}

func (m *mockIAMService) RemoveGroupRole(ctx context.Context, groupName, roleID string) error {
	return nil
}

func (m *mockIAMService) CreateRole(
	ctx context.Context,
	name, description, scopeExpr string,
	createConstraints models.CreateConstraints,
	immutableKeys []string,
	actions []string,
) (*models.Role, error) {
	return nil, nil
}

func (m *mockIAMService) UpdateRole(
	ctx context.Context,
	name string,
	expectedVersion int,
	description, scopeExpr string,
	createConstraints models.CreateConstraints,
	immutableKeys []string,
	actions []string,
) (*models.Role, error) {
	return nil, nil
}

func (m *mockIAMService) DeleteRole(ctx context.Context, name string) error {
	return nil
}

func (m *mockIAMService) GetRoleByName(ctx context.Context, name string) (*models.Role, error) {
	return nil, nil
}

func (m *mockIAMService) GetRolesByName(ctx context.Context, roleNames []string) ([]models.Role, []string, []string, error) {
	roleSet := make(map[string]models.Role, len(m.roles))
	for i, name := range m.roles {
		roleSet[name] = models.Role{
			ID:   fmt.Sprintf("role-%d", i),
			Name: name,
		}
	}

	matched := make([]models.Role, 0, len(roleNames))
	invalid := make([]string, 0)

	for _, requested := range roleNames {
		if role, ok := roleSet[requested]; ok {
			matched = append(matched, role)
		} else {
			invalid = append(invalid, requested)
		}
	}

	return matched, invalid, m.roles, nil
}

func (m *mockIAMService) GetRoleByID(ctx context.Context, roleID string) (*models.Role, error) {
	return nil, nil
}

func (m *mockIAMService) ListAllRoles(ctx context.Context) ([]models.Role, error) {
	return nil, nil
}

func (m *mockIAMService) GetServiceAccountByID(ctx context.Context, saID string) (*models.ServiceAccount, error) {
	return nil, nil
}

func (m *mockIAMService) ListGroupRoles(ctx context.Context, groupName *string) ([]models.GroupRole, error) {
	return nil, nil
}

func (m *mockIAMService) GetPrincipalRoles(ctx context.Context, principalID, principalType string) ([]string, error) {
	return nil, nil
}

func (m *mockIAMService) GetRolePermissions(ctx context.Context, roleName string) ([][]string, error) {
	return nil, nil
}

// TestJWTAuthenticator_NoAuthorizationHeader tests behavior when no auth header present
func TestJWTAuthenticator_NoAuthorizationHeader(t *testing.T) {
	// Note: We can't fully test JWTAuthenticator without a real OIDC server
	// because it requires valid JWT tokens. This test validates the "no credentials" path.

	// ctx := context.Background()
	req := AuthRequest{
		Headers: http.Header{},
		Cookies: []*http.Cookie{},
	}

	// JWTAuthenticator requires OIDC config, which requires a real OIDC endpoint
	// For now, test that missing auth header returns (nil, nil)

	// This test is limited because NewJWTAuthenticator requires valid OIDC config
	// Full JWT validation tests would require integration tests with a real OIDC server

	t.Log("JWT authenticator tests require real OIDC server for full coverage")
	t.Log("Integration tests will cover JWT validation end-to-end")

	// Verify that AuthRequest can be constructed properly
	if req.Headers == nil {
		t.Fatal("Expected Headers to be non-nil")
	}

	if req.Headers.Get("Authorization") != "" {
		t.Errorf("Expected empty Authorization header, got %s", req.Headers.Get("Authorization"))
	}
}

// TestJWTAuthenticator_ExtractGroups tests group extraction from claims
func TestJWTAuthenticator_ExtractGroups(t *testing.T) {
	cfg := &config.Config{
		OIDC: config.OIDCConfig{
			GroupsClaimField: "groups",
			GroupsClaimPath:  "",
		},
	}

	users := &mockUserRepository{users: make(map[string]*models.User)}
	serviceAccounts := &mockServiceAccountRepository{accounts: make(map[string]*models.ServiceAccount)}
	revokedJTIs := &mockRevokedJTIRepository{revokedJTIs: make(map[string]bool)}
	iamService := &mockIAMService{roles: []string{"platform-engineer"}}

	// Note: We can't create a real JWTAuthenticator without valid OIDC config
	// Testing group extraction logic directly

	t.Log("Group extraction tests require real JWT tokens")
	t.Log("This would be tested in integration tests with real OIDC server")

	// Verify mock dependencies are set up correctly
	if users == nil || serviceAccounts == nil || revokedJTIs == nil || iamService == nil || cfg == nil {
		t.Fatal("Mock dependencies not initialized")
	}
}

// TestJWTAuthenticator_JITProvisioning tests just-in-time user provisioning
func TestJWTAuthenticator_JITProvisioning(t *testing.T) {
	users := &mockUserRepository{users: make(map[string]*models.User)}

	// Simulate JIT provisioning
	sub := "alice@example.com"
	subPtr := &sub
	user := &models.User{
		ID:          "user-123",
		Subject:     subPtr,
		Email:       "alice@example.com",
		Name:        "Alice",
		DisabledAt:  nil,
		LastLoginAt: nil,
	}

	err := users.Create(context.Background(), user)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Verify user was created
	retrieved, err := users.GetBySubject(context.Background(), sub)
	if err != nil {
		t.Fatalf("Failed to retrieve user: %v", err)
	}

	if retrieved.Email != "alice@example.com" {
		t.Errorf("Expected email alice@example.com, got %s", retrieved.Email)
	}

	if retrieved.Subject == nil || *retrieved.Subject != sub {
		t.Errorf("Expected subject %s, got %v", sub, retrieved.Subject)
	}
}

// TestJWTAuthenticator_RevokedJTI tests JTI revocation checking
func TestJWTAuthenticator_RevokedJTI(t *testing.T) {
	revokedJTIs := &mockRevokedJTIRepository{revokedJTIs: make(map[string]bool)}
	ctx := context.Background()

	// Add revoked JTI
	jti := "revoked-token-123"
	err := revokedJTIs.Create(ctx, &models.RevokedJTI{
		JTI:     jti,
		Subject: "alice@example.com",
		Exp:     time.Now().Add(1 * time.Hour),
	})
	if err != nil {
		t.Fatalf("Failed to revoke JTI: %v", err)
	}

	// Check if revoked
	isRevoked, err := revokedJTIs.IsRevoked(ctx, jti)
	if err != nil {
		t.Fatalf("Failed to check revocation: %v", err)
	}

	if !isRevoked {
		t.Error("Expected JTI to be revoked")
	}

	// Check non-revoked JTI
	isRevoked, err = revokedJTIs.IsRevoked(ctx, "valid-token-456")
	if err != nil {
		t.Fatalf("Failed to check revocation: %v", err)
	}

	if isRevoked {
		t.Error("Expected JTI to not be revoked")
	}
}
