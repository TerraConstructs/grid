package integration

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

var (
	serverCmd *exec.Cmd
)

func TestMain(m *testing.M) {
	// Pre-flight check: Ensure no leftover gridapi servers are running
	if err := checkForLeftoverServers(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Pre-flight check failed: %v\n", err)
		fmt.Fprintf(os.Stderr, "\nPlease kill any leftover gridapi processes:\n")
		fmt.Fprintf(os.Stderr, "  pkill gridapi\n\n")
		os.Exit(1)
	}

	// Setup: Start server
	if err := startServer(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start server: %v\n", err)
		os.Exit(1)
	}

	// Wait for server to be ready
	if err := waitForServer(30 * time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "Server failed to become ready: %v\n", err)
		stopServer()
		os.Exit(1)
	}

	// Mode 1: Bootstrap test client id to platform-engineers role
	if err := bootstrapMode1TestUser(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to bootstrap Mode 1 test user: %v\n", err)
		stopServer()
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Teardown: Stop server
	stopServer()

	os.Exit(code)
}

func checkForLeftoverServers() error {
	// Check if port 8080 is already in use
	conn, err := http.Get("http://localhost:8080/health")
	if err == nil {
		conn.Body.Close()
		return fmt.Errorf("port 8080 is already in use - found running server responding to /health")
	}

	// Check for gridapi processes
	cmd := exec.Command("pgrep", "-fl", "gridapi serve")
	output, err := cmd.Output()
	if err == nil && len(output) > 0 {
		return fmt.Errorf("found existing gridapi processes:\n%s", string(output))
	}

	return nil
}

func startServer() error {
	gridapiPath := os.Getenv("GRIDAPI_PATH")
	if gridapiPath == "" {
		var err error
		gridapiPath, err = filepath.Abs("../../bin/gridapi")
		if err != nil {
			return fmt.Errorf("failed to get gridapi path: %w", err)
		}
	}

	serverCmd = exec.Command(gridapiPath, "serve",
		"--server-addr", ":8080",
		"--db-url", "postgres://grid:gridpass@localhost:5432/grid?sslmode=disable")

	// Inherit environment variables from parent process
	// This allows Mode 1 (EXTERNAL_IDP_*) and Mode 2 (OIDC_*) config to be passed through
	serverCmd.Env = os.Environ()

	serverCmd.Stdout = os.Stdout
	serverCmd.Stderr = os.Stderr

	if err := serverCmd.Start(); err != nil {
		return fmt.Errorf("failed to start gridapi: %w", err)
	}

	fmt.Printf("Started gridapi server (PID: %d)\n", serverCmd.Process.Pid)
	return nil
}

func stopServer() {
	if serverCmd == nil || serverCmd.Process == nil {
		return
	}

	fmt.Printf("Stopping gridapi server (PID: %d)\n", serverCmd.Process.Pid)

	// Send SIGTERM for graceful shutdown
	if err := serverCmd.Process.Signal(syscall.SIGTERM); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to send SIGTERM: %v\n", err)
		// Force kill if graceful shutdown fails
		_ = serverCmd.Process.Kill()
	}

	// Wait for process to exit (with timeout)
	done := make(chan error, 1)
	go func() {
		done <- serverCmd.Wait()
	}()

	select {
	case <-done:
		fmt.Println("Server stopped gracefully")
	case <-time.After(5 * time.Second):
		fmt.Println("Server shutdown timeout, forcing kill")
		_ = serverCmd.Process.Kill()
	}
}

func bootstrapMode1TestUser() error {
	// Only bootstrap in Mode 1 (External IdP mode)
	externalIdPIssuer := os.Getenv("EXTERNAL_IDP_ISSUER")
	if externalIdPIssuer == "" {
		// Not in Mode 1, skip bootstrap
		return nil
	}

	testClientID := os.Getenv("MODE1_TEST_CLIENT_ID")
	if testClientID == "" {
		// No test client configured, skip bootstrap
		return nil
	}

	fmt.Printf("Bootstrapping Mode 1 group→role mappings for testing\n")

	// Bootstrap group→role mapping for test admins
	// NOTE: The integration-tests Keycloak client is a service account.
	// For this to work, Keycloak must be configured to add a group claim to service account tokens.
	// This is done via the protocol mapper in the Keycloak setup (see tests/fixtures/realm-export.json).
	// The Keycloak setup script should add the service account to the "test-admins" group.

	gridapiPath := os.Getenv("GRIDAPI_PATH")
	if gridapiPath == "" {
		var err error
		gridapiPath, err = filepath.Abs("../../bin/gridapi")
		if err != nil {
			return fmt.Errorf("failed to get gridapi path: %w", err)
		}
	}

	// Bootstrap: "test-admins" group → platform-engineer role
	cmd := exec.Command(gridapiPath, "iam", "bootstrap",
		"--group", "test-admins",
		"--role", "platform-engineer")

	cmd.Env = append(os.Environ(),
		"DATABASE_URL=postgres://grid:gridpass@localhost:5432/grid?sslmode=disable")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("bootstrap command failed: %w\nOutput: %s", err, string(output))
	}

	fmt.Printf("✓ Group→role mapping bootstrapped: test-admins → platform-engineer\n")
	fmt.Println("  Integration-tests service account must have 'test-admins' group in JWT")

	// CRITICAL: Bootstrap runs in separate process with its own cache
	// The test server's cache is stale until we force a refresh
	// Send SIGHUP to trigger immediate cache refresh (handled in cmd/serve.go:277-300)
	fmt.Printf("  Sending SIGHUP to gridapi server (PID: %d) to refresh cache...\n", serverCmd.Process.Pid)
	if err := serverCmd.Process.Signal(syscall.SIGHUP); err != nil {
		return fmt.Errorf("failed to send SIGHUP to server: %w", err)
	}

	// Give the server a moment to process the signal and refresh the cache
	// The refresh is synchronous once SIGHUP is handled (< 100ms typically)
	time.Sleep(500 * time.Millisecond)
	fmt.Println("  ✓ Cache refresh signal sent")

	return nil
}

func waitForServer(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	healthURL := fmt.Sprintf("%s/health", serverURL)
	client := &http.Client{Timeout: 1 * time.Second}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	fmt.Print("Waiting for server to become ready")
	for {
		select {
		case <-ctx.Done():
			fmt.Println(" ✗ timeout")
			return fmt.Errorf("timeout waiting for server health check")
		case <-ticker.C:
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
			if err != nil {
				fmt.Print(".")
				continue
			}

			resp, err := client.Do(req)
			if err != nil {
				fmt.Print(".")
				continue
			}
			resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				fmt.Println(" ✓")
				return nil
			}
			fmt.Print(".")
		}
	}
}
