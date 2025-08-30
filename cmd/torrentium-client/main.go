package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"net/url"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	libp2pws "github.com/libp2p/go-libp2p/p2p/transport/websocket"

	"github.com/pion/webrtc/v3"

	"torrentium/internal/db"
	"torrentium/internal/p2p"
	"torrentium/internal/torrent"
	"torrentium/internal/client"
	torrentiumWebRTC "torrentium/internal/client"
)

// Client struct client application ki state aur components ko hold karta hai.
type Client struct {
	host            host.Host
	trackerConn     *websocket.Conn // WebSocket connection to tracker
	peerName        string
	webRTCPeers     map[peer.ID]*torrentiumWebRTC.WebRTCPeer
	peersMux        sync.RWMutex
	sharingFiles    map[uuid.UUID]string
	activeDownloads map[uuid.UUID]*os.File // Track active file downloads
	downloadsMux    sync.RWMutex

	// Channels for handling responses
	fileListChan        chan []db.File
	peerListChan        chan []db.Peer
	requestResponseChan chan p2p.Message
}

// entry point for the webRTC peer code
func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: Could not load .env file: %v", err)
		log.Println("Proceeding with system environment variables...")
	}

	// Create libp2p host with WebSocket support
	h, err := libp2p.New(
		libp2p.Transport(libp2pws.New),                    // Add WebSocket transport
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0/ws"), // WebSocket listen address
	)
	if err != nil {
		log.Fatal("Failed to create libp2p host:", err)
	}
	log.Printf("Peer libp2p Host ID: %s", h.ID())

	setupGracefulShutdown(h)

	// Get WebSocket tracker URL from .env with fallback
	trackerWSURL := os.Getenv("TRACKER_WS_URL")
	if trackerWSURL == "" {
		trackerWSURL = "ws://localhost:8080/ws" // Listen on all interfaces
		log.Printf("TRACKER_WS_URL not set, using default: %s", trackerWSURL)
	}
	log.Printf("Connecting to tracker at: %s", trackerWSURL)

	client := NewClient(h)
	// WebRTC offers ko handle karne ke liye signaling protocol register kra hain.
	p2p.RegisterSignalingProtocol(h, client.handleWebRTCOffer)

	if err := client.connectToTrackerWS(trackerWSURL); err != nil {
		log.Fatalf("Failed to connect to tracker: %v", err)
	}
	defer client.trackerConn.Close()

	client.commandLoop()
}

func NewClient(h host.Host) *Client {
	return &Client{
		host:                h,
		webRTCPeers:         make(map[peer.ID]*torrentiumWebRTC.WebRTCPeer),
		sharingFiles:        make(map[uuid.UUID]string),
		activeDownloads:     make(map[uuid.UUID]*os.File),
		fileListChan:        make(chan []db.File, 1),
		peerListChan:        make(chan []db.Peer, 1),
		requestResponseChan: make(chan p2p.Message, 1),
	}
}

