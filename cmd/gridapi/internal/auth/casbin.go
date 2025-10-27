package auth

import (
	_ "embed"
	"fmt"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	casbinbunadapter "github.com/terraconstructs/grid/cmd/gridapi/internal/auth/bunadapter"
	"github.com/uptrace/bun"
)

//go:embed model.conf
var casbinModelContent string

// InitEnforcer creates and initializes a Casbin enforcer with embedded model and database adapter
// Uses msales/casbin-bun-adapter to share the existing *bun.DB connection pool
//
// Reference: research.md §1 (lines 429-490), §7 (adapter usage)
func InitEnforcer(db *bun.DB) (casbin.IEnforcer, error) {
	// Create Bun adapter with existing *bun.DB instance
	adapter, err := casbinbunadapter.NewAdapter(db)
	if err != nil {
		return nil, fmt.Errorf("create casbin adapter: %w", err)
	}

	// Load RBAC model from embedded string
	m, err := model.NewModelFromString(casbinModelContent)
	if err != nil {
		return nil, fmt.Errorf("parse casbin model: %w", err)
	}

	// Create enforcer with model and adapter
	enforcer, err := casbin.NewSyncedEnforcer(m, adapter)
	if err != nil {
		return nil, fmt.Errorf("create casbin enforcer: %w", err)
	}

	// Register custom bexprMatch function for label filtering
	// This function evaluates go-bexpr expressions against resource labels
	enforcer.AddFunction("bexprMatch", BexprMatchFunction())

	// Load policies from database
	if err := enforcer.LoadPolicy(); err != nil {
		return nil, fmt.Errorf("load casbin policies: %w", err)
	}

	return enforcer, nil
}
