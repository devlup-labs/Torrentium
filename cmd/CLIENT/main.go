package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	webRTC "torrentium/Internal/client"
	db "torrentium/Internal/db"
	p2p "torrentium/Internal/p2p"

	"github.com/ipfs/go-cid"
	"github.com/joho/godotenv"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"

	// "github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multihash"
	"github.com/pion/webrtc/v3"
	// dht1 "torrentium/Internal/dht/keycache"
)

// RWMutex => But when one goroutine writes (adds/removes a peer), it blocks all readers and writers until it’s done (ensures consistency).
type Client struct {
	host            host.Host
	dht             *dht.IpfsDHT
	webRTCPeers     map[peer.ID]*webRTC.WebRTCPeer
	peersMux        sync.RWMutex
	sharingFiles    map[string]*FileInfo // map[CID]fileInfo
	activeDownloads map[string]*os.File
	downloadsMux    sync.RWMutex
	db              *db.Repository
}

type FileInfo struct {
	FilePath string
	Hash     string
	Size     int64
	Name     string
}

func (c *Client) findProvidersWithTimeout(cid cid.Cid, timeout time.Duration, maxProviders int) ([]peer.AddrInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	providersChan := c.dht.FindProvidersAsync(ctx, cid, maxProviders)

	var providers []peer.AddrInfo
	var totalFound int

	// Use a separate goroutine to collect providers with a reasonable timeout
	done := make(chan struct{})
	go func() {
		defer close(done)
		for provider := range providersChan {
			totalFound++
			if provider.ID != c.host.ID() {
				providers = append(providers, provider)
				fmt.Printf("Found provider %d: %s\n", len(providers), provider.ID)

				// Stop collecting if we have enough
				if len(providers) >= maxProviders {
					break
				}
			}
		}
	}()

	// Wait for either completion or timeout
	select {
	case <-done:
		fmt.Printf("Provider search completed. Found %d total providers, %d unique external providers\n",
			totalFound, len(providers))
	case <-time.After(timeout):
		fmt.Printf("Provider search timed out. Found %d providers so far\n", len(providers))
	}

	return providers, nil
}

// Add connection health checking
func (c *Client) checkConnectionHealth() {
	peers := c.host.Network().Peers()
	fmt.Printf("\n=== Connection Health ===\n")
	fmt.Printf("Connected peers: %d\n", len(peers))

	if len(peers) < 3 {
		fmt.Println("⚠️  Warning: Low peer count. Consider restarting or checking network connectivity.")
	} else {
		fmt.Println("✅ Good peer connectivity")
	}

	// Check DHT routing table size
	routingTableSize := c.dht.RoutingTable().Size()
	fmt.Printf("DHT routing table size: %d\n", routingTableSize)

	if routingTableSize < 10 {
		fmt.Println("⚠️  Warning: Small DHT routing table. File discovery may be limited.")
	} else {
		fmt.Println("✅ Good DHT connectivity")
	}
}

func (c *Client) enhancedSearchByCID(cidStr string) error {
	fileCID, err := cid.Decode(cidStr)
	if err != nil {
		return fmt.Errorf("invalid CID: %w", err)
	}

	fmt.Printf("Searching for CID: %s\n", fileCID.String())

	// Use the improved provider finding with reasonable timeout
	providers, err := c.findProvidersWithTimeout(fileCID, 60*time.Second, 10)
	if err != nil {
		return fmt.Errorf("provider search failed: %w", err)
	}

	if len(providers) == 0 {
		fmt.Println("No providers found for this CID")
		fmt.Println("This could mean:")
		fmt.Println("  - The file is not being shared")
		fmt.Println("  - The provider is offline")
		fmt.Println("  - Network connectivity issues")
		fmt.Println("  - DHT routing problems")
		return nil
	}

	fmt.Printf("Found %d provider(s):\n", len(providers))
	for i, provider := range providers {
		fmt.Printf("  %d. %s\n", i+1, provider.ID)

		// Show if we can connect to this peer
		if c.host.Network().Connectedness(provider.ID) == network.Connected {
			fmt.Printf("     ✅ Already connected\n")
		} else {
			fmt.Printf("     ⏳ Not connected (will attempt during download)\n")
		}
	}

	return nil
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: Could not load .env file: %v", err)
	}

	DB := db.InitDB()
	if DB == nil {
		log.Fatal("Database initialization failed")
	}

	h, d, err := p2p.NewHost(ctx, "/ip4/0.0.0.0/tcp/0")
	if err != nil {
		log.Fatal("Failed to create libp2p host:", err)
	}
	defer h.Close()

	// SAFE BOOTSTRAP - using your existing p2p.Bootstrap function
	go func() {
		if err := p2p.Bootstrap(ctx, h, d); err != nil {
			log.Printf("Error bootstrapping DHT: %v", err)
		}
	}()

	setupGracefulShutdown(h)
	repo := db.NewRepository(DB)
	client := NewClient(h, d, repo)

	client.startDHTMaintenance()

	p2p.RegisterSignalingProtocol(h, client.handleWebRTCOffer)

	client.commandLoop()
}

