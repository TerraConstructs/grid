package middleware

import (
	"context"
	"errors"
	"fmt"
	"log"
	"maps"
	"net/http"
	"strings"

	"github.com/casbin/casbin/v2"
	"github.com/go-chi/chi/v5"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/auth"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	statepkg "github.com/terraconstructs/grid/cmd/gridapi/internal/state"
)

// AuthzDependencies provides the collaborators needed for authorization decisions.
type AuthzDependencies struct {
	Enforcer     casbin.IEnforcer
	StateService *statepkg.Service
}

// NewAuthzMiddleware constructs a Chi middleware that enforces Casbin policies for HTTP requests.
// This middleware is focused on the Terraform HTTP backend; Connect RPC enforcement is handled via interceptors.
func NewAuthzMiddleware(deps AuthzDependencies) (func(http.Handler) http.Handler, error) {
	if deps.Enforcer == nil {
		return nil, errors.New("authz middleware requires casbin enforcer")
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Classify the request first.
			tfstateAction, guid, matched := classifyTerraformRequest(r)
			if !matched {
				// If it's not a tfstate request that this middleware protects, pass through.
				next.ServeHTTP(w, r)
				return
			}

			// From here, we know it's a tfstate request and requires authorization.
			principal, ok := auth.GetUserFromContext(r.Context())
			if !ok || principal.PrincipalID == "" {
				unauthenticated(w)
				return
			}

			log.Printf("authorizing principal %s for procedure %s", principal.PrincipalID, tfstateAction)

			// matched as tfstate request but the action is empty,
			// it means the method was unsupported. Reject it.
			if tfstateAction == "" {
				http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
				return
			}

			if deps.StateService == nil {
				http.Error(w, "state service not configured for authz", http.StatusInternalServerError)
				return
			}

			labels, lockInfo, err := loadStateLabels(r.Context(), deps.StateService, guid)
			if err != nil {
				if errors.Is(err, errStateNotFound) {
					http.NotFound(w, r)
					return
				}
				http.Error(w, "authorization lookup failed", http.StatusInternalServerError)
				return
			}

			if bypassWriteForLockHolder(tfstateAction, principal, lockInfo) {
				next.ServeHTTP(w, r)
				return
			}

			allowed, err := deps.Enforcer.Enforce(principal.PrincipalID, auth.ObjectTypeState, tfstateAction, labels)
			if err != nil {
				http.Error(w, "authorization error", http.StatusInternalServerError)
				return
			}
			if !allowed {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}, nil
}

func classifyTerraformRequest(r *http.Request) (action string, guid string, matched bool) {
	path := r.URL.Path
	matched = strings.HasPrefix(path, "/tfstate/")
	if !matched {
		return "", "", matched
	}

	guid = chi.URLParam(r, "guid")
	if guid == "" {
		// Fallback: extract GUID from path segments if chi param not yet populated.
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) > 1 {
			guid = parts[1]
		}
	}
	if guid == "" {
		// matched as tfstate but no guid, invalid request
		return "", "", matched
	}

	switch r.Method {
	case http.MethodGet:
		return auth.TfstateRead, guid, matched
	case http.MethodPost:
		return auth.TfstateWrite, guid, matched
	case "LOCK":
		return auth.TfstateLock, guid, matched
	case "UNLOCK":
		return auth.TfstateUnlock, guid, matched
	case http.MethodPut:
		switch {
		case strings.HasSuffix(path, "/lock"):
			return auth.TfstateLock, guid, matched
		case strings.HasSuffix(path, "/unlock"):
			return auth.TfstateUnlock, guid, matched
		default:
			return "", guid, matched
		}
	default:
		return "", guid, matched
	}
}

var errStateNotFound = errors.New("state not found")

func loadStateLabels(ctx context.Context, service *statepkg.Service, guid string) (map[string]any, *models.LockInfo, error) {
	state, err := service.GetStateByGUID(ctx, guid)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, nil, errStateNotFound
		}
		return nil, nil, fmt.Errorf("load state: %w", err)
	}

	labels := make(map[string]any, len(state.Labels))
	maps.Copy(labels, state.Labels)

	return labels, state.LockInfo, nil
}

func bypassWriteForLockHolder(action string, principal auth.AuthenticatedPrincipal, lockInfo *models.LockInfo) bool {
	if lockInfo == nil {
		return false
	}

	if action != auth.TfstateWrite && action != auth.TfstateUnlock {
		return false
	}

	// Compare the current principal's ID with the server-set OwnerPrincipalID,
	// not the client-provided 'Who' string.
	return principal.PrincipalID != "" && principal.PrincipalID == lockInfo.OwnerPrincipalID
}
