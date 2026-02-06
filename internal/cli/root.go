package cli

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"torrentium/internal/core"

	"github.com/spf13/cobra"
)

var client *core.Client

var rootCmd = &cobra.Command{
	Use:   "torrentium",
	Short: "Decentralized P2P file sharing",
	Long: `
╔══════════════════════════════════════════════════════════════════════════════╗
║                    TORRENTIUM - Decentralized P2P File Sharing               ║
╠══════════════════════════════════════════════════════════════════════════════╣
║  A peer-to-peer file sharing application using libp2p and WebRTC             ║
╚══════════════════════════════════════════════════════════════════════════════╝`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Skip client init for help commands
		if cmd.Name() == "help" || cmd.Name() == "completion" {
			return
		}
	},
}

func Execute(c *core.Client) {
	client = c

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nShutting down...")
		os.Exit(0)
	}()

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(downloadCmd)
	rootCmd.AddCommand(peersCmd)
	rootCmd.AddCommand(announceCmd)
	rootCmd.AddCommand(healthCmd)
	rootCmd.AddCommand(debugCmd)
	rootCmd.AddCommand(interactiveCmd)
}
