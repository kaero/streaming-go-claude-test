package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/kaero/streaming/config"
	"github.com/kaero/streaming/internal/templates"
	"github.com/kaero/streaming/internal/transcoder"
)

// Handler holds all HTTP handlers for the streaming server
type Handler struct {
	config    *config.Config
	tm        *transcoder.Manager
	templates *templates.Templates
}

// Video represents a video file with metadata
type Video struct {
	Name   string
	SizeMB int64
}

// ListData holds data for the list template
type ListData struct {
	Videos []Video
}

// PlayerData holds data for the player template
type PlayerData struct {
	VideoFile string
}

// NewHandler creates a new Handler instance
func NewHandler(cfg *config.Config, tm *transcoder.Manager, tmpl *templates.Templates) *Handler {
	return &Handler{
		config:    cfg,
		tm:        tm,
		templates: tmpl,
	}
}

// VideoHandler handles requests for video streaming
func (h *Handler) VideoHandler(w http.ResponseWriter, r *http.Request) {
	// Extract the video file from the request path
	videoFile := strings.TrimPrefix(r.URL.Path, "/video/")
	if videoFile == "" {
		http.Error(w, "Video file not specified", http.StatusBadRequest)
		return
	}
	
	// Check if the requested file exists
	videoPath := filepath.Join(h.config.MediaDir, videoFile)
	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		http.Error(w, "Video file not found", http.StatusNotFound)
		return
	}
	
	// Create the output directory path
	outputDir := filepath.Join(h.config.CacheDir, strings.TrimSuffix(videoFile, filepath.Ext(videoFile)))
	masterPlaylist := filepath.Join(outputDir, videoFile+".m3u8")
	
	// Check if master playlist already exists
	if _, err := os.Stat(masterPlaylist); os.IsNotExist(err) {
		// Prepare video for streaming (transcoding)
		var err error
		masterPlaylist, err = h.tm.PrepareVideo(videoPath)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error preparing video: %v", err), http.StatusInternalServerError)
			return
		}
	}
	
	// Redirect to the master playlist
	relativePlaylist := strings.TrimPrefix(masterPlaylist, h.config.CacheDir+"/")
	http.Redirect(w, r, "/stream/"+relativePlaylist, http.StatusFound)
}

// StreamHandler serves HLS files
func (h *Handler) StreamHandler(w http.ResponseWriter, r *http.Request) {
	// Extract the file path from the request
	filePath := strings.TrimPrefix(r.URL.Path, "/stream/")
	fullPath := filepath.Join(h.config.CacheDir, filePath)
	
	// Check if the file exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	
	// Set appropriate content type based on file extension
	switch filepath.Ext(fullPath) {
	case ".m3u8":
		w.Header().Set("Content-Type", "application/x-mpegURL")
	case ".ts":
		w.Header().Set("Content-Type", "video/MP2T")
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	
	// Add CORS headers for compatibility
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type")
	
	// Handle OPTIONS request for CORS preflight
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	
	// Serve the file
	http.ServeFile(w, r, fullPath)
}

// ListVideosHandler serves a simple UI listing available videos
func (h *Handler) ListVideosHandler(w http.ResponseWriter, r *http.Request) {
	files, err := os.ReadDir(h.config.MediaDir)
	if err != nil {
		http.Error(w, "Error reading media directory", http.StatusInternalServerError)
		return
	}
	
	var videos []Video
	
	// Collect all video files
	for _, file := range files {
		if !file.IsDir() {
			fileInfo, err := file.Info()
			if err != nil {
				continue
			}
			
			ext := strings.ToLower(filepath.Ext(file.Name()))
			// Only list video files
			if ext == ".mp4" || ext == ".mkv" || ext == ".avi" || ext == ".mov" || ext == ".webm" {
				videos = append(videos, Video{
					Name:   file.Name(),
					SizeMB: fileInfo.Size() / (1024 * 1024),
				})
			}
		}
	}
	
	data := ListData{
		Videos: videos,
	}
	
	w.Header().Set("Content-Type", "text/html")
	err = h.templates.ListTemplate(w, data)
	if err != nil {
		http.Error(w, "Error rendering template", http.StatusInternalServerError)
	}
}

// PlayerHandler serves a simple video player for a specific video
func (h *Handler) PlayerHandler(w http.ResponseWriter, r *http.Request) {
	// Extract the video file from the request path
	videoFile := strings.TrimPrefix(r.URL.Path, "/player/")
	if videoFile == "" {
		http.Error(w, "Video file not specified", http.StatusBadRequest)
		return
	}
	
	data := PlayerData{
		VideoFile: videoFile,
	}
	
	w.Header().Set("Content-Type", "text/html")
	err := h.templates.PlayerTemplate(w, data)
	if err != nil {
		http.Error(w, "Error rendering template", http.StatusInternalServerError)
	}
}