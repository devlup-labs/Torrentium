package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add <path>",
	Short: "Share a file on the network",
	Long:  `Add a file to the P2P network for sharing. The file will be announced to the DHT.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if client == nil {
			return fmt.Errorf("client not initialized")
		}
		return client.AddFile(args[0])
	},
}
