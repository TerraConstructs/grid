package cmd

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/bunx"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/dependency"
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

		// Initialize services
		labelPolicyRepo := repository.NewBunLabelPolicyRepository(db)

		svc := state.NewService(stateRepo, cfg.ServerURL).
			WithOutputRepository(outputRepo).
			WithEdgeRepository(edgeRepo).
			WithPolicyRepository(labelPolicyRepo)
		depService := dependency.NewService(edgeRepo, stateRepo).
			WithOutputRepository(outputRepo)
		edgeUpdater := server.NewEdgeUpdateJob(edgeRepo, stateRepo)
		policyService := state.NewPolicyService(labelPolicyRepo, state.NewPolicyValidator())

		// Assemble the shared router with the production-specific middleware.
		routerOpts := server.RouterOptions{
			Service:           svc,
			DependencyService: depService,
			EdgeUpdater:       edgeUpdater,
			PolicyService:     policyService,
			Middleware: []func(http.Handler) http.Handler{
				middleware.Timeout(60 * time.Second),
			},
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