// WebSocket connection to tracker
func (c *Client) connectToTrackerWS(wsURL string) error {
	fmt.Print("Enter your peer name: ")
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return errors.New("failed to read peer name")
	}
	c.peerName = scanner.Text()
	if c.peerName == "" {
		return errors.New("peer name cannot be empty")
	}

	// Parse WebSocket URL
	u, err := url.Parse(wsURL)
	if err != nil {
		return fmt.Errorf("invalid WebSocket URL: %w", err)
	}

	// Connect to WebSocket tracker
	c.trackerConn, _, err = websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to connect to WebSocket tracker: %w", err)
	}
	log.Println("Successfully connected to tracker via WebSocket.")

	// Send handshake directly using WebSocket JSON
	addrs := c.host.Addrs()
	addrStrings := make([]string, len(addrs))
	for i, addr := range addrs {
		addrStrings[i] = addr.String()
	}

	handshakePayload, _ := json.Marshal(p2p.HandshakePayload{
		Name:        c.peerName,
		ListenAddrs: addrStrings,
		PeerID:      c.host.ID().String(),
	})
	msg := p2p.Message{Command: "HANDSHAKE", Payload: handshakePayload}

	if err := c.trackerConn.WriteJSON(msg); err != nil {
		return fmt.Errorf("failed to send handshake to tracker: %w", err)
	}

	// Wait for welcome message
	var welcomeMsg p2p.Message
	log.Printf("%T", welcomeMsg.Payload)
	if err := c.trackerConn.ReadJSON(&welcomeMsg); err != nil {
		log.Printf("Error reading welcome message: %v", welcomeMsg.Payload)
		log.Printf("Error reading welcome message: %T", welcomeMsg.Payload)
		return fmt.Errorf("failed to read welcome message from tracker: %w", err)
	}
	log.Printf("✅ Tracker handshake complete. Welcome message: %s", welcomeMsg.Command)

	// Start background message handler
	go c.handleIncomingMessages()

	return nil
}

// handleIncomingMessages handles background messages from tracker
func (c *Client) handleIncomingMessages() {
	for {
		var msg p2p.Message
		if err := c.trackerConn.ReadJSON(&msg); err != nil {
			log.Printf("Error reading message from tracker: %v", err)
			return
		}

		switch msg.Command {
		case "REQUEST_FILE":
			go c.handleFileRequest(msg)
		case "FILE_CHUNK":
			go c.handleFileChunk(msg)
		case "FILE_LIST":
			// Handle file list response
			var files []db.File
			if err := json.Unmarshal(msg.Payload, &files); err != nil {
				log.Printf("Error unmarshaling file list: %v", err)
				continue
			}
			select {
			case c.fileListChan <- files:
				// Successfully sent to channel
			default:
				// Channel full, ignore (shouldn't happen with buffer size 1)
				log.Printf("File list channel full, ignoring response")
			}
		case "PEER_LIST_ALL":
			// Handle peer list response
			var peers []db.Peer
			if err := json.Unmarshal(msg.Payload, &peers); err != nil {
				log.Printf("Error unmarshaling peer list: %v", err)
				continue
			}
			select {
			case c.peerListChan <- peers:
				// Successfully sent to channel
			default:
				// Channel full, ignore (shouldn't happen with buffer size 1)
				log.Printf("Peer list channel full, ignoring response")
			}
		case "FILE_REQUEST_INITIATED", "ERROR", "ACK":
			// Handle generic responses
			select {
			case c.requestResponseChan <- msg:
				// Successfully sent to channel
			default:
				// Channel full, ignore
				log.Printf("Request response channel full, ignoring response")
			}
		default:
			// Ignore other messages in background handler
			log.Printf("Unhandled message command: %s", msg.Command)
		}
	}
}

// handleFileChunk handles incoming file chunks
func (c *Client) handleFileChunk(msg p2p.Message) {
	var chunkPayload p2p.FileTransferPayload
	if err := json.Unmarshal(msg.Payload, &chunkPayload); err != nil {
		log.Printf("Error unmarshaling file chunk: %v", err)
		return
	}

	// Check if we're downloading this file
	c.downloadsMux.RLock()
	outputFile, exists := c.activeDownloads[chunkPayload.FileID]
	c.downloadsMux.RUnlock()

	if !exists {
		// Not our download, ignore
		return
	}

	// Write chunk to file
	if _, err := outputFile.Write(chunkPayload.ChunkData); err != nil {
		log.Printf("Failed to write chunk to file: %v", err)
		return
	}

	log.Printf("Received chunk %d for file %s", chunkPayload.ChunkIndex, chunkPayload.Filename)

	if chunkPayload.IsLast {
		log.Printf("File download completed: %s", chunkPayload.Filename)
		outputFile.Close()

		// Remove from active downloads
		c.downloadsMux.Lock()
		delete(c.activeDownloads, chunkPayload.FileID)
		c.downloadsMux.Unlock()
	}
}

