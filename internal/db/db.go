package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
)

type LocalFile struct {
	ID        string
	CID       string
	Filename  string
	FileSize  int64
	FilePath  string
	FileHash  string
	CreatedAt time.Time
}

type Download struct {
	ID           string
	CID          string
	Filename     string
	FileSize     int64
	DownloadPath string
	DownloadedAt time.Time
	Status       string
}

// Piece tracks chunked pieces for resume and verification
type Piece struct {
	ID        string
	CID       string
	Index     int64
	Offset    int64
	Size      int64
	Hash      string
	Have      bool
	UpdatedAt time.Time
}

// PeerScore stores reputation and trust metrics
type PeerScore struct {
	PeerID                  string    `json:"peer_id"`
	Score                   float64   `json:"score"`
	SeenAt                  time.Time `json:"seen_at"`
	LeechedData             float64   `json:"leeched_data"`
	SeededData              float64   `json:"seeded_data"`
	LastSeen                time.Time `json:"last_seen"`
	AverageUploadSpeed      int       `json:"average_upload_speed"`
	AverageOnlineTimePerDay int       `json:"average_online_time_per_day"`
	TotalFileChunksUploaded int       `json:"total_file_chunks_uploaded"`
	SuccessfulTransfers     int       `json:"successful_transfers"`
	TotalTransfers          int       `json:"total_transfers"`
	OriginalChunksShared    int       `json:"original_chunks_shared"`
	NoOfSuccChunksLastK     int       `json:"no_of_succ_chunks_last_k"`
	CurrentTrustScore       float64   `json:"current_trust_score"` // 0-1 range
}

type Repository struct {
	DB *sql.DB
}

func NewRepository(db *sql.DB) *Repository { return &Repository{DB: db} }

var DB *sql.DB

func InitDB() *sql.DB {
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: Could not load .env file: %v", err)
	}
	dbpath := os.Getenv("SQLITE_DB_PATH")
	if dbpath == "" {
		dbpath = "./peer.db"
	}
	var err error
	DB, err = sql.Open("sqlite3", dbpath)
	if err != nil {
		log.Fatalf("Error creating DB connection: %v", err)
	}
	if err = DB.Ping(); err != nil {
		log.Fatalf("Error connecting to DB: %v", err)
	}
	if err := createTables(DB); err != nil {
		log.Fatalf("Error creating tables: %v", err)
	}
	log.Println("Successfully connected to peer database")
	return DB
}

