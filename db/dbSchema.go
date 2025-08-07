package db

import (
	"time"

	"github.com/google/uuid"
)

type Peer struct {
	ID         uuid.UUID `db:"id"`
	PeerID     string    `db:"peer_id"`
	Name       string    `db:"name"`
	Multiaddrs []string  `db:"multiaddrs"`
	IsOnline   bool      `db:"is_online"`
	LastSeen   time.Time `db:"last_seen"`
	CreatedAt  time.Time `db:"created_at"`
}

type File struct {
	ID          uuid.UUID `db:"id"`
	FileHash    string    `db:"file_hash"`
	Filename    string    `db:"filename"`
	FileSize    int64     `db:"file_size"`
	ContentType string    `db:"content_type"`
	CreatedAt   time.Time `db:"created_at"`
}

type PeerFile struct {
	ID          uuid.UUID `db:"id"`
	PeerID      uuid.UUID `db:"peer_id"`
	FileID      uuid.UUID `db:"file_id"`
	AnnouncedAt time.Time `db:"announced_at"`
	Score       float64   `db:"score"`
}

type TrustScore struct {
	ID                  uuid.UUID `db:"id"`
	PeerID              uuid.UUID `db:"peer_id"`
	Score               float64   `db:"score"`
	SuccessfulTransfers int       `db:"successful_transfers"`
	FailedTransfers     int       `db:"failed_transfers"`
	UpdatedAt           time.Time `db:"updated_at"`
}

type ActiveConnection struct {
	ID          uuid.UUID  `db:"id"`
	RequesterID uuid.UUID  `db:"requester_id"`
	ProviderID  uuid.UUID  `db:"provider_id"`
	FileID      uuid.UUID  `db:"file_id"`
	Status      string     `db:"status"`
	StartedAt   time.Time  `db:"started_at"`
	CompletedAt *time.Time `db:"completed_at"`
}