// handleFileRequest handles incoming file requests from other peers via tracker
func (c *Client) handleFileRequest(msg p2p.Message) {
	var payload p2p.RequestFilePayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.Printf("Error unmarshaling file request: %v", err)
		return
	}

	log.Printf("Received file request for FileID: %s from peer: %s", payload.FileID, payload.RequesterPeerID)

	// Check if we have this file
	filePath, exists := c.sharingFiles[payload.FileID]
	if !exists {
		log.Printf("File %s not found in sharing files", payload.FileID)
		return
	}

	// Open and send the file
	if err := c.sendFileToTracker(payload.FileID, filePath, payload.RequesterPeerID); err != nil {
		log.Printf("Error sending file: %v", err)
	}
}

// sendFileToTracker sends file chunks to tracker for forwarding to requester
func (c *Client) sendFileToTracker(fileID uuid.UUID, filePath string, requesterPeerID string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Get file info
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	const chunkSize = 64 * 1024 // 64KB chunks
	buffer := make([]byte, chunkSize)
	chunkIndex := 0

	log.Printf("Sending file %s (%d bytes) to peer %s", filepath.Base(filePath), fileInfo.Size(), requesterPeerID)

	for {
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			return fmt.Errorf("failed to read file chunk: %w", err)
		}

		if n == 0 {
			break
		}

		isLast := err == io.EOF
		chunkData := buffer[:n]

		chunkPayload := p2p.FileTransferPayload{
			FileID:     fileID,
			ChunkIndex: chunkIndex,
			ChunkData:  chunkData,
			IsLast:     isLast,
			TotalSize:  fileInfo.Size(),
			Filename:   filepath.Base(filePath),
		}

		payloadJSON, _ := json.Marshal(chunkPayload)
		chunkMsg := p2p.Message{
			Command: "FILE_CHUNK",
			Payload: payloadJSON,
		}

		if err := c.trackerConn.WriteJSON(chunkMsg); err != nil {
			return fmt.Errorf("failed to send chunk %d: %w", chunkIndex, err)
		}

		log.Printf("Sent chunk %d (%d bytes)", chunkIndex, n)
		chunkIndex++

		if isLast {
			log.Printf("File transfer completed for %s", filepath.Base(filePath))
			break
		}
	}

	return nil
}

// traker se online peers ki list request karta hai
func (c *Client) listPeers() error {
	if err := c.trackerConn.WriteJSON(p2p.Message{Command: "LIST_PEERS"}); err != nil {
		return err
	}

	// Wait for response from background handler
	select {
	case peers := <-c.peerListChan:
		fmt.Println("\nOnline Peers:")
		fmt.Println("----------------------------------------")
		if len(peers) <= 1 {
			fmt.Println("You are the only peer currently online.")
		} else {
			for _, peer := range peers {
				if peer.PeerID == c.host.ID().String() {
					continue // khud ko list mein nahi show karna hai
				}
				fmt.Printf("  Name: %s\n  ID:   %s\n", peer.Name, peer.PeerID)
				fmt.Print("  Addrs:")
				if len(peer.Multiaddrs) > 0 {
					fmt.Printf(" %s\n", peer.Multiaddrs[0])
				} else {
					fmt.Printf(" No addresses available\n")
				}
				fmt.Println("----------------------------------------")
			}
		}
		return nil
	case <-time.After(10 * time.Second):
		return fmt.Errorf("timeout waiting for peer list response")
	}
}