func NewClient(h host.Host, d *dht.IpfsDHT, repo *db.Repository) *Client {
	return &Client{
		host:            h,
		dht:             d,
		webRTCPeers:     make(map[peer.ID]*webRTC.WebRTCPeer),
		sharingFiles:    make(map[string]*FileInfo),
		activeDownloads: make(map[string]*os.File),
		db:              repo,
	}
}

func (c *Client) commandLoop() {
	scanner := bufio.NewScanner(os.Stdin)
	c.printInstructions()
	//fmt.Println("ggggh")
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
			c.printInstructions()
		case "add":
			if len(args) != 1 {
				fmt.Println("Usage: add <filepath>")
			} else {
				err = c.addFile(args[0])
			}
		case "list":
			c.listLocalFiles()
		case "search":
			if len(args) != 1 {
				fmt.Println("Usage: search <query>")
			} else {
				// First check connection health
				c.checkConnectionHealth()

				if strings.HasPrefix(args[0], "bafy") || strings.HasPrefix(args[0], "Qm") {
					err = c.enhancedSearchByCID(args[0])
				} else {
					fmt.Printf("Searching for files containing '%s'...\n", args[0])
					fmt.Println("Note: Direct filename search requires content indexing.")
					fmt.Println("Try using the CID if you have it, or check with known peers.")
				}
			}
		case "download":
			if len(args) != 1 {
				fmt.Println("Usage: download <CID>")
			} else {
				err = c.downloadFile(args[0])
			}
		case "peers":
			c.listConnectedPeers()
		case "exit":
			return
		case "health":
			c.checkConnectionHealth()
		case "debug":
			c.debugNetworkStatus()
		case "connect":
			if len(args) != 1 {
				fmt.Println("Usage: connect <multiaddr>")
				fmt.Println("Example: connect /ip4/127.0.0.1/tcp/54437/p2p/12D3KooWBLZFWsGZxoCFC8NsFgKvD6WJ6xV9UmYdR8t2C1kqYTcd")
			} else {
				err = c.connectToPeer(args[0])
			}
		default:
			fmt.Println("Unknown command. Type 'help' for available commands.")
		}

		if err != nil {
			log.Printf("Error: %v", err)
		}
	}
}

func (c *Client) printInstructions() {
	fmt.Println("\n=== Decentralized P2P File Sharing ===")
	fmt.Println("Commands:")
	fmt.Println("  add <filepath>     - Share a file on the network")
	fmt.Println("  list              - List your shared files")
	fmt.Println("  search <query>    - Search for files on the network")
	fmt.Println("  download <CID>    - Download a file by CID")
	fmt.Println("  peers            - Show connected peers")
	fmt.Println("  debug            - Show detailed network debug info")
	fmt.Println("  connect <addr>   - Manually connect to a peer")
	fmt.Println("  announce <CID>   - Re-announce a file to DHT")
	fmt.Println("  health           - Check connection health")
	fmt.Println("  help             - Show this help")
	fmt.Println("  exit             - Exit the application")
	fmt.Printf("\nYour Peer ID: %s\n", c.host.ID())
	fmt.Printf("Listening on: %v\n\n", c.host.Addrs())
}

// STEP 2: Add these debugging and connection functions to your Client:

