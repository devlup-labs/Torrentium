package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var peersCmd = &cobra.Command{
	Use:   "peers",
	Short: "Show connected peers",
	Long:  `Display all peers currently connected to your node.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if client == nil {
			return fmt.Errorf("client not initialized")
		}
		client.ListConnectedPeers()
		return nil
	},
}
