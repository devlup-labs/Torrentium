package db

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	DB *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{DB: db}
}

func (r *Repository) UpsertPeer(ctx context.Context, peerID, name string, multiaddrs []string) (uuid.UUID, error) {
	now := time.Now()
	var peerUUID uuid.UUID

	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("could not begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `SELECT id FROM peers WHERE peer_id = $1`
	err = tx.QueryRow(ctx, query, peerID).Scan(&peerUUID)

	if err == nil {
		updateQuery := `UPDATE peers SET is_online = true, last_seen = $1, multiaddrs = $2 WHERE id = $3`
		_, updateErr := tx.Exec(ctx, updateQuery, now, multiaddrs, peerUUID)
		if updateErr != nil {
			return uuid.Nil, fmt.Errorf("failed to update existing peer: %w", updateErr)
		}
	} else if errors.Is(err, pgx.ErrNoRows) {
		insertPeerQuery := `
            INSERT INTO peers (peer_id, name, multiaddrs, is_online, last_seen, created_at)
            VALUES ($1, $2, $3, true, $4, $4)
            RETURNING id
        `
		insertErr := tx.QueryRow(ctx, insertPeerQuery, peerID, name, multiaddrs, now).Scan(&peerUUID)
		if insertErr != nil {
			return uuid.Nil, fmt.Errorf("failed to insert new peer: %w", insertErr)
		}

		insertTrustQuery := `INSERT INTO trust_scores (peer_id, score, updated_at) VALUES ($1, 0.50, $2)`
		_, trustErr := tx.Exec(ctx, insertTrustQuery, peerUUID, now)
		if trustErr != nil {
			return uuid.Nil, fmt.Errorf("failed to insert trust score: %w", trustErr)
		}
	} else {
		return uuid.Nil, fmt.Errorf("failed to check for peer existence: %w", err)
	}

	return peerUUID, tx.Commit(ctx)
}

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

func (r *Repository) SetPeerOffline(ctx context.Context, peerID string) error {
	now := time.Now()
	_, err := r.DB.Exec(ctx,
		`UPDATE peers SET is_online=false, last_seen=$1 WHERE peer_id=$2`,
		now, peerID)
	return err
}

func (r *Repository) MarkAllPeersOffline(ctx context.Context) error {
	_, err := r.DB.Exec(ctx, `UPDATE peers SET is_online=false WHERE is_online=true`)
	return err
}

func (r *Repository) GetPeerInfoByDBID(ctx context.Context, peerDBID uuid.UUID) (*Peer, error) {
	var peer Peer
	err := r.DB.QueryRow(ctx, `SELECT id, peer_id, name, multiaddrs, is_online, last_seen, created_at FROM peers WHERE id = $1`, peerDBID).Scan(&peer.ID, &peer.PeerID, &peer.Name, &peer.Multiaddrs, &peer.IsOnline, &peer.LastSeen, &peer.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &peer, nil
}

func (r *Repository) InsertFile(ctx context.Context, fileHash, filename string, fileSize int64, contentType string) (uuid.UUID, error) {
	var fileID uuid.UUID
	err := r.DB.QueryRow(ctx, `SELECT id FROM files WHERE file_hash = $1`, fileHash).Scan(&fileID)
	if err == nil {
		return fileID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, err
	}

	err = r.DB.QueryRow(ctx,
		`INSERT INTO files (file_hash, filename, file_size, content_type, created_at) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		fileHash, filename, fileSize, contentType, time.Now()).Scan(&fileID)
	return fileID, err
}

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

func (r *Repository) InsertPeerFile(ctx context.Context, peerLibp2pID string, fileID uuid.UUID) (uuid.UUID, error) {
	var peerUUID uuid.UUID
	err := r.DB.QueryRow(ctx, `SELECT id FROM peers WHERE peer_id = $1`, peerLibp2pID).Scan(&peerUUID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to find peer with peer_id=%s: %w", peerLibp2pID, err)
	}
	var peerFileID uuid.UUID
	query := `
        WITH ins AS (
            INSERT INTO peer_files (peer_id, file_id, announced_at)
            VALUES ($1, $2, $3)
            ON CONFLICT (peer_id, file_id) DO NOTHING
            RETURNING id
        )
        SELECT id FROM ins
        UNION ALL
        SELECT id FROM peer_files WHERE peer_id = $1 AND file_id = $2 AND NOT EXISTS (SELECT 1 FROM ins)
    `
	err = r.DB.QueryRow(ctx, query, peerUUID, fileID, time.Now()).Scan(&peerFileID)
	if err != nil {
		log.Printf("[Repository] InsertPeerFile error for peerUUID %s and fileID %s: %v", peerUUID, fileID, err)
		return uuid.Nil, err
	}

	return peerFileID, nil
}

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