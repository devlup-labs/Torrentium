package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	webRTC "torrentium/internal/client"
	"torrentium/internal/types"

	"github.com/dustin/go-humanize"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multihash"
	"github.com/schollz/progressbar/v3"
)

func (c *Client) AddFile(filePath string) error {
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
	pieceSz := int64(types.DefaultPieceSize)
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
		if err := c.DB.UpsertPiece(ctx, fileCID.String(), idx, offset, size, ph, true); err != nil {
			return err
		}
	}

	if err := c.DB.AddLocalFile(ctx, fileCID.String(), info.Name(), info.Size(), filePath, fileHashStr); err != nil {
		return fmt.Errorf("failed to store file metadata: %w", err)
	}

	c.SharingFiles[fileCID.String()] = &types.FileInfo{
		FilePath: filePath,
		Hash:     fileHashStr,
		Size:     info.Size(),
		Name:     info.Name(),
		PieceSz:  pieceSz,
	}

	log.Printf("Announcing file %s with CID %s to DHT...", info.Name(), fileCID.String())
	provideCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if err := c.DHT.Provide(provideCtx, fileCID, true); err != nil {
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

func (c *Client) ListLocalFiles() {
	ctx := context.Background()
	files, err := c.DB.GetLocalFiles(ctx)
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

func (c *Client) SearchByText(q string) error {
	ctx := context.Background()
	matches, err := c.DB.SearchByFilename(ctx, q)
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

func (c *Client) EnhancedSearchByCID(cidStr string) error {
	fileCID, err := cid.Decode(cidStr)
	if err != nil {
		return fmt.Errorf("invalid CID: %w", err)
	}
	fmt.Printf("Searching for CID: %s\n", fileCID.String())
	providers, err := c.FindProvidersWithTimeout(fileCID, 60*time.Second, types.MaxProviders)
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
		if c.Host.Network().Connectedness(provider.ID) == network.Connected {
			fmt.Printf(" - Already connected\n")
		} else {
			fmt.Printf(" - Not connected\n")
		}
	}
	return nil
}

func (c *Client) DownloadFile(cidStr string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fileCID, err := cid.Decode(cidStr)
	if err != nil {
		return fmt.Errorf("invalid CID: %w", err)
	}

	fmt.Printf("Looking for providers of CID: %s\n", fileCID.String())
	providers, err := c.FindProvidersWithTimeout(fileCID, 60*time.Second, types.MaxProviders)
	if err != nil {
		return fmt.Errorf("provider search failed: %w", err)
	}

	if len(providers) == 0 {
		return fmt.Errorf("no providers found")
	}

	fmt.Printf("Found %d providers. Getting file manifest...\n", len(providers))
	relayAddrStr := "/dns4/relay-torrentium.onrender.com/tcp/443/wss/p2p/12D3KooWKsLZ7VmZTq7qBHj2cv4DczbEoNFLLDaLLk9ADVxDnqS6"

	var manifest types.ControlMessage
	var firstPeer *webRTC.SimpleWebRTCPeer

	for _, p := range providers {
		// Try direct connection first
		peerConn, err := c.InitiateWebRTCConnectionWithRetry(p.ID, 1)
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

			if err := c.Host.Connect(ctx, targetInfo); err != nil {
				log.Printf("❌ Relay dial failed: %v", err)
				continue
			}

			log.Printf("✅ Relay dial to %s successful", p.ID)

			//perform WebRTC handshake
			peerConn, err = c.InitiateWebRTCConnectionWithRetry(p.ID, 1)
			if err != nil {
				log.Printf("⚠ WebRTC connection via relay failed: %v", err)
				continue
			}
		}

		// If WebRTC connected, fetch manifest
		if peerConn != nil {
			manifest, err = c.RequestManifest(peerConn, cidStr)
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
		if err := c.DB.UpsertPiece(ctx, cidStr, piece.Index, piece.Offset, piece.Size, piece.Hash, false); err != nil {
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

	pieces, _ := c.DB.GetPieces(ctx, cidStr)
	if len(pieces) == 0 {
		return fmt.Errorf("failed to retrieve piece information after receiving manifest")
	}

	state := &types.DownloadState{
		File:            localFile,
		Manifest:        manifest,
		TotalPieces:     int(manifest.NumPieces),
		Pieces:          pieces,
		Completed:       make(chan bool, 1),
		Progress:        progressbar.DefaultBytes(manifest.TotalSize, "downloading..."),
		PieceStatus:     make([]bool, int(manifest.NumPieces)),
		PieceAssignees:  make(map[int]peer.ID),
		PieceBuffers:    make(map[int][][]byte),
		CompletedPieces: 0,
		PieceTimers:     make(map[int]*time.Timer),
		RetryCounts:     make(map[int]int),
	}
	c.DownloadsMux.Lock()
	c.ActiveDownloads[cidStr] = state
	c.DownloadsMux.Unlock()

	// Parallel downloads
	var wg sync.WaitGroup
	peersToUse := providers
	if len(peersToUse) > types.MaxParallelDownloads {
		peersToUse = peersToUse[:types.MaxParallelDownloads]
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
				peerConn, connErr = c.InitiateWebRTCConnectionWithRetry(peerInfo.ID, 2)
				if connErr != nil {
					log.Printf("Chunk peer connect failed: %v", connErr)
					return
				}
				defer peerConn.Close()
			}
			c.DownloadChunksFromPeer(peerConn, state, startPiece, endPiece)
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

func (c *Client) DownloadChunksFromPeer(peer *webRTC.SimpleWebRTCPeer, state *types.DownloadState, startPiece, endPiece int) {
	for i := startPiece; i < endPiece; i++ {
		state.Mu.Lock()
		if state.PieceStatus[i] {
			state.Mu.Unlock()
			continue
		}
		state.PieceAssignees[i] = peer.GetSignalingStream().Conn().RemotePeer()
		state.Mu.Unlock()

		req := types.ControlMessage{
			Command: "REQUEST_PIECE",
			CID:     state.Manifest.CID,
			Index:   int64(i),
		}

		state.Mu.Lock() //piece timeout
		state.PieceTimers[i] = time.AfterFunc(types.PieceTimeout, func() {
			log.Printf("Piece %d timed out, re-requesting...", i)
			c.ReRequestPiece(state, i)
		})
		state.Mu.Unlock()

		if err := peer.SendJSONReliable(req); err != nil {
			log.Printf("Failed to request piece %d from %s: %v", i, peer.GetSignalingStream().Conn().RemotePeer(), err)
			return
		}
	}
}
