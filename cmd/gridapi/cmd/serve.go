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
	"github.com/terraconstructs/grid/cmd/gridapi/internal/dependency"
	gridmiddleware "github.com/terraconstructs/grid/cmd/gridapi/internal/middleware"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/repository"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/server"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/state"
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

		// Initialize services
		svc := state.NewService(stateRepo, cfg.ServerURL).
			WithOutputRepository(outputRepo).
			WithEdgeRepository(edgeRepo).
			WithPolicyRepository(labelPolicyRepo)
		depService := dependency.NewService(edgeRepo, stateRepo).
			WithOutputRepository(outputRepo)
		edgeUpdater := server.NewEdgeUpdateJob(edgeRepo, stateRepo)
		policyService := state.NewPolicyService(labelPolicyRepo, state.NewPolicyValidator())

		var chiMiddleware []func(http.Handler) http.Handler
		var connectInterceptors []connect.Interceptor
		var oidcRouter chi.Router
		var relyingParty *auth.RelyingParty
		var provider *auth.Provider

		authnDeps := gridmiddleware.AuthnDependencies{
			Sessions:        sessionRepo,
			Users:           userRepo,
			UserRoles:       userRoleRepo,
			ServiceAccounts: serviceAccountRepo,
			RevokedJTIs:     revokedJTIRepo,
			GroupRoles:      groupRoleRepo,
			Roles:           roleRepo,
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

		if oidcEnabled {
			enforcer, err := auth.InitEnforcer(db, cfg.CasbinModelPath)
			if err != nil {
				return fmt.Errorf("configure casbin enforcer: %w", err)
			}
			enforcer.EnableAutoSave(true)
			authnDeps.Enforcer = enforcer

			authnMiddleware, err := gridmiddleware.NewAuthnMiddleware(cfg, authnDeps)
			if err != nil {
				return fmt.Errorf("configure authentication middleware: %w", err)
			}
			chiMiddleware = append(chiMiddleware, authnMiddleware)

			// HTTP-specific authz middleware (for tfstate endpoints)
			authzMiddleware, err := gridmiddleware.NewAuthzMiddleware(gridmiddleware.AuthzDependencies{
				Enforcer:     enforcer,
				StateService: svc,
			})
			if err != nil {
				return fmt.Errorf("configure authorization middleware: %w", err)
			}
			chiMiddleware = append(chiMiddleware, authzMiddleware)

			// Connect-specific authz interceptor
			authzInterceptor := gridmiddleware.NewAuthzInterceptor(gridmiddleware.AuthzDependencies{
				Enforcer:     enforcer,
				StateService: svc,
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

		// Wait for interrupt signal
		shutdown := make(chan os.Signal, 1)
		signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

		select {
		case err := <-serverErrors:
			return fmt.Errorf("server error: %w", err)
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
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
