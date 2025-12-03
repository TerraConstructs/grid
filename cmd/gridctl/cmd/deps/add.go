package deps

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/cmd/gridctl/internal/config"
	"github.com/terraconstructs/grid/cmd/gridctl/internal/dirctx"
	"github.com/terraconstructs/grid/pkg/sdk"
)

var (
	addFromLogicID string
	addFromOutput  string
	addToLogicID   string
	addToInputName string
	addMockValue   string
)

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a dependency between states",
	Long: `Creates a new dependency edge from a producer state's output to a consumer state's input.
The edge will be tracked and automatically updated when the producer's output changes.

If --output is not specified, an interactive prompt will show available outputs.
If --to is not specified, the .grid context will be used (if available).`,
	Args: cobra.NoArgs,
	RunE: func(cobraCmd *cobra.Command, args []string) error {
		cfg := config.MustFromContext(cobraCmd.Context())

		fromLogicID := strings.TrimSpace(addFromLogicID)
		fromOutput := strings.TrimSpace(addFromOutput)
		toLogicID := strings.TrimSpace(addToLogicID)
		toInputName := strings.TrimSpace(addToInputName)
		mockJSON := strings.TrimSpace(addMockValue)

		if fromLogicID == "" {
			return fmt.Errorf("--from flag is required")
		}

		// Resolve --to from context if not provided
		if toLogicID == "" {
			// Try to read .grid context
			gridCtx, err := dirctx.ReadGridContext()
			if err != nil {
				pterm.Warning.Printf("Warning: .grid file corrupted or invalid, ignoring: %v\n", err)
				return fmt.Errorf("--to flag is required (no .grid context found)")
			} else if gridCtx != nil {
				toLogicID = gridCtx.StateLogicID
				pterm.Info.Printf("Using --to from .grid context: %s\n", toLogicID)
			} else {
				return fmt.Errorf("--to flag is required (no .grid context found)")
			}
		}

		gridClient, err := sdkClient(cobraCmd.Context())
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cobraCmd.Context(), 30*time.Second)
		defer cancel()

		// If --output not provided, fetch outputs and prompt interactively
		outputKeys := []string{fromOutput}
		if fromOutput == "" {
			// Fetch outputs from from-state
			outputs, err := gridClient.ListStateOutputs(ctx, sdk.StateReference{LogicID: fromLogicID})
			if err != nil {
				return fmt.Errorf("failed to list outputs from state %s: %w", fromLogicID, err)
			}

			// Prompt user to select outputs
			selectedKeys, err := promptSelectOutputs(outputs, cfg.NonInteractive)
			if err != nil {
				return err
			}

			outputKeys = selectedKeys
		}

		// Create dependency for each selected output
		for _, outputKey := range outputKeys {
			result, err := gridClient.AddDependency(ctx, sdk.AddDependencyInput{
				From:          sdk.StateReference{LogicID: fromLogicID},
				FromOutput:    outputKey,
				To:            sdk.StateReference{LogicID: toLogicID},
				ToInputName:   toInputName,
				MockValueJSON: mockJSON,
			})
			if err != nil {
				return fmt.Errorf("failed to add dependency for %s: %w", outputKey, err)
			}

			inputName := toInputName
			if result.Edge.ToInputName != "" {
				inputName = result.Edge.ToInputName
			}
			if inputName == "" {
				inputName = "(auto-generated)"
			}

			message := fmt.Sprintf(
				"Dependency added: %s.%s -> %s as '%s' (edge ID: %d)",
				result.Edge.From.LogicID,
				result.Edge.FromOutput,
				result.Edge.To.LogicID,
				inputName,
				result.Edge.ID,
			)

			if result.AlreadyExists {
				fmt.Printf("Dependency already exists: %s\n", message)
			} else {
				fmt.Println(message)
			}
		}

		return nil
	},
}

func init() {
	addCmd.Flags().StringVar(&addFromLogicID, "from", "", "Producer state logic ID (required)")
	addCmd.Flags().StringVar(&addFromOutput, "output", "", "Producer output key (optional, will prompt if not provided)")
	addCmd.Flags().StringVar(&addToLogicID, "to", "", "Consumer state logic ID (optional, uses .grid context if available)")
	addCmd.Flags().StringVar(&addToInputName, "to-input", "", "Input variable name in consumer state (optional)")
	addCmd.Flags().StringVar(&addMockValue, "mock", "", "Mock value JSON for initial state (optional)")
	_ = addCmd.MarkFlagRequired("from")
	// --output and --to are no longer required (will prompt/use context if not provided)
}

// promptSelectOutputs displays an interactive multi-select prompt for output keys
// Returns selected output keys, or error if in non-interactive mode without explicit selection
func promptSelectOutputs(outputs []sdk.OutputKey, nonInteractive bool) ([]string, error) {
	if nonInteractive {
		return nil, fmt.Errorf("cannot prompt in non-interactive mode: specify --output explicitly")
	}

	// If only one output, auto-select it
	if len(outputs) == 1 {
		pterm.Info.Printf("Auto-selected single output: %s\n", outputs[0].Key)
		return []string{outputs[0].Key}, nil
	}

	// If no outputs, cannot proceed
	if len(outputs) == 0 {
		return nil, fmt.Errorf("state has no outputs available")
	}

	// Build display options with sensitive marker
	options := make([]string, len(outputs))
	for i, out := range outputs {
		if out.Sensitive {
			options[i] = fmt.Sprintf("%s (⚠️  sensitive)", out.Key)
		} else {
			options[i] = out.Key
		}
	}

	// Create interactive multi-select
	printer := pterm.DefaultInteractiveMultiselect.
		WithOptions(options).
		WithFilter(true).
		WithCheckmark(&pterm.Checkmark{Checked: pterm.Green("✓"), Unchecked: " "})

	selectedOptions, err := printer.Show("Select outputs to create dependencies:")
	if err != nil {
		return nil, fmt.Errorf("failed to show interactive prompt: %w", err)
	}

	// Extract actual output keys (strip sensitive marker if present)
	selectedKeys := make([]string, 0, len(selectedOptions))
	for _, selected := range selectedOptions {
		// Remove " (⚠️  sensitive)" suffix if present
		key := strings.TrimSuffix(selected, " (⚠️  sensitive)")
		selectedKeys = append(selectedKeys, key)
	}

	if len(selectedKeys) == 0 {
		return nil, fmt.Errorf("no outputs selected")
	}

	return selectedKeys, nil
}
