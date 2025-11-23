package policy

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/terraconstructs/grid/cmd/gridctl/internal/client"
	"github.com/terraconstructs/grid/pkg/sdk"
)

var (
	serverURL      string //nolint:unused // Used by SetServerURL for external configuration
	nonInteractive bool   //nolint:unused // Used by SetNonInteractive for external configuration
	clientProvider *client.Provider
)

// PolicyCmd is the parent command for label policy operations.
var PolicyCmd = &cobra.Command{
	Use:   "policy",
	Short: "Manage label validation policy",
	Long:  `Commands for inspecting and managing the Grid label validation policy.`,
}

func init() {
	PolicyCmd.AddCommand(getCmd)
	PolicyCmd.AddCommand(setCmd)
	PolicyCmd.AddCommand(complianceCmd)
}

// SetServerURL sets the Grid API server URL for policy commands.
func SetServerURL(url string) {
	serverURL = url
}

// SetNonInteractive controls interactive prompts (reserved for future use).
func SetNonInteractive(value bool) {
	nonInteractive = value
}

// SetClientProvider injects the shared authenticated client provider.
func SetClientProvider(provider *client.Provider) {
	clientProvider = provider
}

func sdkClient(ctx context.Context) (*sdk.Client, error) {
	if clientProvider == nil {
		return nil, fmt.Errorf("client provider not configured")
	}
	return clientProvider.SDKClient(ctx)
}
