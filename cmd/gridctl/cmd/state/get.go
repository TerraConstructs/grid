package state

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/cmd/gridctl/internal/dirctx"
	"github.com/terraconstructs/grid/pkg/sdk"
)

var (
	getLogicID string
	getGUID    string
	getLink    bool
	getForce   bool
	getPath    string
	getFormat  string
)

var getCmd = &cobra.Command{
	Use:   "get [<logic-id>]",
	Short: "Get details of a Terraform state",
	Long: `Retrieves comprehensive information about a Terraform state including dependencies,
dependents, and outputs. Uses .grid context if no identifier is provided.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cobraCmd *cobra.Command, args []string) error {
		// Build explicit reference from flags/args
		explicitRef := dirctx.StateRef{}
		if getLogicID != "" {
			explicitRef.LogicID = getLogicID
		} else if getGUID != "" {
			explicitRef.GUID = getGUID
		} else if len(args) == 1 {
			explicitRef.LogicID = args[0]
		}

		// Try to read .grid context
		contextRef := dirctx.StateRef{}
		gridCtx, err := dirctx.ReadGridContext()
		if err != nil {
			pterm.Warning.Printf("Warning: .grid file corrupted or invalid, ignoring: %v\n", err)
		} else if gridCtx != nil {
			contextRef.LogicID = gridCtx.StateLogicID
			contextRef.GUID = gridCtx.StateGUID
		}

		// Resolve final state reference
		stateRef, err := dirctx.ResolveStateRef(explicitRef, contextRef)
		if err != nil {
			return err
		}

		gridClient, err := sdkClient(cobraCmd.Context())
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cobraCmd.Context(), 10*time.Second)
		defer cancel()

		// Call GetStateInfo for enhanced display
		info, err := gridClient.GetStateInfo(ctx, sdk.StateReference{
			LogicID: stateRef.LogicID,
			GUID:    stateRef.GUID,
		})
		if err != nil {
			return fmt.Errorf("failed to get state info: %w", err)
		}

		switch getFormat {
		case "text":
			printState(info)
		case "json":
			printStateJSON(info)
		default:
			return fmt.Errorf("invalid format: %s", getFormat)
		}

		// Handle --link flag: write .grid file
		if getLink {
			if err := linkDirectory(info, getPath, getForce); err != nil {
				return err
			}
		}

		return nil
	},
}

// linkDirectory writes the .grid file to link a directory to a state
func linkDirectory(info *sdk.StateInfo, path string, force bool) error {
	// Default to current directory
	if path == "" {
		path = "."
	}

	// Resolve to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Change to target directory for .grid file operations
	originalDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(absPath); err != nil {
		return fmt.Errorf("failed to change to directory %s: %w", absPath, err)
	}

	// Build .grid context
	now := time.Now()
	gridCtx := &dirctx.DirectoryContext{
		Version:      dirctx.GridFileVersion,
		StateGUID:    info.State.GUID,
		StateLogicID: info.State.LogicID,
		ServerURL:    ServerURL,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	// Write .grid context file using shared dirctx validation + write
	if err := dirctx.WriteGridContextWithValidation(gridCtx, force); err != nil {
		// Non-fatal: warn but don't fail the command
		pterm.Warning.Printf("Warning: Cannot write .grid file (permission denied?), state context will not be saved: %v\n", err)
		pterm.Info.Println("State retrieved successfully, but you'll need to specify --logic-id for subsequent commands")
	} else {
		pterm.Success.Printf("Saved state context to .grid file\n")
		pterm.Info.Println("Subsequent commands in this directory will use this state automatically")
	}

	return nil
}

func init() {
	getCmd.Flags().StringVar(&getLogicID, "logic-id", "", "State logic ID (overrides positional arg and context)")
	getCmd.Flags().StringVar(&getGUID, "guid", "", "State GUID (overrides positional arg and context)")
	getCmd.Flags().BoolVar(&getLink, "link", false, "Write (or rewrite) .grid file to link directory to this state")
	getCmd.Flags().BoolVar(&getForce, "force", false, "Overwrite existing .grid file (used with --link)")
	getCmd.Flags().StringVar(&getPath, "path", ".", "Directory path to write .grid file (used with --link)")
	getCmd.Flags().StringVar(&getFormat, "format", "text", "Output format (text|json)")
}

func printState(info *sdk.StateInfo) {
	// Print state header
	fmt.Printf("State: %s (guid: %s)\n", info.State.LogicID, info.State.GUID)
	fmt.Printf("Created: %s\n", info.CreatedAt.Format("2006-01-02 15:04:05"))
	if !info.UpdatedAt.IsZero() {
		fmt.Printf("Updated: %s\n", info.UpdatedAt.Format("2006-01-02 15:04:05"))
	}
	fmt.Println()

	fmt.Println("Labels:")
	labels := info.Labels
	if len(labels) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, label := range sdk.SortLabels(labels) {
			fmt.Printf("  %s = %v\n", label.Key, label.Value)
		}
	}
	fmt.Println()

	// Print dependencies (incoming edges)
	fmt.Println("Dependencies (consuming from):")
	if len(info.Dependencies) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, dep := range info.Dependencies {
			toInput := dep.ToInputName
			if toInput == "" {
				toInput = "(auto-generated)"
			}
			fmt.Printf("  %s.%s → %s\n", dep.From.LogicID, dep.FromOutput, toInput)
		}
	}
	fmt.Println()

	// Print dependents (outgoing edges)
	fmt.Println("Dependents (consumed by):")
	if len(info.Dependents) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, dep := range info.Dependents {
			fmt.Printf("  %s (using %s)\n", dep.To.LogicID, dep.FromOutput)
		}
	}
	fmt.Println()

	// Print outputs
	fmt.Println("Outputs:")
	if len(info.Outputs) == 0 {
		fmt.Println("  (none - no Terraform state uploaded yet)")
	} else {
		for _, out := range info.Outputs {
			sensitive := ""
			if out.Sensitive {
				sensitive = " (⚠️  sensitive)"
			}
			fmt.Printf("  %s%s\n", out.Key, sensitive)
		}
	}
	fmt.Println()

	// Print backend config
	fmt.Println("Terraform HTTP Backend endpoints:")
	fmt.Printf("  Address: %s\n", info.BackendConfig.Address)
	fmt.Printf("  Lock:    %s\n", info.BackendConfig.LockAddress)
	fmt.Printf("  Unlock:  %s\n", info.BackendConfig.UnlockAddress)
}

func printStateJSON(info *sdk.StateInfo) {
	object := make(map[string]any)
	// Metadata
	object["logic_id"] = info.State.LogicID
	object["guid"] = info.State.GUID
	object["created_at"] = info.CreatedAt.Format(time.RFC3339)
	if !info.UpdatedAt.IsZero() {
		object["updated_at"] = info.UpdatedAt.Format(time.RFC3339)
	}
	object["labels"] = sdk.SortLabels(info.Labels)
	// Dependencies
	dependencies := []map[string]any{}
	for _, dep := range info.Dependencies {
		toInput := dep.ToInputName
		if toInput == "" {
			toInput = "(auto-generated)"
		}
		dependencies = append(dependencies, map[string]any{
			"edge_id":       dep.ID,
			"from_logic_id": dep.From.LogicID,
			"from_output":   dep.FromOutput,
			"to_input":      toInput,
		})
	}
	object["dependencies"] = dependencies
	// Dependents
	dependents := []map[string]string{}
	for _, dep := range info.Dependents {
		dependents = append(dependents, map[string]string{
			"to_logic_id": dep.To.LogicID,
			"from_output": dep.FromOutput,
		})
	}
	object["dependents"] = dependents
	// Outputs
	outputs := []map[string]any{}
	for _, out := range info.Outputs {
		outputs = append(outputs, map[string]any{
			"key":       out.Key,
			"sensitive": out.Sensitive,
		})
	}
	object["outputs"] = outputs
	// Backend config
	backendConfig := map[string]string{
		"address":        info.BackendConfig.Address,
		"lock_address":   info.BackendConfig.LockAddress,
		"unlock_address": info.BackendConfig.UnlockAddress,
	}
	object["backend_config"] = backendConfig

	// Marshal to JSON
	data, err := json.MarshalIndent(object, "", "  ")
	if err != nil {
		pterm.Error.Printf("Failed to marshal state info to JSON: %v\n", err)
		return
	}
	fmt.Println(string(data))
}
