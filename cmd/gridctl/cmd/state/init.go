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

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/cmd/gridctl/internal/dirctx"
	"github.com/terraconstructs/grid/pkg/sdk"
)

//go:embed templates/backend.hcl.tmpl
var backendTemplate string

var initCmd = &cobra.Command{
	Use:   "init [<logic-id>]",
	Short: "Initialize Terraform backend configuration",
	Long: `Looks up a state by logic_id and generates a backend.tf file with
the Terraform HTTP backend configuration. Prompts before overwriting existing files.

If logic-id is not provided, the .grid context will be used (if available).`,
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

		// Lookup state by logic ID
		state, err := gridClient.GetState(ctx, sdk.StateReference{
			LogicID: stateRef.LogicID,
			GUID:    stateRef.GUID,
		})
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
