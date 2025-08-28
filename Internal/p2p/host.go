package p2p

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"os"
	"time"

	// "sync"
	// "time"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"

	//"github.com/libp2p/go-libp2p/core/routing"
	// "github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multiaddr"
	ma "github.com/multiformats/go-multiaddr"
)

const privKeyFile = "private_key"

// NewHost creates a new libp2p host with a Kademlia DHT.
func NewHost(ctx context.Context, listenAddr string) (host.Host, *dht.IpfsDHT, error) {
	priv, err := loadOrGeneratePrivateKey()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load/generate private key: %w", err)
	}

	maddr, err := ma.NewMultiaddr(listenAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse listen address '%s': %w", listenAddr, err)
	}

	var idht *dht.IpfsDHT
	fmt.Println("debugging 11")

	h, err := libp2p.New(
		libp2p.Identity(priv),
		libp2p.ListenAddrs(maddr),
	)
	if err != nil {
		panic(err)
	}

	// Create DHT with reference to host `h`
	// idht, err := dht.New(ctx, h)
	// if err != nil {
	// 	panic(err)
	// }

	h, err = libp2p.New(
		libp2p.Identity(priv),
		libp2p.ListenAddrs(maddr),
		// libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
		// 	return idht, nil
		// }),
		libp2p.EnableAutoRelayWithPeerSource(
			func(ctx context.Context, num int) <-chan peer.AddrInfo {
				ch := make(chan peer.AddrInfo, num)
				go func() {
					defer close(ch)

					peers, err := idht.GetClosestPeers(ctx, h.ID().String()) // ✅ h is available now
					if err != nil {
						return
					}

					for i := range num {
						select {
						case <-ctx.Done():
							return
						default:
							info := peer.AddrInfo{ID: peers[i]}
							ch <- info
						}
					}
				}()
				return ch
			},
		),
	)
	fmt.Println("debugging 14")

	if err != nil {
		return nil, nil, err
	}

	idht, err = dht.New(ctx, h)
	if err != nil {
		return nil, nil, err
	}

	// Bootstrap the DHT
	if err := idht.Bootstrap(ctx); err != nil {
		return nil, nil, err
	}

	log.Printf("Host created with ID: %s", h.ID())
	log.Printf("Host listening on: %s/p2p/%s", h.Addrs()[0], h.ID())
	return h, idht, nil
}




// Add these improvements to your main function and Client struct

// Improved bootstrapping function
func Bootstrap(ctx context.Context, h host.Host, d *dht.IpfsDHT) error {
	// Default IPFS bootstrap nodes
	bootstrapNodes := []string{
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zp7VCk8JpNUQLoUPF3HfrDAQGS52a8",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX89HWoNT4gEoNA7MzZqaGzyCu5w",
		"/ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
	}

	fmt.Println("Connecting to bootstrap nodes...")
	connected := 0
	
	for _, addrStr := range bootstrapNodes {
		addr, err := multiaddr.NewMultiaddr(addrStr)
		if err != nil {
			log.Printf("Invalid bootstrap address %s: %v", addrStr, err)
			continue
		}

		pi, err := peer.AddrInfoFromP2pAddr(addr)
		if err != nil {
			log.Printf("Failed to parse bootstrap peer info %s: %v", addrStr, err)
			continue
		}

		connectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		if err := h.Connect(connectCtx, *pi); err != nil {
			log.Printf("Failed to connect to bootstrap node %s: %v", pi.ID, err)
		} else {
			fmt.Printf("Connected to bootstrap node: %s\n", pi.ID)
			connected++
		}
		cancel()
	}

	if connected == 0 {
		return fmt.Errorf("failed to connect to any bootstrap nodes")
	}

	fmt.Printf("Successfully connected to %d bootstrap nodes\n", connected)

	// Bootstrap the DHT
	fmt.Println("Bootstrapping DHT...")
	if err := d.Bootstrap(ctx); err != nil {
		return fmt.Errorf("failed to bootstrap DHT: %w", err)
	}

	// Wait for DHT to become ready
	fmt.Println("Waiting for DHT to become ready...")
	select {
	case <-d.RefreshRoutingTable():
		fmt.Println("DHT routing table refreshed")
	case <-time.After(60 * time.Second):
		fmt.Println("DHT bootstrap timeout, continuing anyway...")
	}

	return nil
}

// // You'll also need this improved bootstrap function as a method of Client
// func Bootstrap(ctx context.Context) error {
// 	// Default IPFS bootstrap nodes
// 	bootstrapNodes := []string{
// 		"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
// 		"/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJyrVwtbZg5gBMjTezGAJN",
// 		"/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zp7VCk8JpNUQLoUPF3HfrDAQGS52a8",
// 		"/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX89HWoNT4gEoNA7MzZqaGzyCu5w",
// 		"/ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
// 	}

// 	fmt.Println("Connecting to bootstrap nodes...")
// 	connected := 0

// 	for _, addrStr := range bootstrapNodes {
// 		addr, err := multiaddr.NewMultiaddr(addrStr)
// 		if err != nil {
// 			log.Printf("Invalid bootstrap address %s: %v", addrStr, err)
// 			continue
// 		}

// 		pi, err := peer.AddrInfoFromP2pAddr(addr)
// 		if err != nil {
// 			log.Printf("Failed to parse bootstrap peer info %s: %v", addrStr, err)
// 			continue
// 		}

// 		connectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
// 		if err := c.host.Connect(connectCtx, *pi); err != nil {
// 			log.Printf("Failed to connect to bootstrap node %s: %v", pi.ID, err)
// 		} else {
// 			fmt.Printf("Connected to bootstrap node: %s\n", pi.ID)
// 			connected++
// 		}
// 		cancel()
// 	}

// 	if connected == 0 {
// 		return fmt.Errorf("failed to connect to any bootstrap nodes")
// 	}

// 	fmt.Printf("Successfully connected to %d bootstrap nodes\n", connected)

// 	// Bootstrap the DHT
// 	fmt.Println("Bootstrapping DHT...")
// 	if err := c.dht.Bootstrap(ctx); err != nil {
// 		return fmt.Errorf("failed to bootstrap DHT: %w", err)
// 	}

// 	// Wait for DHT to become ready
// 	fmt.Println("Waiting for DHT to become ready...")
// 	select {
// 	case <-c.dht.RefreshRoutingTable():
// 		fmt.Println("DHT routing table refreshed")
// 	case <-time.After(60 * time.Second):
// 		fmt.Println("DHT bootstrap timeout, continuing anyway...")
// 	}

// 	return nil
// }

func loadOrGeneratePrivateKey() (crypto.PrivKey, error) {
	privBytes, err := os.ReadFile(privKeyFile)
	if os.IsNotExist(err) {
		priv, _, err := crypto.GenerateEd25519Key(rand.Reader)
		if err != nil {
			return nil, err
		}

		privBytes, err := crypto.MarshalPrivateKey(priv)
		if err != nil {
			return nil, err
		}

		if err := os.WriteFile(privKeyFile, privBytes, 0600); err != nil {
			return nil, fmt.Errorf("failed to write private key to file: %w", err)
		}

		log.Println("Generated new libp2p private key.")
		return priv, nil
	} else if err != nil {
		return nil, err
	}

	log.Println("Loaded existing libp2p private key.")
	return crypto.UnmarshalPrivateKey(privBytes)
}
