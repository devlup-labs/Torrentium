package db

import (
	"context"

	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx"
	"github.com/jackc/pgx/v5/pgxpool"
)

//repository struct mein saare DB operations hai
type Repository struct {
	DB *pgxpool.Pool
}

//ek naya repo bna rha hai (say for a new user)
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{DB: db}
}


// yeh combined function hai insert + update = upsert (insert new peer and update if already exists)
func (r *Repository) UpsertPeer(ctx context.Context, peerID, name string, multiaddrs []string) (uuid.UUID, error) {
	now := time.Now()
	var peerUUID uuid.UUID

	query := `
        WITH ins AS (
            INSERT INTO peers (peer_id, name, multiaddrs, is_online, last_seen, created_at)
            VALUES ($1, $2, $3, true, $4, $4)
            ON CONFLICT (peer_id) DO UPDATE
            SET is_online = true, last_seen = $4, multiaddrs = $3
            RETURNING id, (created_at = $4) as is_new
        ),
        ts AS (
            INSERT INTO trust_scores (peer_id, score, updated_at)
            SELECT id, 0.50, $4 FROM ins
            WHERE ins.is_new = true
            ON CONFLICT (peer_id) DO NOTHING
        )
        SELECT id FROM ins;
    `
	err := r.DB.QueryRow(ctx, query, peerID, name, multiaddrs, now).Scan(&peerUUID)
	if err != nil {
		log.Printf("[Repository] UpsertPeer error for peerID %s: %v", peerID, err)
		return uuid.Nil, err
	}
	return peerUUID, nil
}


//currently online peers ko return karta hai
func (r *Repository) FindOnlinePeers(ctx context.Context) ([]Peer, error) {
	query := `SELECT id, peer_id, name, multiaddrs, is_online, last_seen, created_at FROM peers WHERE is_online = true`
	rows, err := r.DB.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var peers []Peer
	for rows.Next() {
		var peer Peer
		if err := rows.Scan(&peer.ID, &peer.PeerID, &peer.Name, &peer.Multiaddrs, &peer.IsOnline, &peer.LastSeen, &peer.CreatedAt); err != nil {
			return nil, err
		}
		peers = append(peers, peer)
	}
	return peers, rows.Err()
}

// Jab koi peer disconnect kare, use offline mark karne ke liye
func (r *Repository) SetPeerOffline(ctx context.Context, peerID string) error {
	now := time.Now()
	_, err := r.DB.Exec(ctx,
		`UPDATE peers SET is_online=false, last_seen=$1 WHERE peer_id=$2`,
		now, peerID)
	return err
}

// Server start hone par sab peers ko offline mark karta hai 
// tracker ko jab host karenge aur restart karna pada toh saare peeer disconnect ho jaenge aur offline mark ho jaenge
func (r *Repository) MarkAllPeersOffline(ctx context.Context) error {
	_, err := r.DB.Exec(ctx, `UPDATE peers SET is_online=false WHERE is_online=true`)
	return err
}


// Peer ki full info return karta hai DB ID ke basis par
func (r *Repository) GetPeerInfoByDBID(ctx context.Context, peerDBID uuid.UUID) (*Peer, error) {
	var peer Peer
	err := r.DB.QueryRow(ctx, `SELECT id, peer_id, name, multiaddrs, is_online, last_seen, created_at FROM peers WHERE id = $1`, peerDBID).Scan(&peer.ID, &peer.PeerID, &peer.Name, &peer.Multiaddrs, &peer.IsOnline, &peer.LastSeen, &peer.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &peer, nil
}



// File ko DB mein insert karta hai (hash, size, type etc. ke saath)
// Aur agar file peehle se exit kar rhi hai toh name update kar dega (hash compare karne ke baad)
func (r *Repository) InsertFile(ctx context.Context, fileHash, filename string, fileSize int64, contentType string) (uuid.UUID, error) {
	var fileID uuid.UUID
	err := r.DB.QueryRow(ctx, `SELECT id FROM files WHERE file_hash = $1`, fileHash).Scan(&fileID)
	if err == nil {
		return fileID, nil // File exists
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, err 
	}

	// File does not exist, insert it and return the new ID.
	err = r.DB.QueryRow(ctx,
		`INSERT INTO files (file_hash, filename, file_size, content_type, created_at) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		fileHash, filename, fileSize, contentType, time.Now()).Scan(&fileID)
	return fileID, err
}


// Tracker par available saari files ka list deta hai
func (r *Repository) FindAllFiles(ctx context.Context) ([]File, error) {
	query := `SELECT id, file_hash, filename, file_size, content_type, created_at FROM files ORDER BY created_at DESC`
	rows, err := r.DB.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []File
	for rows.Next() {
		var file File
		if err := rows.Scan(&file.ID, &file.FileHash, &file.Filename, &file.FileSize, &file.ContentType, &file.CreatedAt); err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, rows.Err()
}


//peer ki chosen file tracker pe register karta hai
//peer_id + file_id ka combination unique relation store hota hai
func (r *Repository) InsertPeerFile(ctx context.Context, peerLibp2pID string, fileID uuid.UUID) (uuid.UUID, error) {
	var peerUUID uuid.UUID
	err := r.DB.QueryRow(ctx, `SELECT id FROM peers WHERE peer_id = $1`, peerLibp2pID).Scan(&peerUUID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to find peer with peer_id=%s: %w", peerLibp2pID, err)
	}

	var peerFileID uuid.UUID
	query := `
        INSERT INTO peer_files (peer_id, file_id, announced_at)
        VALUES ($1, $2, $3)
        ON CONFLICT (peer_id, file_id) DO NOTHING
        RETURNING id
    `
	err = r.DB.QueryRow(ctx, query, peerUUID, fileID, time.Now()).Scan(&peerFileID)
	if err != nil && err.Error() == "no rows in result set" {
		err = r.DB.QueryRow(ctx, `SELECT id FROM peer_files WHERE peer_id = $1 AND file_id = $2`, peerUUID, fileID).Scan(&peerFileID)
	}

	return peerFileID, err
}

// Kisi file ke liye saare online peers dikhata hai (abhi ke liye basic trust score dikhata hai)
func (r *Repository) FindOnlineFilePeersByID(ctx context.Context, fileID uuid.UUID) ([]PeerFile, error) {
	query := `
        SELECT pf.id, pf.file_id, pf.peer_id, pf.announced_at, COALESCE(ts.score, 0.5) as score
        FROM peer_files pf
        JOIN peers p ON pf.peer_id = p.id
        LEFT JOIN trust_scores ts ON p.id = ts.peer_id
        WHERE pf.file_id = $1 AND p.is_online = true
    `
	rows, err := r.DB.Query(ctx, query, fileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var peerFiles []PeerFile
	for rows.Next() {
		var pfile PeerFile
		if err := rows.Scan(&pfile.ID, &pfile.FileID, &pfile.PeerID, &pfile.AnnouncedAt, &pfile.Score); err != nil {
			return nil, err
		}
		peerFiles = append(peerFiles, pfile)
	}
	return peerFiles, rows.Err()
}
