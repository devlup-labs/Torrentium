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

	webRTC "torrentium/internal/client"
	db "torrentium/internal/db"
	p2p "torrentium/internal/p2p"

	"github.com/dustin/go-humanize"
	"github.com/ipfs/go-cid"
	"github.com/joho/godotenv"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/pion/webrtc/v3"
	"github.com/schollz/progressbar/v3"

	"github.com/libp2p/go-libp2p/core/peer"

	// "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/client"
	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multihash"
)

const (
	DefaultPieceSize       = 1 << 20 // 1 MiB pieces
	MaxProviders           = 10
	MaxChunk               = 16 * 1024 // 16KiB chunks
	MaxParallelDownloads   = 3
	PieceTimeout           = 300 * time.Second // Timeout for downloading a single piece
	RetransmissionTimeout  = 5 * time.Second
	KeepAliveInterval      = 15 * time.Second
	PingInterval           = 10 * time.Second
	MaxRTT                 = 500 * time.Millisecond
	MinDelay               = 0
	MaxDelay               = 100 * time.Millisecond
	ExponentialBackoffBase = 1 * time.Second
	MaxBackoff             = 32 * time.Second
	// Trust score settings
	SendTrustScoreImmediately = true // Send trust score updates immediately after each piece
)

type Client struct {
	host             host.Host
	dht              *dht.IpfsDHT
	webRTCPeers      map[peer.ID]*webRTC.SimpleWebRTCPeer
	peersMux         sync.RWMutex
	sharingFiles     map[string]*FileInfo
	activeDownloads  map[string]*DownloadState
	downloadsMux     sync.RWMutex
	db               *db.Repository
	unackedChunks    map[string]map[int64]map[int]controlMessage
	unackedChunksMux sync.RWMutex
	congestionCtrl   map[peer.ID]time.Duration
	pingTimes        map[peer.ID]time.Time
	rttMeasurements  map[peer.ID][]time.Duration
	rttMux           sync.Mutex
	// Trust score related fields
	providerTrustScores map[string]float64
	trustScoreMux       sync.RWMutex
	// Transaction tracking
	transactionTrackers map[string]*TransactionTracker
	trackerMux          sync.RWMutex
}

// TransactionTracker tracks metrics for each peer during downloads
type TransactionTracker struct {
	PeerID             string
	StartTime          time.Time
	ChunksReceived     int
	SuccessfulChunks   int
	TotalDataReceived  int64
	AverageUploadSpeed int // Moving average
	TransferSuccessful bool
	ChunkSpeeds        []int       // Speed for each chunk received
	ChunkStartTimes    []time.Time // Start time for each chunk
}

type FileInfo struct {
	FilePath string
	Hash     string
	Size     int64
	Name     string
	PieceSz  int64
}

type controlMessage struct {
	Command     string     `json:"command"`
	CID         string     `json:"cid,omitempty"`
	PieceSize   int64      `json:"piece_size,omitempty"`
	TotalSize   int64      `json:"total_size,omitempty"`
	HashHex     string     `json:"hash_hex,omitempty"`
	NumPieces   int64      `json:"num_pieces,omitempty"`
	Pieces      []db.Piece `json:"pieces,omitempty"`
	PieceHash   string     `json:"piece_hash,omitempty"`
	Index       int64      `json:"index,omitempty"`
	Filename    string     `json:"filename,omitempty"`
	ChunkIndex  int        `json:"chunk_index,omitempty"`
	TotalChunks int        `json:"total_chunks,omitempty"`
	Payload     string     `json:"payload,omitempty"`
	Sequence    int        `json:"sequence,omitempty"`
	// Trust score fields
	TrustScore float64 `json:"trust_score,omitempty"`
	PeerID     string  `json:"peer_id,omitempty"`
	// Trust score update parameters
	TrustScoreUpdate *TrustScoreUpdateParams `json:"trust_score_update,omitempty"`
}

// TrustScoreUpdateParams contains data needed for seeder to calculate trust score
type TrustScoreUpdateParams struct {
	LeecherPeerID      string `json:"leecher_peer_id"`
	ChunksReceived     int    `json:"chunks_received"`     // #12: from current transaction
	SuccessfulChunks   int    `json:"successful_chunks"`   // #13: from current transaction
	TotalDataReceived  int64  `json:"total_data_received"` // #14: from current transaction (file size for #2)
	AverageChunkSpeed  int    `json:"average_chunk_speed"` // #5: average speed for chunks assigned to this seeder
	TransferSuccessful bool   `json:"transfer_successful"` // #15: yes/no parameter
}

type DownloadState struct {
	File            *os.File
	Manifest        controlMessage
	TotalPieces     int
	Pieces          []db.Piece
	Completed       chan bool
	Progress        *progressbar.ProgressBar
	PieceStatus     []bool // true if piece is downloaded
	PieceAssignees  map[int]peer.ID
	pieceBuffers    map[int][][]byte // Buffer to reassemble chunks into pieces
	mu              sync.Mutex
	completedPieces int
	pieceTimers     map[int]*time.Timer // Timers for each piece
	retryCounts     map[int]int         // Retry counts for exponential backoff
}

var (
	manifestWaiters = make(map[string]chan controlMessage)
	manifestChMu    sync.Mutex
)

func setupGracefulShutdown(h host.Host) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-ch
		log.Println("Shutting down gracefully...")
		_ = h.Close()
		os.Exit(0)
	}()
}

func NewClient(h host.Host, d *dht.IpfsDHT, repo *db.Repository) *Client {
	c := &Client{
		host:                h,
		dht:                 d,
		webRTCPeers:         make(map[peer.ID]*webRTC.SimpleWebRTCPeer),
		sharingFiles:        make(map[string]*FileInfo),
		activeDownloads:     make(map[string]*DownloadState),
		db:                  repo,
		unackedChunks:       make(map[string]map[int64]map[int]controlMessage),
		congestionCtrl:      make(map[peer.ID]time.Duration),
		pingTimes:           make(map[peer.ID]time.Time),
		rttMeasurements:     make(map[peer.ID][]time.Duration),
		providerTrustScores: make(map[string]float64),
		transactionTrackers: make(map[string]*TransactionTracker),
	}
	go c.monitorCongestion()
	return c
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

	h, d, err := p2p.NewHost(
		ctx,
		"/ip4/0.0.0.0/tcp/0",
		nil, // temporarily, if you don’t have client yet
	)
	if err != nil {
		log.Fatal("Failed to create libp2p host:", err)
	}
	defer h.Close()

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

	// Register trust score protocol handler
	client.registerTrustScoreProtocol()

	client.commandLoop()
}

func (c *Client) commandLoop() {
	scanner := bufio.NewScanner(os.Stdin)
	c.printInstructions()
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
				fmt.Println("Usage: add <path>")
			} else {
				err = c.addFile(args[0])
			}
		case "list":
			c.listLocalFiles()
		case "search":
			if len(args) != 1 {
				fmt.Println("Usage: search <cid|text>")
			} else {
				c.checkConnectionHealth()
				if strings.HasPrefix(args[0], "bafy") || strings.HasPrefix(args[0], "Qm") {
					err = c.enhancedSearchByCID(args[0])
				} else {
					err = c.searchByText(args[0])
				}
			}
		case "download":
			if len(args) != 1 {
				fmt.Println("Usage: download <cid>")
			} else {
				err = c.downloadFile(args[0])
			}
		case "peers":
			c.listConnectedPeers()
		case "connect":
			if len(args) != 1 {
				fmt.Println("Usage: connect <multiaddr>")
				fmt.Println("Example: connect /ip4/127.0.0.1/tcp/54437/p2p/12D3KooWBLZFWsGZxoCFC8NsFgKvD6WJ6xV9UmYdR8t2C1kqYTcd")
			} else {
				err = c.connectToPeer(args[0])
			}
		case "announce":
			if len(args) != 1 {
				fmt.Println("Usage: announce <cid>")
			} else {
				err = c.announceFile(args[0])
			}
		case "health":
			c.checkConnectionHealth()
		case "nettest":
			c.performNetworkDiagnostics()
		case "localtest":
			c.performLocalWebRTCTest()
		case "debug":
			c.debugNetworkStatus()
		case "trustscore":
			c.checkMyTrustScore()
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

func (c *Client) printInstructions() {
	fmt.Println("\n=== Decentralized P2P File Sharing ===")
	fmt.Println("Commands:")
	fmt.Println(" add <path>           - Share a file on the network")
	fmt.Println(" list                 - List your shared files")
	fmt.Println(" search <cid|text>    - Search by CID or filename text")
	fmt.Println(" download <cid>       - Download a file by CID")
	fmt.Println(" peers                - Show connected peers")
	fmt.Println(" connect <multiaddr>  - Manually connect to a peer")
	fmt.Println(" announce <cid>       - Re-announce a file to DHT")
	fmt.Println(" health               - Check connection health")
	fmt.Println(" nettest              - Perform comprehensive network diagnostics")
	fmt.Println(" localtest            - Test local WebRTC functionality")
	fmt.Println(" debug                - Show detailed network debug info")
	fmt.Println(" trustscore           - Check your current trust score and metrics")
	fmt.Println(" help                 - Show this help")
	fmt.Println(" exit                 - Exit the application")
	fmt.Printf("\nYour Peer ID: %s\n", c.host.ID())
	fmt.Printf("Listening on: %v\n\n", c.host.Addrs())
}

