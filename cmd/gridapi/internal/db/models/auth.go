package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

// User represents a human principal.
// In external IdP mode the Subject field stores the upstream provider ID.
// In internal IdP mode the PasswordHash field stores the bcrypt hash for local login.
type User struct {
	bun.BaseModel `bun:"table:users,alias:u"`

	ID           string     `bun:"id,pk,type:uuid,default:gen_random_uuid()"`
	Subject      *string    `bun:"subject,unique"` // Optional OIDC subject (e.g., "keycloak|123")
	Email        string     `bun:"email,notnull,unique"`
	Name         string     `bun:"name"`
	PasswordHash *string    `bun:"password_hash"` // bcrypt hash (internal IdP mode)
	CreatedAt    time.Time  `bun:"created_at,notnull,default:current_timestamp"`
	UpdatedAt    time.Time  `bun:"updated_at,notnull,default:current_timestamp"`
	LastLoginAt  *time.Time `bun:"last_login_at"`
	DisabledAt   *time.Time `bun:"disabled_at"`
}

// PrincipalSubject returns the stable identifier used for Casbin bindings.
// Falls back to the database ID when no upstream subject exists (internal IdP mode).
func (u *User) PrincipalSubject() string {
	if u == nil {
		return ""
	}
	if u.Subject != nil && *u.Subject != "" {
		return *u.Subject
	}
	return u.ID
}

// ServiceAccount represents a non-interactive authentication principal (e.g., CI/CD pipeline)
type ServiceAccount struct {
	bun.BaseModel `bun:"table:service_accounts,alias:sa"`

	ID               string    `bun:"id,pk,type:uuid,default:gen_random_uuid()"`
	ClientID         string    `bun:"client_id,notnull,unique"`
	ClientSecretHash string    `bun:"client_secret_hash,notnull"`
	Name             string    `bun:"name,notnull"`
	Description      string    `bun:"description"`
	ScopeLabels      LabelMap  `bun:"scope_labels,type:jsonb,notnull,default:'{}'"`
	CreatedAt        time.Time `bun:"created_at,notnull,default:current_timestamp"`
	CreatedBy        string    `bun:"created_by,notnull,type:uuid"` // FK to users(id)
	LastUsedAt       time.Time `bun:"last_used_at"`
	SecretRotatedAt  time.Time `bun:"secret_rotated_at"`
	Disabled         bool      `bun:"disabled,notnull,default:false"`
}

// CreateConstraints represents label constraints for state creation
type CreateConstraints map[string]CreateConstraint
type CreateConstraint struct {
	AllowedValues []string `json:"allowed_values"`
	Required      bool     `json:"required"`
}

// Scan implements sql.Scanner for reading from database
func (cc *CreateConstraints) Scan(value any) error {
	if value == nil {
		*cc = make(CreateConstraints)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan CreateConstraints: expected []byte, got %T", value)
	}
	return json.Unmarshal(bytes, cc)
}

// Value implements driver.Valuer for writing to database
func (cc CreateConstraints) Value() (driver.Value, error) {
	if cc == nil {
		return "{}", nil
	}
	bytes, err := json.Marshal(cc)
	if err != nil {
		return nil, err
	}
	return string(bytes), nil
}

// Role defines role metadata for admin UI and audit
type Role struct {
	bun.BaseModel `bun:"table:roles,alias:r"`

	ID                string            `bun:"id,pk,type:uuid,default:gen_random_uuid()"`
	Name              string            `bun:"name,notnull,unique"`
	Description       string            `bun:"description"`
	ScopeExpr         string            `bun:"scope_expr"` // go-bexpr expression string
	CreateConstraints CreateConstraints `bun:"create_constraints,type:jsonb"`
	ImmutableKeys     []string          `bun:"immutable_keys,type:text[],array"`
	CreatedAt         time.Time         `bun:"created_at,notnull,default:current_timestamp"`
	UpdatedAt         time.Time         `bun:"updated_at,notnull,default:current_timestamp"`
	Version           int               `bun:"version,notnull,default:1"`
}

// UserRole maps identities (users or service accounts) to roles
type UserRole struct {
	bun.BaseModel `bun:"table:user_roles,alias:ur"`

	ID                  string    `bun:"id,pk,type:uuid,default:gen_random_uuid()"`
	UserID              *string   `bun:"user_id,type:uuid"`            // FK to users(id), nullable
	ServiceAccountID    *string   `bun:"service_account_id,type:uuid"` // FK to service_accounts(id), nullable
	RoleID              string    `bun:"role_id,notnull,type:uuid"`    // FK to roles(id)
	LabelFilterOverride LabelMap  `bun:"label_filter_override,type:jsonb"`
	AssignedAt          time.Time `bun:"assigned_at,notnull,default:current_timestamp"`
	AssignedBy          string    `bun:"assigned_by,notnull,type:uuid"` // FK to users(id)
}

// GroupRole maps SSO groups to roles
type GroupRole struct {
	bun.BaseModel `bun:"table:group_roles,alias:gr"`

	ID         string    `bun:"id,pk,type:uuid,default:gen_random_uuid()"`
	GroupName  string    `bun:"group_name,notnull"`
	RoleID     string    `bun:"role_id,notnull,type:uuid"` // FK to roles(id)
	AssignedAt time.Time `bun:"assigned_at,notnull,default:current_timestamp"`
	AssignedBy string    `bun:"assigned_by,notnull,type:uuid"` // FK to users(id)
}

// Session tracks active sessions for human users and service accounts
type Session struct {
	bun.BaseModel `bun:"table:sessions,alias:sess"`

	ID               string    `bun:"id,pk,type:uuid,default:gen_random_uuid()"`
	UserID           *string   `bun:"user_id,type:uuid"`            // FK to users(id), nullable
	ServiceAccountID *string   `bun:"service_account_id,type:uuid"` // FK to service_accounts(id), nullable
	TokenHash        string    `bun:"token_hash,notnull,unique"`    // SHA256 hash of bearer token
	IDToken          string    `bun:"id_token,type:text"`           // OIDC ID token (JWT) for human sessions
	RefreshToken     string    `bun:"refresh_token,type:text"`      // OIDC refresh token
	ExpiresAt        time.Time `bun:"expires_at,notnull"`
	CreatedAt        time.Time `bun:"created_at,notnull,default:current_timestamp"`
	LastUsedAt       time.Time `bun:"last_used_at,notnull,default:current_timestamp"`
	UserAgent        *string   `bun:"user_agent"`           // Nullable for service account sessions
	IPAddress        *string   `bun:"ip_address,type:inet"` // Nullable for service account sessions
	Revoked          bool      `bun:"revoked,notnull,default:false"`
}

// RevokedJTI tracks revoked JWT tokens by their JTI claim for denylist-based revocation
type RevokedJTI struct {
	bun.BaseModel `bun:"table:revoked_jti,alias:rjti"`

	JTI       string    `bun:"jti,pk"`                                       // JWT ID (jti claim from token)
	Subject   string    `bun:"subject,notnull"`                              // Subject (sub claim) - user or service account ID
	Exp       time.Time `bun:"exp,notnull"`                                  // Token expiration time (for cleanup)
	RevokedAt time.Time `bun:"revoked_at,notnull,default:current_timestamp"` // When the token was revoked
	RevokedBy *string   `bun:"revoked_by"`                                   // Optional: who revoked it (user ID)
}
