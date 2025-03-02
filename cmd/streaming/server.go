package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/kaero/streaming/config"
	"github.com/kaero/streaming/internal/database"
	"github.com/kaero/streaming/internal/handlers"
	"github.com/kaero/streaming/internal/templates"
	"github.com/kaero/streaming/internal/transcoder"
	"github.com/kaero/streaming/internal/utils"
)

// runServer sets up and starts the HTTP server
func runServer() error {
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
	if listenHost != "" {
		cfg.Server.Host = listenHost
	}
	if listenPort != 0 {
		cfg.Server.Port = listenPort
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
	
	// Initialize templates
	tmpl := templates.New()

	// Create HTTP handlers
	h := handlers.NewHandler(cfg, tm, tmpl, db)

	// Setup HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc("/", h.ListVideosHandler)
	mux.HandleFunc("/video/", h.VideoHandler)
	mux.HandleFunc("/stream/", h.StreamHandler)
	mux.HandleFunc("/player/", h.PlayerHandler)

	// Get server address
	serverAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)

	// Setup HTTP server
	server := &http.Server{
		Addr:    serverAddr,
		Handler: mux,
	}

	// Setup signal handling for graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start the server in a goroutine
	go func() {
		log.Printf("Starting server on http://%s", serverAddr)
		log.Printf("Media directory: %s", cfg.Media.MediaDir)
		log.Printf("Cache directory: %s", cfg.Media.CacheDir)
		log.Printf("Database path: %s", cfg.Database.Path)
		
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Error starting server: %v", err)
		}
	}()

	// Handle refresh requests from the web UI
	refreshCh := h.RefreshChannel()
	
	go func() {
		for range refreshCh {
			log.Println("Received library refresh request from web UI")
			// In a real implementation, we would communicate to the librarian service
			// For now, we'll just log the request
		}
	}()

	// Start cache cleanup goroutine
	go utils.CleanupCache(cfg)

	// Wait for interrupt signal
	<-stop
	log.Println("Shutting down server...")

	return nil
}