func (c *Client) debugNetworkStatus() {
	fmt.Println("\n=== Network Debug Info ===")
	fmt.Printf("Our Peer ID: %s\n", c.host.ID())
	fmt.Printf("Our Addresses:\n")
	for _, addr := range c.host.Addrs() {
		fmt.Printf(" %s/p2p/%s\n", addr, c.host.ID())
	}

	peers := c.host.Network().Peers()
	fmt.Printf("\nConnected Peers (%d):\n", len(peers))
	for i, peerID := range peers {
		conn := c.host.Network().ConnsToPeer(peerID)
		if len(conn) > 0 {
			fmt.Printf(" %d. %s\n", i+1, peerID)
			fmt.Printf("    Address: %s\n", conn[0].RemoteMultiaddr())
		}
	}

	routingTableSize := c.dht.RoutingTable().Size()
	fmt.Printf("\nDHT Routing Table Size: %d\n", routingTableSize)

	fmt.Printf("\nShared Files (%d):\n", len(c.sharingFiles))
	for cid, fileInfo := range c.sharingFiles {
		fmt.Printf(" CID: %s\n", cid)
		fmt.Printf(" File: %s\n", fileInfo.Name)
		fmt.Printf(" ---\n")
	}
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
	fmt.Println(" - Successfully announced to DHT")
	return nil
}

func (c *Client) startDHTMaintenance() {
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			log.Println("Performing DHT maintenance...")
			c.dht.RefreshRoutingTable()
			peers := c.host.Network().Peers()
			log.Printf("Connected to %d peers", len(peers))
			if len(peers) < 5 {
				log.Println("Low peer count; re-bootstrapping...")
				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				_ = p2p.Bootstrap(ctx, c.host, c.dht)
				cancel()
			}
		}
	}()
}

func (c *Client) checkConnectionHealth() {
	peers := c.host.Network().Peers()
	fmt.Printf("\n=== Connection Health ===\n")
	fmt.Printf("Connected peers: %d\n", len(peers))
	if len(peers) < 3 {
		fmt.Println(" - Warning: Low peer count. Consider restarting or checking network connectivity.")
	} else {
		fmt.Println(" - Good peer connectivity")
	}

	routingTableSize := c.dht.RoutingTable().Size()
	fmt.Printf("DHT routing table size: %d\n", routingTableSize)
	if routingTableSize < 10 {
		fmt.Println(" - Warning: Small DHT routing table. File discovery may be limited.")
	} else {
		fmt.Println(" - Good DHT connectivity")
	}
}

func (c *Client) performNetworkDiagnostics() {
	fmt.Println("\n=== Network Diagnostics ===")

	// Check our addresses and detect local network
	fmt.Printf("Our listening addresses:\n")
	hasLocalAddr := false
	for _, addr := range c.host.Addrs() {
		addrStr := addr.String()
		fmt.Printf(" - %s/p2p/%s\n", addr, c.host.ID())
		if strings.Contains(addrStr, "192.168.") || strings.Contains(addrStr, "10.") || strings.Contains(addrStr, "172.") {
			hasLocalAddr = true
		}
	}

	if hasLocalAddr {
		fmt.Println("Local network addresses detected - optimized for same-network connections")
	} else {
		fmt.Println("No local network addresses detected")
	}

	// Test ICE connectivity
	fmt.Println("\nTesting ICE connectivity...")
	if err := webRTC.TestICEConnectivity(); err != nil {
		fmt.Printf("ICE connectivity test failed: %v\n", err)
		fmt.Println("This indicates potential WebRTC connection issues")
	} else {
		fmt.Println("ICE connectivity test passed")
	}

	// Check libp2p connectivity
	peers := c.host.Network().Peers()
	fmt.Printf("\nlibp2p peer connections: %d\n", len(peers))
	if len(peers) > 0 {
		fmt.Println("Connected peers:")
		for i, peerID := range peers {
			if i >= 5 { // Limit output
				fmt.Printf(" ... and %d more\n", len(peers)-5)
				break
			}
			conn := c.host.Network().ConnsToPeer(peerID)
			if len(conn) > 0 {
				remoteAddr := conn[0].RemoteMultiaddr().String()
				fmt.Printf(" - %s (%s)\n", peerID, remoteAddr)
				if strings.Contains(remoteAddr, "192.168.") || strings.Contains(remoteAddr, "10.") || strings.Contains(remoteAddr, "172.") {
					fmt.Printf("Local network peer\n")
				}
			}
		}
	}

	// Check DHT health
	routingTableSize := c.dht.RoutingTable().Size()
	fmt.Printf("\nDHT routing table size: %d\n", routingTableSize)

	fmt.Println("\nFor same-network connections:")
	fmt.Println("   1. Make sure both peers are connected to libp2p first")
	fmt.Println("   2. WebRTC should work directly without TURN servers")
	fmt.Println("   3. Use 'peers' command to see connected peers")
	fmt.Println("   4. Use 'connect <multiaddr>' to manually connect")

	fmt.Println("\nDiagnostics complete.")
}

func (c *Client) performLocalWebRTCTest() {
	fmt.Println("\n=== Local Network WebRTC Test ===")

	// Create a simple WebRTC peer for testing
	testPeer, err := webRTC.NewSimpleWebRTCPeer(func(msg webrtc.DataChannelMessage, peer *webRTC.SimpleWebRTCPeer) {
		log.Printf("Test received message: %s", string(msg.Data))
	}, func(peerID peer.ID) {
		// No-op for this test
	})
	if err != nil {
		fmt.Printf("Failed to create test WebRTC peer: %v\n", err)
		return
	}
	defer testPeer.Close()

	// Try to create an offer to test the process
	offer, err := testPeer.CreateOffer()
	if err != nil {
		fmt.Printf("Failed to create WebRTC offer: %v\n", err)
		return
	}

	fmt.Printf("WebRTC offer created successfully (length: %d chars)\n", len(offer))

	// Try to wait for ICE gathering
	fmt.Println("Waiting 5 seconds for ICE candidate gathering...")
	time.Sleep(5 * time.Second)

	fmt.Println("Local WebRTC test completed - basic functionality working")
	fmt.Println("If downloads still fail, the issue is likely in the peer-to-peer signaling")
}

func (c *Client) addFile(filePath string) error {
	ctx := context.Background()
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return fmt.Errorf("failed to calculate hash: %w", err)
	}

	fileHashBytes := hasher.Sum(nil)
	fileHashStr := hex.EncodeToString(fileHashBytes)
	mhash, err := multihash.Encode(fileHashBytes, multihash.SHA2_256)
	if err != nil {
		return fmt.Errorf("failed to create multihash: %w", err)
	}

	fileCID := cid.NewCidV1(cid.Raw, mhash)

	// Create pieces manifest
	pieceSz := int64(DefaultPieceSize)
	numPieces := (info.Size() + pieceSz - 1) / pieceSz
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}

	for idx := int64(0); idx < numPieces; idx++ {
		offset := idx * pieceSz
		size := min64(pieceSz, info.Size()-offset)
		h := sha256.New()
		if _, err := io.CopyN(h, f, size); err != nil {
			return err
		}
		ph := hex.EncodeToString(h.Sum(nil))
		if err := c.db.UpsertPiece(ctx, fileCID.String(), idx, offset, size, ph, true); err != nil {
			return err
		}
	}

	if err := c.db.AddLocalFile(ctx, fileCID.String(), info.Name(), info.Size(), filePath, fileHashStr); err != nil {
		return fmt.Errorf("failed to store file metadata: %w", err)
	}

	c.sharingFiles[fileCID.String()] = &FileInfo{
		FilePath: filePath,
		Hash:     fileHashStr,
		Size:     info.Size(),
		Name:     info.Name(),
		PieceSz:  pieceSz,
	}

	log.Printf("Announcing file %s with CID %s to DHT...", info.Name(), fileCID.String())
	provideCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if err := c.dht.Provide(provideCtx, fileCID, true); err != nil {
		log.Printf(" - Warning: Failed to announce to DHT: %v", err)
	} else {
		log.Println(" - Successfully announced file to DHT")
	}

	fmt.Printf("✓ File '%s' is now being shared\n", info.Name())
	fmt.Printf(" CID: %s\n", fileCID.String())
	fmt.Printf(" Hash: %s\n", fileHashStr)
	fmt.Printf(" Size: %s\n", humanize.Bytes(uint64(info.Size())))
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
		fmt.Println(" - No files being shared.")
		return
	}
	fmt.Println("\n=== Your Shared Files ===")
	for _, file := range files {
		fmt.Printf("Name: %s\n", file.Filename)
		fmt.Printf(" CID: %s\n", file.CID)
		fmt.Printf(" Size: %s\n", humanize.Bytes(uint64(file.FileSize)))
		fmt.Printf(" Path: %s\n", file.FilePath)
		fmt.Println(" ---")
	}
}

func (c *Client) searchByText(q string) error {
	ctx := context.Background()
	matches, err := c.db.SearchByFilename(ctx, q)
	if err != nil {
		return err
	}
	if len(matches) == 0 {
		fmt.Printf("Searching for files containing '%s'...\n", q)
		fmt.Println("Note: Direct filename search requires content indexing.")
		fmt.Println("Try using the CID if you have it, or check with known peers.")
		return nil
	}
	fmt.Printf("Local index matches for '%s':\n", q)
	for _, m := range matches {
		fmt.Printf("- %s  CID:%s\n", m.Filename, m.CID)
	}
	return nil
}

