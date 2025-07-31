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
	"github.com/multiformats/go-multiaddr"

	// "github.com/multiformats/go-multiaddr"
	"github.com/pion/webrtc/v3"

	"torrentium/db"
	"torrentium/p2p"
	"torrentium/torrentfile"
	"torrentium/webRTC"
	torrentiumWebRTC "torrentium/webRTC"
)

// Client struct client application ki state aur components ko hold karta hai.
type Client struct {
	host          host.Host
	trackerStream network.Stream // Tracker ke saath communication stream
	encoder       *json.Encoder
	decoder       *json.Decoder
	peerName      string
	webRTCPeers   map[peer.ID]*torrentiumWebRTC.WebRTCPeer // Active WebRTC connections ka map
	peersMux      sync.RWMutex
	sharingFiles  map[uuid.UUID]string
}

// entry point for the webRTC peer code
func main() {
	if err := godotenv.Load("C:\\Users\\myind\\Downloads\\Torrentium\\.env.example"); err != nil {
		log.Fatal("Unable to access .env file:", err)
	}

	h, err := libp2p.New()
	if err != nil {
		log.Fatal("Failed to create libp2p host:", err)
	}
	log.Printf("Peer libp2p Host ID: %s", h.ID())

	setupGracefulShutdown(h)

	// .env file se tracker ka multiaddress fetch karte hain
	trackerMultiAddrStr := os.Getenv("TRACKER_ADDR")
	if trackerMultiAddrStr == "" {
		log.Fatal("TRACKER_ADDR environment variable is not set or .env file not found.")
	}

	trackerAddrInfo, err := peer.AddrInfoFromString(trackerMultiAddrStr)
	if err != nil {
		log.Fatal("Invalid TRACKER_ADDR in .env file:", err)
	}

	client := NewClient(h)
	// WebRTC offers ko handle karne ke liye signaling protocol register kra hain.
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

// yeh function trackerr se connection bnata hai aur handshake perform karta hai
func (c *Client) connectToTracker(trackerAddr peer.AddrInfo) error {
	fmt.Print("Enter your peer name: ")
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return errors.New("failed to read peer name")
	}
	c.peerName = scanner.Text()
	if c.peerName == "" {
		return errors.New("peer name cannot be empty")
	}

	//currently tracker se connect karne ke liye 10 secs ka timeout hai
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := c.host.Connect(ctx, trackerAddr); err != nil {
		return fmt.Errorf("failed to connect to tracker's AddrInfo: %w", err)
	}
	log.Println("Successfully connected to tracker peer.")

	//tracker ke saath communication ke liye ek new stream
	s, err := c.host.NewStream(context.Background(), trackerAddr.ID, p2p.TrackerProtocolID)
	if err != nil {
		return fmt.Errorf("failed to open stream to tracker: %w", err)
	}
	c.trackerStream = s
	c.encoder = json.NewEncoder(s)
	c.decoder = json.NewDecoder(s)

	// Send name and listen addresses in handshake
	addrs := c.host.Addrs()
	addrStrings := make([]string, len(addrs))
	for i, addr := range addrs {
		addrStrings[i] = addr.String()
	}

	handshakePayload, _ := json.Marshal(p2p.HandshakePayload{
		Name:        c.peerName,
		ListenAddrs: addrStrings,
	})
	msg := p2p.Message{Command: "HANDSHAKE", Payload: handshakePayload}

	if err := c.encoder.Encode(msg); err != nil {
		return fmt.Errorf("failed to send handshake to tracker: %w", err)
	}

	//tracker se welcome message ka wait karte hai after sending the handshake
	var welcomeMsg p2p.Message
	if err := c.decoder.Decode(&welcomeMsg); err != nil {
		return fmt.Errorf("failed to read welcome message from tracker: %w", err)
	}
	log.Printf("✅ Tracker handshake complete. Welcome message: %s", welcomeMsg.Command)
	return nil
}

// traker se online peers ki list request karta hai
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
			addr := peer.Multiaddrs
			fmt.Printf(" %s\n", addr[0])
			fmt.Println("----------------------------------------")
		}
	}
	return nil
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
	})

	// Send the ANNOUNCE_FILE command to the tracker.
	if err := c.encoder.Encode(p2p.Message{Command: "ANNOUNCE_FILE", Payload: payload}); err != nil {
		return err
	}

	var resp p2p.Message
	if err := c.decoder.Decode(&resp); err != nil {
		return err
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

	fmt.Println("\nAvailable Files:")
	for _, file := range files {
		fmt.Println("--------------------")
		fmt.Printf("  ID: %s\n  Name: %s\n  Size: %s\n", file.ID, file.Filename, torrentiumWebRTC.FormatFileSize(file.FileSize))
	}
	fmt.Println("--------------------")
	return nil
}

