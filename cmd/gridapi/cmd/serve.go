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

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/bunx"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/repository"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/server"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/state"
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

		// Initialize repository and service
		repo := repository.NewBunStateRepository(db)
		svc := state.NewService(repo, cfg.ServerURL)

		// Create Chi router
		r := chi.NewRouter()

		// Middleware
		r.Use(middleware.RequestID)
		r.Use(middleware.RealIP)
		r.Use(middleware.Logger)
		r.Use(middleware.Recoverer)
		r.Use(middleware.Timeout(60 * time.Second))

		// Mount Connect RPC handlers
		server.MountConnectHandlers(r, svc)

		// Mount Terraform HTTP Backend handlers
		server.MountTerraformBackend(r, svc)

		// Health check endpoint
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})

		// Create HTTP server
		srv := &http.Server{
			Addr:         cfg.ServerAddr,
			Handler:      r,
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
