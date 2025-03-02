package library

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	
	"github.com/kaero/streaming/config"
	"github.com/kaero/streaming/internal/database"
	"github.com/kaero/streaming/internal/transcoder"
)

// Manager handles the media library operations
type Manager struct {
	config    *config.Config
	db        *database.DB
	tm        *transcoder.Manager
	watcher   *fsnotify.Watcher
	watcherMu sync.Mutex
	isWatching bool
	stopChan   chan struct{}
}

// New creates a new library manager
func New(cfg *config.Config, db *database.DB, tm *transcoder.Manager) (*Manager, error) {
	return &Manager{
		config:    cfg,
		db:        db,
		tm:        tm,
		stopChan:  make(chan struct{}),
	}, nil
}

// ScanLibrary scans the media directory for new videos
func (m *Manager) ScanLibrary() error {
	log.Println("Scanning library for new videos...")
	
	mediaDir := m.config.Media.MediaDir
	
	// Walk through the media directory
	return filepath.Walk(mediaDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		// Skip directories
		if info.IsDir() {
			return nil
		}
		
		// Check if it's a video file
		ext := strings.ToLower(filepath.Ext(info.Name()))
		if !isVideoFile(ext) {
			return nil
		}
		
		// Check if this video already exists in the database
		exists, err := m.db.VideoExists(path)
		if err != nil {
			log.Printf("Error checking video existence: %v", err)
			return nil
		}
		
		// If the video doesn't exist in the database, add it
		if !exists {
			id, err := m.db.AddVideo(info.Name(), path, info.Size())
			if err != nil {
				log.Printf("Error adding video to database: %v", err)
				return nil
			}
			
			log.Printf("Added new video to library: %s (ID: %d)", info.Name(), id)
		}
		
		return nil
	})
}

// ProcessPendingVideos processes all pending videos
func (m *Manager) ProcessPendingVideos() error {
	pendingVideos, err := m.db.GetPendingVideos()
	if err != nil {
		return fmt.Errorf("failed to get pending videos: %w", err)
	}
	
	if len(pendingVideos) == 0 {
		log.Println("No pending videos to process")
		return nil
	}
	
	log.Printf("Processing %d pending videos", len(pendingVideos))
	
	// Create a worker pool
	numWorkers := m.config.Library.ProcessingThreads
	if numWorkers <= 0 {
		numWorkers = 1
	}
	
	// Create a channel for jobs
	jobs := make(chan *database.Video, len(pendingVideos))
	
	// Create a wait group to wait for all workers
	var wg sync.WaitGroup
	
	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for video := range jobs {
				m.processVideo(video)
			}
		}(i)
	}
	
	// Send jobs to the workers
	for _, video := range pendingVideos {
		jobs <- video
	}
	
	// Close the jobs channel
	close(jobs)
	
	// Wait for all workers to finish
	wg.Wait()
	
	return nil
}

// processVideo processes a single video
func (m *Manager) processVideo(video *database.Video) {
	log.Printf("Processing video: %s", video.Filename)
	
	// Update status to processing
	if err := m.db.SetVideoProcessing(video.ID); err != nil {
		log.Printf("Error setting video as processing: %v", err)
		return
	}
	
	// Process the video
	masterPath, err := m.tm.PrepareVideo(video.Path)
	if err != nil {
		log.Printf("Error processing video: %v", err)
		m.db.SetVideoError(video.ID, err.Error())
		return
	}
	
	// Get video duration (in the future we can get this from ffmpeg)
	duration := 0.0 // For now, we don't have a way to get the duration
	
	// Update status to ready
	if err := m.db.SetVideoReady(video.ID, duration); err != nil {
		log.Printf("Error setting video as ready: %v", err)
		return
	}
	
	log.Printf("Video processed successfully: %s, output at: %s", video.Filename, masterPath)
}

// StartWatching starts watching the media directory for changes
func (m *Manager) StartWatching() error {
	m.watcherMu.Lock()
	defer m.watcherMu.Unlock()
	
	if m.isWatching {
		return nil // Already watching
	}
	
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}
	
	m.watcher = watcher
	m.isWatching = true
	
	// Add the media directory to the watcher
	if err := watcher.Add(m.config.Media.MediaDir); err != nil {
		return fmt.Errorf("failed to watch media directory: %w", err)
	}
	
	// Start the watcher goroutine
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				
				if event.Op&(fsnotify.Create|fsnotify.Write) != 0 {
					// Check if it's a video file
					ext := strings.ToLower(filepath.Ext(event.Name))
					if !isVideoFile(ext) {
						continue
					}
					
					// Get file info
					info, err := os.Stat(event.Name)
					if err != nil {
						log.Printf("Error getting file info: %v", err)
						continue
					}
					
					// Skip directories
					if info.IsDir() {
						continue
					}
					
					// Check if this video already exists in the database
					exists, err := m.db.VideoExists(event.Name)
					if err != nil {
						log.Printf("Error checking video existence: %v", err)
						continue
					}
					
					// If the video doesn't exist in the database, add it
					if !exists {
						id, err := m.db.AddVideo(filepath.Base(event.Name), event.Name, info.Size())
						if err != nil {
							log.Printf("Error adding video to database: %v", err)
							continue
						}
						
						log.Printf("Added new video to library: %s (ID: %d)", info.Name(), id)
					}
				}
				
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("Watcher error: %v", err)
				
			case <-m.stopChan:
				watcher.Close()
				return
			}
		}
	}()
	
	log.Printf("Started watching media directory: %s", m.config.Media.MediaDir)
	return nil
}

// StopWatching stops watching the media directory
func (m *Manager) StopWatching() {
	m.watcherMu.Lock()
	defer m.watcherMu.Unlock()
	
	if !m.isWatching {
		return
	}
	
	close(m.stopChan)
	m.isWatching = false
	
	log.Println("Stopped watching media directory")
}

// StartPeriodicScan starts periodic scanning
func (m *Manager) StartPeriodicScan() {
	interval := m.config.Library.ScanIntervalMinutes
	if interval <= 0 {
		log.Println("Periodic scanning disabled")
		return
	}
	
	log.Printf("Starting periodic library scan every %d minutes", interval)
	
	go func() {
		ticker := time.NewTicker(time.Duration(interval) * time.Minute)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				if err := m.ScanLibrary(); err != nil {
					log.Printf("Error scanning library: %v", err)
				}
				
				if err := m.ProcessPendingVideos(); err != nil {
					log.Printf("Error processing pending videos: %v", err)
				}
				
			case <-m.stopChan:
				return
			}
		}
	}()
}

// isVideoFile checks if a file extension is a video format
func isVideoFile(ext string) bool {
	videoExts := []string{".mp4", ".mkv", ".avi", ".mov", ".webm", ".flv", ".wmv"}
	for _, e := range videoExts {
		if ext == e {
			return true
		}
	}
	return false
}

// Close closes the library manager
func (m *Manager) Close() {
	m.StopWatching()
	// The stopChan is already closed in StopWatching()
}