package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check connection health",
	Long:  `Display the current health status of your P2P connections and DHT routing table.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if client == nil {
			return fmt.Errorf("client not initialized")
		}
		client.CheckConnectionHealth()
		return nil
	},
}
