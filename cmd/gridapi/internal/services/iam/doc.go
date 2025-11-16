// Package iam provides identity and access management services for Grid API.
//
// The IAM service centralizes all authentication, authorization, session management,
// and role resolution logic. It provides:
//
//   - Authentication via multiple strategies (JWT, Session cookies)
//   - Immutable group→role cache with lock-free reads
//   - Read-only Casbin policy evaluation
//   - Session lifecycle management
//   - User and service account management
//
// Architecture:
//
//   - Authenticator interface: Pluggable authentication strategies
//   - Principal struct: Unified authentication result (immutable)
//   - GroupRoleCache: Atomic snapshot cache for group→role mappings
//   - Service interface: Facade for all IAM operations
//
// Request Flow:
//
//	Request → MultiAuth → Authenticator.Authenticate() → Principal (with Roles)
//	       ↓
//	   Handler → IAM.Authorize(principal) → Casbin (read-only)
//
// The key design principle is that roles are resolved ONCE at authentication time
// and stored in the Principal struct. Authorization then uses these pre-resolved roles
// without mutating any shared state.
package iam