func (c *Client) debugNetworkStatus() {
	fmt.Println("\n=== Network Debug Info ===")

	// Show our own info
	fmt.Printf("Our Peer ID: %s\n", c.host.ID())
	fmt.Printf("Our Addresses:\n")
	for _, addr := range c.host.Addrs() {
		fmt.Printf("  %s/p2p/%s\n", addr, c.host.ID())
	}

	// Show connected peers
	peers := c.host.Network().Peers()
	fmt.Printf("\nConnected Peers (%d):\n", len(peers))
	for i, peerID := range peers {
		conn := c.host.Network().ConnsToPeer(peerID)
		if len(conn) > 0 {
			fmt.Printf("  %d. %s\n", i+1, peerID)
			fmt.Printf("     Address: %s\n", conn[0].RemoteMultiaddr())
		}
	}

	// Show DHT info
	routingTableSize := c.dht.RoutingTable().Size()
	fmt.Printf("\nDHT Routing Table Size: %d\n", routingTableSize)

	// Show shared files
	fmt.Printf("\nShared Files (%d):\n", len(c.sharingFiles))
	for cid, fileInfo := range c.sharingFiles {
		fmt.Printf("  CID: %s\n", cid)
		fmt.Printf("  File: %s\n", fileInfo.Name)
	}
}

func (c *Client) connectToPeer(multiaddrStr string) error {
	addr, err := multiaddr.NewMultiaddr(multiaddrStr)
	if err != nil {
		return fmt.Errorf("invalid multiaddr: %w", err)
	}

	peerInfo, err := peer.AddrInfoFromP2pAddr(addr)
	if err != nil {
		return fmt.Errorf("failed to parse peer info: %w", err)
	}

	fmt.Printf("Attempting to connect to peer %s...\n", peerInfo.ID)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := c.host.Connect(ctx, *peerInfo); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	fmt.Printf("✅ Successfully connected to peer %s\n", peerInfo.ID)
	return nil
}

func (c *Client) announceFile(cidStr string) error {
	fileCID, err := cid.Decode(cidStr)
	if err != nil {
		return fmt.Errorf("invalid CID: %w", err)
	}

	fmt.Printf("Re-announcing CID %s to DHT...\n", cidStr)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := c.dht.Provide(ctx, fileCID, true); err != nil {
		return fmt.Errorf("failed to announce: %w", err)
	}

	fmt.Println("✅ Successfully announced to DHT")
	return nil
}

func (c *Client) addFile(filePath string) error {
	ctx := context.Background()

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return fmt.Errorf("failed to calculate hash: %w", err)
	}

	fileHashBytes := hasher.Sum(nil)
	fileHashStr := hex.EncodeToString(fileHashBytes)

	mhash, err := multihash.Encode(fileHashBytes, multihash.SHA2_256)
	if err != nil {
		return fmt.Errorf("failed to create multihash: %w", err)
	}

	// Gives CID(content identifier) a hash to the added file
	fileCID := cid.NewCidV1(cid.Raw, mhash)

	// add file to the local DB
	if err := c.db.AddLocalFile(ctx, fileCID.String(), info.Name(), info.Size(), filePath, fileHashStr); err != nil {
		return fmt.Errorf("failed to store file metadata: %w", err)
	}

	c.sharingFiles[fileCID.String()] = &FileInfo{
		FilePath: filePath,
		Hash:     fileHashStr,
		Size:     info.Size(),
		Name:     info.Name(),
	}

	log.Printf("Announcing file %s with CID %s to DHT...", info.Name(), fileCID.String())
	provideCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// here we are announcing file in DHT
	if err := c.dht.Provide(provideCtx, fileCID, true); err != nil {
		log.Printf("Warning: Failed to announce to DHT: %v", err)
	} else {
		log.Println("Successfully announced file to DHT")
	}

	fmt.Printf("✓ File '%s' is now being shared\n", info.Name())
	fmt.Printf("  CID: %s\n", fileCID.String())
	fmt.Printf("  Hash: %s\n", fileHashStr)
	fmt.Printf("  Size: %s\n", formatFileSize(info.Size()))

	return nil
}

