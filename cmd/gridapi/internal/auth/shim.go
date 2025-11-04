package auth

import (
	"encoding/base64"
	"net/http"
	"strings"
)

// TerraformBasicAuthShim rewrites Terraform's Basic Auth credential promotion into a standard
// Bearer token header. The password component of the decoded credential is treated as the bearer
// token and injected into the Authorization header so downstream middleware can operate on a
// canonical representation. Requests that already present a Bearer token are left untouched.
func TerraformBasicAuthShim(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if shouldPromoteBasicAuth(r) {
			if token := tokenFromBasicAuth(r.Header.Get("Authorization")); token != "" {
				r.Header.Set("Authorization", "Bearer "+token)
			}
		}

		next.ServeHTTP(w, r)
	})
}

func shouldPromoteBasicAuth(r *http.Request) bool {
	if r == nil {
		return false
	}

	// Only promote for Terraform HTTP backend routes.
	if !strings.HasPrefix(r.URL.Path, "/tfstate") {
		return false
	}

	authz := r.Header.Get("Authorization")
	if authz == "" {
		return false
	}
	if strings.HasPrefix(authz, "Bearer ") {
		return false
	}

	return strings.HasPrefix(authz, "Basic ")
}

func tokenFromBasicAuth(header string) string {
	payload := strings.TrimPrefix(header, "Basic ")
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return ""
	}

	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return ""
	}

	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return ""
	}

	password := strings.TrimSpace(parts[1])
	if password == "" {
		return ""
	}

	return password
}