// commandLoop user se input leta hai aur uske hisab se actions perform karta hai, jab tak connection close nhi ho jata
func (c *Client) commandLoop() {
	scanner := bufio.NewScanner(os.Stdin)
	webRTC.PrintClientInstructions()
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
			webRTC.PrintClientInstructions()
		case "add":
			if len(args) != 1 {
				err = errors.New("usage: add <filepath>")
			} else {
				err = c.addFile(args[0])
			}
		case "list":
			err = c.listFiles()
		case "listpeers":
			err = c.listPeers()
		case "get":
			if len(args) != 1 {
				err = errors.New("usage: get <file_id> <output_path>")
			} else {
				err = c.get(args[0], "downloaded_"+args[0])
			}
		case "exit":
			return
		default:
			err = errors.New("unknown command")
		}
		if err != nil {
			log.Printf("Error: %v", err)
		}
	}
}

// ek local file ko tracker par announce karta hai
func (c *Client) addFile(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return err
	}
	fileHash := fmt.Sprintf("%x", hasher.Sum(nil))

	// Create the payload to send to the tracker.
	payload, _ := json.Marshal(p2p.AnnounceFilePayload{
		FileHash: fileHash,
		Filename: filepath.Base(filePath),
		FileSize: info.Size(),
		PeerID:   c.host.ID().String(),
	})

	// Send the ANNOUNCE_FILE command to the tracker.
	if err := c.trackerConn.WriteJSON(p2p.Message{Command: "ANNOUNCE_FILE", Payload: payload}); err != nil {
		return err
	}

	// Wait for response from background handler
	var resp p2p.Message
	select {
	case resp = <-c.requestResponseChan:
		// Got response
	case <-time.After(10 * time.Second):
		return fmt.Errorf("timeout waiting for tracker response")
	}

	// Wait for the "ACK" (acknowledgement) from the tracker.
	if resp.Command != "ACK" {
		return fmt.Errorf("tracker responded with error: %s", resp.Payload)
	}

	// **THE FIX: Store the file information for sharing.**
	var ackPayload p2p.AnnounceAckPayload
	if err := json.Unmarshal(resp.Payload, &ackPayload); err != nil {
		return fmt.Errorf("failed to parse tracker's ACK payload: %w", err)
	}
	c.sharingFiles[ackPayload.FileID] = filePath // Add the file to the map.

	// Create the corresponding .torrent file.
	if err := torrentfile.CreateTorrentFile(filePath); err != nil {
		log.Printf("Warning: failed to create .torrent file: %v", err)
	}

	fmt.Printf("File '%s' announced successfully and is ready to be shared.\n", filepath.Base(filePath))
	return nil
}

// listFiles tracker par available sabhi files ki list get karta hai.
func (c *Client) listFiles() error {
	if err := c.trackerConn.WriteJSON(p2p.Message{Command: "LIST_FILES"}); err != nil {
		return err
	}

	// Wait for response from background handler
	select {
	case files := <-c.fileListChan:
		if len(files) == 0 {
			fmt.Println("No files available on the tracker.")
			return nil
		}

		fmt.Println("\nAvailable Files:")
		for _, file := range files {
			fmt.Println("--------------------")
			fmt.Printf("  ID: %s\n  Name: %s\n  Size: %s\n", file.ID, file.Filename, torrentiumWebRTC.FormatFileSize(file.FileSize))
		}
		fmt.Println("--------------------")
		return nil
	case <-time.After(10 * time.Second):
		return fmt.Errorf("timeout waiting for file list response")
	}
}

