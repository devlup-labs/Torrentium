package core

import (
	"sync"
	"time"

	webRTC "torrentium/internal/client"
	db "torrentium/internal/db"
	"torrentium/internal/types"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

type Client struct {
	Host             host.Host
	DHT              *dht.IpfsDHT
	WebRTCPeers      map[peer.ID]*webRTC.SimpleWebRTCPeer
	PeersMux         sync.RWMutex
	SharingFiles     map[string]*types.FileInfo
	ActiveDownloads  map[string]*types.DownloadState
	DownloadsMux     sync.RWMutex
	DB               *db.Repository
	UnackedChunks    map[string]map[int64]map[int]types.ControlMessage
	UnackedChunksMux sync.RWMutex
	CongestionCtrl   map[peer.ID]time.Duration
	PingTimes        map[peer.ID]time.Time
	RTTMeasurements  map[peer.ID][]time.Duration
	RTTMux           sync.Mutex

	ManifestWaiters map[string]chan types.ControlMessage
	ManifestChMu    sync.Mutex
}

func NewClient(h host.Host, d *dht.IpfsDHT, repo *db.Repository) *Client {
	c := &Client{
		Host:            h,
		DHT:             d,
		WebRTCPeers:     make(map[peer.ID]*webRTC.SimpleWebRTCPeer),
		SharingFiles:    make(map[string]*types.FileInfo),
		ActiveDownloads: make(map[string]*types.DownloadState),
		DB:              repo,
		UnackedChunks:   make(map[string]map[int64]map[int]types.ControlMessage),
		CongestionCtrl:  make(map[peer.ID]time.Duration),
		PingTimes:       make(map[peer.ID]time.Time),
		RTTMeasurements: make(map[peer.ID][]time.Duration),
		ManifestWaiters: make(map[string]chan types.ControlMessage),
	}
	go c.MonitorCongestion()
	return c
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
