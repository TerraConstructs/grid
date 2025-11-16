package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	statepkg "github.com/terraconstructs/grid/cmd/gridapi/internal/services/state"
)

func init() {
	// Register custom HTTP methods for Terraform HTTP Backend
	chi.RegisterMethod("LOCK")
	chi.RegisterMethod("UNLOCK")
}

// MountTerraformBackend registers Terraform HTTP Backend handlers on the router
// with proper method whitelisting for LOCK/UNLOCK custom methods
func MountTerraformBackend(r chi.Router, service *statepkg.Service, edgeUpdater *EdgeUpdateJob) {
	handlers := NewTerraformHandlers(service, edgeUpdater)

	// GET /tfstate/{guid} - retrieve state
	r.Get("/tfstate/{guid}", handlers.GetState)

	// POST /tfstate/{guid} - update state
	r.Post("/tfstate/{guid}", handlers.UpdateState)

	// LOCK /tfstate/{guid}/lock (with PUT fallback for Terraform compatibility)
	// Terraform sends custom LOCK method, but some clients may use PUT
	r.Method("LOCK", "/tfstate/{guid}/lock", http.HandlerFunc(handlers.LockState))
	r.Put("/tfstate/{guid}/lock", handlers.LockState)

	// UNLOCK /tfstate/{guid}/unlock (with PUT fallback)
	r.Method("UNLOCK", "/tfstate/{guid}/unlock", http.HandlerFunc(handlers.UnlockState))
	r.Put("/tfstate/{guid}/unlock", handlers.UnlockState)
}
