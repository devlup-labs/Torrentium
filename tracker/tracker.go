package tracker

import (
	"context"
	"log"
	"sync"
	"torrentium/db"

	"github.com/google/uuid"
)

type Tracker struct {
	peers    map[string]bool
	repo     *db.Repository
	peersMux sync.RWMutex
}

func NewTracker() *Tracker {
	return &Tracker{
		peers: make(map[string]bool),
		repo:  db.NewRepository(db.DB),
	}
}

func (t *Tracker) AddPeer(ctx context.Context, peerID, name string, multiaddrs []string) error {
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

func (t *Tracker) RemovePeer(ctx context.Context, peerID string) {
	t.peersMux.Lock()
	delete(t.peers, peerID)
	t.peersMux.Unlock()

	if err := t.repo.SetPeerOffline(ctx, peerID); err != nil {
		log.Printf("Failed to set peer %s offline in DB: %v", peerID, err)
	}
}

func (t *Tracker) ListPeers() []string {
	t.peersMux.RLock()
	defer t.peersMux.RUnlock()

	list := make([]string, 0, len(t.peers))
	for peer := range t.peers {
		list = append(list, peer)
	}
	return list
}

func (t *Tracker) GetOnlinePeers(ctx context.Context) ([]db.Peer, error) {
	return t.repo.FindOnlinePeers(ctx)
}

func (t *Tracker) GetOnlinePeersForFile(ctx context.Context, fileID uuid.UUID) ([]db.PeerFile, error) {
	return t.repo.FindOnlineFilePeersByID(ctx, fileID)
}

func (t *Tracker) GetAllFiles(ctx context.Context) ([]db.File, error) {
	return t.repo.FindAllFiles(ctx)
}

func (t *Tracker) AddFileWithPeer(ctx context.Context, fileHash, filename string, fileSize int64, peerID string) (uuid.UUID, error) {
	fileID, err := t.repo.InsertFile(ctx, fileHash, filename, fileSize, "")
	if err != nil {
		return uuid.Nil, err
	}
	_, err = t.repo.InsertPeerFile(ctx, peerID, fileID)
	if err != nil {
		return uuid.Nil, err
	}

	return fileID, nil
}

func (t *Tracker) GetPeerInfoByDBID(ctx context.Context, peerDBID uuid.UUID) (*db.Peer, error) {
	return t.repo.GetPeerInfoByDBID(ctx, peerDBID)
}