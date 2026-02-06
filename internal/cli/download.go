package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var downloadCmd = &cobra.Command{
	Use:   "download <cid>",
	Short: "Download a file by CID",
	Long:  `Download a file from the P2P network using its Content Identifier (CID).`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if client == nil {
			return fmt.Errorf("client not initialized")
		}
		return client.DownloadFile(args[0])
	},
}
