package main

import (
	
	"time"
)

// weight struc
type Weights struct {
	SuccessfulChunks  float64
	UploadSpeed       float64
	SeedLeechRatio    float64
	SuccessRate       float64
	ChunkAvailability float64
	OnlineTime        float64
}

// peersData struct
type PeerData struct {
	LeechedData             float64//seeder
	SeededData              float64//seeder
	SinceLastSeen           time.Time//seeder
	AverageUploadSpeed      int//leecher
	AverageOnlineTimePerDay int//seeder
	TotalFileChunksUploaded int//seeder
	CurrTraSuccChunk        int//leecher
	CurrTraTotalChunk       int//leecher
	OriginalChunksShared    int//seeder
	NoOfSuccChunksLastK     int//seeder
	TransSucc               bool//leecher
}

type Friend struct {
	ID         int     `json:"id"`
	Name       string  `json:"name"`
	TrustScore float64 `json:"trust_score"`
}

