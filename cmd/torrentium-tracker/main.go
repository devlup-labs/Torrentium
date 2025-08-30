package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"

	"torrentium/internal/db"
	"torrentium/internal/p2p"
	tracker "torrentium/internal/tracker"

	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now
	},
}

// ConnectionManager manages WebSocket connections by peer ID
type ConnectionManager struct {
	connections map[string]*websocket.Conn
	mu          sync.RWMutex
}

func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		connections: make(map[string]*websocket.Conn),
	}
}

func (cm *ConnectionManager) AddConnection(peerID string, conn *websocket.Conn) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.connections[peerID] = conn
}

func (cm *ConnectionManager) RemoveConnection(peerID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	delete(cm.connections, peerID)
}

func (cm *ConnectionManager) GetConnection(peerID string) (*websocket.Conn, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	conn, exists := cm.connections[peerID]
	return conn, exists
}

func (cm *ConnectionManager) SendToConnection(peerID string, msg p2p.Message) error {
	conn, exists := cm.GetConnection(peerID)
	if !exists {
		return nil // Connection not found
	}
	return conn.WriteJSON(msg)
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Unable to access .env file")
	}

	// Initialize database
	db.InitDB()

	// Clear stale peer statuses
	repo := db.NewRepository(db.DB)
	if err := repo.MarkAllPeersOffline(context.Background()); err != nil {
		log.Printf("Warning: Could not mark all peers offline on startup: %v", err)
	}
	log.Println("-> Cleared stale online peer statuses.")

	// Get WebSocket listen address
	wsAddr := os.Getenv("TRACKER_WS_ADDR")
	if wsAddr == "" {
		wsAddr = ":8080" // Default WebSocket port
	}

	// Create tracker instance
	t := tracker.NewTracker()
	log.Println("-> Tracker Initialized")

	// Create connection manager
	cm := NewConnectionManager()

	// Setup WebSocket handler
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handleWebSocketConnection(w, r, t, cm)
	})

	log.Printf("-> WebSocket tracker listening on %s", wsAddr)
	log.Fatal(http.ListenAndServe(wsAddr, nil))
}

func handleWebSocketConnection(w http.ResponseWriter, r *http.Request, t *tracker.Tracker, cm *ConnectionManager) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	log.Println("New WebSocket connection established")

	var connectedPeerID string // Track which peer this connection belongs to

	// Handle the connection
	for {
		var msg p2p.Message
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			break
		}

		log.Printf("Received message: Command=%s", msg.Command)

		// Handle file chunks specially - forward them to the requester
		if msg.Command == "FILE_CHUNK" {
			handleFileChunk(msg, cm)
			continue
		}

		response := handleTrackerMessage(msg, t, cm)
		log.Printf("Sending response: Command=%s", response.Command)

		// Track the peer ID after successful handshake
		if msg.Command == "HANDSHAKE" && response.Command == "WELCOME" {
			var payload p2p.HandshakePayload
			if err := json.Unmarshal(msg.Payload, &payload); err == nil {
				connectedPeerID = payload.PeerID
				cm.AddConnection(connectedPeerID, conn)
				log.Printf("Tracked connection for peer: %s", connectedPeerID)
			}
		}

		if err := conn.WriteJSON(response); err != nil {
			log.Printf("WebSocket write error: %v", err)
			break
		}
	}

	// Mark peer as offline when connection closes
	if connectedPeerID != "" {
		log.Printf("Marking peer %s as offline due to connection close", connectedPeerID)
		cm.RemoveConnection(connectedPeerID)
		t.RemovePeer(connectedPeerID)
	}

	log.Println("WebSocket connection closed")
}

// handleFileChunk forwards file chunks to the requesting peer
func handleFileChunk(msg p2p.Message, cm *ConnectionManager) {
	var chunkPayload p2p.FileTransferPayload
	if err := json.Unmarshal(msg.Payload, &chunkPayload); err != nil {
		log.Printf("Error unmarshaling file chunk: %v", err)
		return
	}

	log.Printf("Received file chunk %d for FileID %s, forwarding to requester", chunkPayload.ChunkIndex, chunkPayload.FileID)

	// Forward chunk to the requester peer
	// For simplicity, we'll broadcast to all connections and let the client filter
	// In a production system, you'd track active transfers properly
	cm.mu.RLock()
	for peerID, conn := range cm.connections {
		if peerID != "" { // Don't send back to sender
			if err := conn.WriteJSON(msg); err != nil {
				log.Printf("Failed to forward chunk to peer %s: %v", peerID, err)
			}
		}
	}
	cm.mu.RUnlock()
}