func (c *Client) listLocalFiles() {
	ctx := context.Background()
	files, err := c.db.GetLocalFiles(ctx)
	if err != nil {
		log.Printf("Error retrieving files: %v", err)
		return
	}

	if len(files) == 0 {
		fmt.Println("No files being shared.")
		return
	}

	fmt.Println("\n=== Your Shared Files ===")
	for _, file := range files {
		fmt.Printf("Name: %s\n", file.Filename)
		fmt.Printf("  CID: %s\n", file.CID)
		fmt.Printf("  Size: %s\n", formatFileSize(file.FileSize))
		fmt.Printf("  Path: %s\n", file.FilePath)
		fmt.Println("  ---")
	}
}

// Updated downloadFile function with better provider filtering
func (c *Client) downloadFile(cidStr string) error {
	ctx := context.Background()

	fileCID, err := cid.Decode(cidStr)
	if err != nil {
		return fmt.Errorf("invalid CID: %w", err)
	}

	fmt.Printf("Looking for providers of CID: %s\n", fileCID.String())

	// Use the improved provider finding function
	providers, err := c.findProvidersWithTimeout(fileCID, 120*time.Second, 5)
	if err != nil {
		return fmt.Errorf("provider search failed: %w", err)
	}

	if len(providers) == 0 {
		return fmt.Errorf("no providers found for CID: %s", fileCID.String())
	}

	fmt.Printf("Found %d provider(s). Attempting connections...\n", len(providers))

	// Filter providers that support our signaling protocol
	var compatibleProviders []peer.AddrInfo
	for _, provider := range providers {
		fmt.Printf("Checking protocol support for provider: %s\n", provider.ID)

		// First try to connect to get protocol information
		connectCtx, connectCancel := context.WithTimeout(ctx, 30*time.Second)
		if err := c.host.Connect(connectCtx, provider); err != nil {
			fmt.Printf("Failed to connect to provider %s for protocol check: %v\n", provider.ID, err)
			connectCancel()
			continue
		}
		connectCancel()

		// Wait for protocol negotiation to complete
		time.Sleep(2 * time.Second)

		if c.supportsSignalingProtocol(provider.ID) {
			compatibleProviders = append(compatibleProviders, provider)
			fmt.Printf("✅ Provider %s supports signaling protocol\n", provider.ID)
		} else {
			fmt.Printf("❌ Provider %s doesn't support signaling protocol\n", provider.ID)
		}
	}

	if len(compatibleProviders) == 0 {
		return fmt.Errorf("no compatible providers found (no signaling protocol support)")
	}

	fmt.Printf("Found %d compatible provider(s)\n", len(compatibleProviders))

	// Try each compatible provider until one succeeds
	var lastErr error
	for i, provider := range compatibleProviders {
		fmt.Printf("Trying provider %d/%d: %s\n", i+1, len(compatibleProviders), provider.ID)

		webrtcPeer, err := c.initiateWebRTCConnectionWithRetry(provider.ID, 2) // Reduced retries per provider
		if err != nil {
			fmt.Printf("Failed to connect to provider %s: %v\n", provider.ID, err)
			lastErr = err
			continue
		}

		// Successfully connected, start download
		downloadPath := fmt.Sprintf("%s.download", cidStr)
		localFile, err := os.Create(downloadPath)
		if err != nil {
			webrtcPeer.Close()
			return fmt.Errorf("failed to create download file: %w", err)
		}

		webrtcPeer.SetFileWriter(localFile)
		fmt.Printf("Downloading to %s...\n", downloadPath)

		request := map[string]string{
			"command": "REQUEST_FILE",
			"cid":     cidStr,
		}

		if err := webrtcPeer.Send(request); err != nil {
			webrtcPeer.Close()
			localFile.Close()
			os.Remove(downloadPath)
			fmt.Printf("Failed to send file request to provider %s: %v\n", provider.ID, err)
			lastErr = err
			continue
		}

		fmt.Println("Download initiated successfully!")
		return nil
	}

	return fmt.Errorf("failed to connect to any compatible provider. Last error: %w", lastErr)
}

func (c *Client) listConnectedPeers() {
	peers := c.host.Network().Peers()
	fmt.Printf("\n=== Connected Peers (%d) ===\n", len(peers))

	for _, peerID := range peers {
		conn := c.host.Network().ConnsToPeer(peerID)
		if len(conn) > 0 {
			fmt.Printf("Peer: %s\n", peerID)
			fmt.Printf("  Address: %s\n", conn[0].RemoteMultiaddr())
		}
	}
}

