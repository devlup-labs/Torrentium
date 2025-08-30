package tracker

import (
	"context"
	"log"
	"sync"
	"torrentium/internal/db"

	"github.com/google/uuid"
)

type Tracker struct {
	peers    map[string]bool // (In-memory map )jo currently connected peers hai unke IDs ko store karta hai.
	repo     *db.Repository
	peersMux sync.RWMutex // peers map ko concurrency clashes se bachane ke liye reead and write Mutex.
}

// ek naya tracker instance initialize karte hai
func NewTracker() *Tracker {
	return &Tracker{
		peers: make(map[string]bool),
		repo:  db.NewRepository(db.DB),
	}
}

// Yeh peer ko in-memory list mein aur database mein (upsert) add karta hai.
func (t *Tracker) AddPeer(peerID, name, ip string) error {
	// Map ko lock karte hain taaki race conditions na ho.
	t.peersMux.Lock()
	t.peers[peerID] = true
	t.peersMux.Unlock()

	ctx := context.Background()
	// For WebSocket connections, we don't have multiaddrs, so pass empty slice
	// UpsertPeer already sets is_online = true for both new and existing peers
	_, err := t.repo.UpsertPeer(ctx, peerID, name, []string{})
	if err != nil {
		log.Printf("Failed to upsert peer %s: %v", peerID, err)
		return err
	}

	log.Printf("Peer %s (%s) added and set online", name, peerID)
	return nil
}

// IsPeerConnected checks if a peer is currently connected via WebSocket
func (t *Tracker) IsPeerConnected(peerID string) bool {
	t.peersMux.RLock()
	connected := t.peers[peerID]
	t.peersMux.RUnlock()
	return connected
}

// Yeh peer ko in-memory list mein aur database mein (upsert) add karta hai. (Context version)
func (t *Tracker) AddPeerWithContext(ctx context.Context, peerID, name string, multiaddrs []string) error {
	// Map ko lock karte hain taaki race conditions na ho.
	t.peersMux.Lock()
	t.peers[peerID] = true
	t.peersMux.Unlock()

	_, err := t.repo.UpsertPeer(ctx, peerID, name, multiaddrs)
	if err != nil {
		log.Printf("Failed to upsert peer %s: %v", peerID, err)
		return err
	}
	return nil
}

// Yeh use in-memory list se delete karta hai aur database mein offline mark karta hai.
func (t *Tracker) RemovePeer(peerID string) {
	t.peersMux.Lock()
	delete(t.peers, peerID)
	t.peersMux.Unlock()

	ctx := context.Background()
	if err := t.repo.SetPeerOffline(ctx, peerID); err != nil {
		log.Printf("Failed to set peer %s offline in DB: %v", peerID, err)
	}
}

// Yeh use in-memory list se delete karta hai aur database mein offline mark karta hai. (Context version)
func (t *Tracker) RemovePeerWithContext(ctx context.Context, peerID string) {
	t.peersMux.Lock()
	delete(t.peers, peerID)
	t.peersMux.Unlock()

	if err := t.repo.SetPeerOffline(ctx, peerID); err != nil {
		log.Printf("Failed to set peer %s offline in DB: %v", peerID, err)
	}
}

// ListPeers in-memory mein store kiye gaye sabhi online peers ke IDs ki list return karta hai.
func (t *Tracker) ListPeers() []string {
	t.peersMux.RLock()
	defer t.peersMux.RUnlock()

	list := make([]string, 0, len(t.peers))
	for peer := range t.peers {
		list = append(list, peer)
	}
	return list
}

// GetConnectedPeersDetails returns detailed information for currently connected peers only
func (t *Tracker) GetConnectedPeersDetails(ctx context.Context) ([]db.Peer, error) {
	// Get list of currently connected peer IDs from memory
	connectedPeerIDs := t.ListPeers()

	if len(connectedPeerIDs) == 0 {
		return []db.Peer{}, nil
	}

	// Get details for these specific peers from database
	return t.repo.FindPeersByIDs(ctx, connectedPeerIDs)
}

// GetOnlinePeers database se sabhi online peers ki details fetch karta hai.
func (t *Tracker) GetOnlinePeers(ctx context.Context) ([]db.Peer, error) {
	return t.repo.FindOnlinePeers(ctx)
}

// GetOnlinePeersForFile ek specific file ke liye sabhi online peers ki list database se fetch karta hai.
func (t *Tracker) GetOnlinePeersForFile(ctx context.Context, fileID uuid.UUID) ([]db.PeerFile, error) {
	return t.repo.FindOnlineFilePeersByID(ctx, fileID)
}

// GetAllFiles database mein available sabhi files ki list return karta hai.
func (t *Tracker) GetAllFiles(ctx context.Context) ([]db.File, error) {
	return t.repo.FindAllFiles(ctx)
}

// AddFileWithPeer ek file ko database mein add karta hai aur use ek peer ke saath link kar deta hai.
func (t *Tracker) AddFileWithPeer(ctx context.Context, fileHash, filename string, fileSize int64, peerID string) (uuid.UUID, error) {
	// Pehle file ko `files` table mein insert karte hain (ya agar exist karti hai to ID get karte hain).
	fileID, err := t.repo.InsertFile(ctx, fileHash, filename, fileSize, "")
	if err != nil {
		return uuid.Nil, err
	}
	// Fir `peer_files` table mein entry banakar file aur peer ko link karte hain.
	_, err = t.repo.InsertPeerFile(ctx, peerID, fileID)
	if err != nil {
		return uuid.Nil, err
	}

	return fileID, nil
}

// GetPeerInfoByDBID database se ek peer ki info uske db ID ka use karke fetch karta hai.
func (t *Tracker) GetPeerInfoByDBID(ctx context.Context, peerDBID uuid.UUID) (*db.Peer, error) {
	return t.repo.GetPeerInfoByDBID(ctx, peerDBID)
}

// WebSocket handler wrapper methods

// AnnounceFile WebSocket handler ke liye wrapper method
func (t *Tracker) AnnounceFile(fileHash, filename string, fileSize int64, peerID string) (uuid.UUID, error) {
	ctx := context.Background()
	return t.AddFileWithPeer(ctx, fileHash, filename, fileSize, peerID)
}

// ListFiles WebSocket handler ke liye wrapper method
func (t *Tracker) ListFiles() []db.File {
	ctx := context.Background()
	files, err := t.GetAllFiles(ctx)
	if err != nil {
		log.Printf("Error getting files: %v", err)
		return []db.File{}
	}
	return files
}

// GetPeersForFile WebSocket handler ke liye wrapper method
func (t *Tracker) GetPeersForFile(fileID uuid.UUID) []db.PeerFile {
	ctx := context.Background()
	peers, err := t.GetOnlinePeersForFile(ctx, fileID)
	if err != nil {
		log.Printf("Error getting peers for file: %v", err)
		return []db.PeerFile{}
	}
	return peers
}

// GetPeerInfo WebSocket handler ke liye wrapper method
func (t *Tracker) GetPeerInfo(peerDBID uuid.UUID) *db.Peer {
	ctx := context.Background()
	peer, err := t.GetPeerInfoByDBID(ctx, peerDBID)
	if err != nil {
		log.Printf("Error getting peer info: %v", err)
		return nil
	}
	return peer
}
