package state

import (
	"bufio"
	"context"
	_ "embed"
	"fmt"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/pkg/sdk"
)

//go:embed templates/backend.hcl.tmpl
var backendTemplate string

var initCmd = &cobra.Command{
	Use:   "init <logic-id>",
	Short: "Initialize Terraform backend configuration",
	Long: `Looks up a state by logic_id and generates a backend.tf file with
the Terraform HTTP backend configuration. Prompts before overwriting existing files.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cobraCmd *cobra.Command, args []string) error {
		logicID := args[0]

		// Create SDK client
		client := sdk.NewClient(ServerURL)

		// Lookup state by logic ID
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		state, err := client.GetState(ctx, sdk.StateReference{LogicID: logicID})
		if err != nil {
			return fmt.Errorf("failed to get state config: %w", err)
		}

		// Check if backend.tf already exists
		filename := "backend.tf"
		if _, err := os.Stat(filename); err == nil {
			// File exists, prompt for overwrite
			fmt.Printf("File %s already exists. Overwrite? (y/n): ", filename)
			reader := bufio.NewReader(os.Stdin)
			response, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read input: %w", err)
			}
			response = strings.ToLower(strings.TrimSpace(response))
			if response != "y" && response != "yes" {
				fmt.Println("Aborted.")
				return nil
			}
		}

		// Parse and render template
		tmpl, err := template.New("backend").Parse(backendTemplate)
		if err != nil {
			return fmt.Errorf("failed to parse template: %w", err)
		}

		f, err := os.Create(filename)
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}
		defer f.Close()

		data := struct {
			Address       string
			LockAddress   string
			UnlockAddress string
		}{
			Address:       state.BackendConfig.Address,
			LockAddress:   state.BackendConfig.LockAddress,
			UnlockAddress: state.BackendConfig.UnlockAddress,
		}

		if err := tmpl.Execute(f, data); err != nil {
			return fmt.Errorf("failed to render template: %w", err)
		}

		fmt.Printf("Created %s for state %s\n", filename, state.GUID)
		fmt.Println("Run 'terraform init' or 'tofu init' to initialize the backend.")

		return nil
	},
}