func (c *Client) initiateWebRTCConnectionWithRetry(targetPeerID peer.ID, maxRetries int) (*webRTC.WebRTCPeer, error) {
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			// Exponential backoff: 2s, 4s, 8s...
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			fmt.Printf("Retrying in %v... (attempt %d/%d)\n", backoff, attempt, maxRetries)
			time.Sleep(backoff)
		}

		peer, err := c.initiateWebRTCConnection(targetPeerID)
		if err == nil {
			return peer, nil
		}

		lastErr = err
		fmt.Printf("Attempt %d failed: %v\n", attempt, err)
	}

	return nil, fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
}
func (c *Client) initiateWebRTCConnection(targetPeerID peer.ID) (*webRTC.WebRTCPeer, error) {
	log.Printf("Creating signaling stream to peer %s...", targetPeerID)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second) // Increased from 60s
	defer cancel()

	info := peer.AddrInfo{ID: targetPeerID, Addrs: c.host.Peerstore().Addrs(targetPeerID)}

	// Try multiple methods to resolve peer addresses
	if len(info.Addrs) == 0 {
		fmt.Printf("No cached addresses for peer %s, resolving via DHT...\n", targetPeerID)

		dhtCtx, dhtCancel := context.WithTimeout(ctx, 30*time.Second)
		pinfo, err := c.dht.FindPeer(dhtCtx, targetPeerID)
		dhtCancel()

		if err == nil && len(pinfo.Addrs) > 0 {
			info = pinfo
			fmt.Printf("Found %d addresses via DHT\n", len(info.Addrs))
		} else {
			fmt.Printf("DHT lookup failed: %v\n", err)

			fmt.Printf("Attempting to refresh DHT and retry...\n")
			go func() {
				refreshCtx, refreshCancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer refreshCancel()
				c.dht.RefreshRoutingTable()

				if pinfo, err := c.dht.FindPeer(refreshCtx, targetPeerID); err == nil {
					c.host.Peerstore().AddAddrs(pinfo.ID, pinfo.Addrs, time.Hour)
				}
			}()

			time.Sleep(5 * time.Second)

			finalCtx, finalCancel := context.WithTimeout(ctx, 20*time.Second)
			if pinfo, err := c.dht.FindPeer(finalCtx, targetPeerID); err == nil {
				info = pinfo
				fmt.Printf("Found %d addresses after DHT refresh\n", len(info.Addrs))
			}
			finalCancel()
		}
	}

	if len(info.Addrs) == 0 {
		return nil, fmt.Errorf("peer %s has no known multiaddrs after all resolution attempts", targetPeerID)
	}

	c.host.Peerstore().AddAddrs(info.ID, info.Addrs, time.Hour)

	// Enhanced connection establishment with verification
	if c.host.Network().Connectedness(info.ID) != network.Connected {
		fmt.Printf("Connecting to peer %s...\n", info.ID)
		connectCtx, connectCancel := context.WithTimeout(ctx, 45*time.Second) // Increased from 30s
		err := c.host.Connect(connectCtx, info)
		connectCancel()

		if err != nil {
			return nil, fmt.Errorf("failed to connect to peer %s: %w", info.ID, err)
		}
		fmt.Printf("Successfully connected to peer %s\n", info.ID)

		// Wait a moment for connection to stabilize
		time.Sleep(2 * time.Second)
	}

	// Verify connection is actually ready for streams
	if !c.isConnectionReady(info.ID) {
		return nil, fmt.Errorf("connection to peer %s is not ready for streams", info.ID)
	}

	// Create signaling stream with retry mechanism
	var s network.Stream
	var err error

	// Try creating stream with progressive timeout increases
	timeouts := []time.Duration{15 * time.Second, 30 * time.Second, 45 * time.Second}

	for attempt, timeout := range timeouts {
		fmt.Printf("Attempting to create signaling stream (attempt %d/%d, timeout: %v)...\n",
			attempt+1, len(timeouts), timeout)

		streamCtx, streamCancel := context.WithTimeout(ctx, timeout)
		s, err = c.host.NewStream(streamCtx, targetPeerID, p2p.SignalingProtocolID)
		streamCancel()

		if err == nil {
			fmt.Printf("Successfully created signaling stream on attempt %d\n", attempt+1)
			break
		}

		fmt.Printf("Stream creation attempt %d failed: %v\n", attempt+1, err)

		// If this isn't the last attempt, wait before retrying
		if attempt < len(timeouts)-1 {
			waitTime := time.Duration(2<<uint(attempt)) * time.Second // 2s, 4s, 8s
			fmt.Printf("Waiting %v before retry...\n", waitTime)
			time.Sleep(waitTime)

			// Check if connection is still alive
			if c.host.Network().Connectedness(info.ID) != network.Connected {
				fmt.Println("Connection lost, attempting to reconnect...")
				reconnectCtx, reconnectCancel := context.WithTimeout(ctx, 30*time.Second)
				if reconnectErr := c.host.Connect(reconnectCtx, info); reconnectErr != nil {
					reconnectCancel()
					return nil, fmt.Errorf("failed to reconnect to peer %s: %w", info.ID, reconnectErr)
				}
				reconnectCancel()
				time.Sleep(2 * time.Second) // Let connection stabilize
			}
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create signaling stream after %d attempts: %w", len(timeouts), err)
	}

	webRTCPeer, err := webRTC.NewWebRTCPeer(c.onDataChannelMessage)
	if err != nil {
		s.Close()
		return nil, fmt.Errorf("failed to create WebRTC peer: %w", err)
	}

	webRTCPeer.SetSignalingStream(s)
	c.addWebRTCPeer(targetPeerID, webRTCPeer)

	// Create offer with timeout
	// offerCtx, offerCancel := context.WithTimeout(ctx, 15*time.Second)
	offer, err := webRTCPeer.CreateOffer() // Assuming this method exists
	// offerCancel()

	if err != nil {
		// Fallback to original method if context method doesn't exist
		offer, err = webRTCPeer.CreateOffer()
		if err != nil {
			webRTCPeer.Close()
			return nil, fmt.Errorf("failed to create offer: %w", err)
		}
	}

	// Send offer with timeout
	encoder := json.NewEncoder(s)
	encodeCh := make(chan error, 1)
	go func() {
		encodeCh <- encoder.Encode(offer)
	}()

	select {
	case err := <-encodeCh:
		if err != nil {
			webRTCPeer.Close()
			return nil, fmt.Errorf("failed to send offer: %w", err)
		}
	case <-time.After(10 * time.Second):
		webRTCPeer.Close()
		return nil, fmt.Errorf("timeout sending offer")
	}

	// Receive answer with timeout
	var answer string
	decoder := json.NewDecoder(s)
	decodeCh := make(chan error, 1)
	go func() {
		decodeCh <- decoder.Decode(&answer)
	}()

	select {
	case err := <-decodeCh:
		if err != nil {
			webRTCPeer.Close()
			return nil, fmt.Errorf("failed to receive answer: %w", err)
		}
	case <-time.After(15 * time.Second):
		webRTCPeer.Close()
		return nil, fmt.Errorf("timeout receiving answer")
	}

	if err := webRTCPeer.SetAnswer(answer); err != nil {
		webRTCPeer.Close()
		return nil, fmt.Errorf("failed to set answer: %w", err)
	}

	if err := webRTCPeer.WaitForConnection(45 * time.Second); err != nil { // Increased from 30s
		webRTCPeer.Close()
		return nil, fmt.Errorf("failed to establish WebRTC connection: %w", err)
	}

	log.Printf("WebRTC connection established with %s", targetPeerID)
	return webRTCPeer, nil
}

