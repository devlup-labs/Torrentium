package types

import (
	"os"
	"sync"
	"time"

	db "torrentium/internal/db"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/schollz/progressbar/v3"
)

const (
	DefaultPieceSize       = 1 << 20 // 1 MiB pieces
	MaxProviders           = 10
	MaxChunk               = 16 * 1024 // 16KiB chunks
	MaxParallelDownloads   = 3
	PieceTimeout           = 300 * time.Second // Timeout for downloading a single piece
	RetransmissionTimeout  = 5 * time.Second
	KeepAliveInterval      = 15 * time.Second
	PingInterval           = 10 * time.Second
	MaxRTT                 = 500 * time.Millisecond
	MinDelay               = 0
	MaxDelay               = 100 * time.Millisecond
	ExponentialBackoffBase = 1 * time.Second
	MaxBackoff             = 32 * time.Second
)

type FileInfo struct {
	FilePath string
	Hash     string
	Size     int64
	Name     string
	PieceSz  int64
}

type ControlMessage struct {
	Command     string     `json:"command"`
	CID         string     `json:"cid,omitempty"`
	PieceSize   int64      `json:"piece_size,omitempty"`
	TotalSize   int64      `json:"total_size,omitempty"`
	HashHex     string     `json:"hash_hex,omitempty"`
	NumPieces   int64      `json:"num_pieces,omitempty"`
	Pieces      []db.Piece `json:"pieces,omitempty"`
	PieceHash   string     `json:"piece_hash,omitempty"`
	Index       int64      `json:"index,omitempty"`
	Filename    string     `json:"filename,omitempty"`
	ChunkIndex  int        `json:"chunk_index,omitempty"`
	TotalChunks int        `json:"total_chunks,omitempty"`
	Payload     string     `json:"payload,omitempty"`
	Sequence    int        `json:"sequence,omitempty"`
}

type DownloadState struct {
	File            *os.File
	Manifest        ControlMessage
	TotalPieces     int
	Pieces          []db.Piece
	Completed       chan bool
	Progress        *progressbar.ProgressBar
	PieceStatus     []bool // true if piece is downloaded
	PieceAssignees  map[int]peer.ID
	PieceBuffers    map[int][][]byte // Buffer to reassemble chunks into pieces
	Mu              sync.Mutex
	CompletedPieces int
	PieceTimers     map[int]*time.Timer // Timers for each piece
	RetryCounts     map[int]int         // Retry counts for exponential backoff
}
