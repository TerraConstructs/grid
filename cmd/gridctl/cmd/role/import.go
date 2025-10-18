package role

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/cmd/gridctl/internal/client"
	"github.com/terraconstructs/grid/pkg/sdk"
)

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import roles from a JSON file",
	RunE: func(cmd *cobra.Command, args []string) error {
		inputFile, _ := cmd.Flags().GetString("file")
		force, _ := cmd.Flags().GetBool("force")

		data, err := os.ReadFile(inputFile)
		if err != nil {
			return fmt.Errorf("failed to read input file: %w", err)
		}

		httpClient, err := client.NewAuthenticatedClient(ServerURL)
		if err != nil {
			return err
		}

		gridClient := sdk.NewClient(ServerURL, sdk.WithHTTPClient(httpClient))

		result, err := gridClient.ImportRoles(cmd.Context(), sdk.ImportRolesInput{
			RolesJSON: string(data),
			Force:     force,
		})
		if err != nil {
			return fmt.Errorf("failed to import roles: %w", err)
		}

		fmt.Printf("Role import complete:\n")
		fmt.Printf("  Imported: %d\n", result.ImportedCount)
		fmt.Printf("  Skipped:  %d\n", result.SkippedCount)
		if len(result.Errors) > 0 {
			fmt.Println("Errors:")
			for _, e := range result.Errors {
				fmt.Printf("  - %s\n", e)
			}
		}

		return nil
	},
}

func init() {
	importCmd.Flags().String("file", "roles.json", "Input file for imported roles")
	importCmd.Flags().Bool("force", false, "Overwrite existing roles")
}
