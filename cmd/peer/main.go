package main

import (
	"context"
	"fmt"
	"os"
	"time"
	"torrentium/relay"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

func main() {
	role := os.Args[1]
	ctx := context.Background()
	h, dht := relay.Newhost(ctx, "")

	// optional: DHT bootstrap if needed
	_ = dht // used in your DHT routing if needed

	switch role {
	case "receive":
		relay.RegisterHandlers(h)
		fmt.Println("Waiting to receive file...")
		select {} // block forever

	case "send":
		if len(os.Args) < 4 {
			fmt.Println("Usage: go run main.go send <target-multiaddr> <file>")
			return
		}

		targetAddrStr := os.Args[2]
		filePath := os.Args[3]

		maddr, err := multiaddr.NewMultiaddr(targetAddrStr)
		if err != nil {
			fmt.Println("Invalid multiaddress:", err)
			return
		}

		info, err := peer.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			fmt.Println("Failed to get peer info:", err)
			return
		}

		if err := h.Connect(ctx, *info); err != nil {
			fmt.Println("Connection failed:", err)
			return
		}

		fmt.Println("Connected. Sending file...")
		err = relay.SendFile(ctx, h, info.ID, filePath)
		if err != nil {
			fmt.Println("SendFile error:", err)
		} else {
			fmt.Println("File sent successfully!")
		}

		time.Sleep(2 * time.Second) // give time to finish before exit
	default:
		fmt.Println("Unknown role:", role)
	}

}
