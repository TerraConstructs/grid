package auth

import (
	"fmt"
	"strings"
	"sync"

	"github.com/hashicorp/go-bexpr"
)

// bexprCache stores compiled go-bexpr evaluators for performance
// Key: scope expression string, Value: *bexpr.Evaluator
var bexprCache = &sync.Map{}

// BexprMatchFunction returns the bexprMatch function for Casbin
// This function evaluates go-bexpr expressions against resource labels
//
// Reference: research.md §1 (lines 443-482)
func BexprMatchFunction() func(args ...any) (any, error) {
	return func(args ...any) (any, error) {
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
		return EvaluateBexpr(scopeExpr, labels), nil
	}
}

// EvaluateBexpr evaluates a go-bexpr expression against resource labels
// Empty scopeExpr returns true (no constraint)
// Caches compiled evaluators for performance
//
// Reference: research.md §1 (lines 443-482)
func EvaluateBexpr(scopeExpr string, labels map[string]any) bool {
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
