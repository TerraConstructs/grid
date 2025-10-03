package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	statepkg "github.com/terraconstructs/grid/cmd/gridapi/internal/state"
)

// StateService defines the interface for state operations needed by Terraform handlers
type StateService interface {
	GetStateByGUID(ctx context.Context, guid string) (*models.State, error)
	UpdateStateContent(ctx context.Context, guid string, content []byte, lockID string) (*statepkg.StateSummary, error)
	LockState(ctx context.Context, guid string, lockInfo *models.LockInfo) error
	UnlockState(ctx context.Context, guid string, lockID string) error
	GetStateLock(ctx context.Context, guid string) (*models.LockInfo, error)
}

// TerraformHandlers wires the Terraform HTTP Backend REST endpoints
type TerraformHandlers struct {
	service     StateService
	edgeUpdater *EdgeUpdateJob
}

// NewTerraformHandlers creates a new handler set for Terraform backend operations
func NewTerraformHandlers(service *statepkg.Service, edgeUpdater *EdgeUpdateJob) *TerraformHandlers {
	return &TerraformHandlers{
		service:     service,
		edgeUpdater: edgeUpdater,
	}
}

// GetState handles GET /tfstate/{guid} - retrieve current state
func (h *TerraformHandlers) GetState(w http.ResponseWriter, r *http.Request) {
	guid := chi.URLParam(r, "guid")
	if guid == "" {
		http.Error(w, "guid is required", http.StatusBadRequest)
		return
	}

	// Fetch state via service (which uses repository)
	state, err := h.service.GetStateByGUID(r.Context(), guid)
	if err != nil {
		if isNotFoundError(err) {
			http.Error(w, fmt.Sprintf("state not found: %s", guid), http.StatusNotFound)
		} else {
			http.Error(w, fmt.Sprintf("failed to get state: %v", err), http.StatusInternalServerError)
		}
		return
	}

	// Return state content (or minimal valid Terraform state if no state yet)
	if len(state.StateContent) == 0 {
		// Return minimal valid Terraform state that satisfies version requirement
		emptyState := []byte(`{"version":4,"terraform_version":"","serial":0,"lineage":"","outputs":null,"resources":null}`)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(emptyState)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(state.StateContent)
}

// UpdateState handles POST /tfstate/{guid} - update state content
func (h *TerraformHandlers) UpdateState(w http.ResponseWriter, r *http.Request) {
	guid := chi.URLParam(r, "guid")
	if guid == "" {
		http.Error(w, "guid is required", http.StatusBadRequest)
		return
	}

	// Check if state exists first (before parsing body)
	_, err := h.service.GetStateByGUID(r.Context(), guid)
	if err != nil {
		if isNotFoundError(err) {
			http.Error(w, fmt.Sprintf("state not found: %s", guid), http.StatusNotFound)
		} else {
			http.Error(w, fmt.Sprintf("failed to get state: %v", err), http.StatusInternalServerError)
		}
		return
	}

	// Read state content from request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to read request body: %v", err), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Validate it's valid JSON
	if !json.Valid(body) {
		http.Error(w, "request body must be valid JSON", http.StatusBadRequest)
		return
	}

	// Get lock ID from query parameter (Terraform sends ?ID=lockID when holding lock)
	lockID := r.URL.Query().Get("ID")

	// Update state content via service (service will verify lock ID if state is locked)
	summary, err := h.service.UpdateStateContent(r.Context(), guid, body, lockID)
	if err != nil {
		if isNotFoundError(err) {
			http.Error(w, fmt.Sprintf("state not found: %s", guid), http.StatusNotFound)
		} else if isLockedError(err) {
			http.Error(w, fmt.Sprintf("state is locked: %v", err), http.StatusLocked)
		} else {
			http.Error(w, fmt.Sprintf("failed to update state: %v", err), http.StatusInternalServerError)
		}
		return
	}

	// Trigger EdgeUpdateJob asynchronously (best effort, fire-and-forget)
	if h.edgeUpdater != nil {
		go h.edgeUpdater.UpdateEdges(context.Background(), guid, body)
	}

	// Check if size threshold exceeded (10MB warning)
	if summary.SizeBytes > models.StateSizeWarningThreshold {
		w.Header().Set("X-Grid-State-Size-Warning", fmt.Sprintf("State size (%d bytes) exceeds recommended threshold (%d bytes)", summary.SizeBytes, models.StateSizeWarningThreshold))
	}

	w.WriteHeader(http.StatusOK)
}

// LockState handles LOCK/PUT /tfstate/{guid}/lock - acquire lock
func (h *TerraformHandlers) LockState(w http.ResponseWriter, r *http.Request) {
	guid := chi.URLParam(r, "guid")
	if guid == "" {
		http.Error(w, "guid is required", http.StatusBadRequest)
		return
	}

	// Check if state exists first (before parsing body)
	_, err := h.service.GetStateByGUID(r.Context(), guid)
	if err != nil {
		if isNotFoundError(err) {
			http.Error(w, fmt.Sprintf("state not found: %s", guid), http.StatusNotFound)
		} else {
			http.Error(w, fmt.Sprintf("failed to get state: %v", err), http.StatusInternalServerError)
		}
		return
	}

	// Parse lock info from request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to read request body: %v", err), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var lockInfo models.LockInfo
	if err := json.Unmarshal(body, &lockInfo); err != nil {
		http.Error(w, fmt.Sprintf("failed to parse lock info: %v", err), http.StatusBadRequest)
		return
	}

	// Set created timestamp if not provided
	if lockInfo.Created.IsZero() {
		lockInfo.Created = time.Now()
	}

	// Attempt to acquire lock via service
	err = h.service.LockState(r.Context(), guid, &lockInfo)
	if err != nil {
		if isNotFoundError(err) {
			http.Error(w, fmt.Sprintf("state not found: %s", guid), http.StatusNotFound)
		} else if isAlreadyLockedError(err) {
			// Return 423 Locked with current lock info
			currentLock, lockErr := h.service.GetStateLock(r.Context(), guid)
			if lockErr == nil && currentLock != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusLocked)
				json.NewEncoder(w).Encode(currentLock)
			} else {
				http.Error(w, fmt.Sprintf("state is already locked: %v", err), http.StatusLocked)
			}
		} else {
			http.Error(w, fmt.Sprintf("failed to lock state: %v", err), http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
}

// UnlockState handles UNLOCK/PUT /tfstate/{guid}/unlock - release lock
func (h *TerraformHandlers) UnlockState(w http.ResponseWriter, r *http.Request) {
	guid := chi.URLParam(r, "guid")
	if guid == "" {
		http.Error(w, "guid is required", http.StatusBadRequest)
		return
	}

	// Check if state exists first (before parsing body)
	_, err := h.service.GetStateByGUID(r.Context(), guid)
	if err != nil {
		if isNotFoundError(err) {
			http.Error(w, fmt.Sprintf("state not found: %s", guid), http.StatusNotFound)
		} else {
			http.Error(w, fmt.Sprintf("failed to get state: %v", err), http.StatusInternalServerError)
		}
		return
	}

	// Parse lock info from request body to get lock ID
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to read request body: %v", err), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var lockInfo models.LockInfo
	if err := json.Unmarshal(body, &lockInfo); err != nil {
		http.Error(w, fmt.Sprintf("failed to parse lock info: %v", err), http.StatusBadRequest)
		return
	}

	// Unlock via service
	err = h.service.UnlockState(r.Context(), guid, lockInfo.ID)
	if err != nil {
		if isNotFoundError(err) {
			http.Error(w, fmt.Sprintf("state not found: %s", guid), http.StatusNotFound)
		} else if isLockMismatchError(err) {
			http.Error(w, fmt.Sprintf("lock ID mismatch: %v", err), http.StatusConflict)
		} else {
			http.Error(w, fmt.Sprintf("failed to unlock state: %v", err), http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
}

// Helper functions for error classification
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return contains(errStr, "not found")
}

func isLockedError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return contains(errStr, "locked") || contains(errStr, "cannot update")
}

func isAlreadyLockedError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return contains(errStr, "already locked") || contains(errStr, "affected rows = 0")
}

func isLockMismatchError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return contains(errStr, "mismatch") || contains(errStr, "expected")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
