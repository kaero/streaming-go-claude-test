package database

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// VideoStatus represents the processing status of a video
type VideoStatus string

// Video status constants
const (
	StatusPending    VideoStatus = "pending"
	StatusProcessing VideoStatus = "processing"
	StatusReady      VideoStatus = "ready"
	StatusError      VideoStatus = "error"
)

// Video represents a video file in the library
type Video struct {
	ID           int64
	Filename     string
	Path         string
	Size         int64
	Duration     float64
	Status       VideoStatus
	ErrorMessage string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// DB handles database operations
type DB struct {
	db *sql.DB
}

// New creates a new database connection
func New(dbPath string) (*DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Create instance
	instance := &DB{db: db}

	// Initialize database schema
	if err := instance.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize database schema: %w", err)
	}

	return instance, nil
}

// Close closes the database connection
func (d *DB) Close() error {
	return d.db.Close()
}

// initSchema creates the necessary tables if they don't exist
func (d *DB) initSchema() error {
	// Create videos table
	_, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS videos (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			filename TEXT NOT NULL,
			path TEXT NOT NULL UNIQUE,
			size INTEGER NOT NULL,
			duration REAL DEFAULT 0,
			status TEXT NOT NULL,
			error_message TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create videos table: %w", err)
	}

	return nil
}

// AddVideo adds a new video to the database
func (d *DB) AddVideo(filename, path string, size int64) (int64, error) {
	result, err := d.db.Exec(
		"INSERT INTO videos (filename, path, size, status) VALUES (?, ?, ?, ?)",
		filename, path, size, StatusPending,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to add video: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert ID: %w", err)
	}

	return id, nil
}

// GetVideo retrieves a video by its ID
func (d *DB) GetVideo(id int64) (*Video, error) {
	var video Video
	err := d.db.QueryRow(`
		SELECT id, filename, path, size, duration, status, error_message, 
		       created_at, updated_at
		FROM videos
		WHERE id = ?
	`, id).Scan(
		&video.ID, &video.Filename, &video.Path, &video.Size, 
		&video.Duration, &video.Status, &video.ErrorMessage,
		&video.CreatedAt, &video.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get video: %w", err)
	}

	return &video, nil
}

// GetVideoByPath retrieves a video by its file path
func (d *DB) GetVideoByPath(path string) (*Video, error) {
	var video Video
	err := d.db.QueryRow(`
		SELECT id, filename, path, size, duration, status, error_message, 
		       created_at, updated_at
		FROM videos
		WHERE path = ?
	`, path).Scan(
		&video.ID, &video.Filename, &video.Path, &video.Size, 
		&video.Duration, &video.Status, &video.ErrorMessage,
		&video.CreatedAt, &video.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No video found, not an error
		}
		return nil, fmt.Errorf("failed to get video by path: %w", err)
	}

	return &video, nil
}

// ListVideos retrieves all videos
func (d *DB) ListVideos() ([]*Video, error) {
	rows, err := d.db.Query(`
		SELECT id, filename, path, size, duration, status, error_message, 
		       created_at, updated_at
		FROM videos
		ORDER BY filename
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list videos: %w", err)
	}
	defer rows.Close()

	var videos []*Video
	for rows.Next() {
		var video Video
		err := rows.Scan(
			&video.ID, &video.Filename, &video.Path, &video.Size, 
			&video.Duration, &video.Status, &video.ErrorMessage,
			&video.CreatedAt, &video.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan video row: %w", err)
		}
		videos = append(videos, &video)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating video rows: %w", err)
	}

	return videos, nil
}

// ListVideosByStatus retrieves videos with a specific status
func (d *DB) ListVideosByStatus(status VideoStatus) ([]*Video, error) {
	rows, err := d.db.Query(`
		SELECT id, filename, path, size, duration, status, error_message, 
		       created_at, updated_at
		FROM videos
		WHERE status = ?
		ORDER BY filename
	`, status)
	if err != nil {
		return nil, fmt.Errorf("failed to list videos by status: %w", err)
	}
	defer rows.Close()

	var videos []*Video
	for rows.Next() {
		var video Video
		err := rows.Scan(
			&video.ID, &video.Filename, &video.Path, &video.Size, 
			&video.Duration, &video.Status, &video.ErrorMessage,
			&video.CreatedAt, &video.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan video row: %w", err)
		}
		videos = append(videos, &video)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating video rows: %w", err)
	}

	return videos, nil
}

// UpdateVideoStatus updates the status of a video
func (d *DB) UpdateVideoStatus(id int64, status VideoStatus, errorMsg string) error {
	_, err := d.db.Exec(
		"UPDATE videos SET status = ?, error_message = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		status, errorMsg, id,
	)
	if err != nil {
		return fmt.Errorf("failed to update video status: %w", err)
	}

	return nil
}

// SetVideoProcessing marks a video as being processed
func (d *DB) SetVideoProcessing(id int64) error {
	return d.UpdateVideoStatus(id, StatusProcessing, "")
}

// SetVideoReady marks a video as ready
func (d *DB) SetVideoReady(id int64, duration float64) error {
	_, err := d.db.Exec(
		"UPDATE videos SET status = ?, duration = ?, error_message = '', updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		StatusReady, duration, id,
	)
	if err != nil {
		return fmt.Errorf("failed to update video as ready: %w", err)
	}

	return nil
}

// SetVideoError marks a video as having an error
func (d *DB) SetVideoError(id int64, errorMsg string) error {
	return d.UpdateVideoStatus(id, StatusError, errorMsg)
}

// DeleteVideo removes a video from the database
func (d *DB) DeleteVideo(id int64) error {
	_, err := d.db.Exec("DELETE FROM videos WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete video: %w", err)
	}

	return nil
}

// GetPendingVideos retrieves videos that need processing
func (d *DB) GetPendingVideos() ([]*Video, error) {
	return d.ListVideosByStatus(StatusPending)
}

// VideoExists checks if a video exists in the database
func (d *DB) VideoExists(path string) (bool, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM videos WHERE path = ?", path).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check if video exists: %w", err)
	}

	return count > 0, nil
}

// HasProcessedVideo checks if a given path already has been processed
func (d *DB) HasProcessedVideo(originalPath string) (bool, error) {
	filename := filepath.Base(originalPath)
	
	var count int
	err := d.db.QueryRow(
		"SELECT COUNT(*) FROM videos WHERE filename = ? AND status = ?", 
		filename, StatusReady,
	).Scan(&count)
	
	if err != nil {
		return false, fmt.Errorf("failed to check for processed video: %w", err)
	}
	
	return count > 0, nil
}