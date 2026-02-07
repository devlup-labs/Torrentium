package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var announceCmd = &cobra.Command{
	Use:   "announce <cid>",
	Short: "Re-announce a file to DHT",
	Long:  `Re-announce a file's CID to the DHT to refresh its availability.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if client == nil {
			return fmt.Errorf("client not initialized")
		}
		return client.AnnounceFile(args[0])
	},
}
