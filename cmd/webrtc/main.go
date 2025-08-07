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
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/p2p/transport/websocket"
	"github.com/multiformats/go-multiaddr"
	"github.com/pion/webrtc/v3"

	"torrentium/db"
	"torrentium/p2p"
	"torrentium/torrentfile"
	torrentiumWebRTC "torrentium/webRTC"
)

type Client struct {
	host          host.Host
	trackerStream network.Stream
	encoder       *json.Encoder
	decoder       *json.Decoder
	peerName      string
	webRTCPeers   map[peer.ID]*torrentiumWebRTC.WebRTCPeer
	peersMux      sync.RWMutex
	sharingFiles  map[uuid.UUID]string
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Unable to access .env file:", err)
	}

	h, err := libp2p.New(
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0/ws"),
		libp2p.Transport(websocket.New),
	)

	if err != nil {
		log.Fatal("Failed to create libp2p host:", err)
	}

	log.Printf("Peer libp2p Host ID: %s", h.ID())
	for _, addr := range h.Addrs() {
		log.Printf("Listening on: %s", addr)
	}

	setupGracefulShutdown(h)

	trackerMultiAddrStr := os.Getenv("TRACKER_ADDR")
	if trackerMultiAddrStr == "" {
		log.Fatal("TRACKER_ADDR environment variable is not set.")
	}

	trackerAddrInfo, err := peer.AddrInfoFromString(trackerMultiAddrStr)
	if err != nil {
		log.Fatal("Invalid TRACKER_ADDR in .env file:", err)
	}

	client := NewClient(h)
	p2p.RegisterSignalingProtocol(h, client.handleWebRTCOffer)

	if err := client.connectToTracker(*trackerAddrInfo); err != nil {
		log.Fatalf("Failed to connect to tracker: %v", err)
	}
	defer client.trackerStream.Close()

	client.commandLoop()
}

func NewClient(h host.Host) *Client {
	return &Client{
		host:         h,
		webRTCPeers:  make(map[peer.ID]*torrentiumWebRTC.WebRTCPeer),
		sharingFiles: make(map[uuid.UUID]string),
	}
}

func (c *Client) connectToTracker(trackerAddr peer.AddrInfo) error {
	fmt.Print("Enter your peer name: ")
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return errors.New("failed to read peer name")
	}
	c.peerName = strings.TrimSpace(scanner.Text())
	if c.peerName == "" {
		return errors.New("peer name cannot be empty")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := c.host.Connect(ctx, trackerAddr); err != nil {
		return fmt.Errorf("failed to connect to tracker: %w", err)
	}
	log.Println("Successfully connected to tracker peer.")

	s, err := c.host.NewStream(context.Background(), trackerAddr.ID, p2p.TrackerProtocolID)
	if err != nil {
		return fmt.Errorf("failed to open stream to tracker: %w", err)
	}
	c.trackerStream = s
	c.encoder = json.NewEncoder(s)
	c.decoder = json.NewDecoder(s)

	var addrStrings []string
	for _, addr := range c.host.Addrs() {
		addrStrings = append(addrStrings, addr.String())
	}

	handshakePayload, _ := json.Marshal(p2p.HandshakePayload{
		Name:        c.peerName,
		ListenAddrs: addrStrings,
	})
	msg := p2p.Message{Command: "HANDSHAKE", Payload: handshakePayload}

	if err := c.encoder.Encode(msg); err != nil {
		return fmt.Errorf("failed to send handshake to tracker: %w", err)
	}

	var welcomeMsg p2p.Message
	if err := c.decoder.Decode(&welcomeMsg); err != nil {
		return fmt.Errorf("failed to read welcome message from tracker: %w", err)
	}
	log.Printf("Tracker handshake complete. Welcome: %s", welcomeMsg.Command)
	return nil
}

