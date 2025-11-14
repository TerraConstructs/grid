package tfstate

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/btcsuite/btcutil/base58"
)

// ComputeFingerprint produces a deterministic SHA-256 fingerprint of a Terraform output value
func ComputeFingerprint(value interface{}) string {
	canonical := canonicalJSON(value)
	if canonical == nil {
		return ""
	}

	hash := sha256.Sum256(canonical)
	return base58.Encode(hash[:])
}

// canonicalJSON produces deterministic JSON encoding for Terraform output values
// Handles: nil, bool, float64, int, string, []interface{}, map[string]interface{}
func canonicalJSON(v interface{}) []byte {
	switch val := v.(type) {
	case nil:
		return []byte("null")

	case bool:
		if val {
			return []byte("true")
		}
		return []byte("false")

	case float64:
		// Use standard JSON encoding for numbers
		b, _ := json.Marshal(val)
		return b

	case int:
		b, _ := json.Marshal(val)
		return b

	case string:
		b, _ := json.Marshal(val)
		return b

	case []interface{}:
		// Array: encode each element and join
		var elements [][]byte
		for _, elem := range val {
			elements = append(elements, canonicalJSON(elem))
		}
		return joinArrayElements(elements)

	case map[string]interface{}:
		// Object: sort keys, encode key-value pairs
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		var pairs [][]byte
		for _, k := range keys {
			keyJSON, _ := json.Marshal(k)
			valueJSON := canonicalJSON(val[k])
			pair := append(keyJSON, ':')
			pair = append(pair, valueJSON...)
			pairs = append(pairs, pair)
		}
		return joinObjectPairs(pairs)

	default:
		// Fallback to standard JSON encoding for unknown types
		b, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		return b
	}
}

// joinArrayElements joins array elements into canonical JSON array format
func joinArrayElements(elements [][]byte) []byte {
	if len(elements) == 0 {
		return []byte("[]")
	}

	result := []byte("[")
	for i, elem := range elements {
		result = append(result, elem...)
		if i < len(elements)-1 {
			result = append(result, ',')
		}
	}
	result = append(result, ']')
	return result
}

// joinObjectPairs joins key-value pairs into canonical JSON object format
func joinObjectPairs(pairs [][]byte) []byte {
	if len(pairs) == 0 {
		return []byte("{}")
	}

	result := []byte("{")
	for i, pair := range pairs {
		result = append(result, pair...)
		if i < len(pairs)-1 {
			result = append(result, ',')
		}
	}
	result = append(result, '}')
	return result
}

// CompareFingerprints compares two fingerprints for equality
func CompareFingerprints(a, b string) bool {
	return a != "" && b != "" && a == b
}

// FormatFingerprint returns a truncated fingerprint for display (first 12 characters)
func FormatFingerprint(fingerprint string) string {
	if len(fingerprint) <= 12 {
		return fingerprint
	}
	return fmt.Sprintf("%s...", fingerprint[:12])
}
