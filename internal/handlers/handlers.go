package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/kaero/streaming/config"
	"github.com/kaero/streaming/internal/database"
	"github.com/kaero/streaming/internal/templates"
	"github.com/kaero/streaming/internal/transcoder"
)

// Handler holds all HTTP handlers for the streaming server
type Handler struct {
	config    *config.Config
	tm        *transcoder.Manager
	templates *templates.Templates
	db        *database.DB
	refreshCh chan struct{}
}

// VideoView represents a video file with UI metadata
type VideoView struct {
	Name     string
	SizeMB   int64
	Status   string
	CanPlay  bool
	ErrorMsg string
}

// ListData holds data for the list template
type ListData struct {
	Videos   []VideoView
	ShowScan bool
}

// PlayerData holds data for the player template
type PlayerData struct {
	VideoFile string
}

// NewHandler creates a new Handler instance
func NewHandler(cfg *config.Config, tm *transcoder.Manager, tmpl *templates.Templates, db *database.DB) *Handler {
	return &Handler{
		config:    cfg,
		tm:        tm,
		templates: tmpl,
		db:        db,
		refreshCh: make(chan struct{}, 1),
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
	
	// Check if the requested file exists in the database
	videoPath := filepath.Join(h.config.Media.MediaDir, videoFile)
	dbVideo, err := h.db.GetVideoByPath(videoPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error retrieving video from database: %v", err), http.StatusInternalServerError)
		return
	}
	
	// If the video isn't in the database, check if the file exists 
	// and return an error - videos must be processed by the librarian first
	if dbVideo == nil {
		if _, err := os.Stat(videoPath); os.IsNotExist(err) {
			http.Error(w, "Video file not found", http.StatusNotFound)
			return
		}
		
		http.Error(w, "Video exists but hasn't been processed yet", http.StatusPreconditionFailed)
		return
	}
	
	// Check the status of the video
	switch dbVideo.Status {
	case database.StatusPending, database.StatusProcessing:
		http.Error(w, "Video is still being processed, please wait", http.StatusAccepted)
		return
		
	case database.StatusError:
		http.Error(w, fmt.Sprintf("Error processing video: %s", dbVideo.ErrorMessage), http.StatusInternalServerError)
		return
		
	case database.StatusReady:
		// Video is ready, continue to serve it
		break
		
	default:
		http.Error(w, "Unknown video status", http.StatusInternalServerError)
		return
	}
	
	// Create the output directory path
	outputDir := filepath.Join(h.config.Media.CacheDir, strings.TrimSuffix(videoFile, filepath.Ext(videoFile)))
	masterPlaylist := filepath.Join(outputDir, videoFile+".m3u8")
	
	// Check if master playlist exists
	if _, err := os.Stat(masterPlaylist); os.IsNotExist(err) {
		http.Error(w, "Video playlist not found, reprocess the video", http.StatusNotFound)
		return
	}
	
	// Redirect to the master playlist
	relativePlaylist := strings.TrimPrefix(masterPlaylist, h.config.Media.CacheDir+"/")
	http.Redirect(w, r, "/stream/"+relativePlaylist, http.StatusFound)
}

// StreamHandler serves HLS files
func (h *Handler) StreamHandler(w http.ResponseWriter, r *http.Request) {
	// Extract the file path from the request
	filePath := strings.TrimPrefix(r.URL.Path, "/stream/")
	fullPath := filepath.Join(h.config.Media.CacheDir, filePath)
	
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
	// Handle the scan library action
	if r.URL.Query().Get("scan") == "true" {
		// Send a refresh signal
		select {
		case h.refreshCh <- struct{}{}:
			// Signal sent successfully
		default:
			// Channel is full, a refresh is already pending
		}
		
		// Redirect back to the list page
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	
	// Get all videos from the database
	dbVideos, err := h.db.ListVideos()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error retrieving videos from database: %v", err), http.StatusInternalServerError)
		return
	}
	
	var videos []VideoView
	
	// Convert database videos to view models
	for _, dbVideo := range dbVideos {
		canPlay := dbVideo.Status == database.StatusReady
		errorMsg := ""
		if dbVideo.Status == database.StatusError && dbVideo.ErrorMessage.Valid {
			errorMsg = dbVideo.ErrorMessage.String
		}
		
		videos = append(videos, VideoView{
			Name:     dbVideo.Filename,
			SizeMB:   dbVideo.Size / (1024 * 1024),
			Status:   string(dbVideo.Status),
			CanPlay:  canPlay,
			ErrorMsg: errorMsg,
		})
	}
	
	// Check for files in the media directory that aren't in the database
	files, err := os.ReadDir(h.config.Media.MediaDir)
	if err != nil {
		// Log the error but continue with whatever we have from the database
		fmt.Printf("Error reading media directory: %v\n", err)
	} else {
		// Check for video files not in the database
		for _, file := range files {
			if file.IsDir() {
				continue
			}
			
			fileInfo, err := file.Info()
			if err != nil {
				continue
			}
			
			ext := strings.ToLower(filepath.Ext(file.Name()))
			// Check if it's a video file
			if ext == ".mp4" || ext == ".mkv" || ext == ".avi" || ext == ".mov" || ext == ".webm" {
				// Check if this file is already in the videos list
				found := false
				for _, v := range videos {
					if v.Name == file.Name() {
						found = true
						break
					}
				}
				
				// If not found, add it as an unprocessed video
				if !found {
					videos = append(videos, VideoView{
						Name:     file.Name(),
						SizeMB:   fileInfo.Size() / (1024 * 1024),
						Status:   "unprocessed",
						CanPlay:  false,
						ErrorMsg: "Video has not been processed yet",
					})
				}
			}
		}
	}
	
	data := ListData{
		Videos:   videos,
		ShowScan: true,
	}
	
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
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
	
	// Check if the video is ready for playing
	videoPath := filepath.Join(h.config.Media.MediaDir, videoFile)
	dbVideo, err := h.db.GetVideoByPath(videoPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error retrieving video from database: %v", err), http.StatusInternalServerError)
		return
	}
	
	// Check if the video exists
	if dbVideo == nil {
		http.Error(w, "Video not found in the library", http.StatusNotFound)
		return
	}
	
	// Check if the video is ready
	if dbVideo.Status != database.StatusReady {
		http.Error(w, "Video is not ready for playback", http.StatusPreconditionFailed)
		return
	}
	
	data := PlayerData{
		VideoFile: videoFile,
	}
	
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err = h.templates.PlayerTemplate(w, data)
	if err != nil {
		http.Error(w, "Error rendering template", http.StatusInternalServerError)
	}
}

// RefreshChannel returns a channel that signals when a library refresh is requested
func (h *Handler) RefreshChannel() <-chan struct{} {
	return h.refreshCh
}