package server

import (
	"net/http"

	"github.com/terraconstructs/grid/api/state/v1/statev1connect"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/dependency"
	statepkg "github.com/terraconstructs/grid/cmd/gridapi/internal/state"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// RouterOptions controls the construction of the Grid HTTP router.
// The zero value is valid; sensible defaults are applied where fields are not set.
type RouterOptions struct {
	Service           *statepkg.Service
	DependencyService *dependency.Service
	EdgeUpdater       *EdgeUpdateJob

	// CORSOptions customises the access-control configuration. When nil,
	// DefaultCORSOptions() is applied. A copy of the provided options is used.
	CORSOptions *cors.Options

	// Middleware are appended after the default middleware stack
	// (RequestID, RealIP, Logger, Recoverer).
	Middleware []func(http.Handler) http.Handler

	// HealthHandler overrides the default /health handler.
	HealthHandler http.HandlerFunc

	// ExtraRoutes can register additional endpoints after the built-in handlers.
	ExtraRoutes func(chi.Router)
}

// DefaultCORSOptions returns the shared development CORS policy.
func DefaultCORSOptions() cors.Options {
	return cors.Options{
		AllowedOrigins: []string{
			"http://localhost:5173",
			"http://127.0.0.1:5173",
		},
		AllowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodOptions},
		AllowedHeaders: []string{
			"Content-Type",
			"Connect-Protocol-Version",
			"Connect-Timeout-Ms",
			"Connect-Protocol",
			"Connect-Content-Encoding",
			"Grpc-Timeout",
			"X-Grpc-Web",
			"X-User-Agent",
		},
		ExposedHeaders: []string{
			"Connect-Protocol-Version",
			"Connect-Content-Encoding",
			"Connect-Protocol",
			"Grpc-Status",
			"Grpc-Message",
			"Grpc-Status-Details-Bin",
		},
		AllowCredentials: false,
		MaxAge:           300,
	}
}

func defaultHealthHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

// NewRouter assembles a chi.Router with shared middleware, CORS policy, and
// the Grid handlers mounted. The router can be tailored via RouterOptions for
// CLI usage, tests, or other entrypoints.
func NewRouter(opts RouterOptions) chi.Router {
	r := chi.NewRouter()

	// Baseline middleware shared across entrypoints.
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	for _, mw := range opts.Middleware {
		if mw != nil {
			r.Use(mw)
		}
	}

	corsCfg := DefaultCORSOptions()
	if opts.CORSOptions != nil {
		corsCfg = *opts.CORSOptions
	}
	r.Use(cors.Handler(corsCfg))

	if opts.Service != nil {
		MountConnectHandlers(r, opts.Service, opts.DependencyService)
	}
	if opts.Service != nil && opts.EdgeUpdater != nil {
		MountTerraformBackend(r, opts.Service, opts.EdgeUpdater)
	}

	healthHandler := opts.HealthHandler
	if healthHandler == nil {
		healthHandler = defaultHealthHandler
	}
	r.Get("/health", healthHandler)

	if opts.ExtraRoutes != nil {
		opts.ExtraRoutes(r)
	}

	return r
}

// NewH2CHandler wraps the shared router with an h2c server to provide HTTP/2 over
// cleartext, matching the expectations of Connect clients during development.
func NewH2CHandler(opts RouterOptions) http.Handler {
	return h2c.NewHandler(NewRouter(opts), &http2.Server{})
}

// NewHTTPServer remains for compatibility with older callers that only need the
// state service mounted with default settings.
func NewHTTPServer(service *statepkg.Service) http.Handler {
	return NewH2CHandler(RouterOptions{Service: service})
}

// MountConnectHandlers mounts Connect RPC handlers on the provided router.
func MountConnectHandlers(r chi.Router, service *statepkg.Service, depService *dependency.Service) {
	stateHandler := NewStateServiceHandler(service)
	stateHandler.depService = depService
	path, handler := statev1connect.NewStateServiceHandler(stateHandler)
	r.Mount(path, handler)
}
