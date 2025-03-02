package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/kaero/streaming/config"
	"github.com/kaero/streaming/internal/database"
	"github.com/kaero/streaming/internal/library"
	"github.com/kaero/streaming/internal/transcoder"
	"github.com/kaero/streaming/internal/utils"
)

// runLibrarian sets up and starts the librarian service
func runLibrarian() error {
	// Load configuration
	var err error
	cfg, err = config.InitConfig(cfgFile)
	if err != nil {
		return fmt.Errorf("error initializing config: %w", err)
	}

	// Override with command-line flags if provided
	if mediaDir != "" {
		cfg.Media.MediaDir = mediaDir
	}
	if cacheDir != "" {
		cfg.Media.CacheDir = cacheDir
	}
	if dbPath != "" {
		cfg.Database.Path = dbPath
	}
	if scanOnStart {
		cfg.Library.ScanOnStart = scanOnStart
	}
	if watchForChanges {
		cfg.Library.WatchForChanges = watchForChanges
	}
	if scanIntervalMinutes > 0 {
		cfg.Library.ScanIntervalMinutes = scanIntervalMinutes
	}
	if processingThreads > 0 {
		cfg.Library.ProcessingThreads = processingThreads
	}

	// Create required directories
	if err := utils.CreateDirectories(cfg); err != nil {
		return fmt.Errorf("error creating directories: %w", err)
	}

	// Initialize database
	db, err := database.New(cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("error initializing database: %w", err)
	}
	defer db.Close()

	// Create transcoding manager
	tm := transcoder.NewManager(cfg)

	// Create library manager
	lm, err := library.New(cfg, db, tm)
	if err != nil {
		return fmt.Errorf("error creating library manager: %w", err)
	}
	defer lm.Close()

	// Setup signal handling for graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start the library manager
	log.Printf("Starting librarian service")
	log.Printf("Media directory: %s", cfg.Media.MediaDir)
	log.Printf("Cache directory: %s", cfg.Media.CacheDir)
	log.Printf("Database path: %s", cfg.Database.Path)
	log.Printf("Scan on start: %t", cfg.Library.ScanOnStart)
	log.Printf("Watch for changes: %t", cfg.Library.WatchForChanges)
	log.Printf("Scan interval: %d minutes", cfg.Library.ScanIntervalMinutes)
	log.Printf("Processing threads: %d", cfg.Library.ProcessingThreads)

	// Scan library on start if requested
	if cfg.Library.ScanOnStart {
		log.Println("Scanning library for new videos...")
		if err := lm.ScanLibrary(); err != nil {
			log.Printf("Error scanning library: %v", err)
		}

		// Process pending videos
		if err := lm.ProcessPendingVideos(); err != nil {
			log.Printf("Error processing pending videos: %v", err)
		}
	}

	// Watch for file system changes if requested
	if cfg.Library.WatchForChanges {
		if err := lm.StartWatching(); err != nil {
			log.Printf("Error starting file watcher: %v", err)
		}
	}

	// Start periodic scanning if interval is set
	if cfg.Library.ScanIntervalMinutes > 0 {
		lm.StartPeriodicScan()
	}

	// Wait for interrupt signal
	<-stop
	log.Println("Shutting down librarian service...")

	return nil
}