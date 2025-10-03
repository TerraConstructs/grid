package server

import (
	"net/http"

	"github.com/terraconstructs/grid/api/state/v1/statev1connect"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/dependency"
	statepkg "github.com/terraconstructs/grid/cmd/gridapi/internal/state"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// NewHTTPServer creates a new HTTP server with Connect RPC and Terraform handlers.
func NewHTTPServer(service *statepkg.Service) http.Handler {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// Mount Connect RPC handlers
	stateHandler := NewStateServiceHandler(service)
	path, handler := statev1connect.NewStateServiceHandler(stateHandler)
	r.Mount(path, handler)

	// Wrap with h2c for HTTP/2 support (Connect RPC prefers HTTP/2)
	return h2c.NewHandler(r, &http2.Server{})
}

// MountConnectHandlers mounts Connect RPC handlers on the provided router
func MountConnectHandlers(r chi.Router, service *statepkg.Service, depService *dependency.Service) {
	stateHandler := NewStateServiceHandler(service)
	stateHandler.depService = depService
	path, handler := statev1connect.NewStateServiceHandler(stateHandler)
	r.Mount(path, handler)
}
