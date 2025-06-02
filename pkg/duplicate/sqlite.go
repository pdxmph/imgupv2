package duplicate

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Upload represents a cached upload record
type Upload struct {
	FileMD5    string
	Service    string
	RemoteID   string
	RemoteURL  string
	ImageURL   string
	UploadTime time.Time
	Filename   string
	FileSize   int64
}

// SQLiteCache implements local duplicate checking via SQLite
type SQLiteCache struct {
	db *sql.DB
}

// NewSQLiteCache creates a new SQLite-based cache
func NewSQLiteCache(dbPath string) (*SQLiteCache, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create cache directory: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	cache := &SQLiteCache{db: db}
	if err := cache.init(); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize database: %w", err)
	}

	return cache, nil
}

// init creates the database schema
func (c *SQLiteCache) init() error {
	schema := `
	CREATE TABLE IF NOT EXISTS uploads (
		file_md5 TEXT PRIMARY KEY,
		service TEXT NOT NULL,
		remote_id TEXT NOT NULL,
		remote_url TEXT NOT NULL,
		image_url TEXT,
		upload_time INTEGER,
		filename TEXT,
		file_size INTEGER
	);

	CREATE INDEX IF NOT EXISTS idx_service_id ON uploads(service, remote_id);
	CREATE INDEX IF NOT EXISTS idx_filename ON uploads(filename);
	`

	_, err := c.db.Exec(schema)
	return err
}

// Check looks up a file by MD5 hash
func (c *SQLiteCache) Check(ctx context.Context, md5Hash string) (*Upload, error) {
	query := `
		SELECT file_md5, service, remote_id, remote_url, image_url, 
		       upload_time, filename, file_size
		FROM uploads
		WHERE file_md5 = ?
	`

	var upload Upload
	var uploadTime int64

	err := c.db.QueryRowContext(ctx, query, md5Hash).Scan(
		&upload.FileMD5,
		&upload.Service,
		&upload.RemoteID,
		&upload.RemoteURL,
		&upload.ImageURL,
		&uploadTime,
		&upload.Filename,
		&upload.FileSize,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query upload: %w", err)
	}

	upload.UploadTime = time.Unix(uploadTime, 0)
	return &upload, nil
}

// Record saves an upload to the cache
func (c *SQLiteCache) Record(upload *Upload) error {
	query := `
		INSERT OR REPLACE INTO uploads 
		(file_md5, service, remote_id, remote_url, image_url, upload_time, filename, file_size)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := c.db.Exec(
		query,
		upload.FileMD5,
		upload.Service,
		upload.RemoteID,
		upload.RemoteURL,
		upload.ImageURL,
		upload.UploadTime.Unix(),
		upload.Filename,
		upload.FileSize,
	)

	if err != nil {
		return fmt.Errorf("record upload: %w", err)
	}

	return nil
}

// FindByRemoteID looks up an upload by service and remote ID
func (c *SQLiteCache) FindByRemoteID(ctx context.Context, service, remoteID string) (*Upload, error) {
	query := `
		SELECT file_md5, service, remote_id, remote_url, image_url, 
		       upload_time, filename, file_size
		FROM uploads
		WHERE service = ? AND remote_id = ?
	`

	var upload Upload
	var uploadTime int64

	err := c.db.QueryRowContext(ctx, query, service, remoteID).Scan(
		&upload.FileMD5,
		&upload.Service,
		&upload.RemoteID,
		&upload.RemoteURL,
		&upload.ImageURL,
		&uploadTime,
		&upload.Filename,
		&upload.FileSize,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query by remote ID: %w", err)
	}

	upload.UploadTime = time.Unix(uploadTime, 0)
	return &upload, nil
}

// FindByFilename searches for uploads with matching filename
func (c *SQLiteCache) FindByFilename(ctx context.Context, filename string) ([]*Upload, error) {
	query := `
		SELECT file_md5, service, remote_id, remote_url, image_url, 
		       upload_time, filename, file_size
		FROM uploads
		WHERE filename = ?
		ORDER BY upload_time DESC
	`

	rows, err := c.db.QueryContext(ctx, query, filename)
	if err != nil {
		return nil, fmt.Errorf("query by filename: %w", err)
	}
	defer rows.Close()

	var uploads []*Upload
	for rows.Next() {
		var upload Upload
		var uploadTime int64

		err := rows.Scan(
			&upload.FileMD5,
			&upload.Service,
			&upload.RemoteID,
			&upload.RemoteURL,
			&upload.ImageURL,
			&uploadTime,
			&upload.Filename,
			&upload.FileSize,
		)
		if err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}

		upload.UploadTime = time.Unix(uploadTime, 0)
		uploads = append(uploads, &upload)
	}

	return uploads, rows.Err()
}

// Close closes the database connection
func (c *SQLiteCache) Close() error {
	return c.db.Close()
}

// DefaultCachePath returns the default cache database path
func DefaultCachePath() string {
	return filepath.Join(os.Getenv("HOME"), ".config", "imgupv2", "uploads.db")
}
