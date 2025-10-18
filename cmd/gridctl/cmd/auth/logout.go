package auth

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/cmd/gridctl/internal/auth"
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out from Grid",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := auth.NewFileStore()
		if err != nil {
			return fmt.Errorf("failed to create credential store: %w", err)
		}

		if err := store.DeleteCredentials(); err != nil {
			return fmt.Errorf("failed to delete credentials: %w", err)
		}

		fmt.Println("Logged out successfully")
		return nil
	},
}