// get function ek file ko download karne ka process shuru karta hai.
func (c *Client) get(fileIDStr string, outputPath string) error {

	// pehle, fileID parse karte hai to loacate and identify the file
	fileID, err := uuid.Parse(fileIDStr)
	if err != nil {
		return fmt.Errorf("invalid file ID format: %w", err)
	}

	// then, tracker se un peers ki list fetch kartee hai jinke paas woh file hai (fileID linked with peerID in peer_files)
	payload, _ := json.Marshal(p2p.GetPeersPayload{FileID: fileID})
	if err := c.encoder.Encode(p2p.Message{Command: "GET_PEERS_FOR_FILE", Payload: payload}); err != nil {
		return fmt.Errorf("failed to send GET_PEERS_FOR_FILE request: %w", err)
	}

	var resp p2p.Message
	if err := c.decoder.Decode(&resp); err != nil {
		return fmt.Errorf("failed to decode tracker response for peers: %w", err)
	}
	if resp.Command != "PEER_LIST" {
		return fmt.Errorf("tracker responded with an error instead of a peer list: %s", resp.Payload)
	}

	var peers []db.PeerFile
	if err := json.Unmarshal(resp.Payload, &peers); err != nil {
		return fmt.Errorf("failed to unmarshal peer list: %w", err)
	}
	if len(peers) == 0 {
		return errors.New("no online peers found for this file")
	}

	//abhi ke liye host ko choice de rhe hai from whom to download the file from
	//but isme trust score logic implement karna hai
	fmt.Println("Found online peers:")
	for i, p := range peers {
		fmt.Printf("  [%d] Peer DB ID: %s (Score: %.2f)\n", i, p.PeerID, p.Score)
	}
	fmt.Print("Select a peer to download from (enter number): ")
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return errors.New("failed to read user input for peer selection")
	}
	choice, err := strconv.Atoi(scanner.Text())
	if err != nil || choice < 0 || choice >= len(peers) {
		return errors.New("invalid selection")
	}
	selectedPeer := peers[choice]

	//jis peer ko select kiya hai, uski multiaddress info fetch karte hai
	peerInfoPayload, _ := json.Marshal(p2p.GetPeerInfoPayload{PeerDBID: selectedPeer.PeerID})
	if err := c.encoder.Encode(p2p.Message{Command: "GET_PEER_INFO", Payload: peerInfoPayload}); err != nil {
		return fmt.Errorf("failed to send GET_PEER_INFO request: %w", err)
	}
	if err := c.decoder.Decode(&resp); err != nil || resp.Command != "PEER_INFO" {
		return errors.New("could not retrieve peer's libp2p info from tracker")
	}
	var peerInfo db.Peer
	if err := json.Unmarshal(resp.Payload, &peerInfo); err != nil {
		return fmt.Errorf("failed to unmarshal peer info: %w", err)
	}

	// uss multiaddress ko apne host ki peerStore mein local session ke liye save kar dete hai
	targetPeerID, err := peer.Decode(peerInfo.PeerID)
	if err != nil {
		return fmt.Errorf("could not decode peer's libp2p ID: %w", err)
	}

	if len(peerInfo.Multiaddrs) == 0 {
		return fmt.Errorf("peer %s has no listed multiaddrs", targetPeerID)
	}
	maddr, err := multiaddr.NewMultiaddr(peerInfo.Multiaddrs[0])
	if err != nil {
		return fmt.Errorf("could not parse peer's multiaddress '%s': %w", peerInfo.Multiaddrs[0], err)
	}

	// This is the crucial step that prevents the "no addresses" error.
	c.host.Peerstore().AddAddr(targetPeerID, maddr, peerstore.TempAddrTTL)
	log.Printf("Added peer %s with address %s to local peerstore", targetPeerID, maddr)

	// ab jab peer ka address pta hai toh webRTC connection initiate kardenge
	log.Println("Initiating WebRTC connection...")
	webRTCPeer, err := c.initiateWebRTCConnection(targetPeerID)
	if err != nil {
		return fmt.Errorf("WebRTC connection failed: %w", err)
	}
	c.addWebRTCPeer(targetPeerID, webRTCPeer)
	log.Println("WebRTC connection established")

	outputFile, err := os.Create(outputPath)
	if err != nil {
		webRTCPeer.Close()
		return fmt.Errorf("failed to create output file: %w", err)
	}
	webRTCPeer.SetFileWriter(outputFile)

	//file download request send karenge over the established data channel
	log.Printf("Requesting file %s from peer...", fileID)
	requestPayload := map[string]string{"command": "REQUEST_FILE", "file_id": fileID.String()}
	if err := webRTCPeer.Send(requestPayload); err != nil {
		webRTCPeer.Close()
		return fmt.Errorf("failed to send file request: %w", err)
	}

	fmt.Println("File request sent. Waiting for transfer to complete...")
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