func handleTrackerMessage(msg p2p.Message, t *tracker.Tracker, cm *ConnectionManager) p2p.Message {
	log.Printf("Processing command: %s", msg.Command)
	switch msg.Command {
	case "HANDSHAKE":
		var payload p2p.HandshakePayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			log.Printf("Handshake unmarshal error: %v", err)
			return p2p.Message{Command: "ERROR", Payload: json.RawMessage(`"Invalid handshake payload"`)}
		}

		log.Printf("Handshake from peer: %s (ID: %s)", payload.Name, payload.PeerID)
		// Add peer to tracker
		if err := t.AddPeer(payload.PeerID, payload.Name, "ws-connection"); err != nil {
			log.Printf("AddPeer error: %v", err)
			return p2p.Message{Command: "ERROR", Payload: json.RawMessage(`"Failed to add peer"`)}
		}

		log.Printf("Peer %s added successfully", payload.Name)
		return p2p.Message{Command: "WELCOME", Payload: json.RawMessage(`"Connected to tracker"`)}

	case "LIST_PEERS":
		peers, err := t.GetConnectedPeersDetails(context.Background())
		if err != nil {
			log.Printf("GetConnectedPeersDetails error: %v", err)
			return p2p.Message{Command: "ERROR", Payload: json.RawMessage(`"Failed to get peers"`)}
		}
		peersJSON, _ := json.Marshal(peers)
		return p2p.Message{Command: "PEER_LIST_ALL", Payload: peersJSON}

	case "ANNOUNCE_FILE":
		var payload p2p.AnnounceFilePayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			log.Printf("ANNOUNCE_FILE unmarshal error: %v", err)
			return p2p.Message{Command: "ERROR", Payload: json.RawMessage(`"Invalid announce payload"`)}
		}

		log.Printf("Announcing file: %s, hash: %s, size: %d, peer: %s",
			payload.Filename, payload.FileHash, payload.FileSize, payload.PeerID)

		// Process file announcement
		fileID, err := t.AnnounceFile(payload.FileHash, payload.Filename, payload.FileSize, payload.PeerID)
		if err != nil {
			log.Printf("AnnounceFile error: %v", err)
			return p2p.Message{Command: "ERROR", Payload: json.RawMessage(`"Failed to announce file"`)}
		}

		log.Printf("File announced successfully with ID: %s", fileID)
		ackPayload, _ := json.Marshal(p2p.AnnounceAckPayload{FileID: fileID})
		return p2p.Message{Command: "ACK", Payload: ackPayload}

	case "LIST_FILES":
		log.Printf("Listing files requested")
		files := t.ListFiles()
		log.Printf("Found %d files in database", len(files))
		for i, file := range files {
			log.Printf("File %d: ID=%s, Name=%s, Hash=%s", i+1, file.ID, file.Filename, file.FileHash)
		}
		filesJSON, _ := json.Marshal(files)
		return p2p.Message{Command: "FILE_LIST", Payload: filesJSON}

	case "GET_PEERS_FOR_FILE":
		var payload p2p.GetPeersPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return p2p.Message{Command: "ERROR", Payload: json.RawMessage(`"Invalid get peers payload"`)}
		}

		peers := t.GetPeersForFile(payload.FileID)
		peersJSON, _ := json.Marshal(peers)
		return p2p.Message{Command: "PEER_LIST", Payload: peersJSON}

	case "GET_PEER_INFO":
		var payload p2p.GetPeerInfoPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return p2p.Message{Command: "ERROR", Payload: json.RawMessage(`"Invalid get peer info payload"`)}
		}

		peer := t.GetPeerInfo(payload.PeerDBID)
		if peer == nil {
			return p2p.Message{Command: "ERROR", Payload: json.RawMessage(`"Peer not found"`)}
		}

		peerJSON, _ := json.Marshal(peer)
		return p2p.Message{Command: "PEER_INFO", Payload: peerJSON}

	case "REQUEST_FILE":
		var payload p2p.RequestFilePayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			log.Printf("REQUEST_FILE unmarshal error: %v", err)
			return p2p.Message{Command: "ERROR", Payload: json.RawMessage(`"Invalid request file payload"`)}
		}

		log.Printf("File request: FileID=%s, RequesterPeerID=%s", payload.FileID, payload.RequesterPeerID)

		// Find peers who have this file
		peers := t.GetPeersForFile(payload.FileID)
		if len(peers) == 0 {
			return p2p.Message{Command: "ERROR", Payload: json.RawMessage(`"No peers found for this file"`)}
		}

		// For now, request from the first available peer
		selectedPeer := peers[0]

		// Get peer info to find their peer_id
		peerInfo, err := t.GetPeerInfoByDBID(context.Background(), selectedPeer.PeerID)
		if err != nil {
			return p2p.Message{Command: "ERROR", Payload: json.RawMessage(`"Could not get peer info"`)}
		}

		// Check if the peer is connected via WebSocket
		if !t.IsPeerConnected(peerInfo.PeerID) {
			return p2p.Message{Command: "ERROR", Payload: json.RawMessage(`"Peer is not online"`)}
		}

		log.Printf("Requesting file %s from peer %s for requester %s", payload.FileID, peerInfo.PeerID, payload.RequesterPeerID)

		// Send file request to the peer who has the file
		fileRequestMsg := p2p.Message{
			Command: "REQUEST_FILE",
			Payload: msg.Payload, // Forward the same payload
		}

		// Send request to the file owner
		if err := cm.SendToConnection(peerInfo.PeerID, fileRequestMsg); err != nil {
			log.Printf("Failed to send file request to peer %s: %v", peerInfo.PeerID, err)
			return p2p.Message{Command: "ERROR", Payload: json.RawMessage(`"Failed to contact peer"`)}
		}

		return p2p.Message{Command: "FILE_REQUEST_INITIATED", Payload: json.RawMessage(`"File request sent to peer"`)}

	default:
		return p2p.Message{Command: "ERROR", Payload: json.RawMessage(`"Unknown command"`)}
	}
}
