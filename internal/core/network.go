package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	webRTC "torrentium/internal/client"
	p2p "torrentium/internal/p2p"
	"torrentium/internal/types"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

func (c *Client) DebugNetworkStatus() {
	fmt.Println("\n=== Network Debug Info ===")
	fmt.Printf("Our Peer ID: %s\n", c.Host.ID())
	fmt.Printf("Our Addresses:\n")
	for _, addr := range c.Host.Addrs() {
		fmt.Printf(" %s/p2p/%s\n", addr, c.Host.ID())
	}

	peers := c.Host.Network().Peers()
	fmt.Printf("\nConnected Peers (%d):\n", len(peers))
	for i, peerID := range peers {
		conn := c.Host.Network().ConnsToPeer(peerID)
		if len(conn) > 0 {
			fmt.Printf(" %d. %s\n", i+1, peerID)
			fmt.Printf("    Address: %s\n", conn[0].RemoteMultiaddr())
		}
	}

	routingTableSize := c.DHT.RoutingTable().Size()
	fmt.Printf("\nDHT Routing Table Size: %d\n", routingTableSize)

	fmt.Printf("\nShared Files (%d):\n", len(c.SharingFiles))
	for cid, fileInfo := range c.SharingFiles {
		fmt.Printf(" CID: %s\n", cid)
		fmt.Printf(" File: %s\n", fileInfo.Name)
		fmt.Printf(" ---\n")
	}
}

func (c *Client) AnnounceFile(cidStr string) error {
	fileCID, err := cid.Decode(cidStr)
	if err != nil {
		return fmt.Errorf("invalid CID: %w", err)
	}
	fmt.Printf("Re-announcing CID %s to DHT...\n", cidStr)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := c.DHT.Provide(ctx, fileCID, true); err != nil {
		return fmt.Errorf("failed to announce: %w", err)
	}
	fmt.Println(" - Successfully announced to DHT")
	return nil
}

func (c *Client) StartDHTMaintenance() {
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			log.Println("Performing DHT maintenance...")
			c.DHT.RefreshRoutingTable()
			peers := c.Host.Network().Peers()
			log.Printf("Connected to %d peers", len(peers))
			if len(peers) < 5 {
				log.Println("Low peer count; re-bootstrapping...")
				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				_ = p2p.Bootstrap(ctx, c.Host, c.DHT)
				cancel()
			}
		}
	}()
}

func (c *Client) CheckConnectionHealth() {
	peers := c.Host.Network().Peers()
	fmt.Printf("\n=== Connection Health ===\n")
	fmt.Printf("Connected peers: %d\n", len(peers))
	if len(peers) < 3 {
		fmt.Println(" - Warning: Low peer count. Consider restarting or checking network connectivity.")
	} else {
		fmt.Println(" - Good peer connectivity")
	}

	routingTableSize := c.DHT.RoutingTable().Size()
	fmt.Printf("DHT routing table size: %d\n", routingTableSize)
	if routingTableSize < 10 {
		fmt.Println(" - Warning: Small DHT routing table. File discovery may be limited.")
	} else {
		fmt.Println(" - Good DHT connectivity")
	}
}