func (c *Client) listPeers() error {
	if err := c.encoder.Encode(p2p.Message{Command: "LIST_PEERS"}); err != nil {
		return err
	}
	var resp p2p.Message
	if err := c.decoder.Decode(&resp); err != nil {
		return err
	}
	if resp.Command != "PEER_LIST_ALL" {
		return fmt.Errorf("tracker responded with an error: %s", resp.Payload)
	}

	var peers []db.Peer
	if err := json.Unmarshal(resp.Payload, &peers); err != nil {
		return err
	}

	fmt.Println("\n--- Online Peers ---")
	foundPeers := false
	for _, peer := range peers {
		if peer.PeerID != c.host.ID().String() {
			foundPeers = true
			fmt.Printf("Name: %s\n  ID: %s\n", peer.Name, peer.PeerID)
			if len(peer.Multiaddrs) > 0 {
				fmt.Printf("  Addr: %s\n", peer.Multiaddrs[0])
			}
			fmt.Println("--------------------")
		}
	}
	if !foundPeers {
		fmt.Println("You are the only peer online.")
	}
	return nil
}

func (c *Client) commandLoop() {
	scanner := bufio.NewScanner(os.Stdin)
	torrentiumWebRTC.PrintClientInstructions()

	for {
		fmt.Print("\n> ")
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
			torrentiumWebRTC.PrintClientInstructions()
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
				err = errors.New("usage: get <file_id>")
			} else {
				err = c.get(args[0])
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

	payload, _ := json.Marshal(p2p.AnnounceFilePayload{
		FileHash: fileHash,
		Filename: filepath.Base(filePath),
		FileSize: info.Size(),
	})

	if err := c.encoder.Encode(p2p.Message{Command: "ANNOUNCE_FILE", Payload: payload}); err != nil {
		return err
	}

	var resp p2p.Message
	if err := c.decoder.Decode(&resp); err != nil {
		return err
	}
	if resp.Command != "ACK" {
		return fmt.Errorf("tracker responded with error: %s", resp.Payload)
	}

	var ackPayload p2p.AnnounceAckPayload
	if err := json.Unmarshal(resp.Payload, &ackPayload); err != nil {
		return fmt.Errorf("failed to parse tracker's ACK payload: %w", err)
	}
	c.sharingFiles[ackPayload.FileID] = filePath

	if err := torrentfile.CreateTorrentFile(filePath); err != nil {
		log.Printf("Warning: failed to create .torrent file: %v", err)
	}

	fmt.Printf("File '%s' announced successfully.\n", filepath.Base(filePath))
	return nil
}

func (c *Client) listFiles() error {
	if err := c.encoder.Encode(p2p.Message{Command: "LIST_FILES"}); err != nil {
		return err
	}
	var resp p2p.Message
	if err := c.decoder.Decode(&resp); err != nil {
		return err
	}
	if resp.Command != "FILE_LIST" {
		return fmt.Errorf("tracker responded with error: %s", resp.Payload)
	}

	var files []db.File
	if err := json.Unmarshal(resp.Payload, &files); err != nil {
		return err
	}
	if len(files) == 0 {
		fmt.Println("No files available on the tracker.")
		return nil
	}

	fmt.Println("\n--- Available Files ---")
	for _, file := range files {
		fmt.Printf("ID: %s\n  Name: %s\n  Size: %s\n", file.ID, file.Filename, torrentiumWebRTC.FormatFileSize(file.FileSize))
		fmt.Println("--------------------")
	}
	return nil
}

func (c *Client) get(fileIDStr string) error {
	fileID, err := uuid.Parse(fileIDStr)
	if err != nil {
		return fmt.Errorf("invalid file ID format: %w", err)
	}

	payload, _ := json.Marshal(p2p.GetPeersPayload{FileID: fileID})
	if err := c.encoder.Encode(p2p.Message{Command: "GET_PEERS_FOR_FILE", Payload: payload}); err != nil {
		return fmt.Errorf("failed to send get peers request: %w", err)
	}

	var resp p2p.Message
	if err := c.decoder.Decode(&resp); err != nil {
		return fmt.Errorf("failed to decode tracker response: %w", err)
	}
	if resp.Command != "PEER_LIST" {
		return fmt.Errorf("tracker error: %s", resp.Payload)
	}

	var peers []db.PeerFile
	if err := json.Unmarshal(resp.Payload, &peers); err != nil {
		return fmt.Errorf("failed to unmarshal peer list: %w", err)
	}
	if len(peers) == 0 {
		return errors.New("no online peers found for this file")
	}

	fmt.Println("--- Peers with this file ---")
	for i, p := range peers {
		fmt.Printf("[%d] Peer DB ID: %s (Score: %.2f)\n", i, p.PeerID, p.Score)
	}
	fmt.Print("Select a peer to download from: ")
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return errors.New("failed to read peer selection")
	}
	choice, err := strconv.Atoi(scanner.Text())
	if err != nil || choice < 0 || choice >= len(peers) {
		return errors.New("invalid selection")
	}
	selectedPeer := peers[choice]

	peerInfoPayload, _ := json.Marshal(p2p.GetPeerInfoPayload{PeerDBID: selectedPeer.PeerID})
	if err := c.encoder.Encode(p2p.Message{Command: "GET_PEER_INFO", Payload: peerInfoPayload}); err != nil {
		return fmt.Errorf("failed to request peer info: %w", err)
	}
	if err := c.decoder.Decode(&resp); err != nil || resp.Command != "PEER_INFO" {
		return errors.New("could not retrieve peer info from tracker")
	}
	var peerInfo db.Peer
	if err := json.Unmarshal(resp.Payload, &peerInfo); err != nil {
		return fmt.Errorf("failed to unmarshal peer info: %w", err)
	}

	targetPeerID, err := peer.Decode(peerInfo.PeerID)
	if err != nil {
		return fmt.Errorf("could not decode peer ID: %w", err)
	}

	if len(peerInfo.Multiaddrs) == 0 {
		return fmt.Errorf("peer %s has no listed addresses", targetPeerID)
	}
	maddr, err := multiaddr.NewMultiaddr(peerInfo.Multiaddrs[0])
	if err != nil {
		return fmt.Errorf("could not parse peer multiaddress: %w", err)
	}

	c.host.Peerstore().AddAddr(targetPeerID, maddr, peerstore.TempAddrTTL)
	log.Printf("Added peer %s with address %s to local peerstore", targetPeerID, maddr)

	log.Println("Initiating WebRTC connection...")
	webRTCPeer, err := c.initiateWebRTCConnection(targetPeerID)
	if err != nil {
		return fmt.Errorf("WebRTC connection failed: %w", err)
	}
	c.addWebRTCPeer(targetPeerID, webRTCPeer)
	log.Println("WebRTC connection established.")

	outputPath := "downloaded_" + fileIDStr
	outputFile, err := os.Create(outputPath)
	if err != nil {
		webRTCPeer.Close()
		return fmt.Errorf("failed to create output file: %w", err)
	}
	webRTCPeer.SetFileWriter(outputFile)

	log.Printf("Requesting file %s from peer...", fileID)
	requestPayload := map[string]string{"command": "REQUEST_FILE", "file_id": fileID.String()}
	if err := webRTCPeer.Send(requestPayload); err != nil {
		webRTCPeer.Close()
		return fmt.Errorf("failed to send file request: %w", err)
	}

	fmt.Println("File request sent. Download will start shortly.")
	return nil
}

