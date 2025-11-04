package auth

import (
	"fmt"
	"reflect"
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

		labels, ok := toStringAnyMap(args[1])
		if !ok {
			return false, fmt.Errorf("bexprMatch: second argument must be a map with string keys (labels)")
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

// toStringAnyMap attempts to coerce an arbitrary value into a map[string]any.
// Accepts:
// - map[string]any directly
// - Any map type with string keys (e.g., a named type whose underlying type is map[string]any)
// Returns a fresh map for non-exact types to avoid mutation surprises.
func toStringAnyMap(v any) (map[string]any, bool) {
	if v == nil {
		return map[string]any{}, true
	}
	if m, ok := v.(map[string]any); ok {
		return m, true
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return map[string]any{}, true
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Map {
		return nil, false
	}
	if rv.Type().Key().Kind() != reflect.String {
		return nil, false
	}

	out := make(map[string]any, rv.Len())
	iter := rv.MapRange()
	for iter.Next() {
		k := iter.Key().String()
		out[k] = iter.Value().Interface()
	}
	return out, true
}