func (c *Client) FindProvidersWithTimeout(id cid.Cid, timeout time.Duration, maxProviders int) ([]peer.AddrInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	providersChan := c.DHT.FindProvidersAsync(ctx, id, maxProviders)
	var providers []peer.AddrInfo
	var totalFound int

	done := make(chan struct{})
	go func() {
		defer close(done)
		for provider := range providersChan {
			totalFound++
			if provider.ID != c.Host.ID() {
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

func (c *Client) ConnectToPeer(multiaddrStr string) error {
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

	if err := c.Host.Connect(ctx, *peerInfo); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	fmt.Printf(" - Successfully connected to peer %s\n", peerInfo.ID)

	c.Host.Peerstore().AddAddrs(peerInfo.ID, peerInfo.Addrs, time.Hour)

	return nil
}

func (c *Client) ListConnectedPeers() {
	peers := c.Host.Network().Peers()
	fmt.Printf("\n=== Connected Peers (%d) ===\n", len(peers))
	for _, peerID := range peers {
		conn := c.Host.Network().ConnsToPeer(peerID)
		if len(conn) > 0 {
			fmt.Printf("Peer: %s\n", peerID)
			fmt.Printf(" Address: %s\n", conn[0].RemoteMultiaddr())
		}
	}
}

func (c *Client) InitiateWebRTCConnectionWithRetry(targetPeerID peer.ID, maxRetries int) (*webRTC.SimpleWebRTCPeer, error) {

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
		if pinfo, err := c.DHT.FindPeer(ctx, targetPeerID); err == nil && len(pinfo.Addrs) > 0 {
			info = pinfo
		} else {
			lastErr = fmt.Errorf("dht lookup failed: %w", err)
			continue
		}

		if len(info.Addrs) == 0 {
			lastErr = fmt.Errorf("peer %s has no known multiaddrs", targetPeerID)
			continue
		}

		c.Host.Peerstore().AddAddrs(info.ID, info.Addrs, time.Hour)
		fmt.Println(c.Host.Peerstore())

		if c.Host.Network().Connectedness(info.ID) != network.Connected {
			connectCtx, connectCancel := context.WithTimeout(context.Background(), 20*time.Second)
			err := c.Host.Connect(connectCtx, info)
			connectCancel()
			if err != nil {
				log.Printf("failed to connect to peer %s: %v", info.ID, err)
				fmt.Printf("DHT lookup failed: %v. This could be a network issue now trying connection using relays.\n", err)
				return nil, err
			}

			fmt.Printf("Successfully connected to peer %s\n", info.ID)

			time.Sleep(1 * time.Second)
		}

		webrtcPeer, err := webRTC.NewSimpleWebRTCPeer(c.OnDataChannelMessage, c.OnWebRTCPeerClose)
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
		s, err := c.Host.NewStream(streamCtx, targetPeerID, p2p.SignalingProtocolID)
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
		c.PeersMux.Lock()
		c.WebRTCPeers[targetPeerID] = webrtcPeer
		c.PeersMux.Unlock()

		return webrtcPeer, nil
	}
	return nil, lastErr
}

func (c *Client) HandleWebRTCOffer(offer, remotePeerID string, s network.Stream) (string, error) {
	peerID, err := peer.Decode(remotePeerID)
	if err != nil {
		return "", fmt.Errorf("invalid peer ID: %w", err)
	}

	webrtcPeer, err := webRTC.NewSimpleWebRTCPeer(c.OnDataChannelMessage, c.OnWebRTCPeerClose)
	if err != nil {
		return "", err
	}

	webrtcPeer.SetSignalingStream(s)

	answer, err := webrtcPeer.HandleOffer(offer)
	if err != nil {
		webrtcPeer.Close()
		return "", err
	}

	c.PeersMux.Lock()
	c.WebRTCPeers[peerID] = webrtcPeer
	c.PeersMux.Unlock()

	return answer, nil
}

func (c *Client) OnDataChannelMessage(msg webRTC.DataChannelMessage, peer *webRTC.SimpleWebRTCPeer) {
	if !msg.IsString {
		log.Printf("Received unexpected binary message, expecting JSON.")
		return
	}
	if len(msg.Data) == 0 {
		return
	}
	var ctrl types.ControlMessage
	if err := json.Unmarshal(msg.Data, &ctrl); err != nil {
		var ping map[string]string
		if err2 := json.Unmarshal(msg.Data, &ping); err2 == nil {
			if ping["type"] == "ping" {
				// Respond to ping
				pong := map[string]string{"type": "pong"}
				peer.SendJSONReliable(pong)
				return
			} else if ping["type"] == "pong" {
				c.HandlePong(peer.GetSignalingStream().Conn().RemotePeer())
				return
			}
		}
		log.Printf("Failed to unmarshal control message: %v. Raw message: %s", err, string(msg.Data))
		return
	}
	c.HandleControlMessage(ctrl, peer)
}

func (c *Client) HandleControlMessage(ctrl types.ControlMessage, peer *webRTC.SimpleWebRTCPeer) {
	ctx := context.Background()
	switch ctrl.Command {
	case "REQUEST_MANIFEST":
		c.HandleManifestRequest(ctx, ctrl, peer)
	case "MANIFEST":
		c.ManifestChMu.Lock()
		if ch, ok := c.ManifestWaiters[ctrl.CID]; ok {
			ch <- ctrl
		}
		c.ManifestChMu.Unlock()
	case "REQUEST_PIECE":
		go c.HandlePieceRequest(ctx, ctrl, peer)
	case "PIECE_CHUNK":
		c.HandlePieceChunk(ctrl, peer)
	case "CHUNK_ACK":
		c.HandleChunkAck(ctrl)
	default:
		//do nothing
	}
}

func (c *Client) HandlePieceChunk(ctrl types.ControlMessage, peer *webRTC.SimpleWebRTCPeer) {
	c.DownloadsMux.RLock()
	state, ok := c.ActiveDownloads[ctrl.CID]
	c.DownloadsMux.RUnlock()
	if !ok {
		return
	}

	// Send an ACK back to the sender using reliable channel
	ackMsg := types.ControlMessage{
		Command:  "CHUNK_ACK",
		CID:      ctrl.CID,
		Index:    ctrl.Index,
		Sequence: ctrl.Sequence,
	}
	if err := peer.SendJSONReliable(ackMsg); err != nil {
		log.Printf("Failed to send ACK for chunk %d of piece %d: %v", ctrl.Sequence, ctrl.Index, err)
	}

	state.Mu.Lock()
	defer state.Mu.Unlock()

	if state.PieceStatus[ctrl.Index] {
		return // Already have this piece
	}

	if state.PieceBuffers[int(ctrl.Index)] == nil {
		state.PieceBuffers[int(ctrl.Index)] = make([][]byte, ctrl.TotalChunks)
	}

	chunkData, err := hex.DecodeString(ctrl.Payload)
	if err != nil {
		log.Printf("Failed to decode chunk payload: %v", err)
		return
	}

	state.PieceBuffers[int(ctrl.Index)][ctrl.ChunkIndex] = chunkData
	_ = state.Progress.Add(len(chunkData))

	// Check if piece is complete
	isComplete := true
	var pieceSize int
	for _, chunk := range state.PieceBuffers[int(ctrl.Index)] {
		if chunk == nil {
			isComplete = false
			break
		}
		pieceSize += len(chunk)
	}

	if isComplete {
		//Stop the timer for this piece
		if timer, ok := state.PieceTimers[int(ctrl.Index)]; ok {
			timer.Stop()
			delete(state.PieceTimers, int(ctrl.Index))
		}

		// Reassemble and write piece
		pieceData := make([]byte, 0, pieceSize)
		for _, chunk := range state.PieceBuffers[int(ctrl.Index)] {
			pieceData = append(pieceData, chunk...)
		}

		//Verify piece hash
		h := sha256.New()
		h.Write(pieceData)
		hash := hex.EncodeToString(h.Sum(nil))

		if hash != state.Pieces[ctrl.Index].Hash {
			log.Printf("Piece %d hash mismatch", ctrl.Index)
			state.PieceBuffers[int(ctrl.Index)] = nil // Clear buffer to retry
			return
		}

		if _, err := state.File.WriteAt(pieceData, state.Pieces[ctrl.Index].Offset); err != nil {
			log.Printf("Failed to write piece %d to file: %v", ctrl.Index, err)
			return
		}

		state.PieceStatus[ctrl.Index] = true
		state.CompletedPieces++
		delete(state.PieceBuffers, int(ctrl.Index))

		if state.CompletedPieces == state.TotalPieces {
			state.Completed <- true
		}
	}
}

func (c *Client) HandleChunkAck(ctrl types.ControlMessage) {
	c.UnackedChunksMux.Lock()
	defer c.UnackedChunksMux.Unlock()
	if _, ok := c.UnackedChunks[ctrl.CID]; ok {
		if _, ok := c.UnackedChunks[ctrl.CID][ctrl.Index]; ok {
			delete(c.UnackedChunks[ctrl.CID][ctrl.Index], ctrl.Sequence)
			if len(c.UnackedChunks[ctrl.CID][ctrl.Index]) == 0 {
				delete(c.UnackedChunks[ctrl.CID], ctrl.Index)
			}
		}
		if len(c.UnackedChunks[ctrl.CID]) == 0 {
			delete(c.UnackedChunks, ctrl.CID)
		}
	}
}

func (c *Client) HandlePieceRequest(ctx context.Context, ctrl types.ControlMessage, peer *webRTC.SimpleWebRTCPeer) {
	pieces, err := c.DB.GetPieces(ctx, ctrl.CID)
	if err != nil || int(ctrl.Index) >= len(pieces) {
		log.Printf("Invalid piece request for CID %s, index %d", ctrl.CID, ctrl.Index)
		return
	}

	fileInfo, err := c.DB.GetLocalFileByCID(ctx, ctrl.CID)
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

	totalChunks := (len(pieceBuffer) + types.MaxChunk - 1) / types.MaxChunk
	for i := 0; i < totalChunks; i++ {
		start := i * types.MaxChunk
		end := start + types.MaxChunk
		if end > len(pieceBuffer) {
			end = len(pieceBuffer)
		}
		chunk := pieceBuffer[start:end]

		chunkMsg := types.ControlMessage{
			Command:     "PIECE_CHUNK",
			CID:         ctrl.CID,
			Index:       ctrl.Index,
			ChunkIndex:  i,
			TotalChunks: totalChunks,
			Payload:     hex.EncodeToString(chunk),
			Sequence:    i,
		}

		// Store the sent chunk and start a retransmission timer
		c.UnackedChunksMux.Lock()
		if c.UnackedChunks[ctrl.CID] == nil {
			c.UnackedChunks[ctrl.CID] = make(map[int64]map[int]types.ControlMessage)
		}
		if c.UnackedChunks[ctrl.CID][ctrl.Index] == nil {
			c.UnackedChunks[ctrl.CID][ctrl.Index] = make(map[int]types.ControlMessage)
		}
		c.UnackedChunks[ctrl.CID][ctrl.Index][i] = chunkMsg
		c.UnackedChunksMux.Unlock()
		time.AfterFunc(types.RetransmissionTimeout, func() { c.RetransmitChunk(peer, chunkMsg) })

		if err := peer.SendJSON(chunkMsg); err != nil {
			log.Printf("Failed to send chunk %d of piece %d: %v", i, ctrl.Index, err)
			return
		}
		delay := c.CongestionCtrl[peer.GetSignalingStream().Conn().RemotePeer()]
		time.Sleep(delay)
	}
}

func (c *Client) RetransmitChunk(peer *webRTC.SimpleWebRTCPeer, chunkMsg types.ControlMessage) {
	c.UnackedChunksMux.RLock()
	defer c.UnackedChunksMux.RUnlock()
	if _, ok := c.UnackedChunks[chunkMsg.CID]; ok {
		if _, ok := c.UnackedChunks[chunkMsg.CID][chunkMsg.Index]; ok {
			if _, ok := c.UnackedChunks[chunkMsg.CID][chunkMsg.Index][chunkMsg.Sequence]; ok {
				log.Printf("Retransmitting chunk %d of piece %d", chunkMsg.Sequence, chunkMsg.Index)
				if err := peer.SendJSON(chunkMsg); err != nil {
					log.Printf("Failed to retransmit chunk %d of piece %d: %v", chunkMsg.Sequence, chunkMsg.Index, err)
				}
				// Reset timer
				time.AfterFunc(types.RetransmissionTimeout, func() { c.RetransmitChunk(peer, chunkMsg) })
			}
		}
	}
}

func (c *Client) HandleManifestRequest(ctx context.Context, ctrl types.ControlMessage, peer *webRTC.SimpleWebRTCPeer) {
	localFile, err := c.DB.GetLocalFileByCID(ctx, ctrl.CID)
	if err != nil {
		log.Printf("File not found for manifest: %s", ctrl.CID)
		return
	}

	pieces, err := c.DB.GetPieces(ctx, ctrl.CID)
	if err != nil {
		log.Printf("Error getting pieces: %v", err)
		return
	}

	manifest := types.ControlMessage{
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

func (c *Client) OnWebRTCPeerClose(peerID peer.ID) {
	log.Printf("WebRTC peer disconnected: %s", peerID)
	c.PeersMux.Lock()
	delete(c.WebRTCPeers, peerID)
	c.PeersMux.Unlock()

	// Handle download resumption logic
	c.DownloadsMux.Lock()
	defer c.DownloadsMux.Unlock()

	for cid, state := range c.ActiveDownloads {
		for pieceIndex, assignee := range state.PieceAssignees {
			if assignee == peerID {
				log.Printf("Peer %s disconnected, re-requesting piece %d for download %s", peerID, pieceIndex, cid)
				// Re-queue the piece for download
				go c.ReRequestPiece(state, pieceIndex)
			}
		}
	}
}

func (c *Client) MonitorCongestion() {
	ticker := time.NewTicker(types.PingInterval)
	for range ticker.C {
		c.PeersMux.RLock()
		for pid, peer := range c.WebRTCPeers {
			if connState := peer.GetConnectionState(); connState == webRTC.ConnectionStateConnected {
				c.PingTimes[pid] = time.Now()
				ping := map[string]string{"type": "ping"}
				peer.SendJSONReliable(ping)
			}
		}
		c.PeersMux.RUnlock()
	}
}

func (c *Client) HandlePong(pid peer.ID) {
	if start, ok := c.PingTimes[pid]; ok {
		rtt := time.Since(start)
		c.RTTMux.Lock()
		if _, ok := c.RTTMeasurements[pid]; !ok {
			c.RTTMeasurements[pid] = []time.Duration{}
		}
		c.RTTMeasurements[pid] = append(c.RTTMeasurements[pid], rtt)
		if len(c.RTTMeasurements[pid]) > 10 {
			c.RTTMeasurements[pid] = c.RTTMeasurements[pid][1:]
		}
		avgRTT := time.Duration(0)
		for _, d := range c.RTTMeasurements[pid] {
			avgRTT += d
		}
		avgRTT /= time.Duration(len(c.RTTMeasurements[pid]))
		var delay time.Duration = types.MinDelay
		if avgRTT > types.MaxRTT {
			delay = types.MaxDelay
		} else if avgRTT > types.MaxRTT/2 {
			delay = (types.MaxDelay - types.MinDelay) / 2
		}
		c.CongestionCtrl[pid] = delay
		c.RTTMux.Unlock()
		delete(c.PingTimes, pid)
	}
}

func (c *Client) ReRequestPiece(state *types.DownloadState, pieceIndex int) {
	// Re-assign to another peer with backoff
	retryCount := state.RetryCounts[pieceIndex]
	backoff := types.ExponentialBackoffBase * time.Duration(1<<retryCount)
	if backoff > types.MaxBackoff {
		backoff = types.MaxBackoff
	}
	time.AfterFunc(backoff, func() {
		// For simplicity, we'll just re-request from any connected peer.
		// A more advanced implementation would select a new peer.
		for _, p := range c.WebRTCPeers {
			req := types.ControlMessage{
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
	state.RetryCounts[pieceIndex]++
}

func (c *Client) RequestManifest(peer *webRTC.SimpleWebRTCPeer, cidStr string) (types.ControlMessage, error) {
	req := types.ControlMessage{Command: "REQUEST_MANIFEST", CID: cidStr}
	if err := peer.SendJSONReliable(req); err != nil {
		return types.ControlMessage{}, err
	}

	manifestCh := make(chan types.ControlMessage, 1)
	c.ManifestChMu.Lock()
	c.ManifestWaiters[cidStr] = manifestCh
	c.ManifestChMu.Unlock()

	defer func() {
		c.ManifestChMu.Lock()
		delete(c.ManifestWaiters, cidStr)
		c.ManifestChMu.Unlock()
	}()

	select {
	case manifest := <-manifestCh:
		return manifest, nil
	case <-time.After(30 * time.Second):
		return types.ControlMessage{}, fmt.Errorf("timed out waiting for manifest")
	}
}