func createTables(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS local_files (
			id TEXT PRIMARY KEY,
			cid TEXT UNIQUE NOT NULL,
			filename TEXT NOT NULL,
			file_size INTEGER NOT NULL,
			file_path TEXT NOT NULL,
			file_hash TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS downloads (
			id TEXT PRIMARY KEY,
			cid TEXT UNIQUE NOT NULL,
			filename TEXT NOT NULL,
			file_size INTEGER NOT NULL,
			download_path TEXT NOT NULL,
			downloaded_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			status TEXT DEFAULT 'completed'
		);`,
		`CREATE TABLE IF NOT EXISTS pieces (
			id TEXT PRIMARY KEY,
			cid TEXT NOT NULL,
			idx INTEGER NOT NULL,
			offset INTEGER NOT NULL,
			size INTEGER NOT NULL,
			hash TEXT NOT NULL,
			have INTEGER NOT NULL DEFAULT 0,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (cid, idx)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_pieces_cid ON pieces(cid);`,
		`CREATE TABLE IF NOT EXISTS peer_scores (
			peer_id TEXT PRIMARY KEY,
			score REAL NOT NULL,
			seen_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			leeched_data REAL NOT NULL DEFAULT 0,
			seeded_data REAL NOT NULL DEFAULT 0,
			last_seen DATETIME DEFAULT CURRENT_TIMESTAMP,
			average_upload_speed INTEGER NOT NULL DEFAULT 0,
			average_online_time_per_day INTEGER NOT NULL DEFAULT 8,
			total_file_chunks_uploaded INTEGER NOT NULL DEFAULT 0,
			successful_transfers INTEGER NOT NULL DEFAULT 0,
			total_transfers INTEGER NOT NULL DEFAULT 0,
			original_chunks_shared INTEGER NOT NULL DEFAULT 0,
			no_of_succ_chunks_last_k INTEGER NOT NULL DEFAULT 0,
			current_trust_score REAL NOT NULL DEFAULT 0.5
		);`,
		`CREATE TABLE IF NOT EXISTS metadata_index (
			cid TEXT PRIMARY KEY,
			filename TEXT NOT NULL,
			file_size INTEGER NOT NULL,
			file_hash TEXT NOT NULL
		);`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("schema error: %w", err)
		}
	}
	return nil
}

func (r *Repository) AddLocalFile(ctx context.Context, cid, filename string, fileSize int64, filePath, fileHash string) error {
	q := `INSERT INTO local_files (id, cid, filename, file_size, file_path, file_hash, created_at)
	      VALUES (?, ?, ?, ?, ?, ?, ?)
	      ON CONFLICT(cid) DO UPDATE SET filename=excluded.filename, file_path=excluded.file_path, file_size=excluded.file_size, file_hash=excluded.file_hash`
	_, err := r.DB.ExecContext(ctx, q, uuid.New().String(), cid, filename, fileSize, filePath, fileHash, time.Now())
	if err != nil {
		return err
	}
	// update metadata index for search
	_, _ = r.DB.ExecContext(ctx, `INSERT INTO metadata_index (cid, filename, file_size, file_hash)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(cid) DO UPDATE SET filename=excluded.filename, file_size=excluded.file_size, file_hash=excluded.file_hash`,
		cid, filename, fileSize, fileHash)
	return nil
}

func (r *Repository) GetLocalFiles(ctx context.Context) ([]LocalFile, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT id, cid, filename, file_size, file_path, file_hash, created_at FROM local_files ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var files []LocalFile
	for rows.Next() {
		var f LocalFile
		if err := rows.Scan(&f.ID, &f.CID, &f.Filename, &f.FileSize, &f.FilePath, &f.FileHash, &f.CreatedAt); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

func (r *Repository) GetLocalFileByCID(ctx context.Context, cid string) (*LocalFile, error) {
	var f LocalFile
	err := r.DB.QueryRowContext(ctx, `SELECT id, cid, filename, file_size, file_path, file_hash, created_at FROM local_files WHERE cid = ?`, cid).
		Scan(&f.ID, &f.CID, &f.Filename, &f.FileSize, &f.FilePath, &f.FileHash, &f.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *Repository) DeleteLocalFile(ctx context.Context, cid string) error {
	_, err := r.DB.ExecContext(ctx, `DELETE FROM local_files WHERE cid=?`, cid)
	return err
}

func (r *Repository) AddDownload(ctx context.Context, cid, filename string, fileSize int64, downloadPath string) error {
	_, err := r.DB.ExecContext(ctx, `INSERT INTO downloads (id, cid, filename, file_size, download_path, downloaded_at, status)
		VALUES (?, ?, ?, ?, ?, ?, 'completed') ON CONFLICT(cid) DO UPDATE SET status='completed', downloaded_at=excluded.downloaded_at, download_path=excluded.download_path`,
		uuid.New().String(), cid, filename, fileSize, downloadPath, time.Now())
	return err
}

func (r *Repository) UpsertPiece(ctx context.Context, cid string, idx int64, offset, size int64, hash string, have bool) error {
	_, err := r.DB.ExecContext(ctx, `INSERT INTO pieces (id, cid, idx, offset, size, hash, have, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(cid, idx) DO UPDATE SET have=excluded.have, updated_at=excluded.updated_at`,
		uuid.New().String(), cid, idx, offset, size, hash, boolToInt(have), time.Now())
	return err
}

func (r *Repository) GetPieces(ctx context.Context, cid string) ([]Piece, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT id, cid, idx, offset, size, hash, have, updated_at FROM pieces WHERE cid=? ORDER BY idx ASC`, cid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Piece
	for rows.Next() {
		var p Piece
		var haveInt int
		if err := rows.Scan(&p.ID, &p.CID, &p.Index, &p.Offset, &p.Size, &p.Hash, &haveInt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		p.Have = haveInt == 1
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *Repository) MissingPieces(ctx context.Context, cid string) ([]Piece, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT id, cid, idx, offset, size, hash, have, updated_at FROM pieces WHERE cid=? AND have=0 ORDER BY idx ASC`, cid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Piece
	for rows.Next() {
		var p Piece
		var haveInt int
		if err := rows.Scan(&p.ID, &p.CID, &p.Index, &p.Offset, &p.Size, &p.Hash, &haveInt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		p.Have = false
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *Repository) SetPeerScore(ctx context.Context, peerID string, delta float64) error {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var score float64
	err = tx.QueryRowContext(ctx, `SELECT score FROM peer_scores WHERE peer_id=?`, peerID).Scan(&score)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			_, err = tx.ExecContext(ctx, `INSERT INTO peer_scores (peer_id, score, seen_at) VALUES (?, ?, ?)`, peerID, 10.0+delta, time.Now())
		} else {
			return err
		}
	} else {
		score += delta
		if score < -50 {
			score = -50
		}
		if score > 100 {
			score = 100
		}
		_, err = tx.ExecContext(ctx, `UPDATE peer_scores SET score=?, seen_at=? WHERE peer_id=?`, score, time.Now(), peerID)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) GetPeerScore(ctx context.Context, peerID string) (float64, error) {
	var s float64
	err := r.DB.QueryRowContext(ctx, `SELECT score FROM peer_scores WHERE peer_id=?`, peerID).Scan(&s)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	return s, nil
}

func (r *Repository) SearchByFilename(ctx context.Context, q string) ([]LocalFile, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT cid, filename, file_size, '' as file_path, '' as file_hash, CURRENT_TIMESTAMP FROM metadata_index WHERE filename LIKE ? ORDER BY filename`, "%"+q+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []LocalFile
	for rows.Next() {
		var lf LocalFile
		if err := rows.Scan(&lf.CID, &lf.Filename, &lf.FileSize, &lf.FilePath, &lf.FileHash, &lf.CreatedAt); err != nil {
			return nil, err
		}
		res = append(res, lf)
	}
	return res, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Weights for trust score calculation
type Weights struct {
	SuccessfulChunks  float64
	UploadSpeed       float64
	SeedLeechRatio    float64
	SuccessRate       float64
	ChunkAvailability float64
	OnlineTime        float64
}

// GetDefaultWeights returns the default weight configuration for trust score calculation
func GetDefaultWeights() Weights {
	return Weights{
		SuccessfulChunks:  0.3,
		UploadSpeed:       0.15,
		SeedLeechRatio:    0.2,
		SuccessRate:       0.15,
		ChunkAvailability: 0.1,
		OnlineTime:        0.1,
	}
}

// CalculateDecayFactor calculates the time-based decay factor for trust scores
func (r *Repository) CalculateDecayFactor(lastSeen time.Time) float64 {
	daysSinceLastSeen := int(time.Since(lastSeen).Hours() / 24)
	decayFactor := 0.0 // No decay by default

	if daysSinceLastSeen > 3 {
		// Major decay only after 3 days of inactivity
		daysOverThreshold := float64(daysSinceLastSeen - 3)
		decayFactor = -math.Exp(0.1*daysOverThreshold) / 100
	} else if daysSinceLastSeen > 0 {
		// Very minor decay for 1-3 days (almost negligible)
		decayFactor = -0.001 * float64(daysSinceLastSeen)
	}

	return decayFactor
}

// CalculateTrustScore computes the trust score for a peer based on the algorithm
func (r *Repository) CalculateTrustScore(currentTrustScore float64, peerData *PeerScore, transSucc bool) float64 {
	weights := GetDefaultWeights()

	// Calculate ratios
	leechedBySeeded := 1.0
	if peerData.SeededData > 0 {
		leechedBySeeded = peerData.LeechedData / peerData.SeededData
	}

	// Days since last seen (for decay) - only major decay after 3 days
	daysSinceLastSeen := int(time.Since(peerData.LastSeen).Hours() / 24)
	decayFactor := 0.0 // No decay by default

	if daysSinceLastSeen > 3 {
		// Major decay only after 3 days of inactivity
		daysOverThreshold := float64(daysSinceLastSeen - 3)
		decayFactor = -math.Exp(0.1*daysOverThreshold) / 100
	} else if daysSinceLastSeen > 0 {
		// Very minor decay for 1-3 days (almost negligible)
		decayFactor = -0.001 * float64(daysSinceLastSeen)
	}

	// Success rate
	ratioSuccTotal := 0.0
	if peerData.TotalTransfers > 0 {
		ratioSuccTotal = float64(peerData.SuccessfulTransfers) / float64(peerData.TotalTransfers)
	}

	// Trust score calculation parameters with diminishing returns
	maxPossibleIncrease := 1.0 - currentTrustScore
	alpha := 0.5 * (1 - math.Abs(0.5-currentTrustScore))

	// Apply diminishing returns - make it harder to reach higher trust scores
	difficultyFactor := 1.0
	if currentTrustScore > 0.7 {
		// Exponentially harder after 0.7
		difficultyFactor = math.Pow(1.0-currentTrustScore, 2) // Quadratic difficulty
	} else if currentTrustScore > 0.5 {
		// Linearly harder after 0.5
		difficultyFactor = 1.0 - (currentTrustScore-0.5)*0.5
	}

	scalingFactor := math.Min(1.0, maxPossibleIncrease/(alpha*2.0)) * difficultyFactor
	if scalingFactor < 0.01 {
		scalingFactor = 0.01 // Minimum progress possible
	}

	// Calculate base increase in trust score
	increaseInTrustScore := scalingFactor*(weights.SuccessfulChunks*(math.Log1p(float64(peerData.NoOfSuccChunksLastK))/math.Log1p(100))+
		weights.UploadSpeed*(math.Log1p(float64(peerData.AverageUploadSpeed))/math.Log1p(10000))+
		weights.SeedLeechRatio*(1/(1+leechedBySeeded))+
		weights.SuccessRate*ratioSuccTotal+
		weights.ChunkAvailability*math.Min(1.0, float64(peerData.TotalFileChunksUploaded)/math.Max(1.0, float64(peerData.OriginalChunksShared)))+
		weights.OnlineTime*math.Min(1.0, float64(peerData.AverageOnlineTimePerDay)/24.0)) + decayFactor

	// Apply penalties for bad behavior
	penalty := 0.0

	if ratioSuccTotal < 0.5 { // Low success rate
		penaltyFactor := math.Max(0.05, currentTrustScore*0.3)
		penalty += penaltyFactor
	}

	if leechedBySeeded > 2.0 { // Bad leech/seed ratio
		penaltyFactor := math.Max(0.05, currentTrustScore*0.25)
		penalty += penaltyFactor
	}

	if float64(peerData.AverageUploadSpeed) < 100 { // Very low upload speed
		penaltyFactor := math.Max(0.05, currentTrustScore*0.15)
		penalty += penaltyFactor
	}

	if float64(peerData.NoOfSuccChunksLastK) < 10 { // Very few successful chunks recently
		penaltyFactor := math.Max(0.05, currentTrustScore*0.2)
		penalty += penaltyFactor
	}

	// Apply the penalty and success factor
	succFactor := 1.0
	if !transSucc {
		succFactor = 0.0
	}

	newTrustScore := currentTrustScore + succFactor*alpha*increaseInTrustScore - penalty

	// Clamp to [0, 1] range (not 0-100 like original algorithm)
	if newTrustScore > 1.0 {
		newTrustScore = 1.0
	}
	if newTrustScore < 0.0 {
		newTrustScore = 0.0
	}

	return newTrustScore
}

// GetPeerTrustScore retrieves full trust metrics for a peer
func (r *Repository) GetPeerTrustScore(ctx context.Context, peerID string) (*PeerScore, error) {
	var peer PeerScore
	err := r.DB.QueryRowContext(ctx, `
		SELECT peer_id, score, seen_at, leeched_data, seeded_data, last_seen,
		       average_upload_speed, average_online_time_per_day, total_file_chunks_uploaded,
		       successful_transfers, total_transfers, original_chunks_shared,
		       no_of_succ_chunks_last_k, current_trust_score
		FROM peer_scores WHERE peer_id = ?`, peerID).Scan(
		&peer.PeerID, &peer.Score, &peer.SeenAt, &peer.LeechedData, &peer.SeededData,
		&peer.LastSeen, &peer.AverageUploadSpeed, &peer.AverageOnlineTimePerDay,
		&peer.TotalFileChunksUploaded, &peer.SuccessfulTransfers, &peer.TotalTransfers,
		&peer.OriginalChunksShared, &peer.NoOfSuccChunksLastK, &peer.CurrentTrustScore)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Return default values for new peer
			return &PeerScore{
				PeerID:                  peerID,
				Score:                   30.0, // Legacy score
				SeenAt:                  time.Now(),
				LeechedData:             0.0,
				SeededData:              0.0,
				LastSeen:                time.Now(),
				AverageUploadSpeed:      0,
				AverageOnlineTimePerDay: 8,
				TotalFileChunksUploaded: 0,
				SuccessfulTransfers:     0,
				TotalTransfers:          0,
				OriginalChunksShared:    0,
				NoOfSuccChunksLastK:     0,
				CurrentTrustScore:       0.5, // Default neutral score
			}, nil
		}
		return nil, err
	}

	return &peer, nil
}

// UpdatePeerTrustScore updates trust metrics for a peer
func (r *Repository) UpdatePeerTrustScore(ctx context.Context, peerData *PeerScore) error {
	_, err := r.DB.ExecContext(ctx, `
		INSERT INTO peer_scores (
			peer_id, score, seen_at, leeched_data, seeded_data, last_seen,
			average_upload_speed, average_online_time_per_day, total_file_chunks_uploaded,
			successful_transfers, total_transfers, original_chunks_shared,
			no_of_succ_chunks_last_k, current_trust_score
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(peer_id) DO UPDATE SET
			score = excluded.score,
			seen_at = excluded.seen_at,
			leeched_data = excluded.leeched_data,
			seeded_data = excluded.seeded_data,
			last_seen = excluded.last_seen,
			average_upload_speed = excluded.average_upload_speed,
			average_online_time_per_day = excluded.average_online_time_per_day,
			total_file_chunks_uploaded = excluded.total_file_chunks_uploaded,
			successful_transfers = excluded.successful_transfers,
			total_transfers = excluded.total_transfers,
			original_chunks_shared = excluded.original_chunks_shared,
			no_of_succ_chunks_last_k = excluded.no_of_succ_chunks_last_k,
			current_trust_score = excluded.current_trust_score`,
		peerData.PeerID, peerData.Score, peerData.SeenAt, peerData.LeechedData,
		peerData.SeededData, peerData.LastSeen, peerData.AverageUploadSpeed,
		peerData.AverageOnlineTimePerDay, peerData.TotalFileChunksUploaded,
		peerData.SuccessfulTransfers, peerData.TotalTransfers, peerData.OriginalChunksShared,
		peerData.NoOfSuccChunksLastK, peerData.CurrentTrustScore)

	return err
}
