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

	// Run tests
	code := m.Run()

	// Teardown: Stop server
	stopServer()

	os.Exit(code)
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
		serverCmd.Process.Kill()
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
		serverCmd.Process.Kill()
	}
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
