package cli

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var interactiveCmd = &cobra.Command{
	Use:   "interactive",
	Short: "Start interactive CLI mode",
	Long:  `Start an interactive command-line interface for continuous operation.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if client == nil {
			return fmt.Errorf("client not initialized")
		}
		CommandLoop()
		return nil
	},
}

// CommandLoop runs the interactive CLI mode
func CommandLoop() {
	scanner := bufio.NewScanner(os.Stdin)
	PrintInstructions()
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		parts := strings.Fields(scanner.Text())
		if len(parts) == 0 {
			continue
		}
		cmd, args := parts[0], parts[1:]
		var err error
		switch cmd {
		case "help":
			PrintInstructions()
		case "add":
			if len(args) != 1 {
				fmt.Println("Usage: add <path>")
			} else {
				err = client.AddFile(args[0])
			}
		case "list":
			client.ListLocalFiles()
		case "search":
			if len(args) != 1 {
				fmt.Println("Usage: search <cid|text>")
			} else {
				client.CheckConnectionHealth()
				if strings.HasPrefix(args[0], "bafy") || strings.HasPrefix(args[0], "Qm") {
					err = client.EnhancedSearchByCID(args[0])
				} else {
					err = client.SearchByText(args[0])
				}
			}
		case "download":
			if len(args) != 1 {
				fmt.Println("Usage: download <cid>")
			} else {
				err = client.DownloadFile(args[0])
			}
		case "peers":
			client.ListConnectedPeers()
		case "announce":
			if len(args) != 1 {
				fmt.Println("Usage: announce <cid>")
			} else {
				err = client.AnnounceFile(args[0])
			}
		case "health":
			client.CheckConnectionHealth()
		case "debug":
			client.DebugNetworkStatus()
		case "exit":
			return
		default:
			fmt.Println("Unknown command. Type 'help' for available commands.")
		}
		if err != nil {
			log.Printf("Error: %v", err)
		}
	}
}

// PrintInstructions displays the help banner
func PrintInstructions() {
	peerID := client.Host.ID().String()
	minWidth := len(" Your Peer ID: "+peerID) + 4
	width := 80
	if minWidth > width {
		width = minWidth + 10
	}

	topBorder := "┌" + strings.Repeat("─", width-2) + "┐"
	bottomBorder := "└" + strings.Repeat("─", width-2) + "┘"

	centerLine := func(text string) string {
		if len(text) >= width-4 {
			return "│ " + text[:width-6] + "... │"
		}
		padding := (width - 4 - len(text)) / 2
		leftPad := strings.Repeat(" ", padding)
		rightPad := strings.Repeat(" ", width-4-len(text)-padding)
		return "│ " + leftPad + text + rightPad + " │"
	}

	leftLine := func(text string) string {
		if len(text) >= width-4 {
			return "│ " + text[:width-6] + "... │"
		}
		rightPad := strings.Repeat(" ", width-4-len(text))
		return "│ " + text + rightPad + " │"
	}

	fmt.Println()
	fmt.Println(topBorder)
	fmt.Println(centerLine("DECENTRALIZED P2P FILE SHARING"))
	fmt.Println("│" + strings.Repeat("─", width-2) + "│")
	fmt.Println(centerLine("Available Commands"))
	fmt.Println("│" + strings.Repeat(" ", width-2) + "│")

	commands := [][]string{
		{"add <path>", "Share a file on the network"},
		{"list", "List your shared files"},
		{"search <cid|text>", "Search by CID or filename text"},
		{"download <cid>", "Download a file by CID"},
		{"peers", "Show connected peers"},
		{"announce <cid>", "Re-announce a file to DHT"},
		{"health", "Check connection health"},
		{"debug", "Show detailed network debug info"},
		{"help", "Show this help"},
		{"exit", "Exit the application"},
	}

	for _, cmd := range commands {
		cmdText := fmt.Sprintf(" %-20s - %s", cmd[0], cmd[1])
		fmt.Println(leftLine(cmdText))
	}

	fmt.Println("│" + strings.Repeat(" ", width-2) + "│")
	fmt.Println("│" + strings.Repeat("─", width-2) + "│")

	peerID = client.Host.ID().String()
	fmt.Println(leftLine(" Your Peer ID: " + peerID))

	addrs := client.Host.Addrs()
	fmt.Println(leftLine(" Listening on:"))

	for i, addr := range addrs {
		if i >= 3 {
			moreAddrs := len(addrs) - 3
			fmt.Println(leftLine(fmt.Sprintf("   ... and %d more", moreAddrs)))
			break
		}
		addrStr := addr.String()
		if len(addrStr) > width-8 {
			addrStr = addrStr[:width-11] + "..."
		}
		fmt.Println(leftLine("   " + addrStr))
	}

	fmt.Println(bottomBorder)
	fmt.Println()
}
