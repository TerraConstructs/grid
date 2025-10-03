package deps

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
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
The edge will be tracked and automatically updated when the producer's output changes.`,
	Args: cobra.NoArgs,
	RunE: func(cobraCmd *cobra.Command, args []string) error {
		fromLogicID := strings.TrimSpace(addFromLogicID)
		fromOutput := strings.TrimSpace(addFromOutput)
		toLogicID := strings.TrimSpace(addToLogicID)
		toInputName := strings.TrimSpace(addToInputName)
		mockJSON := strings.TrimSpace(addMockValue)

		if fromLogicID == "" || fromOutput == "" || toLogicID == "" {
			return fmt.Errorf("flags --from, --output, and --to are required")
		}

		client := sdk.NewClient(ServerURL)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		result, err := client.AddDependency(ctx, sdk.AddDependencyInput{
			From:          sdk.StateReference{LogicID: fromLogicID},
			FromOutput:    fromOutput,
			To:            sdk.StateReference{LogicID: toLogicID},
			ToInputName:   toInputName,
			MockValueJSON: mockJSON,
		})
		if err != nil {
			return fmt.Errorf("failed to add dependency: %w", err)
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
		return nil
	},
}

func init() {
	addCmd.Flags().StringVar(&addFromLogicID, "from", "", "Producer state logic ID")
	addCmd.Flags().StringVar(&addFromOutput, "output", "", "Producer output key")
	addCmd.Flags().StringVar(&addToLogicID, "to", "", "Consumer state logic ID")
	addCmd.Flags().StringVar(&addToInputName, "to-input", "", "Input variable name in consumer state (optional)")
	addCmd.Flags().StringVar(&addMockValue, "mock", "", "Mock value JSON for initial state (optional)")
	addCmd.MarkFlagRequired("from")
	addCmd.MarkFlagRequired("output")
	addCmd.MarkFlagRequired("to")
}
