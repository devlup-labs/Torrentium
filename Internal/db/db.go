package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
)

type LocalFile struct {
	ID        string    `db:"id"`
	CID       string    `db:"cid"`
	Filename  string    `db:"filename"`
	FileSize  int64     `db:"file_size"`
	FilePath  string    `db:"file_path"`
	FileHash  string    `db:"file_hash"`
	CreatedAt time.Time `db:"created_at"`
}

// Download represents a file downloaded by this peer
type Download struct {
	ID           string    `db:"id"`
	CID          string    `db:"cid"`
	Filename     string    `db:"filename"`
	FileSize     int64     `db:"file_size"`
	DownloadPath string    `db:"download_path"`
	DownloadedAt time.Time `db:"downloaded_at"`
	Status       string    `db:"status"`
}

type Repository struct {
	DB *sql.DB
}

// To separate out logic od db from overall code!
func NewRepository(db *sql.DB) *Repository {
	return &Repository{DB: db}
}

var DB *sql.DB

func InitDB() *sql.DB {
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: Could not load .env file: %v", err)
	}

	dbpath := os.Getenv("SQLITE_DB_PATH")
	if dbpath == "" {
		dbpath = "./peer.db" // Default path for peer's own database
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
	createLocalFilesTable := `
	CREATE TABLE IF NOT EXISTS local_files (
		id TEXT PRIMARY KEY,
		cid TEXT UNIQUE NOT NULL,
		filename TEXT NOT NULL,
		file_size INTEGER NOT NULL,
		file_path TEXT NOT NULL,
		file_hash TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	createDownloadsTable := `
	CREATE TABLE IF NOT EXISTS downloads (
		id TEXT PRIMARY KEY,
		cid TEXT UNIQUE NOT NULL,
		filename TEXT NOT NULL,
		file_size INTEGER NOT NULL,
		download_path TEXT NOT NULL,
		downloaded_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		status TEXT DEFAULT 'completed'
	);`

	if _, err := db.Exec(createLocalFilesTable); err != nil {
		return fmt.Errorf("error creating local_files table: %w", err)
	}

	if _, err := db.Exec(createDownloadsTable); err != nil {
		return fmt.Errorf("error creating downloads table: %w", err)
	}

	return nil
}


// AddLocalFile adds a file that this peer is sharing
func (r *Repository) AddLocalFile(ctx context.Context, cid, filename string, fileSize int64, filePath, fileHash string) error {
	query := `
	INSERT INTO local_files (id, cid, filename, file_size, file_path, file_hash, created_at)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(cid) DO UPDATE SET
		filename = excluded.filename,
		file_path = excluded.file_path`

	_, err := r.DB.ExecContext(ctx, query,
		uuid.New().String(),
		cid,
		filename,
		fileSize,
		filePath,
		fileHash,
		time.Now(),
	)

	return err
}

// GetLocalFiles returns all files shared by this peer
func (r *Repository) GetLocalFiles(ctx context.Context) ([]LocalFile, error) {
	query := `SELECT id, cid, filename, file_size, file_path, file_hash, created_at
			  FROM local_files ORDER BY created_at DESC`

	rows, err := r.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []LocalFile
	for rows.Next() {
		var file LocalFile
		if err := rows.Scan(
			&file.ID,
			&file.CID,
			&file.Filename,
			&file.FileSize,
			&file.FilePath,
			&file.FileHash,
			&file.CreatedAt,
		); err != nil {
			return nil, err
		}
		files = append(files, file)
	}

	return files, rows.Err()
}

// AddDownload records a completed download
func (r *Repository) AddDownload(ctx context.Context, cid, filename string, fileSize int64, downloadPath string) error {
	query := `
	INSERT INTO downloads (id, cid, filename, file_size, download_path, downloaded_at, status)
	VALUES (?, ?, ?, ?, ?, ?, 'completed')
	ON CONFLICT(cid) DO NOTHING`

	_, err := r.DB.ExecContext(ctx, query,
		uuid.New().String(),
		cid,
		filename,
		fileSize,
		downloadPath,
		time.Now(),
	)

	return err
}

// GetDownloads returns all downloaded files
func (r *Repository) GetDownloads(ctx context.Context) ([]Download, error) {
	query := `SELECT id, cid, filename, file_size, download_path, downloaded_at, status
			  FROM downloads ORDER BY downloaded_at DESC`

	rows, err := r.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var downloads []Download
	for rows.Next() {
		var download Download
		if err := rows.Scan(
			&download.ID,
			&download.CID,
			&download.Filename,
			&download.FileSize,
			&download.DownloadPath,
			&download.DownloadedAt,
			&download.Status,
		); err != nil {
			return nil, err
		}
		downloads = append(downloads, download)
	}

	return downloads, rows.Err()
}

// GetLocalFileByCID returns a specific local file by CID
func (r *Repository) GetLocalFileByCID(ctx context.Context, cid string) (*LocalFile, error) {
	query := `SELECT id, cid, filename, file_size, file_path, file_hash, created_at
			  FROM local_files WHERE cid = ?`

	var file LocalFile
	err := r.DB.QueryRowContext(ctx, query, cid).Scan(
		&file.ID,
		&file.CID,
		&file.Filename,
		&file.FileSize,
		&file.FilePath,
		&file.FileHash,
		&file.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &file, nil
}

// DeleteLocalFile removes a file from sharing
func (r *Repository) DeleteLocalFile(ctx context.Context, cid string) error {
	query := `DELETE FROM local_files WHERE cid = ?`
	_, err := r.DB.ExecContext(ctx, query, cid)
	return err
}