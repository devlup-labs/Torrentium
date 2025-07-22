package tracker

import (
	"context"
	"log"
	"sync"
	"torrentium/db"

	"github.com/google/uuid"
)

type Tracker struct {
	peers    map[string]bool // (In-memory map )jo currently connected peers hai unke IDs ko store karta hai.
	repo     *db.Repository
	peersMux sync.RWMutex  // peers map ko concurrency clashes se bachane ke liye reead and write Mutex.
}

//ek naya tracker instance initialize karte hai
func NewTracker() *Tracker {
	return &Tracker{
		peers: make(map[string]bool),
		repo:  db.NewRepository(db.DB),
	}
}

//Yeh peer ko in-memory list mein aur database mein (upsert) add karta hai.
func (t *Tracker) AddPeer(ctx context.Context, peerID, name string, multiaddrs []string) error {
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


//Yeh use in-memory list se delete karta hai aur database mein offline mark karta hai.
func (t *Tracker) RemovePeer(ctx context.Context, peerID string) {
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