func (c *Client) handleWebRTCOffer(offer, remotePeerIDStr string, s network.Stream) (string, error) {
	remotePeerID, err := peer.Decode(remotePeerIDStr)
	if err != nil {
		return "", err
	}

	log.Printf("Handling WebRTC offer from %s", remotePeerID)

	webRTCPeer, err := webRTC.NewWebRTCPeer(c.onDataChannelMessage)
	if err != nil {
		return "", err
	}

	webRTCPeer.SetSignalingStream(s)
	answer, err := webRTCPeer.CreateAnswer(offer)
	if err != nil {
		webRTCPeer.Close()
		return "", err
	}

	c.addWebRTCPeer(remotePeerID, webRTCPeer)
	return answer, nil
}

func (c *Client) onDataChannelMessage(msg webrtc.DataChannelMessage, p *webRTC.WebRTCPeer) {
	if msg.IsString {
		var message map[string]string
		if err := json.Unmarshal(msg.Data, &message); err != nil {
			log.Printf("Received invalid message: %s", string(msg.Data))
			return
		}

		if cmd, ok := message["command"]; ok && cmd == "REQUEST_FILE" {
			if cid, hasCID := message["cid"]; hasCID {
				go c.sendFile(p, cid)
			}
		} else if status, ok := message["status"]; ok && status == "TRANSFER_COMPLETE" {
			log.Println("File transfer complete!")
			if writer := p.GetFileWriter(); writer != nil {
				writer.Close()
			}
			p.Close()
		}
	} else {
		if writer := p.GetFileWriter(); writer != nil {
			if _, err := writer.Write(msg.Data); err != nil {
				log.Printf("Error writing file chunk: %v", err)
			}
		}
	}
}