// get function ek file ko download karne ka process shuru karta hai using WebSocket.
func (c *Client) get(fileIDStr string, outputPath string) error {
	// pehle, fileID parse karte hai to locate and identify the file
	fileID, err := uuid.Parse(fileIDStr)
	if err != nil {
		return fmt.Errorf("invalid file ID format: %w", err)
	}

	// Create output file
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}

	// Register the download
	c.downloadsMux.Lock()
	c.activeDownloads[fileID] = outputFile
	c.downloadsMux.Unlock()

	// tracker se file request send karte hai
	payload, _ := json.Marshal(p2p.RequestFilePayload{
		FileID:          fileID,
		RequesterPeerID: c.host.ID().String(),
	})

	if err := c.trackerConn.WriteJSON(p2p.Message{Command: "REQUEST_FILE", Payload: payload}); err != nil {
		// Clean up on error
		outputFile.Close()
		c.downloadsMux.Lock()
		delete(c.activeDownloads, fileID)
		c.downloadsMux.Unlock()
		return fmt.Errorf("failed to send REQUEST_FILE request: %w", err)
	}

	var resp p2p.Message

	// Wait for response from background handler
	select {
	case resp = <-c.requestResponseChan:
		// Got response
	case <-time.After(10 * time.Second):
		// Clean up on timeout
		outputFile.Close()
		c.downloadsMux.Lock()
		delete(c.activeDownloads, fileID)
		c.downloadsMux.Unlock()
		return fmt.Errorf("timeout waiting for tracker response")
	}

	if resp.Command == "ERROR" {
		// Clean up on error
		outputFile.Close()
		c.downloadsMux.Lock()
		delete(c.activeDownloads, fileID)
		c.downloadsMux.Unlock()
		return fmt.Errorf("tracker error: %s", resp.Payload)
	}

	if resp.Command != "FILE_REQUEST_INITIATED" {
		// Clean up on error
		outputFile.Close()
		c.downloadsMux.Lock()
		delete(c.activeDownloads, fileID)
		c.downloadsMux.Unlock()
		return fmt.Errorf("unexpected tracker response: %s", resp.Command)
	}

	fmt.Printf("File request initiated. Download will continue in background...\n")
	fmt.Printf("Check the file at: %s\n", outputPath)

	return nil
}

// WebRTC offer/answer exchange process ko handle karta hai
func (c *Client) initiateWebRTCConnection(targetPeerID peer.ID) (*torrentiumWebRTC.WebRTCPeer, error) {
	//signaling ke liye target peer ke saath ek naya stream kholte hai(isse shayad libp2p pe shift karna hai)
	s, err := c.host.NewStream(context.Background(), targetPeerID, p2p.SignalingProtocolID)
	if err != nil {
		return nil, err
	}
	// defer s.Close()

	webRTCPeer, err := torrentiumWebRTC.NewWebRTCPeer(c.onDataChannelMessage)
	if err != nil {
		return nil, err
	}

	webRTCPeer.SetSignalingStream(s)

	// Offer create karke signaling stream par bhejte hain
	offer, err := webRTCPeer.CreateOffer()
	if err != nil {
		return nil, err
	}

	encoder := json.NewEncoder(s)
	if err := encoder.Encode(offer); err != nil {
		return nil, err
	}

	//peer se answer ka wait karte hai
	var answer string
	decoder := json.NewDecoder(s)
	if err := decoder.Decode(&answer); err != nil {
		return nil, err
	}

	if err := webRTCPeer.SetAnswer(answer); err != nil {
		return nil, err
	}

	//connection ko 30 sec ka time diya hai completely establish hone ke liye
	if err := webRTCPeer.WaitForConnection(30 * time.Second); err != nil {
		return nil, err
	}
	return webRTCPeer, nil
}

// fellow peer se aaye WebRTC offer ko handle karta hai
func (c *Client) handleWebRTCOffer(offer, remotePeerIDStr string, s network.Stream) (string, error) {
	remotePeerID, err := peer.Decode(remotePeerIDStr)
	if err != nil {
		return "", err
	}

	log.Printf("Handling incoming WebRTC offer from %s", remotePeerID)
	webRTCPeer, err := torrentiumWebRTC.NewWebRTCPeer(c.onDataChannelMessage)
	if err != nil {
		return "", err
	}

	webRTCPeer.SetSignalingStream(s)

	answer, err := webRTCPeer.CreateAnswer(offer)
	if err != nil {
		webRTCPeer.Close()
		return "", err
	}

	// Naye WebRTC peer ko apne map mein add karte hain.
	c.addWebRTCPeer(remotePeerID, webRTCPeer)
	return answer, nil
}

