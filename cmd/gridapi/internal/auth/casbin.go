package auth

import (
	"fmt"
	"strings"
	"sync"

	"github.com/casbin/casbin/v2"
	"github.com/hashicorp/go-bexpr"
	casbinbunadapter "github.com/terraconstructs/grid/cmd/gridapi/internal/auth/bunadapter"
	"github.com/uptrace/bun"
)

// bexprCache stores compiled go-bexpr evaluators for performance
// Key: scope expression string, Value: *bexpr.Evaluator
var bexprCache = &sync.Map{}

// InitEnforcer creates and initializes a Casbin enforcer with the given model and database adapter
// Uses msales/casbin-bun-adapter to share the existing *bun.DB connection pool
//
// Reference: research.md §1 (lines 429-490), §7 (adapter usage)
func InitEnforcer(db *bun.DB, modelPath string) (casbin.IEnforcer, error) {
	// Create Bun adapter with existing *bun.DB instance
	adapter, err := casbinbunadapter.NewAdapter(db)
	if err != nil {
		return nil, fmt.Errorf("create casbin adapter: %w", err)
	}

	// Load RBAC model from file (model.conf)
	enforcer, err := casbin.NewSyncedEnforcer(modelPath, adapter)
	if err != nil {
		return nil, fmt.Errorf("create casbin enforcer: %w", err)
	}

	// Register custom bexprMatch function for label filtering
	// This function evaluates go-bexpr expressions against resource labels
	enforcer.AddFunction("bexprMatch", func(args ...any) (any, error) {
		if len(args) != 2 {
			return false, fmt.Errorf("bexprMatch requires 2 arguments: scopeExpr, labels")
		}

		scopeExpr, ok := args[0].(string)
		if !ok {
			return false, fmt.Errorf("bexprMatch: first argument must be string (scopeExpr)")
		}

		labels, ok := args[1].(map[string]any)
		if !ok {
			return false, fmt.Errorf("bexprMatch: second argument must be map[string]any (labels)")
		}

		// Evaluate expression against labels
		return evaluateBexpr(scopeExpr, labels), nil
	})

	// Load policies from database
	if err := enforcer.LoadPolicy(); err != nil {
		return nil, fmt.Errorf("load casbin policies: %w", err)
	}

	return enforcer, nil
}

// evaluateBexpr evaluates a go-bexpr expression against resource labels
// Empty scopeExpr returns true (no constraint)
// Caches compiled evaluators for performance
//
// Reference: research.md §1 (lines 443-482)
func evaluateBexpr(scopeExpr string, labels map[string]any) bool {
	// Empty expression means no constraint (allow all)
	if strings.TrimSpace(scopeExpr) == "" {
		return true
	}

	// Check cache for compiled evaluator
	if cached, ok := bexprCache.Load(scopeExpr); ok {
		evaluator := cached.(*bexpr.Evaluator)
		matches, err := evaluator.Evaluate(labels)
		if err != nil {
			// Invalid evaluation (e.g., missing label key) - deny access
			return false
		}
		return matches
	}

	// Compile and cache evaluator
	evaluator, err := bexpr.CreateEvaluator(scopeExpr)
	if err != nil {
		// Invalid expression syntax - deny access
		return false
	}
	bexprCache.Store(scopeExpr, evaluator)

	// Evaluate
	matches, err := evaluator.Evaluate(labels)
	if err != nil {
		return false
	}
	return matches
}
