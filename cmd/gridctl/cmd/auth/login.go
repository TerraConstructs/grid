package auth

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/cmd/gridctl/internal/auth"
	"github.com/terraconstructs/grid/cmd/gridctl/internal/config"
	"github.com/terraconstructs/grid/pkg/sdk"
)

var (
	clientID     string
	clientSecret string
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Grid",
	Long: `Authenticates with the Grid server using device authorization flow.

The CLI automatically discovers the authentication mode from the server
and initiates the appropriate OIDC device flow (external IdP or Grid's internal IdP).

Two methods are supported:
1. Interactive Login (default): Initiates a device authorization flow for human users.
2. Service Account Login: Uses a client ID and secret for non-interactive authentication.
   Use the --client-id and --client-secret flags.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.MustFromContext(cmd.Context())

		store, err := auth.NewFileStore()
		if err != nil {
			return fmt.Errorf("failed to create credential store: %w", err)
		}

		// Service account flow (uses explicit client_id/secret, no discovery needed)
		if clientID != "" && clientSecret != "" {
			fmt.Println("Authenticating as service account...")
			creds, err := sdk.LoginWithServiceAccount(cmd.Context(), cfg.ServerURL, clientID, clientSecret)
			if err != nil {
				return err
			}
			if err := store.SaveCredentials(creds); err != nil {
				return fmt.Errorf("failed to save credentials: %w", err)
			}
			fmt.Println("------------------------------------------------------------")
			fmt.Printf("✅ Service account login successful!\n")
			fmt.Printf("Authenticated with client ID: %s\n", clientID)
			return nil
		}

		// Interactive device flow (SDK handles discovery and authentication)
		meta, err := sdk.LoginInteractive(cmd.Context(), cfg.ServerURL, store)
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
	if clientID == "" && clientSecret == "" {
		if ok, env := sdk.CheckEnvCreds(); ok {
			fmt.Println("Using service account credentials from environment variables.")
			clientID = env.ClientID
			clientSecret = env.ClientSecret
		}
	}
}