func (c *Client) enhancedSearchByCID(cidStr string) error {
	fileCID, err := cid.Decode(cidStr)
	if err != nil {
		return fmt.Errorf("invalid CID: %w", err)
	}
	fmt.Printf("Searching for CID: %s\n", fileCID.String())
	providers, err := c.findProvidersWithTimeout(fileCID, 60*time.Second, MaxProviders)
	if err != nil {
		return fmt.Errorf("provider search failed: %w", err)
	}

	if len(providers) == 0 {
		fmt.Println("No providers found for this CID")
		fmt.Println("This could mean:")
		fmt.Println(" - The file is not being shared")
		fmt.Println(" - The provider is offline")
		fmt.Println(" - Network connectivity issues")
		fmt.Println(" - DHT routing problem")
		return nil
	}

	fmt.Printf("Found %d provider(s):\n", len(providers))
	for i, provider := range providers {
		fmt.Printf(" %d. %s\n", i+1, provider.ID)
		if c.host.Network().Connectedness(provider.ID) == network.Connected {
			fmt.Printf(" - Already connected\n")
		} else {
			fmt.Printf(" - Not connected\n")
		}
	}
	return nil
}

func (c *Client) findProvidersWithTimeout(id cid.Cid, timeout time.Duration, maxProviders int) ([]peer.AddrInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	providersChan := c.dht.FindProvidersAsync(ctx, id, maxProviders)
	var providers []peer.AddrInfo
	var totalFound int

	done := make(chan struct{})
	go func() {
		defer close(done)
		for provider := range providersChan {
			totalFound++
			if provider.ID != c.host.ID() {
				providers = append(providers, provider)
				fmt.Printf(" - Found provider %d: %s\n", len(providers), provider.ID)
				if len(providers) >= maxProviders {
					break
				}
			}
		}
	}()

	select {
	case <-done:
		fmt.Printf("Provider search completed. Found %d total providers, %d unique external providers\n",
			totalFound, len(providers))
	case <-time.After(timeout):
		fmt.Printf("Provider search timed out. Found %d providers so far\n", len(providers))
	}

	return providers, nil
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

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := c.host.Connect(ctx, *peerInfo); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	fmt.Printf(" - Successfully connected to peer %s\n", peerInfo.ID)

	c.host.Peerstore().AddAddrs(peerInfo.ID, peerInfo.Addrs, time.Hour)

	return nil
}

func (c *Client) listConnectedPeers() {
	peers := c.host.Network().Peers()
	fmt.Printf("\n=== Connected Peers (%d) ===\n", len(peers))
	for _, peerID := range peers {
		conn := c.host.Network().ConnsToPeer(peerID)
		if len(conn) > 0 {
			fmt.Printf("Peer: %s\n", peerID)
			fmt.Printf(" Address: %s\n", conn[0].RemoteMultiaddr())
		}
	}
}

func (c *Client) downloadFile(cidStr string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fileCID, err := cid.Decode(cidStr)
	if err != nil {
		return fmt.Errorf("invalid CID: %w", err)
	}

	fmt.Printf("Looking for providers of CID: %s\n", fileCID.String())
	providers, err := c.findProvidersWithTimeout(fileCID, 60*time.Second, MaxProviders)
	if err != nil {
		return fmt.Errorf("provider search failed: %w", err)
	}

	if len(providers) == 0 {
		return fmt.Errorf("no providers found")
	}

	fmt.Printf("Found %d providers. Getting file manifest...\n", len(providers))
	relayAddrStr := "/dns4/relay-torrentium-nnyw.onrender.com/tcp/443/wss/p2p/12D3KooWHs4BxfNGuKmQMixupzFrsX9zLXnSrGPvZM3DCMvjTA6j"

	var manifest controlMessage
	var firstPeer *webRTC.SimpleWebRTCPeer

	for _, p := range providers {
		// Try direct connection first
		peerConn, err := c.initiateWebRTCConnectionWithRetry(p.ID, 1)
		if err != nil {
			// Fallback: relay connection
			log.Println("🔁 Direct connection failed, trying relay...")

			// Build circuit address
			circuitStr := fmt.Sprintf("%s/p2p-circuit/p2p/%s", relayAddrStr, p.ID.String())
			circuitMaddr, err := multiaddr.NewMultiaddr(circuitStr)
			if err != nil {
				log.Printf("Invalid circuit multiaddr: %v", err)
				continue
			}
			targetInfo := peer.AddrInfo{ID: p.ID, Addrs: []multiaddr.Multiaddr{circuitMaddr}}

			if err := c.host.Connect(ctx, targetInfo); err != nil {
				log.Printf("❌ Relay dial failed: %v", err)
				continue
			}

			log.Printf("✅ Relay dial to %s successful", p.ID)

			// Now perform WebRTC handshake
			peerConn, err = c.initiateWebRTCConnectionWithRetry(p.ID, 1)
			if err != nil {
				log.Printf("⚠ WebRTC connection via relay failed: %v", err)
				continue
			}
		}

		// If WebRTC connected, fetch manifest
		if peerConn != nil {
			manifest, err = c.requestManifest(peerConn, cidStr)
			if err == nil {
				firstPeer = peerConn
				break
			}
			peerConn.Close()
		}
	}

	if firstPeer == nil {
		return fmt.Errorf("failed to connect to any provider to get manifest")
	}

	// Store pieces in the database
	for _, piece := range manifest.Pieces {
		if err := c.db.UpsertPiece(ctx, cidStr, piece.Index, piece.Offset, piece.Size, piece.Hash, false); err != nil {
			log.Printf("Failed to store piece info for download: %v", err)
		}
	}

	downloadPath := fmt.Sprintf("%s.download", manifest.Filename)
	finalPath := manifest.Filename
	localFile, err := os.Create(downloadPath)
	if err != nil {
		firstPeer.Close()
		return fmt.Errorf("failed to create file: %w", err)
	}

	pieces, _ := c.db.GetPieces(ctx, cidStr)
	if len(pieces) == 0 {
		return fmt.Errorf("failed to retrieve piece information after receiving manifest")
	}

	state := &DownloadState{
		File:            localFile,
		Manifest:        manifest,
		TotalPieces:     int(manifest.NumPieces),
		Pieces:          pieces,
		Completed:       make(chan bool, 1),
		Progress:        progressbar.DefaultBytes(manifest.TotalSize, "downloading..."),
		PieceStatus:     make([]bool, int(manifest.NumPieces)),
		PieceAssignees:  make(map[int]peer.ID),
		pieceBuffers:    make(map[int][][]byte),
		completedPieces: 0,
		pieceTimers:     make(map[int]*time.Timer),
		retryCounts:     make(map[int]int),
	}
	c.downloadsMux.Lock()
	c.activeDownloads[cidStr] = state
	c.downloadsMux.Unlock()

	// Parallel downloads
	var wg sync.WaitGroup

	// First, request trust scores from all providers
	fmt.Println("Requesting trust scores from providers...")
	err = c.requestTrustScoresFromProviders(providers, relayAddrStr)
	if err != nil {
		log.Printf("Warning: Failed to get some trust scores: %v", err)
	}

	// Calculate weighted piece distribution based on trust scores
	pieceDistribution := c.calculateWeightedPieceDistribution(providers, state.TotalPieces)

	fmt.Printf("Trust-based piece distribution:\n")
	for i, dist := range pieceDistribution {
		if i < len(providers) {
			peerID := providers[i].ID.String()[:8]
			c.trustScoreMux.RLock()
			trustScore := c.providerTrustScores[providers[i].ID.String()]
			c.trustScoreMux.RUnlock()
			fmt.Printf("  Peer %s (trust: %.3f): pieces %d-%d (%d pieces)\n",
				peerID, trustScore, dist.Start, dist.End-1, dist.End-dist.Start)
		}
	}

	peersToUse := providers
	if len(peersToUse) > MaxParallelDownloads {
		peersToUse = peersToUse[:MaxParallelDownloads]
		pieceDistribution = pieceDistribution[:MaxParallelDownloads]
	}

	for i, p := range peersToUse {
		dist := pieceDistribution[i]
		start := dist.Start
		end := dist.End

		wg.Add(1)
		go func(peerInfo peer.AddrInfo, startPiece, endPiece int) {
			defer wg.Done()
			var peerConn *webRTC.SimpleWebRTCPeer
			if peerInfo.ID.String() == firstPeer.GetSignalingStream().Conn().RemotePeer().String() {
				peerConn = firstPeer
			} else {
				var connErr error
				peerConn, connErr = c.initiateWebRTCConnectionWithRetry(peerInfo.ID, 2)
				if connErr != nil {
					log.Printf("Chunk peer connect failed: %v", connErr)
					return
				}
				defer peerConn.Close()
			}
			c.downloadChunksFromPeer(peerConn, state, startPiece, endPiece)
		}(p, start, end)
	}

	<-state.Completed
	localFile.Close()

	if err := os.Rename(downloadPath, finalPath); err != nil {
		return fmt.Errorf("failed to rename file: %w", err)
	}

	fmt.Printf("\nDownload complete. File saved as %s\n", finalPath)
	return nil
}

