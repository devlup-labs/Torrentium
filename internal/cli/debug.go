package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var debugCmd = &cobra.Command{
	Use:   "debug",
	Short: "Show detailed network debug info",
	Long:  `Display detailed network debugging information including peer IDs, addresses, and shared files.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if client == nil {
			return fmt.Errorf("client not initialized")
		}
		client.DebugNetworkStatus()
		return nil
	},
}
