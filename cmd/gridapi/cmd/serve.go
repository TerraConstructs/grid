package cmd

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"github.com/go-chi/chi/v5"
	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/auth"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/bunx"
	gridmiddleware "github.com/terraconstructs/grid/cmd/gridapi/internal/middleware"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/repository"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/server"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/services/dependency"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/services/iam"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/services/inference"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/services/state"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Grid API server",
	Long:  `Starts the HTTP server with Connect RPC and Terraform HTTP Backend endpoints.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Connect to database
		db, err := bunx.NewDB(cfg.DatabaseURL)
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer bunx.Close(db)

		log.Printf("Connected to database")

		// Initialize repositories
		stateRepo := repository.NewBunStateRepository(db)
		edgeRepo := repository.NewBunEdgeRepository(db)
		outputRepo := repository.NewBunStateOutputRepository(db)
		labelPolicyRepo := repository.NewBunLabelPolicyRepository(db)
		userRepo := repository.NewBunUserRepository(db)
		userRoleRepo := repository.NewBunUserRoleRepository(db)
		serviceAccountRepo := repository.NewBunServiceAccountRepository(db)
		sessionRepo := repository.NewBunSessionRepository(db)
		roleRepo := repository.NewBunRoleRepository(db)
		groupRoleRepo := repository.NewBunGroupRoleRepository(db)
		revokedJTIRepo := repository.NewBunRevokedJTIRepository(db)

		// Initialize inference service
		inferrer := inference.NewInferrer()

		// Initialize services
		svc := state.NewService(stateRepo, cfg.ServerURL).
			WithOutputRepository(outputRepo).
			WithEdgeRepository(edgeRepo).
			WithPolicyRepository(labelPolicyRepo).
			WithInferrer(inferrer)
		depService := dependency.NewService(edgeRepo, stateRepo).
			WithOutputRepository(outputRepo)
		edgeUpdater := server.NewEdgeUpdateJob(edgeRepo, stateRepo)
		policyService := state.NewPolicyService(labelPolicyRepo, state.NewPolicyValidator())

		var chiMiddleware []func(http.Handler) http.Handler
		var connectInterceptors []connect.Interceptor
		var oidcRouter chi.Router
		var relyingParty *auth.RelyingParty
		var provider *auth.Provider

		// Phase 6 Note: AuthnDependencies still used by auth handlers
		// (HandleInternalLogin, HandleSSOCallback, HandleWhoAmI, HandleLogout)
		// Phase 6 will refactor these handlers to use IAM service instead
		authnDeps := gridmiddleware.AuthnDependencies{
			Sessions:        sessionRepo,
			Users:           userRepo,
			UserRoles:       userRoleRepo,
			ServiceAccounts: serviceAccountRepo,
			RevokedJTIs:     revokedJTIRepo,
			GroupRoles:      groupRoleRepo,
			Roles:           roleRepo,
			Enforcer:        nil, // Set below if OIDC enabled
		}

		if cfg.OIDC.ExternalIdP != nil {
			rp, err := auth.NewRelyingParty(cmd.Context(), cfg.OIDC.ExternalIdP)
			if err != nil {
				return fmt.Errorf("failed to create relying party: %w", err)
			}
			relyingParty = rp
		}

		if cfg.OIDC.Issuer != "" {
			provider, err = auth.NewOIDCProvider(cmd.Context(), cfg.OIDC, auth.ProviderDependencies{
				Users:           userRepo,
				ServiceAccounts: serviceAccountRepo,
				Sessions:        sessionRepo,
			})
			if err != nil && !errors.Is(err, auth.ErrOIDCDisabled) {
				return fmt.Errorf("configure oidc provider: %w", err)
			}
			if err == nil {
				oidcRouter = provider.Router
				log.Printf("OIDC router created")
			}
		}

		oidcEnabled := cfg.OIDC.ExternalIdP != nil || cfg.OIDC.Issuer != ""

		// Declare iamService outside the block so it's available for RouterOptions
		var iamService iam.Service

		if oidcEnabled {
			enforcer, err := auth.InitEnforcer(db)
			if err != nil {
				return fmt.Errorf("configure casbin enforcer: %w", err)
			}
			// Phase 4: Disable AutoSave - we no longer mutate Casbin state
			// Authorization is now read-only (uses Principal.Roles, no AddGroupingPolicy)
			enforcer.EnableAutoSave(false)
			authnDeps.Enforcer = enforcer

			// Phase 3: Create IAM service (replaces scattered auth logic)
			iamService, err = iam.NewIAMService(
				iam.IAMServiceDependencies{
					Users:           userRepo,
					ServiceAccounts: serviceAccountRepo,
					Sessions:        sessionRepo,
					UserRoles:       userRoleRepo,
					GroupRoles:      groupRoleRepo,
					Roles:           roleRepo,
					RevokedJTIs:     revokedJTIRepo,
					Enforcer:        enforcer,
				},
				iam.IAMServiceConfig{
					Config: cfg,
				},
			)
			if err != nil {
				return fmt.Errorf("create IAM service: %w", err)
			}
			log.Printf("IAM service initialized with authenticators")

			// Phase 7: Start background cache refresh goroutine
			// Refreshes group→role cache periodically to pick up changes
			// Default interval: 5 minutes (configurable via CACHE_REFRESH_INTERVAL)
			refreshInterval := 5 * time.Minute
			if intervalEnv := os.Getenv("CACHE_REFRESH_INTERVAL"); intervalEnv != "" {
				if dur, err := time.ParseDuration(intervalEnv); err == nil {
					refreshInterval = dur
					log.Printf("Using custom cache refresh interval: %v", refreshInterval)
				} else {
					log.Printf("WARNING: Invalid CACHE_REFRESH_INTERVAL '%s', using default 5m", intervalEnv)
				}
			}

			// Create context for cache refresh goroutine
			cacheCtx, cancelCache := context.WithCancel(cmd.Context())
			defer cancelCache() // Cancel when function exits
			go func() {
				// Perform immediate refresh on startup to pick up any existing mappings
				// This ensures the cache is fresh even if bootstrap ran before server started
				if err := iamService.RefreshGroupRoleCache(cacheCtx); err != nil {
					log.Printf("ERROR: Initial cache refresh failed: %v", err)
				} else {
					snapshot := iamService.GetGroupRoleCacheSnapshot()
					log.Printf("INFO: Initial cache refresh complete (version=%d, groups=%d)",
						snapshot.Version, len(snapshot.Mappings))
				}

				ticker := time.NewTicker(refreshInterval)
				defer ticker.Stop()

				for {
					select {
					case <-ticker.C:
						if err := iamService.RefreshGroupRoleCache(cacheCtx); err != nil {
							log.Printf("ERROR: Background cache refresh failed: %v", err)
						} else {
							snapshot := iamService.GetGroupRoleCacheSnapshot()
							log.Printf("INFO: Background cache refreshed (version=%d, groups=%d)",
								snapshot.Version, len(snapshot.Mappings))
						}
					case <-cacheCtx.Done():
						log.Printf("INFO: Stopping background cache refresh")
						return
					}
				}
			}()

			// Terraform Basic Auth Shim: Convert Basic Auth to Bearer token
			// CRITICAL: Must run BEFORE authentication middleware
			// Terraform HTTP backend sends: Authorization: Basic base64(username:token)
			// This shim extracts the token and converts to: Authorization: Bearer token
			chiMiddleware = append(chiMiddleware, auth.TerraformBasicAuthShim)

			// Phase 3: Unified authentication middleware (replaces 3 old middlewares)
			// Tries authenticators in priority: Session → JWT
			multiAuthMiddleware := gridmiddleware.MultiAuthMiddleware(iamService)
			chiMiddleware = append(chiMiddleware, multiAuthMiddleware)

			// Phase 4: Authorization middleware (read-only, uses IAM service)
			authzMiddleware, err := gridmiddleware.NewAuthzMiddleware(gridmiddleware.AuthzDependencies{
				Enforcer:     enforcer,
				StateService: svc,
				IAMService:   iamService,
			})
			if err != nil {
				return fmt.Errorf("configure authorization middleware: %w", err)
			}
			chiMiddleware = append(chiMiddleware, authzMiddleware)

			// Phase 3: Connect RPC authentication (unified, tries all authenticators)
			// Note: Connect interceptors work differently - they see all requests
			// We'll use the same MultiAuth pattern but adapted for Connect
			multiAuthInterceptor := gridmiddleware.NewMultiAuthInterceptor(iamService)
			connectInterceptors = append(connectInterceptors, multiAuthInterceptor)

			// Phase 4: Connect RPC authorization (read-only, uses IAM service)
			authzInterceptor := gridmiddleware.NewAuthzInterceptor(gridmiddleware.AuthzDependencies{
				Enforcer:     enforcer,
				StateService: svc,
				IAMService:   iamService,
			})
			connectInterceptors = append(connectInterceptors, authzInterceptor)
		}

		healthHandler := func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"status":"ok","oidc_enabled":%t}`, oidcEnabled)
		}

		// Assemble the shared router with the production-specific middleware.
		routerOpts := server.RouterOptions{
			Service:             svc,
			DependencyService:   depService,
			EdgeUpdater:         edgeUpdater,
			PolicyService:       policyService,
			Provider:            provider,
			OIDCRouter:          oidcRouter,
			RelyingParty:        relyingParty,
			IAMService:          iamService,
			AuthnDeps:           authnDeps,
			Cfg:                 cfg,
			Middleware:          chiMiddleware,
			ConnectInterceptors: connectInterceptors,
			HealthHandler:       healthHandler,
		}
		r := server.NewRouter(routerOpts)

		// Wrap router with h2c for HTTP/2 cleartext support (required for Connect RPC)
		h2cHandler := h2c.NewHandler(r, &http2.Server{})

		// Create HTTP server
		srv := &http.Server{
			Addr:         cfg.ServerAddr,
			Handler:      h2cHandler,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		}

		// Start server in goroutine
		serverErrors := make(chan error, 1)
		go func() {
			log.Printf("Starting server on %s", cfg.ServerAddr)
			log.Printf("Server URL: %s", cfg.ServerURL)
			serverErrors <- srv.ListenAndServe()
		}()

		// Wait for interrupt signal or cache refresh signal
		shutdown := make(chan os.Signal, 1)
		signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

		// SIGHUP triggers IAM cache refresh (for E2E tests and manual cache updates)
		cacheRefresh := make(chan os.Signal, 1)
		signal.Notify(cacheRefresh, syscall.SIGHUP)

		for {
			select {
			case err := <-serverErrors:
				return fmt.Errorf("server error: %w", err)

			case sig := <-cacheRefresh:
				log.Printf("Received signal %v, refreshing IAM cache", sig)
				if iamService != nil {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					if err := iamService.RefreshGroupRoleCache(ctx); err != nil {
						log.Printf("ERROR: Manual cache refresh failed: %v", err)
					} else {
						snapshot := iamService.GetGroupRoleCacheSnapshot()
						log.Printf("INFO: Manual cache refresh complete via %v (version=%d, groups=%d)",
							sig, snapshot.Version, len(snapshot.Mappings))
					}
					cancel()
				} else {
					log.Printf("WARNING: Received %v but IAM service not initialized (OIDC disabled)", sig)
				}

			case sig := <-shutdown:
				log.Printf("Received signal %v, shutting down gracefully", sig)

				// Graceful shutdown with timeout
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				if err := srv.Shutdown(ctx); err != nil {
					srv.Close()
					return fmt.Errorf("graceful shutdown failed: %w", err)
				}

				log.Printf("Server stopped")
				return nil
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