func (c *Client) downloadChunksFromPeer(peer *webRTC.SimpleWebRTCPeer, state *DownloadState, startPiece, endPiece int) {
	peerID := peer.GetSignalingStream().Conn().RemotePeer().String()

	// Initialize transaction tracker for this peer
	c.trackerMux.Lock()
	c.transactionTrackers[peerID] = &TransactionTracker{
		PeerID:             peerID,
		StartTime:          time.Now(),
		ChunksReceived:     0,
		SuccessfulChunks:   0,
		TotalDataReceived:  0,
		AverageUploadSpeed: 0,
		TransferSuccessful: true, // Will be set to false if any piece fails
		ChunkSpeeds:        make([]int, 0),
		ChunkStartTimes:    make([]time.Time, 0),
	}
	c.trackerMux.Unlock()

	for i := startPiece; i < endPiece; i++ {
		state.mu.Lock()
		if state.PieceStatus[i] {
			state.mu.Unlock()
			continue
		}
		state.PieceAssignees[i] = peer.GetSignalingStream().Conn().RemotePeer()
		state.mu.Unlock()

		req := controlMessage{
			Command: "REQUEST_PIECE",
			CID:     state.Manifest.CID,
			Index:   int64(i),
		}

		// New: Add piece timeout
		state.mu.Lock()
		state.pieceTimers[i] = time.AfterFunc(PieceTimeout, func() {
			log.Printf("Piece %d timed out, re-requesting...", i)
			c.reRequestPiece(state, i)
		})
		state.mu.Unlock()

		if err := peer.SendJSONReliable(req); err != nil {
			log.Printf("Failed to request piece %d from %s: %v", i, peer.GetSignalingStream().Conn().RemotePeer(), err)
			// Mark transaction as failed
			c.trackerMux.Lock()
			if tracker, exists := c.transactionTrackers[peerID]; exists {
				tracker.TransferSuccessful = false
			}
			c.trackerMux.Unlock()
			return
		}
	}
}

func (c *Client) reRequestPiece(state *DownloadState, pieceIndex int) {
	// Re-assign to another peer with backoff
	retryCount := state.retryCounts[pieceIndex]
	backoff := ExponentialBackoffBase * time.Duration(1<<retryCount)
	if backoff > MaxBackoff {
		backoff = MaxBackoff
	}
	time.AfterFunc(backoff, func() {
		// For simplicity, we'll just re-request from any connected peer.
		// A more advanced implementation would select a new peer.
		for _, p := range c.webRTCPeers {
			req := controlMessage{
				Command: "REQUEST_PIECE",
				CID:     state.Manifest.CID,
				Index:   int64(pieceIndex),
			}
			if err := p.SendJSONReliable(req); err == nil {
				log.Printf("Re-requested piece %d from a different peer.", pieceIndex)
				return
			}
		}
		log.Printf("Failed to re-request piece %d: no available peers.", pieceIndex)
	})
	state.retryCounts[pieceIndex]++
}

func (c *Client) requestManifest(peer *webRTC.SimpleWebRTCPeer, cidStr string) (controlMessage, error) {
	req := controlMessage{Command: "REQUEST_MANIFEST", CID: cidStr}
	if err := peer.SendJSONReliable(req); err != nil {
		return controlMessage{}, err
	}

	manifestCh := make(chan controlMessage, 1)
	manifestChMu.Lock()
	manifestWaiters[cidStr] = manifestCh
	manifestChMu.Unlock()

	defer func() {
		manifestChMu.Lock()
		delete(manifestWaiters, cidStr)
		manifestChMu.Unlock()
	}()

	select {
	case manifest := <-manifestCh:
		return manifest, nil
	case <-time.After(30 * time.Second):
		return controlMessage{}, fmt.Errorf("timed out waiting for manifest")
	}
}

func (c *Client) initiateWebRTCConnectionWithRetry(targetPeerID peer.ID, maxRetries int) (*webRTC.SimpleWebRTCPeer, error) {

	// First, test ICE connectivity
	log.Printf("Testing ICE connectivity before attempting WebRTC connection...")
	if err := webRTC.TestICEConnectivity(); err != nil {
		log.Printf("ICE connectivity test failed: %v", err)
		log.Printf("Warning: WebRTC connections may fail due to network restrictions")
	} else {
		log.Printf("ICE connectivity test passed")
	}

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			fmt.Printf("Retrying in %v (attempt %d/%d)...\n", backoff, attempt, maxRetries)
			time.Sleep(backoff)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		info := peer.AddrInfo{ID: targetPeerID}
		if pinfo, err := c.dht.FindPeer(ctx, targetPeerID); err == nil && len(pinfo.Addrs) > 0 {
			info = pinfo
		} else {

			lastErr = fmt.Errorf("dht lookup failed: %w", err)
			continue
		}

		if len(info.Addrs) == 0 {
			lastErr = fmt.Errorf("peer %s has no known multiaddrs", targetPeerID)
			continue
		}

		c.host.Peerstore().AddAddrs(info.ID, info.Addrs, time.Hour)
		fmt.Println(c.host.Peerstore())

		if c.host.Network().Connectedness(info.ID) != network.Connected {
			connectCtx, connectCancel := context.WithTimeout(context.Background(), 20*time.Second)
			err := c.host.Connect(connectCtx, info)
			connectCancel()
			if err != nil {
				log.Printf("failed to connect to peer %s: %v", info.ID, err)
				fmt.Printf("DHT lookup failed: %v. This could be a network issue now trying connection using relays.\n", err)
				return nil, err
			}

			fmt.Printf("Successfully connected to peer %s\n", info.ID)
			// Shorter stabilization time for local network

			time.Sleep(1 * time.Second)
		}

		webrtcPeer, err := webRTC.NewSimpleWebRTCPeer(c.onDataChannelMessage, c.onWebRTCPeerClose)
		if err != nil {
			lastErr = err
			return nil, err
			// continue
		}

		offer, err := webrtcPeer.CreateOffer()
		if err != nil {
			webrtcPeer.Close()
			lastErr = err
			return nil, err
		}

		streamCtx, streamCancel := context.WithTimeout(context.Background(), 30*time.Second)
		s, err := c.host.NewStream(streamCtx, targetPeerID, p2p.SignalingProtocolID)
		streamCancel()
		if err != nil {
			webrtcPeer.Close()
			lastErr = err
			return nil, err

		}

		webrtcPeer.SetSignalingStream(s)
		encoder := json.NewEncoder(s)
		decoder := json.NewDecoder(s)

		offerMsg := map[string]string{"type": "offer", "data": offer}
		if err := encoder.Encode(offerMsg); err != nil {
			webrtcPeer.Close()
			lastErr = err
			continue
		}

		var answerMsg map[string]string
		if err := decoder.Decode(&answerMsg); err != nil {
			webrtcPeer.Close()
			lastErr = fmt.Errorf("failed to decode answer: %w", err)
			continue
		}

		if answerMsg["type"] == "error" {
			webrtcPeer.Close()
			lastErr = fmt.Errorf("peer returned error: %s", answerMsg["data"])
			continue
		}
		if answerMsg["type"] != "answer" {
			webrtcPeer.Close()
			lastErr = fmt.Errorf("expected answer, got: %s", answerMsg["type"])
			continue
		}

		if err := webrtcPeer.HandleAnswer(answerMsg["data"]); err != nil {
			webrtcPeer.Close()
			lastErr = err
			continue
		}

		if err := webrtcPeer.WaitForConnection(90 * time.Second); err != nil {
			webrtcPeer.Close()
			lastErr = fmt.Errorf("WebRTC connection failed: %w", err)
			continue
		}

		// MODIFIED: Add block to wait for the data channels to be ready
		if err := webrtcPeer.WaitForDataChannels(10 * time.Second); err != nil {
			webrtcPeer.Close()
			lastErr = fmt.Errorf("data channels did not open in time: %w", err)
			continue
		}

		fmt.Printf("WebRTC connection established with %s\n", targetPeerID)
		c.peersMux.Lock()
		c.webRTCPeers[targetPeerID] = webrtcPeer
		c.peersMux.Unlock()

		return webrtcPeer, nil
	}
	return nil, lastErr
}

func (c *Client) handleWebRTCOffer(offer, remotePeerID string, s network.Stream) (string, error) {
	peerID, err := peer.Decode(remotePeerID)
	if err != nil {
		return "", fmt.Errorf("invalid peer ID: %w", err)
	}

	webrtcPeer, err := webRTC.NewSimpleWebRTCPeer(c.onDataChannelMessage, c.onWebRTCPeerClose)
	if err != nil {
		return "", err
	}

	webrtcPeer.SetSignalingStream(s)

	answer, err := webrtcPeer.HandleOffer(offer)
	if err != nil {
		webrtcPeer.Close()
		return "", err
	}

	c.peersMux.Lock()
	c.webRTCPeers[peerID] = webrtcPeer
	c.peersMux.Unlock()

	return answer, nil
}

func (c *Client) onDataChannelMessage(msg webrtc.DataChannelMessage, peer *webRTC.SimpleWebRTCPeer) {
	if !msg.IsString {
		log.Printf("Received unexpected binary message, expecting JSON.")
		return
	}
	// Robustness: Handle empty messages that might be causing "Unknown control command: "
	if len(msg.Data) == 0 {
		return
	}
	var ctrl controlMessage
	if err := json.Unmarshal(msg.Data, &ctrl); err != nil {
		var ping map[string]string
		if err2 := json.Unmarshal(msg.Data, &ping); err2 == nil {
			if ping["type"] == "ping" {
				// Respond to ping
				pong := map[string]string{"type": "pong"}
				peer.SendJSONReliable(pong)
				return
			} else if ping["type"] == "pong" {
				c.handlePong(peer.GetSignalingStream().Conn().RemotePeer())
				return
			}
		}
		log.Printf("Failed to unmarshal control message: %v. Raw message: %s", err, string(msg.Data))
		return
	}
	c.handleControlMessage(ctrl, peer)
}

