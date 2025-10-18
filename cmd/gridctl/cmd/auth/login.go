package auth

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/cmd/gridctl/internal/auth"
	"github.com/terraconstructs/grid/pkg/sdk"
)

var (
	clientID     string
	clientSecret string
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Grid",
	Long: `Authenticates with the Grid server.

Two methods are supported:
1. Interactive Login (default): Initiates a device authorization flow for human users. This is typically used with an external IdP.
2. Service Account Login: Uses a client ID and secret for non-interactive authentication. Use the --client-id and --client-secret flags.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: Get issuer from a config file instead of hardcoding
		issuer := ServerURL // Use the global server URL

		store, err := auth.NewFileStore()
		if err != nil {
			return fmt.Errorf("failed to create credential store: %w", err)
		}

		// If client ID and secret are provided, use service account flow
		if clientID != "" && clientSecret != "" {
			fmt.Println("Authenticating as service account...")
			err := sdk.LoginWithServiceAccount(cmd.Context(), issuer, clientID, clientSecret, store)
			if err != nil {
				return err
			}
			fmt.Println("------------------------------------------------------------")
			fmt.Printf("✅ Service account login successful!\n")
			fmt.Printf("Authenticated with client ID: %s\n", clientID)
			return nil
		}

		// Otherwise, use the interactive device flow
		// The default public client ID for the device flow is "gridctl"
		publicClientID := "gridctl"
		meta, err := sdk.LoginWithDeviceCode(cmd.Context(), issuer, publicClientID, store)
		if err != nil {
			return err
		}

		fmt.Println("------------------------------------------------------------")
		fmt.Printf("✅ Interactive login successful!\n")
		fmt.Printf("Authenticated as: %s (%s)\n", meta.User, meta.Email)

		return nil
	},
}

func init() {
	loginCmd.Flags().StringVar(&clientID, "client-id", "", "Client ID for service account authentication")
	loginCmd.Flags().StringVar(&clientSecret, "client-secret", "", "Client secret for service account authentication")
}