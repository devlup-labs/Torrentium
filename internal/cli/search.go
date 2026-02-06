package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search <cid|text>",
	Short: "Search for files by CID or filename",
	Long:  `Search for files on the network. Use a CID (starting with 'bafy' or 'Qm') for DHT lookup, or text for local filename search.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if client == nil {
			return fmt.Errorf("client not initialized")
		}
		client.CheckConnectionHealth()
		query := args[0]
		if strings.HasPrefix(query, "bafy") || strings.HasPrefix(query, "Qm") {
			return client.EnhancedSearchByCID(query)
		}
		return client.SearchByText(query)
	},
}