func (c *Client) handleControlMessage(ctrl controlMessage, peer *webRTC.SimpleWebRTCPeer) {
	ctx := context.Background()
	switch ctrl.Command {
	case "REQUEST_MANIFEST":
		c.handleManifestRequest(ctx, ctrl, peer)
	case "MANIFEST":
		manifestChMu.Lock()
		if ch, ok := manifestWaiters[ctrl.CID]; ok {
			ch <- ctrl
		}
		manifestChMu.Unlock()
	case "REQUEST_PIECE":
		go c.handlePieceRequest(ctx, ctrl, peer)
	case "PIECE_CHUNK":
		c.handlePieceChunk(ctrl, peer)
	case "CHUNK_ACK":
		c.handleChunkAck(ctrl)
	case "REQUEST_TRUST_SCORE":
		c.handleTrustScoreRequest(ctx, ctrl, peer)
	case "TRUST_SCORE_RESPONSE":
		c.handleTrustScoreResponse(ctrl, peer)
	case "UPDATE_TRUST_SCORE":
		c.handleTrustScoreUpdate(ctx, ctrl, peer)
	default:
		// log.Printf("Unknown control command: %s", ctrl.Command)
	}
}

func (c *Client) handlePieceChunk(ctrl controlMessage, peer *webRTC.SimpleWebRTCPeer) {
	c.downloadsMux.RLock()
	state, ok := c.activeDownloads[ctrl.CID]
	c.downloadsMux.RUnlock()
	if !ok {
		return
	}

	// Send an ACK back to the sender using reliable channel
	ackMsg := controlMessage{
		Command:  "CHUNK_ACK",
		CID:      ctrl.CID,
		Index:    ctrl.Index,
		Sequence: ctrl.Sequence,
	}
	if err := peer.SendJSONReliable(ackMsg); err != nil {
		log.Printf("Failed to send ACK for chunk %d of piece %d: %v", ctrl.Sequence, ctrl.Index, err)
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	if state.PieceStatus[ctrl.Index] {
		return // Already have this piece
	}

	if state.pieceBuffers[int(ctrl.Index)] == nil {
		state.pieceBuffers[int(ctrl.Index)] = make([][]byte, ctrl.TotalChunks)
	}

	chunkData, err := hex.DecodeString(ctrl.Payload)
	if err != nil {
		log.Printf("Failed to decode chunk payload: %v", err)
		return
	}

	state.pieceBuffers[int(ctrl.Index)][ctrl.ChunkIndex] = chunkData
	_ = state.Progress.Add(len(chunkData))

	// Track chunk metrics
	peerID := peer.GetSignalingStream().Conn().RemotePeer().String()
	c.trackerMux.Lock()
	if tracker, exists := c.transactionTrackers[peerID]; exists {
		// Record chunk start time (this is an approximation since we don't track request time per chunk)
		chunkStartTime := time.Now()
		tracker.ChunkStartTimes = append(tracker.ChunkStartTimes, chunkStartTime)

		tracker.ChunksReceived++
		tracker.SuccessfulChunks++
		tracker.TotalDataReceived += int64(len(chunkData))

		// Calculate current chunk speed (bytes/second for this chunk)
		// Since we don't track exact request time, use elapsed time since tracker start
		elapsed := time.Since(tracker.StartTime).Seconds()
		if elapsed > 0 && tracker.ChunksReceived > 0 {
			// Calculate average speed for all chunks so far
			avgChunkSpeed := int(float64(tracker.TotalDataReceived) / elapsed)
			tracker.ChunkSpeeds = append(tracker.ChunkSpeeds, avgChunkSpeed)

			// Update moving average
			if tracker.AverageUploadSpeed == 0 {
				tracker.AverageUploadSpeed = avgChunkSpeed
			} else {
				// Moving average
				tracker.AverageUploadSpeed = (tracker.AverageUploadSpeed + avgChunkSpeed) / 2
			}
		}
	}
	c.trackerMux.Unlock()

	// Check if piece is complete
	isComplete := true
	var pieceSize int
	for _, chunk := range state.pieceBuffers[int(ctrl.Index)] {
		if chunk == nil {
			isComplete = false
			break
		}
		pieceSize += len(chunk)
	}

	if isComplete {
		// Stop the timer for this piece
		if timer, ok := state.pieceTimers[int(ctrl.Index)]; ok {
			timer.Stop()
			delete(state.pieceTimers, int(ctrl.Index))
		}

		// Reassemble and write piece
		pieceData := make([]byte, 0, pieceSize)
		for _, chunk := range state.pieceBuffers[int(ctrl.Index)] {
			pieceData = append(pieceData, chunk...)
		}

		// Verify piece hash
		h := sha256.New()
		h.Write(pieceData)
		hash := hex.EncodeToString(h.Sum(nil))

		if hash != state.Pieces[ctrl.Index].Hash {
			log.Printf("Piece %d hash mismatch", ctrl.Index)
			state.pieceBuffers[int(ctrl.Index)] = nil // Clear buffer to retry
			return
		}

		if _, err := state.File.WriteAt(pieceData, state.Pieces[ctrl.Index].Offset); err != nil {
			log.Printf("Failed to write piece %d to file: %v", ctrl.Index, err)
			return
		}

		state.PieceStatus[ctrl.Index] = true
		state.completedPieces++
		delete(state.pieceBuffers, int(ctrl.Index))

		// Send trust score update immediately while WebRTC connection is still active
		c.sendTrustScoreUpdate(peer, int(ctrl.Index), state)

		if state.completedPieces == state.TotalPieces {
			state.Completed <- true
		}
	}
}

func (c *Client) handleChunkAck(ctrl controlMessage) {
	c.unackedChunksMux.Lock()
	defer c.unackedChunksMux.Unlock()
	if _, ok := c.unackedChunks[ctrl.CID]; ok {
		if _, ok := c.unackedChunks[ctrl.CID][ctrl.Index]; ok {
			delete(c.unackedChunks[ctrl.CID][ctrl.Index], ctrl.Sequence)
			if len(c.unackedChunks[ctrl.CID][ctrl.Index]) == 0 {
				delete(c.unackedChunks[ctrl.CID], ctrl.Index)
			}
		}
		if len(c.unackedChunks[ctrl.CID]) == 0 {
			delete(c.unackedChunks, ctrl.CID)
		}
	}
}

func (c *Client) handlePieceRequest(ctx context.Context, ctrl controlMessage, peer *webRTC.SimpleWebRTCPeer) {
	pieces, err := c.db.GetPieces(ctx, ctrl.CID)
	if err != nil || int(ctrl.Index) >= len(pieces) {
		log.Printf("Invalid piece request for CID %s, index %d", ctrl.CID, ctrl.Index)
		return
	}

	fileInfo, err := c.db.GetLocalFileByCID(ctx, ctrl.CID)
	if err != nil {
		log.Printf("File not found for piece request: %s", ctrl.CID)
		return
	}

	file, err := os.Open(fileInfo.FilePath)
	if err != nil {
		log.Printf("Failed to open file for piece request: %v", err)
		return
	}
	defer file.Close()

	piece := pieces[ctrl.Index]
	pieceBuffer := make([]byte, piece.Size)
	_, err = file.ReadAt(pieceBuffer, piece.Offset)
	if err != nil {
		log.Printf("Failed to read piece %d: %v", ctrl.Index, err)
		return
	}

	totalChunks := (len(pieceBuffer) + MaxChunk - 1) / MaxChunk
	for i := 0; i < totalChunks; i++ {
		start := i * MaxChunk
		end := start + MaxChunk
		if end > len(pieceBuffer) {
			end = len(pieceBuffer)
		}
		chunk := pieceBuffer[start:end]

		chunkMsg := controlMessage{
			Command:     "PIECE_CHUNK",
			CID:         ctrl.CID,
			Index:       ctrl.Index,
			ChunkIndex:  i,
			TotalChunks: totalChunks,
			Payload:     hex.EncodeToString(chunk),
			Sequence:    i,
		}

		// Store the sent chunk and start a retransmission timer
		c.unackedChunksMux.Lock()
		if c.unackedChunks[ctrl.CID] == nil {
			c.unackedChunks[ctrl.CID] = make(map[int64]map[int]controlMessage)
		}
		if c.unackedChunks[ctrl.CID][ctrl.Index] == nil {
			c.unackedChunks[ctrl.CID][ctrl.Index] = make(map[int]controlMessage)
		}
		c.unackedChunks[ctrl.CID][ctrl.Index][i] = chunkMsg
		c.unackedChunksMux.Unlock()
		time.AfterFunc(RetransmissionTimeout, func() { c.retransmitChunk(peer, chunkMsg) })

		if err := peer.SendJSON(chunkMsg); err != nil {
			log.Printf("Failed to send chunk %d of piece %d: %v", i, ctrl.Index, err)
			return
		}
		delay := c.congestionCtrl[peer.GetSignalingStream().Conn().RemotePeer()]
		time.Sleep(delay)
	}
}

func (c *Client) retransmitChunk(peer *webRTC.SimpleWebRTCPeer, chunkMsg controlMessage) {
	c.unackedChunksMux.RLock()
	defer c.unackedChunksMux.RUnlock()
	if _, ok := c.unackedChunks[chunkMsg.CID]; ok {
		if _, ok := c.unackedChunks[chunkMsg.CID][chunkMsg.Index]; ok {
			if _, ok := c.unackedChunks[chunkMsg.CID][chunkMsg.Index][chunkMsg.Sequence]; ok {
				log.Printf("Retransmitting chunk %d of piece %d", chunkMsg.Sequence, chunkMsg.Index)
				if err := peer.SendJSON(chunkMsg); err != nil {
					log.Printf("Failed to retransmit chunk %d of piece %d: %v", chunkMsg.Sequence, chunkMsg.Index, err)
				}
				// Reset timer
				time.AfterFunc(RetransmissionTimeout, func() { c.retransmitChunk(peer, chunkMsg) })
			}
		}
	}
}

func (c *Client) handleManifestRequest(ctx context.Context, ctrl controlMessage, peer *webRTC.SimpleWebRTCPeer) {
	localFile, err := c.db.GetLocalFileByCID(ctx, ctrl.CID)
	if err != nil {
		log.Printf("File not found for manifest: %s", ctrl.CID)
		return
	}

	pieces, err := c.db.GetPieces(ctx, ctrl.CID)
	if err != nil {
		log.Printf("Error getting pieces: %v", err)
		return
	}

	manifest := controlMessage{
		Command:   "MANIFEST",
		CID:       ctrl.CID,
		TotalSize: localFile.FileSize,
		HashHex:   localFile.FileHash,
		NumPieces: int64(len(pieces)),
		Pieces:    pieces,
		Filename:  localFile.Filename,
		// From:      string(c.host.ID()),
		// Target:    ctrl.From,
	}

	if err := peer.SendJSONReliable(manifest); err != nil {
		log.Printf("Error sending manifest: %v", err)
	}
}

// MODIFIED: Added a log message for better debugging
func (c *Client) onWebRTCPeerClose(peerID peer.ID) {
	log.Printf("WebRTC peer disconnected: %s", peerID)
	c.peersMux.Lock()
	delete(c.webRTCPeers, peerID)
	c.peersMux.Unlock()

	// Handle download resumption logic
	c.downloadsMux.Lock()
	defer c.downloadsMux.Unlock()

	for cid, state := range c.activeDownloads {
		for pieceIndex, assignee := range state.PieceAssignees {
			if assignee == peerID {
				log.Printf("Peer %s disconnected, re-requesting piece %d for download %s", peerID, pieceIndex, cid)
				// Re-queue the piece for download
				go c.reRequestPiece(state, pieceIndex)
			}
		}
	}
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// New: monitorCongestion
func (c *Client) monitorCongestion() {
	ticker := time.NewTicker(PingInterval)
	for range ticker.C {
		c.peersMux.RLock()
		for pid, peer := range c.webRTCPeers {
			if connState := peer.GetConnectionState(); connState == webRTC.ConnectionStateConnected {
				c.pingTimes[pid] = time.Now()
				ping := map[string]string{"type": "ping"}
				peer.SendJSONReliable(ping)
			}
		}
		c.peersMux.RUnlock()
	}
}

func (c *Client) handlePong(pid peer.ID) {
	if start, ok := c.pingTimes[pid]; ok {
		rtt := time.Since(start)
		c.rttMux.Lock()
		if _, ok := c.rttMeasurements[pid]; !ok {
			c.rttMeasurements[pid] = []time.Duration{}
		}
		c.rttMeasurements[pid] = append(c.rttMeasurements[pid], rtt)
		if len(c.rttMeasurements[pid]) > 10 {
			c.rttMeasurements[pid] = c.rttMeasurements[pid][1:]
		}
		avgRTT := time.Duration(0)
		for _, d := range c.rttMeasurements[pid] {
			avgRTT += d
		}
		avgRTT /= time.Duration(len(c.rttMeasurements[pid]))
		var delay time.Duration = MinDelay
		if avgRTT > MaxRTT {
			delay = MaxDelay
		} else if avgRTT > MaxRTT/2 {
			delay = (MaxDelay - MinDelay) / 2
		}
		c.congestionCtrl[pid] = delay
		c.rttMux.Unlock()
		delete(c.pingTimes, pid)
	}
}

// handleTrustScoreRequest handles a request for trust score with decay applied
func (c *Client) handleTrustScoreRequest(ctx context.Context, ctrl controlMessage, peer *webRTC.SimpleWebRTCPeer) {
	// Get current trust score data for MYSELF (the seeder), not the requesting peer
	myPeerID := c.host.ID().String()
	peerData, err := c.db.GetPeerTrustScore(ctx, myPeerID)
	if err != nil {
		log.Printf("Error getting my own trust score: %v", err)
		return
	}

	// Update my last seen time since I'm actively responding (no decay needed for active peers)
	peerData.LastSeen = time.Now()
	err = c.db.UpdatePeerTrustScore(ctx, peerData)
	if err != nil {
		log.Printf("Error updating my last seen time: %v", err)
	}

	// Send back MY current trust score (no decay applied since I'm active)
	response := controlMessage{
		Command:    "TRUST_SCORE_RESPONSE",
		PeerID:     myPeerID, // Send my own peer ID
		TrustScore: peerData.CurrentTrustScore,
	}

	log.Printf("Sending my trust score %.3f to requesting peer %s", peerData.CurrentTrustScore, peer.GetSignalingStream().Conn().RemotePeer().String()[:8])
	peer.SendJSONReliable(response)
}

// handleTrustScoreResponse handles a trust score response
func (c *Client) handleTrustScoreResponse(ctrl controlMessage, peer *webRTC.SimpleWebRTCPeer) {
	// Store the received trust score for provider prioritization
	c.trustScoreMux.Lock()
	if c.providerTrustScores == nil {
		c.providerTrustScores = make(map[string]float64)
	}
	c.providerTrustScores[ctrl.PeerID] = ctrl.TrustScore
	c.trustScoreMux.Unlock()

	log.Printf("Received trust score %.3f for peer %s", ctrl.TrustScore, ctrl.PeerID[:8])
}

// handleTrustScoreUpdate processes trust score update from leecher and calculates new trust score
func (c *Client) handleTrustScoreUpdate(ctx context.Context, ctrl controlMessage, peer *webRTC.SimpleWebRTCPeer) {
	if ctrl.TrustScoreUpdate == nil {
		log.Printf("Received trust score update with no parameters")
		return
	}

	params := ctrl.TrustScoreUpdate
	log.Printf("Received trust score update from leecher %s: %d chunks, %.2f MB, transfer success: %v",
		params.LeecherPeerID[:8], params.ChunksReceived,
		float64(params.TotalDataReceived)/(1024*1024), params.TransferSuccessful)

	// Get current peer trust data for the SEEDER (myself) from seeder's database
	peerData, err := c.db.GetPeerTrustScore(ctx, c.host.ID().String())
	if err != nil {
		log.Printf("Error getting seeder's own trust data for update: %v", err)
		return
	}

	// Update metrics based on specifications:

	// #9: Add 1 to total transfers in seeder DB
	peerData.TotalTransfers++

	// #8: If all chunks were shared successfully, add 1 to successful transfers
	if params.TransferSuccessful {
		peerData.SuccessfulTransfers++
	}

	// #2: Since we're the seeder, when leecher downloads from us, we increment our SeededData
	peerData.SeededData += float64(params.TotalDataReceived) / (1024 * 1024) // Convert to MB

	// #3: We don't update LeechedData since we're seeding, not leeching

	// #11: Number of successful chunks shared (same as #10)
	// #10: Add number of chunks just shared
	log.Printf("Debug: Before update - NoOfSuccChunksLastK=%d, adding %d",
		peerData.NoOfSuccChunksLastK, params.SuccessfulChunks)
	peerData.NoOfSuccChunksLastK += params.SuccessfulChunks // Accumulate successful chunks
	log.Printf("Debug: After update - NoOfSuccChunksLastK=%d", peerData.NoOfSuccChunksLastK)
	peerData.TotalFileChunksUploaded += params.ChunksReceived // Add total chunks received by leecher
	peerData.OriginalChunksShared += params.ChunksReceived    // Add chunks shared in this transaction

	// #5: Update average upload speed with 25:1 weighting
	if peerData.AverageUploadSpeed == 0 {
		peerData.AverageUploadSpeed = params.AverageChunkSpeed
	} else {
		peerData.AverageUploadSpeed = int((float64(peerData.AverageUploadSpeed)*25.0 + float64(params.AverageChunkSpeed)*1.0) / 26.0)
	}

	// #6: Set average online time to 8 hours
	peerData.AverageOnlineTimePerDay = 8

	// #4: Update last seen to present time
	peerData.LastSeen = time.Now()

	// Store old trust score before updating
	oldTrustScore := peerData.CurrentTrustScore

	// Calculate new trust score
	newTrustScore := c.db.CalculateTrustScore(peerData.CurrentTrustScore, peerData, params.TransferSuccessful)

	// Debug logging
	log.Printf("Debug: oldTrustScore=%.3f, newTrustScore=%.3f, chunks=%d, successful=%v",
		oldTrustScore, newTrustScore, params.ChunksReceived, params.TransferSuccessful)

	peerData.CurrentTrustScore = newTrustScore

	// Update in seeder's database
	err = c.db.UpdatePeerTrustScore(ctx, peerData)
	if err != nil {
		log.Printf("Error updating peer trust score in seeder DB: %v", err)
		return
	}

	log.Printf("Seeder updated its own trust score based on leecher %s feedback: %.3f -> %.3f",
		params.LeecherPeerID[:8], oldTrustScore, newTrustScore)
}

// PieceDistribution represents how pieces are distributed among peers
type PieceDistribution struct {
	Start int
	End   int
}

// requestTrustScoresFromProviders requests trust scores from all providers
func (c *Client) requestTrustScoresFromProviders(providers []peer.AddrInfo, relayAddrStr string) error {
	var wg sync.WaitGroup
	errors := make(chan error, len(providers))

	// Clear previous trust scores
	c.trustScoreMux.Lock()
	c.providerTrustScores = make(map[string]float64)
	c.trustScoreMux.Unlock()

	for _, p := range providers {
		wg.Add(1)
		go func(peerInfo peer.AddrInfo) {
			defer wg.Done()

			// Try to connect and request trust score
			peerConn, err := c.initiateWebRTCConnectionWithRetry(peerInfo.ID, 1)
			if err != nil {
				// Try relay connection
				circuitStr := fmt.Sprintf("%s/p2p-circuit/p2p/%s", relayAddrStr, peerInfo.ID.String())
				circuitMaddr, err := multiaddr.NewMultiaddr(circuitStr)
				if err != nil {
					errors <- fmt.Errorf("relay address error for peer %s: %w", peerInfo.ID.String()[:8], err)
					return
				}

				relayInfo := peer.AddrInfo{ID: peerInfo.ID, Addrs: []multiaddr.Multiaddr{circuitMaddr}}
				peerConn, err = c.initiateWebRTCConnectionWithRetry(relayInfo.ID, 1)
				if err != nil {
					errors <- fmt.Errorf("failed to connect to peer %s: %w", peerInfo.ID.String()[:8], err)
					return
				}
			}
			defer peerConn.Close()

			// Request trust score
			trustRequest := controlMessage{
				Command: "REQUEST_TRUST_SCORE",
				PeerID:  peerInfo.ID.String(),
			}

			if err := peerConn.SendJSONReliable(trustRequest); err != nil {
				errors <- fmt.Errorf("failed to send trust request to peer %s: %w", peerInfo.ID.String()[:8], err)
				return
			}

			// Wait for response (with timeout)
			timeout := time.After(10 * time.Second)
			for {
				select {
				case <-timeout:
					// Set default trust score if no response
					c.trustScoreMux.Lock()
					c.providerTrustScores[peerInfo.ID.String()] = 0.5 // Neutral score
					c.trustScoreMux.Unlock()
					return
				case <-time.After(100 * time.Millisecond):
					// Check if we got the trust score
					c.trustScoreMux.RLock()
					_, exists := c.providerTrustScores[peerInfo.ID.String()]
					c.trustScoreMux.RUnlock()
					if exists {
						return
					}
				}
			}
		}(p)
	}

	wg.Wait()
	close(errors)

	// Collect any errors (for logging)
	var errorList []error
	for err := range errors {
		errorList = append(errorList, err)
	}

	if len(errorList) > 0 {
		return fmt.Errorf("encountered %d errors while requesting trust scores", len(errorList))
	}

	return nil
}

// calculateWeightedPieceDistribution distributes pieces based on trust scores
func (c *Client) calculateWeightedPieceDistribution(providers []peer.AddrInfo, totalPieces int) []PieceDistribution {
	if len(providers) == 0 {
		return []PieceDistribution{}
	}

	// Get trust scores
	c.trustScoreMux.RLock()
	trustScores := make([]float64, len(providers))
	totalTrustScore := 0.0

	for i, p := range providers {
		score, exists := c.providerTrustScores[p.ID.String()]
		if !exists {
			score = 0.5 // Default neutral score
		}
		trustScores[i] = score
		totalTrustScore += score
	}
	c.trustScoreMux.RUnlock()

	// Handle edge case
	if totalTrustScore == 0 {
		totalTrustScore = float64(len(providers)) * 0.5
		for i := range trustScores {
			trustScores[i] = 0.5
		}
	}

	// Calculate weighted distribution
	distribution := make([]PieceDistribution, len(providers))
	currentStart := 0

	for i, score := range trustScores {
		weight := score / totalTrustScore
		piecesForPeer := int(float64(totalPieces) * weight)

		// Ensure at least 1 piece per peer (if possible)
		if piecesForPeer == 0 && currentStart < totalPieces {
			piecesForPeer = 1
		}

		// Handle last peer getting remaining pieces
		if i == len(providers)-1 {
			piecesForPeer = totalPieces - currentStart
		}

		distribution[i] = PieceDistribution{
			Start: currentStart,
			End:   currentStart + piecesForPeer,
		}

		currentStart += piecesForPeer
	}

	return distribution
}

// finalizePeerTransaction sends trust score parameters to seeder for calculation
func (c *Client) finalizePeerTransaction(peerID string) {
	c.trackerMux.Lock()
	tracker, exists := c.transactionTrackers[peerID]
	if !exists {
		c.trackerMux.Unlock()
		return
	}
	// Remove from active trackers
	delete(c.transactionTrackers, peerID)
	c.trackerMux.Unlock()

	log.Printf("Finalizing transaction with peer %s: %d chunks, %.2f MB, %d KB/s",
		peerID[:8], tracker.ChunksReceived, float64(tracker.TotalDataReceived)/(1024*1024),
		tracker.AverageUploadSpeed/1024)

	// Create trust score update parameters
	trustParams := &TrustScoreUpdateParams{
		LeecherPeerID:      c.host.ID().String(),
		ChunksReceived:     tracker.ChunksReceived,
		SuccessfulChunks:   tracker.SuccessfulChunks,
		TotalDataReceived:  tracker.TotalDataReceived,
		AverageChunkSpeed:  tracker.AverageUploadSpeed, // TODO: Calculate proper chunk speed average
		TransferSuccessful: tracker.TransferSuccessful,
	}

	// Send trust score update request to the seeder (peer)
	c.sendTrustScoreUpdateToSeeder(peerID, trustParams)

	log.Printf("Sent trust score update parameters to seeder %s", peerID[:8])
}

// checkMyTrustScore displays the current user's trust score and metrics
func (c *Client) checkMyTrustScore() {
	ctx := context.Background()
	myPeerID := c.host.ID().String()

	// Get our own trust metrics
	peerData, err := c.db.GetPeerTrustScore(ctx, myPeerID)
	if err != nil {
		fmt.Printf("Error retrieving trust score: %v\n", err)
		return
	}

	fmt.Println("\n=== Your Trust Score & Metrics ===")
	fmt.Printf("Peer ID: %s\n", myPeerID[:16]+"...")
	fmt.Printf("Current Trust Score: %.3f/1.000\n", peerData.CurrentTrustScore)

	// Trust score rating
	var rating string
	switch {
	case peerData.CurrentTrustScore >= 0.8:
		rating = "Excellent"
	case peerData.CurrentTrustScore >= 0.6:
		rating = "Good"
	case peerData.CurrentTrustScore >= 0.4:
		rating = "Average"
	case peerData.CurrentTrustScore >= 0.2:
		rating = "Poor"
	default:
		rating = "Very Poor ❌"
	}
	fmt.Printf("Rating: %s\n\n", rating)

	// Transfer statistics
	fmt.Println("=== Transfer Statistics ===")
	fmt.Printf("Total Transfers: %d\n", peerData.TotalTransfers)
	fmt.Printf("Successful Transfers: %d\n", peerData.SuccessfulTransfers)
	if peerData.TotalTransfers > 0 {
		successRate := float64(peerData.SuccessfulTransfers) / float64(peerData.TotalTransfers) * 100
		fmt.Printf("Success Rate: %.1f%%\n", successRate)
	} else {
		fmt.Printf("Success Rate: N/A (no transfers yet)\n")
	}

	// Data statistics
	fmt.Println("\n=== Data Statistics ===")
	fmt.Printf("Data Seeded: %.2f MB\n", peerData.SeededData)
	fmt.Printf("Data Leeched: %.2f MB\n", peerData.LeechedData)
	if peerData.SeededData > 0 {
		ratioLeechSeed := peerData.LeechedData / peerData.SeededData
		fmt.Printf("Leech/Seed Ratio: %.2f\n", ratioLeechSeed)
		if ratioLeechSeed <= 1.0 {
			fmt.Printf("Sharing Behavior: Good Seeder 📤\n")
		} else if ratioLeechSeed <= 2.0 {
			fmt.Printf("Sharing Behavior: Moderate 🔄\n")
		} else {
			fmt.Printf("Sharing Behavior: Heavy Leecher 📥\n")
		}
	} else {
		fmt.Printf("Leech/Seed Ratio: N/A (no seeding yet)\n")
		fmt.Printf("Sharing Behavior: New User 🆕\n")
	}

	// Performance metrics
	fmt.Println("\n=== Performance Metrics ===")
	fmt.Printf("Average Upload Speed: %d KB/s\n", peerData.AverageUploadSpeed/1024)
	fmt.Printf("Recent Successful Chunks: %d\n", peerData.NoOfSuccChunksLastK)
	fmt.Printf("Total Chunks Uploaded: %d\n", peerData.TotalFileChunksUploaded)
	fmt.Printf("Average Online Time: %d hours/day\n", peerData.AverageOnlineTimePerDay)

	// Activity information
	fmt.Println("\n=== Activity Information ===")
	fmt.Printf("Last Seen: %s\n", peerData.LastSeen.Format("2006-01-02 15:04:05"))
	fmt.Printf("Last Updated: %s\n", peerData.SeenAt.Format("2006-01-02 15:04:05"))

	// Tips for improvement
	if peerData.CurrentTrustScore < 0.6 {
		fmt.Println("\n=== Tips to Improve Trust Score ===")
		if peerData.TotalTransfers == 0 {
			fmt.Println("• Start sharing files to build your reputation")
		}
		if peerData.SuccessfulTransfers < peerData.TotalTransfers {
			fmt.Println("• Ensure stable internet connection for reliable transfers")
		}
		if peerData.SeededData == 0 {
			fmt.Println("• Share files with others to improve your seed/leech ratio")
		}
		if peerData.AverageUploadSpeed < 1024 {
			fmt.Println("• Improve your upload speed for better performance rating")
		}
	}

	fmt.Println()
}

// sendTrustScoreUpdateToSeeder sends trust score parameters to the seeder
func (c *Client) sendTrustScoreUpdateToSeeder(seederPeerID string, params *TrustScoreUpdateParams) {
	// First try WebRTC connection
	c.peersMux.RLock()
	var seederPeer *webRTC.SimpleWebRTCPeer
	for pID, peer := range c.webRTCPeers {
		if pID.String() == seederPeerID {
			seederPeer = peer
			break
		}
	}
	c.peersMux.RUnlock()

	// Create control message with trust score update
	updateMsg := controlMessage{
		Command:          "UPDATE_TRUST_SCORE",
		TrustScoreUpdate: params,
	}

	// Try WebRTC first
	if seederPeer != nil {
		err := seederPeer.SendJSONReliable(updateMsg)
		if err == nil {
			log.Printf("Successfully sent trust score update to seeder %s via WebRTC", seederPeerID[:8])
			return
		}
		log.Printf("WebRTC failed for trust score update to %s: %v", seederPeerID[:8], err)
	}

	// Fallback to libp2p stream
	err := c.sendTrustScoreViaLibp2p(seederPeerID, updateMsg)
	if err != nil {
		log.Printf("Failed to send trust score update to seeder %s via libp2p: %v", seederPeerID[:8], err)
		return
	}

	log.Printf("Successfully sent trust score update to seeder %s via libp2p", seederPeerID[:8])
}

// sendTrustScoreViaLibp2p sends trust score update via libp2p stream
func (c *Client) sendTrustScoreViaLibp2p(seederPeerID string, updateMsg controlMessage) error {
	// Convert string to peer.ID
	peerID, err := peer.Decode(seederPeerID)
	if err != nil {
		return fmt.Errorf("invalid peer ID: %w", err)
	}

	// Open a stream to the peer using a dedicated trust score protocol
	stream, err := c.host.NewStream(context.Background(), peerID, "/torrentium/trust-score/1.0.0")
	if err != nil {
		return fmt.Errorf("failed to open stream: %w", err)
	}
	defer stream.Close()

	// Send the trust score update message
	encoder := json.NewEncoder(stream)
	err = encoder.Encode(updateMsg)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

// finalizeAllPeerTransactions sends trust score updates for all tracked transactions
func (c *Client) finalizeAllPeerTransactions() {
	c.trackerMux.Lock()
	defer c.trackerMux.Unlock()

	for peerID, tracker := range c.transactionTrackers {
		log.Printf("Finalizing transaction with peer %s: %d chunks, %.2f MB, %d KB/s",
			peerID[:8], tracker.ChunksReceived, float64(tracker.TotalDataReceived)/(1024*1024),
			tracker.AverageUploadSpeed/1024)

		// Calculate average chunk speed for pieces assigned to this seeder
		averageChunkSpeed := 0
		if len(tracker.ChunkSpeeds) > 0 {
			totalSpeed := 0
			for _, speed := range tracker.ChunkSpeeds {
				totalSpeed += speed
			}
			averageChunkSpeed = totalSpeed / len(tracker.ChunkSpeeds)
		}

		// Create trust score update parameters
		trustParams := &TrustScoreUpdateParams{
			LeecherPeerID:      c.host.ID().String(),
			ChunksReceived:     tracker.ChunksReceived,
			SuccessfulChunks:   tracker.SuccessfulChunks,
			TotalDataReceived:  tracker.TotalDataReceived,
			AverageChunkSpeed:  averageChunkSpeed, // Calculated average of all chunk speeds
			TransferSuccessful: tracker.TransferSuccessful,
		}

		// Send trust score update request to the seeder (peer)
		c.sendTrustScoreUpdateToSeeder(peerID, trustParams)

		log.Printf("Sent trust score update parameters to seeder %s (avg chunk speed: %d KB/s)",
			peerID[:8], averageChunkSpeed/1024)
	}

	// Clear all transaction trackers
	c.transactionTrackers = make(map[string]*TransactionTracker)
}

// registerTrustScoreProtocol sets up the trust score update protocol handler
func (c *Client) registerTrustScoreProtocol() {
	c.host.SetStreamHandler("/torrentium/trust-score/1.0.0", func(s network.Stream) {
		defer s.Close()

		log.Printf("Received trust score update from %s", s.Conn().RemotePeer())

		// Decode the incoming trust score message
		var updateMsg controlMessage
		decoder := json.NewDecoder(s)
		err := decoder.Decode(&updateMsg)
		if err != nil {
			log.Printf("Error decoding trust score message: %v", err)
			return
		}

		// Process the trust score update
		if updateMsg.Command == "UPDATE_TRUST_SCORE" {
			ctx := context.Background()
			c.handleTrustScoreUpdate(ctx, updateMsg, nil) // nil peer since this came via libp2p
		} else {
			log.Printf("Unexpected message type in trust score protocol: %s", updateMsg.Command)
		}
	})
}

// sendImmediateTrustScoreUpdate sends trust score update immediately when a piece completes
func (c *Client) sendImmediateTrustScoreUpdate(pieceIndex int64, state *DownloadState) {
	// Find which peer was assigned this piece
	state.mu.Lock()
	assignedPeer, exists := state.PieceAssignees[int(pieceIndex)]
	state.mu.Unlock()

	if !exists {
		return // No peer assigned to this piece
	}

	peerID := assignedPeer.String()

	// Get transaction tracker for this peer
	c.trackerMux.RLock()
	tracker, exists := c.transactionTrackers[peerID]
	c.trackerMux.RUnlock()

	if !exists {
		return // No tracker for this peer
	}

	// Calculate current average chunk speed for this peer
	averageChunkSpeed := 0
	if len(tracker.ChunkSpeeds) > 0 {
		totalSpeed := 0
		for _, speed := range tracker.ChunkSpeeds {
			totalSpeed += speed
		}
		averageChunkSpeed = totalSpeed / len(tracker.ChunkSpeeds)
	}

	// Create trust score update parameters for this piece
	trustParams := &TrustScoreUpdateParams{
		LeecherPeerID:      c.host.ID().String(),
		ChunksReceived:     tracker.ChunksReceived,
		SuccessfulChunks:   tracker.SuccessfulChunks,
		TotalDataReceived:  tracker.TotalDataReceived,
		AverageChunkSpeed:  averageChunkSpeed,
		TransferSuccessful: true, // This piece was successful
	}

	// Send immediately via libp2p (more reliable than waiting for WebRTC)
	updateMsg := controlMessage{
		Command:          "UPDATE_TRUST_SCORE",
		TrustScoreUpdate: trustParams,
	}

	err := c.sendTrustScoreViaLibp2p(peerID, updateMsg)
	if err != nil {
		log.Printf("Failed to send immediate trust score update for piece %d to peer %s: %v",
			pieceIndex, peerID[:8], err)
	} else {
		log.Printf("Sent immediate trust score update for piece %d to peer %s",
			pieceIndex, peerID[:8])
	}
}

// sendTrustScoreUpdate sends trust score parameters immediately while WebRTC connection is active
func (c *Client) sendTrustScoreUpdate(peer *webRTC.SimpleWebRTCPeer, pieceIndex int, state *DownloadState) {
	peerID := peer.GetSignalingStream().Conn().RemotePeer().String()

	// Get transaction tracker for this peer
	c.trackerMux.RLock()
	tracker, exists := c.transactionTrackers[peerID]
	c.trackerMux.RUnlock()

	if !exists {
		return // No tracker for this peer
	}

	// Calculate the data size for this specific piece only
	pieceSize := int64(0)
	if pieceIndex < len(state.Pieces) {
		pieceSize = state.Pieces[pieceIndex].Size
	}

	// Calculate current average chunk speed for this peer
	averageChunkSpeed := 0
	if len(tracker.ChunkSpeeds) > 0 {
		totalSpeed := 0
		for _, speed := range tracker.ChunkSpeeds {
			totalSpeed += speed
		}
		averageChunkSpeed = totalSpeed / len(tracker.ChunkSpeeds)
	}

	// Create trust score update parameters for THIS PIECE ONLY
	trustParams := &TrustScoreUpdateParams{
		LeecherPeerID:      c.host.ID().String(),
		ChunksReceived:     1,         // Just this piece
		SuccessfulChunks:   1,         // This piece was successful
		TotalDataReceived:  pieceSize, // Only this piece's data
		AverageChunkSpeed:  averageChunkSpeed,
		TransferSuccessful: true, // This piece was successful
	}

	// Send through the same WebRTC connection
	updateMsg := controlMessage{
		Command:          "UPDATE_TRUST_SCORE",
		TrustScoreUpdate: trustParams,
	}

	err := peer.SendJSONReliable(updateMsg)
	if err != nil {
		log.Printf("Failed to send trust score update for piece %d to peer %s: %v",
			pieceIndex, peerID[:8], err)
	} else {
		log.Printf("✅ Sent trust score update for piece %d to peer %s (size: %.2f MB, speed: %d KB/s)",
			pieceIndex, peerID[:8], float64(pieceSize)/(1024*1024), averageChunkSpeed/1024)

		// Update leecher's own database to record that it has leeched data
		c.updateLeecherDatabase(trustParams)
	}
} // updateLeecherDatabase updates the leecher's own database to record downloaded data
func (c *Client) updateLeecherDatabase(params *TrustScoreUpdateParams) {
	ctx := context.Background()

	// Get leecher's own trust data
	leecherData, err := c.db.GetPeerTrustScore(ctx, c.host.ID().String())
	if err != nil {
		log.Printf("Error getting leecher's own trust data: %v", err)
		return
	}

	// Update leecher's LeechedData (data downloaded from seeders)
	leecherData.LeechedData += float64(params.TotalDataReceived) / (1024 * 1024) // Convert to MB

	// Update last seen
	leecherData.LastSeen = time.Now()

	// Update in leecher's database
	err = c.db.UpdatePeerTrustScore(ctx, leecherData)
	if err != nil {
		log.Printf("Error updating leecher's own database: %v", err)
		return
	}

	log.Printf("📥 Updated leecher's own LeechedData: +%.2f MB", float64(params.TotalDataReceived)/(1024*1024))
}
