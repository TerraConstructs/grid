package server

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/auth"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/services/iam"
)

// HandleCacheRefresh handles POST /admin/cache/refresh
// Manually triggers a refresh of the group→role cache
//
// Authorization: Requires admin:cache-refresh permission
// Response: JSON with cache version, group count, and timestamp
func HandleCacheRefresh(iamService iamAdminService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Get authenticated principal from context (set by auth middleware)
		principal, ok := auth.GetUserFromContext(ctx)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Convert to iam.Principal for authorization (only roles needed)
		iamPrincipal := &iam.Principal{
			Roles: principal.Roles,
		}

		// Check admin:cache-refresh permission
		allowed, err := iamService.Authorize(ctx, iamPrincipal, auth.ObjectTypeAdmin, auth.AdminCacheRefresh, nil)
		if err != nil {
			log.Printf("Authorization check failed: %v", err)
			http.Error(w, "Authorization failed", http.StatusInternalServerError)
			return
		}
		if !allowed {
			http.Error(w, "Forbidden: requires admin:cache-refresh permission", http.StatusForbidden)
			return
		}

		// Refresh the cache
		if err := iamService.RefreshGroupRoleCache(ctx); err != nil {
			log.Printf("ERROR: Manual cache refresh failed: %v", err)
			http.Error(w, "Cache refresh failed", http.StatusInternalServerError)
			return
		}

		// Get snapshot for response
		snapshot := iamService.GetGroupRoleCacheSnapshot()

		// Return success response
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "success",
			"version":   snapshot.Version,
			"groups":    len(snapshot.Mappings),
			"timestamp": snapshot.CreatedAt.Unix(),
		})

		log.Printf("INFO: Manual cache refresh triggered by %s (version=%d, groups=%d)",
			principal.PrincipalID, snapshot.Version, len(snapshot.Mappings))
	}
}
