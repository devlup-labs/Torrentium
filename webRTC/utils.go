package webRTC

import (
	"fmt"
)

func FormatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func PrintClientInstructions() {
	fmt.Println(`
--- Torrentium Client Commands ---
  help          Show this help message.
  add <path>    Announce a local file to the tracker.
  list          List all files available on the tracker.
  listpeers     List all currently online peers.
  get <file_id> Find and download a file from a peer.
  exit          Shutdown the client.`)
}