// WebRTC data channel par aaye messages ko process karta hai
func (c *Client) onDataChannelMessage(msg webrtc.DataChannelMessage, p *torrentiumWebRTC.WebRTCPeer) {

	if msg.IsString {
		var message map[string]string
		if err := json.Unmarshal(msg.Data, &message); err != nil {
			log.Printf("Received un-parseable message: %s", string(msg.Data))
			return
		}

		if cmd, ok := message["command"]; ok && cmd == "REQUEST_FILE" {
			fileIDStr, hasFileID := message["file_id"]
			if !hasFileID {
				log.Println("Received file request without a file_id.")
				return
			}
			fileID, err := uuid.Parse(fileIDStr)
			if err != nil {
				log.Printf("Received file request with invalid file ID: %s", fileIDStr)
				return
			}
			// Start sending the file in a new concurrent routine.
			go c.sendFile(p, fileID)
		} else if status, ok := message["status"]; ok && status == "TRANSFER_COMPLETE" {
			log.Println("File transfer complete!")
			if writer := p.GetFileWriter(); writer != nil {
				writer.Close() // Close the output file.
			}
			p.Close() // Close the WebRTC connection.
		}

	} else {
		// This is the downloader receiving file chunks.
		if writer := p.GetFileWriter(); writer != nil {
			if _, err := writer.Write(msg.Data); err != nil {
				log.Printf("Error writing file chunk: %v", err)
			}
		} else {
			log.Println("Received binary data but no file writer is active.")
		}
	}
}

func (c *Client) sendFile(p *torrentiumWebRTC.WebRTCPeer, fileID uuid.UUID) {
	log.Printf("Processing request to send file with ID: %s", fileID)

	filePath, ok := c.sharingFiles[fileID]
	if !ok {
		log.Printf("Error: Received request for file ID %s, but I am not sharing it.", fileID)
		p.Send(map[string]string{"error": "File not found"})
		return
	}

	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("Error opening file %s to send: %v", filePath, err)
		p.Send(map[string]string{"error": "Could not open file"})
		return
	}
	defer file.Close()

	log.Printf("Starting file transfer for %s", filepath.Base(filePath))
	buffer := make([]byte, 16*1024) // 16KB chunks
	for {
		bytesRead, err := file.Read(buffer)
		if err != nil {
			if err == io.EOF {
				break // End of file
			}
			log.Printf("Error reading file chunk: %v", err)
			return
		}
		if err := p.SendRaw(buffer[:bytesRead]); err != nil {
			log.Printf("Error sending file chunk: %v", err)
			return
		}
	}
	log.Printf("Finished sending file %s", filepath.Base(filePath))
	// Send a "transfer complete" message so the receiver can clean up.
	p.Send(map[string]string{"status": "TRANSFER_COMPLETE"})
}

// ek naye WebRTC peer ko thread-safe tarike se map mein add karta hai (race condition avoid karne ke liye)
func (c *Client) addWebRTCPeer(id peer.ID, p *torrentiumWebRTC.WebRTCPeer) {
	c.peersMux.Lock()
	defer c.peersMux.Unlock()
	c.webRTCPeers[id] = p
}

// Ctrl+C jaise signals ko handle karta hai taaki program theek se band ho
func setupGracefulShutdown(h host.Host) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		log.Println("Shutting down...")
		if err := h.Close(); err != nil {
			log.Printf("Error closing libp2p host: %v", err)
		}
		os.Exit(0)
	}()
}

// func printClientInstructions() {
// 	fmt.Println(`
// 📖 Torrentium Client Commands:
//   help          - Show this help message.
//   add <path>    - Announce a local file to the tracker.
//   list          - List all files available on the tracker.
//   listpeers     - List all currently online peers.
//   get <file_id> - Find and download a file from a peer.
//   exit          - Shutdown the client.`)
// }
