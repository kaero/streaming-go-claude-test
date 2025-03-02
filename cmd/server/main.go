package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/kaero/streaming/config"
	"github.com/kaero/streaming/internal/handlers"
	"github.com/kaero/streaming/internal/templates"
	"github.com/kaero/streaming/internal/transcoder"
	"github.com/kaero/streaming/internal/utils"
)

func main() {
	// Load configuration
	cfg := config.DefaultConfig()

	// Create required directories
	if err := utils.CreateDirectories(cfg); err != nil {
		log.Fatalf("Error creating directories: %v", err)
	}

	// Create transcoding manager
	tm := transcoder.NewManager(cfg)
	
	// Initialize templates
	tmpl := templates.New()

	// Create HTTP handlers
	h := handlers.NewHandler(cfg, tm, tmpl)

	// Setup HTTP routes
	http.HandleFunc("/", h.ListVideosHandler)
	http.HandleFunc("/video/", h.VideoHandler)
	http.HandleFunc("/stream/", h.StreamHandler)
	http.HandleFunc("/player/", h.PlayerHandler)

	// Start cache cleanup goroutine
	go utils.CleanupCache(cfg)

	// Start the server
	serverAddr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("Starting server on http://localhost%s", serverAddr)
	log.Fatal(http.ListenAndServe(serverAddr, nil))
}