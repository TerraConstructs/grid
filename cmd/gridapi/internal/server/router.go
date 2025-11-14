package server

import (
	"log"
	"net/http"

	"connectrpc.com/connect"
	"github.com/terraconstructs/grid/api/state/v1/statev1connect"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/auth"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/config"
	gridmiddleware "github.com/terraconstructs/grid/cmd/gridapi/internal/middleware"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/services/dependency"
	statepkg "github.com/terraconstructs/grid/cmd/gridapi/internal/services/state"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// RouterOptions controls the construction of the Grid HTTP router.
// The zero value is valid; sensible defaults are applied where fields are not set.
type RouterOptions struct {
	Service             *statepkg.Service
	DependencyService   *dependency.Service
	EdgeUpdater         *EdgeUpdateJob
	PolicyService       *statepkg.PolicyService
	Provider            *auth.Provider
	RelyingParty        *auth.RelyingParty
	IAMService          iamAdminService // Compile-time verified IAM service contract
	AuthnDeps           gridmiddleware.AuthnDependencies
	Cfg                 *config.Config
	OIDCRouter          chi.Router
	CORSOptions         *cors.Options
	Middleware          []func(http.Handler) http.Handler
	ConnectInterceptors []connect.Interceptor
	HealthHandler       http.HandlerFunc
	ExtraRoutes         func(chi.Router)
}

// DefaultCORSOptions returns the shared development CORS policy.
func DefaultCORSOptions() cors.Options {
	return cors.Options{
		AllowedOrigins: []string{
			"http://localhost:5173",
			"http://127.0.0.1:5173",
			"http://localhost:5174",
			"http://127.0.0.1:5174",
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
			"Authorization",
		},
		ExposedHeaders: []string{
			"Connect-Protocol-Version",
			"Connect-Content-Encoding",
			"Connect-Protocol",
			"Grpc-Status",
			"Grpc-Message",
			"Grpc-Status-Details-Bin",
		},
		AllowCredentials: true,
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

	corsCfg := DefaultCORSOptions()
	if opts.CORSOptions != nil {
		corsCfg = *opts.CORSOptions
	}
	r.Use(cors.Handler(corsCfg))

	// Apply custom middleware passed from the caller.
	for _, mw := range opts.Middleware {
		if mw != nil {
			r.Use(mw)
		}
	}

	// Internal IdP mode: Mount OIDC provider and internal login endpoint
	if opts.OIDCRouter != nil {
		log.Println("Mounting OIDC router")
		r.Mount("/", opts.OIDCRouter)
		if opts.IAMService != nil {
			r.Post("/auth/login", HandleInternalLogin(opts.IAMService))
		} else {
			log.Println("WARNING: Skipping /auth/login - IAMService not available")
		}
	}

	// External IdP mode: Mount SSO endpoints
	if opts.RelyingParty != nil {
		r.Get("/auth/sso/login", HandleSSOLogin(opts.RelyingParty))
		if opts.IAMService != nil {
			r.Get("/auth/sso/callback", HandleSSOCallback(opts.RelyingParty, opts.IAMService))
		} else {
			log.Println("WARNING: Skipping /auth/sso/callback - IAMService not available")
		}
	}

	// Common auth endpoints: Available in both internal and external IdP modes
	if opts.OIDCRouter != nil || opts.RelyingParty != nil {
		if opts.IAMService != nil {
			r.Get("/api/auth/whoami", HandleWhoAmI(opts.IAMService))
			r.Post("/auth/logout", HandleLogout(opts.IAMService))

			// Admin endpoints (requires appropriate permissions)
			r.Post("/admin/cache/refresh", HandleCacheRefresh(opts.IAMService))
		} else {
			log.Println("WARNING: Skipping /api/auth/whoami and /auth/logout - IAMService not available")
		}
	}

	if opts.Service != nil {
		MountConnectHandlers(r, opts)
	}
	if opts.Service != nil && opts.EdgeUpdater != nil {
		MountTerraformBackend(r, opts.Service, opts.EdgeUpdater)
	}

	healthHandler := opts.HealthHandler
	if healthHandler == nil {
		healthHandler = defaultHealthHandler
	}
	r.Get("/health", healthHandler)

	// Authentication configuration discovery endpoint for SDK clients
	if opts.Cfg != nil {
		r.Get("/auth/config", HandleAuthConfig(opts.Cfg))
	}

	if opts.ExtraRoutes != nil {
		opts.ExtraRoutes(r)
	}

	return r
}

// NewH2CHandler wraps the shared router with an h2c server to provide HTTP/2 over
// cleartext, matching the expectations of Connect clients during development.
func NewH2CHandler(opts RouterOptions) (http.Handler, error) {
	router := NewRouter(opts)
	return h2c.NewHandler(router, &http2.Server{}), nil
}

// NewHTTPServer remains for compatibility with older callers that only need the
// state service mounted with default settings.
func NewHTTPServer(service *statepkg.Service) (http.Handler, error) {
	return NewH2CHandler(RouterOptions{Service: service})
}

// MountConnectHandlers mounts Connect RPC handlers on the provided router.
func MountConnectHandlers(r chi.Router, opts RouterOptions) {
	stateHandler := NewStateServiceHandler(opts.Service, &opts.AuthnDeps, opts.Cfg)
	stateHandler.depService = opts.DependencyService
	if opts.PolicyService != nil {
		stateHandler.WithPolicyService(opts.PolicyService)
	}
	if opts.IAMService != nil {
		stateHandler.WithIAMService(opts.IAMService)
	}
	path, handler := statev1connect.NewStateServiceHandler(
		stateHandler,
		connect.WithInterceptors(opts.ConnectInterceptors...),
	)
	r.Mount(path, handler)
}
