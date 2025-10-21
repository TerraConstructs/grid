package role

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/pkg/sdk"
)

var exportCmd = &cobra.Command{
	Use:   "export [role_names...]",
	Short: "Export roles to a JSON file",
	RunE: func(cmd *cobra.Command, args []string) error {
		outputFile, _ := cmd.Flags().GetString("output")

		gridClient, err := sdkClient(cmd.Context())
		if err != nil {
			return err
		}

		result, err := gridClient.ExportRoles(cmd.Context(), sdk.ExportRolesInput{
			RoleNames: args,
		})
		if err != nil {
			return fmt.Errorf("failed to export roles: %w", err)
		}

		if err := os.WriteFile(outputFile, []byte(result.RolesJSON), 0644); err != nil {
			return fmt.Errorf("failed to write to output file: %w", err)
		}

		fmt.Printf("Exported roles to %s\n", outputFile)
		return nil
	},
}

func init() {
	exportCmd.Flags().String("output", "roles.json", "Output file for exported roles")
}
