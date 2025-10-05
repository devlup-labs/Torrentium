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
		host:            h,
		dht:             d,
		webRTCPeers:     make(map[peer.ID]*webRTC.SimpleWebRTCPeer),
		sharingFiles:    make(map[string]*FileInfo),
		activeDownloads: make(map[string]*DownloadState),
		db:              repo,
		unackedChunks:   make(map[string]map[int64]map[int]controlMessage),
		congestionCtrl:  make(map[peer.ID]time.Duration),
		pingTimes:       make(map[peer.ID]time.Time),
		rttMeasurements: make(map[peer.ID][]time.Duration),
	}
	go c.monitorCongestion()
	return c
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := godotenv.Load(); err != nil {
		//log.Printf("Warning: Could not load .env file: %v", err)
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
		case "announce":
			if len(args) != 1 {
				fmt.Println("Usage: announce <cid>")
			} else {
				err = c.announceFile(args[0])
			}
		case "health":
			c.checkConnectionHealth()
		case "debug":
			c.debugNetworkStatus()
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
	peerID := c.host.ID().String()
	minWidth := len(" Your Peer ID: " + peerID) + 4 // 4 for border characters
	width := 80
	if minWidth > width {
		width = minWidth + 10 
	}
	
	// Create the box
	topBorder := "┌" + strings.Repeat("─", width-2) + "┐"
	bottomBorder := "└" + strings.Repeat("─", width-2) + "┘"
	
	// Helper function to create a centered line
	centerLine := func(text string) string {
		if len(text) >= width-4 {
			return "│ " + text[:width-6] + "... │"
		}
		padding := (width - 4 - len(text)) / 2
		leftPad := strings.Repeat(" ", padding)
		rightPad := strings.Repeat(" ", width-4-len(text)-padding)
		return "│ " + leftPad + text + rightPad + " │"
	}
	
	// Helper function to create a left-aligned line
	leftLine := func(text string) string {
		if len(text) >= width-4 {
			return "│ " + text[:width-6] + "... │"
		}
		rightPad := strings.Repeat(" ", width-4-len(text))
		return "│ " + text + rightPad + " │"
	}
	
	// Print the fancy box
	fmt.Println()
	fmt.Println(topBorder)
	fmt.Println(centerLine("DECENTRALIZED P2P FILE SHARING"))
	fmt.Println("│" + strings.Repeat("─", width-2) + "│")
	fmt.Println(centerLine("Available Commands"))
	fmt.Println("│" + strings.Repeat(" ", width-2) + "│")
	
	// Commands with descriptions
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
	
	// Network info
	peerID = c.host.ID().String()
	fmt.Println(leftLine(" Your Peer ID: " + peerID))
	
	// Show listening addresses
	addrs := c.host.Addrs()
	fmt.Println(leftLine(" Listening on:"))
	
	for i, addr := range addrs {
		if i >= 3 { // Limit to 3 addresses to fit in box
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
	relayAddrStr := "/dns4/relay-torrentium.onrender.com/tcp/443/wss/p2p/12D3KooWMbTZL5taZH4CK9hCkTLkXaPadBoMR3KJZRFhbYBPrdkK"

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

			//perform WebRTC handshake
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
	peersToUse := providers
	if len(peersToUse) > MaxParallelDownloads {
		peersToUse = peersToUse[:MaxParallelDownloads]
	}
	chunksPerPeer := state.TotalPieces / len(peersToUse)

	for i, p := range peersToUse {
		start := i * chunksPerPeer
		end := start + chunksPerPeer
		if i == len(peersToUse)-1 {
			end = state.TotalPieces
		}

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

	fmt.Printf("\n✅ Download complete. File saved as %s\n", finalPath)
	return nil
}

// func (c *Client) downloadChunksFromPeer(peer *webRTC.SimpleWebRTCPeer, state *DownloadState, startPiece, endPiece int) {
// 	for i := startPiece; i < endPiece; i++ {
// 		state.mu.Lock()
// 		if state.PieceStatus[i] {
// 			state.mu.Unlock()
// 			continue
// 		}
// 		state.PieceAssignees[i] = peer.GetSignalingStream().Conn().RemotePeer()
// 		state.mu.Unlock()

// 		req := controlMessage{
// 			Command: "REQUEST_PIECE",
// 			CID:     state.Manifest.CID,
// 			Index:   int64(i),
// 		}

// 		state.mu.Lock()   //piece timeout
// 		state.pieceTimers[i] = time.AfterFunc(PieceTimeout, func() {
// 			log.Printf("Piece %d timed out, re-requesting...", i)
// 			c.reRequestPiece(state, i)
// 		})
// 		state.mu.Unlock()

// 		if err := peer.SendJSONReliable(req); err != nil {
// 			log.Printf("Failed to request piece %d from %s: %v", i, peer.GetSignalingStream().Conn().RemotePeer(), err)
// 			return
// 		}
// 	}
// }

func (c *Client) downloadChunksFromPeer(peer *webRTC.SimpleWebRTCPeer, state *DownloadState, startPiece, endPiece int) {
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

        state.mu.Lock()
        state.pieceTimers[i] = time.AfterFunc(PieceTimeout, func() {
            log.Printf("Piece %d timed out, re-requesting...", i)
            c.reRequestPiece(state, i)
        })
        state.mu.Unlock()

        // Use reliable channel for piece requests (control messages)
        if err := peer.SendJSONReliable(req); err != nil {
            log.Printf("Failed to request piece %d from %s: %v", i, peer.GetSignalingStream().Conn().RemotePeer(), err)
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

			time.Sleep(1 * time.Second)
		}

		webrtcPeer, err := webRTC.NewSimpleWebRTCPeer(c.onDataChannelMessage, c.onWebRTCPeerClose)
		if err != nil {
			lastErr = err
			return nil, err
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

		//wait for the data channels to be ready
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
	default:
		//do nothing
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
		//Stop the timer for this piece
		if timer, ok := state.pieceTimers[int(ctrl.Index)]; ok {
			timer.Stop()
			delete(state.pieceTimers, int(ctrl.Index))
		}

		// Reassemble and write piece
		pieceData := make([]byte, 0, pieceSize)
		for _, chunk := range state.pieceBuffers[int(ctrl.Index)] {
			pieceData = append(pieceData, chunk...)
		}

		//Verify piece hash
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

	channelIndex := peer.GetFileChannelForPiece(int(ctrl.Index))

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

		if err := peer.SendOnFileChannel(channelIndex, chunkMsg); err != nil {
            log.Printf("Failed to send chunk %d of piece %d on channel %d: %v", i, ctrl.Index, channelIndex, err)
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
	}

	if err := peer.SendJSONReliable(manifest); err != nil {
		log.Printf("Error sending manifest: %v", err)
	}
}

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