func (c *Client) sendFile(p *webRTC.WebRTCPeer, cidStr string) {
	log.Printf("Processing file request for CID: %s", cidStr)

	fileInfo, ok := c.sharingFiles[cidStr]
	if !ok {
		log.Printf("File not found: %s", cidStr)
		p.Send(map[string]string{"error": "File not found"})
		return
	}

	file, err := os.Open(fileInfo.FilePath)
	if err != nil {
		log.Printf("Error opening file: %v", err)
		p.Send(map[string]string{"error": "Could not open file"})
		return
	}
	defer file.Close()

	log.Printf("Starting file transfer: %s", fileInfo.Name)

	buffer := make([]byte, 64*1024)
	for {
		bytesRead, err := file.Read(buffer)
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("Error reading file: %v", err)
			return
		}

		if err := p.SendRaw(buffer[:bytesRead]); err != nil {
			log.Printf("Error sending chunk: %v", err)
			return
		}
	}

	log.Printf("File transfer complete: %s", fileInfo.Name)
	p.Send(map[string]string{"status": "TRANSFER_COMPLETE"})
}

func (c *Client) addWebRTCPeer(id peer.ID, p *webRTC.WebRTCPeer) {
	c.peersMux.Lock()
	defer c.peersMux.Unlock()
	c.webRTCPeers[id] = p
}

func setupGracefulShutdown(h host.Host) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		log.Println("Shutting down...")
		if err := h.Close(); err != nil {
			log.Printf("Error closing host: %v", err)
		}
		os.Exit(0)
	}()
}

func formatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

// Periodic DHT maintenance
func (c *Client) startDHTMaintenance() {
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			log.Println("Performing DHT maintenance...")

			// Refresh routing table
			c.dht.RefreshRoutingTable()

			// Log connected peer count
			peers := c.host.Network().Peers()
			log.Printf("Connected to %d peers", len(peers))

			// If we have too few connections, try to bootstrap again
			if len(peers) < 5 {
				log.Println("Low peer count, attempting to reconnect to bootstrap nodes...")
				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				if err := p2p.Bootstrap(ctx, c.host, c.dht); err != nil {
					log.Printf("Bootstrap retry failed: %v", err)
				}
				cancel()
			}

		}
	}()
}

// Helper function to verify connection readiness
func (c *Client) isConnectionReady(peerID peer.ID) bool {
	// Check if peer is connected
	if c.host.Network().Connectedness(peerID) != network.Connected {
		return false
	}

	// Check if we have active connections
	conns := c.host.Network().ConnsToPeer(peerID)
	if len(conns) == 0 {
		return false
	}

	// Verify at least one connection is not closing
	for _, conn := range conns {
		if conn.IsClosed() {
			continue
		}
		// Connection exists and is not closed
		return true
	}

	return false
}

// Enhanced protocol checking function (add this as a new method)
func (c *Client) supportsSignalingProtocol(peerID peer.ID) bool {
	protocols, err := c.host.Peerstore().GetProtocols(peerID)
	if err != nil {
		return false
	}

	for _, protocol := range protocols {
		if protocol == p2p.SignalingProtocolID {
			return true
		}
	}

	// If not in cache, try to probe
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// This is a simple probe - try to create a very short-lived stream
	if s, err := c.host.NewStream(ctx, peerID, p2p.SignalingProtocolID); err == nil {
		s.Close()
		return true
	}

	return false
}
