package p2p

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	relayv2client "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/client"
	ma "github.com/multiformats/go-multiaddr"
)

const privKeyFile = "private_key"

func reserveWithRelay(ctx context.Context, relayAddrStr string, h host.Host) error {
	maddr, err := ma.NewMultiaddr(relayAddrStr)
	if err != nil {
		return fmt.Errorf("invalid relay multiaddr: %w", err)
	}
	relayInfo, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		return fmt.Errorf("could not parse relay AddrInfo: %w", err)
	}
	if err := h.Connect(ctx, *relayInfo); err != nil {
		return fmt.Errorf("failed to connect to relay: %w", err)
	}
	res, err := relayv2client.Reserve(ctx, h, *relayInfo)
	if err != nil {
		return fmt.Errorf("reservation failed: %w", err)
	}
	log.Printf("✅ Reservation with relay successful. Expires at: %v", res.Expiration)
	return nil
}
func NewHost(
	ctx context.Context,
	listenAddr string,
	onOffer func(offer, remotePeerID string, s network.Stream) (string, error),
) (host.Host, *dht.IpfsDHT, error) {

	//Identity key
	priv, err := loadOrGeneratePrivateKey()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load/generate private key: %w", err)
	}

	// 📡 Local listen address
	maddr, err := ma.NewMultiaddr(listenAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse listen address '%s': %w", listenAddr, err)
	}

	// Relay config (static relay on Render)
	relayAddrStr := "/dns4/relay-torrentium.onrender.com/tcp/443/wss/p2p/12D3KooWMbTZL5taZH4CK9hCkTLkXaPadBoMR3KJZRFhbYBPrdkK"

	relayMaddr, err := ma.NewMultiaddr(relayAddrStr)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid relay multiaddr: %w", err)
	}
	relayInfo, err := peer.AddrInfoFromP2pAddr(relayMaddr)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid relay peer info: %w", err)
	}

	//Create host with relay + autorelay
	h, err := libp2p.New(
		libp2p.Identity(priv),
		libp2p.ListenAddrs(maddr),
		libp2p.EnableRelay(), // act as relay client
		libp2p.EnableAutoRelayWithStaticRelays([]peer.AddrInfo{*relayInfo}),
		libp2p.EnableHolePunching(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("libp2p peer not initialized: %w", err)
	}

	//Connect to relay
	if err := h.Connect(ctx, *relayInfo); err != nil {
		return nil, nil, fmt.Errorf("❌ Failed to connect to relay: %w", err)
	}
	log.Println("✅ Connected to relay")

	//Reserve relay slot
	if err := reserveWithRelay(ctx, relayAddrStr, h); err != nil {
		return nil, nil, fmt.Errorf("❌ Relay reservation failed: %w", err)
	}
	log.Println("✅ Relay reservation successful")

	//DHT setup
	idht, err := dht.New(ctx, h)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize DHT: %w", err)
	}
	if err := idht.Bootstrap(ctx); err != nil {
		return nil, nil, fmt.Errorf("failed to bootstrap DHT: %w", err)
	}

	//Register WebRTC signaling protocol (your handler)
	RegisterSignalingProtocol(h, onOffer)

	log.Printf("✅ Host created with ID: %s", h.ID())
	for _, addr := range h.Addrs() {
		log.Printf("🟢 Listening on: %s/p2p/%s", addr, h.ID())
	}

	return h, idht, nil
}

func Bootstrap(ctx context.Context, h host.Host, d *dht.IpfsDHT) error {
	bootstrapNodes := []string{
		// Official IPFS bootstrap nodes (mix of DNS and direct IP)
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zp7VCk8JpNUQLoUPF3HfrDAQGS52a8",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX89HWoNT4gEoNA7MzZqaGzyCu5w",

		// Direct IP addresses as fallback (more reliable)
		"/ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
		"/ip4/104.236.179.241/tcp/4001/p2p/QmSoLPppuBtQSGwKDZT2M73ULpjvfd3aZ6ha4oFGL1KrGM",
		"/ip4/128.199.219.111/tcp/4001/p2p/QmSoLSafTMBsPKadTEgaXctDQVcqN88CNLHXMkTNwMKPnu",
		"/ip4/104.236.76.40/tcp/4001/p2p/QmSoLV4Bbm51jM9C4gDYZQ9Cy3U6aXMJDAbzgu2fzaDs64",

		// Alternative public nodes
		"/ip4/147.75.77.187/tcp/4001/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
		"/ip6/2604:1380:1000:6000::1/tcp/4001/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
	}

	fmt.Println("Connecting to bootstrap nodes...")
	connected := 0
	required := 5
	for i, addrStr := range bootstrapNodes {
		// Stop early if we have enough connections
		if connected >= required {
			fmt.Printf("Already connected to %d nodes, stopping early\n", connected)
			break
		}

		addr, err := ma.NewMultiaddr(addrStr)
		if err != nil {
			log.Printf("Invalid bootstrap address %s: %v", addrStr, err)
			continue
		}

		pi, err := peer.AddrInfoFromP2pAddr(addr)
		if err != nil {
			log.Printf("Failed to parse bootstrap peer info %s: %v", addrStr, err)
			continue
		}

		connectCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		if err := h.Connect(connectCtx, *pi); err != nil {
			//log.Printf("Failed to connect to bootstrap node %s: %v", pi.ID, err)
		} else {
			fmt.Printf("Connected to bootstrap node: %s\n", pi.ID)
			connected++
		}
		cancel()

		// Added small delay between connections to avoid overwhelming
		if i < len(bootstrapNodes)-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	if connected < required {
		return fmt.Errorf("insufficient bootstrap connections: got %d, need at least %d", connected, required)
	}

	fmt.Printf("Successfully connected to %d bootstrap nodes (minimum %d required)\n", connected, required)

	// Bootstrap the DHT
	fmt.Println("Bootstrapping DHT...")
	if err := d.Bootstrap(ctx); err != nil {
		return fmt.Errorf("failed to bootstrap DHT: %w", err)
	}

	// Wait for DHT to become ready with better feedback
	fmt.Println("Waiting for DHT to become ready...")
	readyTimeout := time.After(45 * time.Second)
	checkTicker := time.NewTicker(5 * time.Second)
	defer checkTicker.Stop()

	for {
		select {
		case <-readyTimeout:
			routingTableSize := d.RoutingTable().Size()
			if routingTableSize > 0 {
				fmt.Printf("DHT partially ready (routing table size: %d), continuing...\n", routingTableSize)
			} else {
				fmt.Println("DHT bootstrap timeout, but continuing anyway...")
			}
			return nil

		case <-checkTicker.C:
			routingTableSize := d.RoutingTable().Size()
			fmt.Printf("DHT routing table size: %d\n", routingTableSize)
			if routingTableSize >= 10 {
				fmt.Println("DHT is ready with good routing table!")
				return nil
			}

		case <-d.RefreshRoutingTable():
			routingTableSize := d.RoutingTable().Size()
			fmt.Printf("DHT routing table refreshed (size: %d)\n", routingTableSize)
			if routingTableSize >= 5 {
				fmt.Println("DHT is ready!")
				return nil
			}
		}
	}
}

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
