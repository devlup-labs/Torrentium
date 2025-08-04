package relay

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	libp2p "github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	host "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	routing "github.com/libp2p/go-libp2p/core/routing"
	multiaddr "github.com/multiformats/go-multiaddr"
)

func Newhost(ctx context.Context, relayAddrStr string) (host.Host, *dht.IpfsDHT) {
	var kad_dht *dht.IpfsDHT
	h, err := libp2p.New(
		libp2p.EnableRelay(),
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"),
		libp2p.EnableNATService(),
		libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
			kad_dht, err := dht.New(ctx, h)
			if err != nil {
				return nil, err
			}
			return kad_dht, err
		}),
	)
	if err != nil {
		log.Fatal("failed to form new peer", err)
	}
	fmt.Println("Host created with ID:", h.ID())
	fmt.Println("🟢 Listening on:")
	fmt.Println("heehhe3")
	if relayAddrStr != "" {
		fmt.Println("heehhe1")
		// Convert relay multiaddr string to multiaddr object
		relayAddr, err := multiaddr.NewMultiaddr(relayAddrStr)
		if err != nil {
			log.Fatal("❌ Invalid relay multiaddr:", err)
		}
		fmt.Println("heehhe2")
		// Extract peer ID + addresses from relay multiaddr
		relayInfo, _ := peer.AddrInfoFromP2pAddr(relayAddr)
		fmt.Println("heehhe3")
		// Try connecting to the relay
		if err := h.Connect(ctx, *relayInfo); err != nil {
			log.Println("⚠️ Relay connection failed:", err)
		} else {
			log.Println("🔗 Connected to relay:", relayInfo.ID)
		}
	}

	if err := kad_dht.Bootstrap(ctx); err != nil {
		log.Fatal("❌ Failed to bootstrap DHT:", err)
	}
	log.Println("🚀 DHT bootstrapped successfully")

	// Return host and DHT so caller can use them
	return h, kad_dht
}
func RegisterHandlers(h host.Host) {
	h.SetStreamHandler("/fileshare/1.0.0", func(s network.Stream) {
		log.Println("📥 Incoming file stream from", s.Conn().RemotePeer())

		// Create local file
		outFile, err := os.Create("received_file.txt")
		if err != nil {
			log.Println("❌ Failed to create file:", err)
			s.Close()
			return
		}
		defer outFile.Close()

		// Copy data from stream to file
		n, err := io.Copy(outFile, s)
		if err != nil {
			log.Println("❌ Failed to receive file:", err)
		} else {
			log.Printf("✅ Received %d bytes successfully\n", n)
		}

		s.Close()
	})
}
func SendFile(ctx context.Context, h host.Host, target peer.ID, filePath string) error {
	stream, err := h.NewStream(ctx, target, "/fileshare/1.0.0")
	if err != nil {
		return fmt.Errorf(" failed to open stream: %w", err)
	}
	defer stream.Close()

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf(" failed to open file: %w", err)
	}
	defer file.Close()

	// Copy file contents into the stream
	_, err = io.Copy(stream, file)
	if err != nil {
		return fmt.Errorf("failed to send file: %w", err)
	}
	return nil
}
