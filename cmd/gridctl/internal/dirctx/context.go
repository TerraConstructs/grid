package dirctx

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

const (
	// GridFileName is the name of the context file
	GridFileName = ".grid"
	// GridFileVersion is the current schema version
	GridFileVersion = "1"
)

// DirectoryContext represents the Grid state context for a directory
type DirectoryContext struct {
	Version      string    `json:"version"`
	StateGUID    string    `json:"state_guid"`
	StateLogicID string    `json:"state_logic_id"`
	ServerURL    string    `json:"server_url"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Validate checks if the DirectoryContext is valid
func (dc *DirectoryContext) Validate() error {
	if dc.Version != GridFileVersion {
		return fmt.Errorf("unsupported .grid file version: %s (expected %s)", dc.Version, GridFileVersion)
	}

	if dc.StateGUID == "" {
		return fmt.Errorf("state_guid is required")
	}

	// Validate GUID format
	if _, err := uuid.Parse(dc.StateGUID); err != nil {
		return fmt.Errorf("invalid state_guid format: %w", err)
	}

	if dc.StateLogicID == "" {
		return fmt.Errorf("state_logic_id is required")
	}

	return nil
}

// ReadGridContext reads the .grid file from the current directory
// Returns nil, nil if the file doesn't exist
// Returns nil, error if the file is corrupted or invalid
func ReadGridContext() (*DirectoryContext, error) {
	data, err := os.ReadFile(GridFileName)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No context file, not an error
		}
		return nil, fmt.Errorf("failed to read .grid file: %w", err)
	}

	var ctx DirectoryContext
	if err := json.Unmarshal(data, &ctx); err != nil {
		return nil, fmt.Errorf("corrupted .grid file (invalid JSON): %w", err)
	}

	if err := ctx.Validate(); err != nil {
		return nil, fmt.Errorf("invalid .grid file: %w", err)
	}

	return &ctx, nil
}

// WriteGridContext writes the directory context to .grid file atomically
// Uses temp file + rename pattern for atomic writes on POSIX systems
func WriteGridContext(ctx *DirectoryContext) error {
	if err := ctx.Validate(); err != nil {
		return fmt.Errorf("invalid context: %w", err)
	}

	// Marshal with 2-space indentation for readability
	data, err := json.MarshalIndent(ctx, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal context: %w", err)
	}

	// Add trailing newline for better git diffs
	data = append(data, '\n')

	// Write to temp file first
	tmpPath := GridFileName + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write .grid.tmp: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, GridFileName); err != nil {
		// Clean up temp file on failure
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename .grid.tmp to .grid: %w", err)
	}

	return nil
}

// TODO: Clean up - not used?

// StateRef represents a state identifier (either logic_id or guid)
type StateRef struct {
	LogicID string
	GUID    string
}

// IsEmpty returns true if neither logic_id nor guid is set
func (sr StateRef) IsEmpty() bool {
	return sr.LogicID == "" && sr.GUID == ""
}

// String returns a string representation for display
func (sr StateRef) String() string {
	if sr.LogicID != "" {
		return sr.LogicID
	}
	if sr.GUID != "" {
		return sr.GUID
	}
	return "<empty>"
}

// ResolveStateRef resolves the final state reference by applying priority:
// 1. Explicit parameters (explicitRef) take highest priority
// 2. Context from .grid file (contextRef) as fallback
// 3. Error if neither is provided
func ResolveStateRef(explicitRef, contextRef StateRef) (StateRef, error) {
	// Explicit parameters override context
	if !explicitRef.IsEmpty() {
		return explicitRef, nil
	}

	// Fall back to context
	if !contextRef.IsEmpty() {
		return contextRef, nil
	}

	// Neither provided
	return StateRef{}, fmt.Errorf("state identifier required: specify --logic-id/--guid or run in a directory with .grid context")
}

// TODO: Clean up - not used?

// GetGridFilePath returns the absolute path to the .grid file in the current directory
func GetGridFilePath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}
	return filepath.Join(cwd, GridFileName), nil
}