func (c *Client) initiateWebRTCConnection(targetPeerID peer.ID) (*torrentiumWebRTC.WebRTCPeer, error) {
	s, err := c.host.NewStream(context.Background(), targetPeerID, p2p.SignalingProtocolID)
	if err != nil {
		return nil, err
	}

	webRTCPeer, err := torrentiumWebRTC.NewWebRTCPeer(c.onDataChannelMessage)
	if err != nil {
		return nil, err
	}
	webRTCPeer.SetSignalingStream(s)

	offer, err := webRTCPeer.CreateOffer()
	if err != nil {
		return nil, err
	}

	encoder := json.NewEncoder(s)
	if err := encoder.Encode(offer); err != nil {
		return nil, err
	}

	var answer string
	decoder := json.NewDecoder(s)
	if err := decoder.Decode(&answer); err != nil {
		return nil, err
	}

	if err := webRTCPeer.SetAnswer(answer); err != nil {
		return nil, err
	}

	if err := webRTCPeer.WaitForConnection(30 * time.Second); err != nil {
		return nil, err
	}
	return webRTCPeer, nil
}

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

	c.addWebRTCPeer(remotePeerID, webRTCPeer)
	return answer, nil
}

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
			go c.sendFile(p, fileID)
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
				break
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
	p.Send(map[string]string{"status": "TRANSFER_COMPLETE"})
}

func (c *Client) addWebRTCPeer(id peer.ID, p *torrentiumWebRTC.WebRTCPeer) {
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
			log.Printf("Error closing libp2p host: %v", err)
		}
		os.Exit(0)
	}()
}