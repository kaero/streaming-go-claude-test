package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/kaero/streaming/config"
	"github.com/kaero/streaming/internal/transcoder"
)

// Handler holds all HTTP handlers for the streaming server
type Handler struct {
	config *config.Config
	tm     *transcoder.Manager
}

// NewHandler creates a new Handler instance
func NewHandler(cfg *config.Config, tm *transcoder.Manager) *Handler {
	return &Handler{
		config: cfg,
		tm:     tm,
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
	
	// Create a simple HTML page with links to videos
	html := `<!DOCTYPE html>
<html>
<head>
    <title>Go Video Streaming Server</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }
        h1 { color: #333; }
        ul { list-style-type: none; padding: 0; }
        li { margin: 10px 0; padding: 10px; background-color: #f5f5f5; border-radius: 5px; }
        a { color: #007bff; text-decoration: none; }
        a:hover { text-decoration: underline; }
    </style>
</head>
<body>
    <h1>Available Videos</h1>
    <ul>`
	
	// List all video files
	for _, file := range files {
		if !file.IsDir() {
			fileInfo, err := file.Info()
			if err != nil {
				continue
			}
			
			ext := strings.ToLower(filepath.Ext(file.Name()))
			// Only list video files
			if ext == ".mp4" || ext == ".mkv" || ext == ".avi" || ext == ".mov" || ext == ".webm" {
				html += fmt.Sprintf(`
        <li>
            <a href="/video/%s">%s</a>
            <div>Size: %d MB</div>
        </li>`, 
				file.Name(), 
				file.Name(), 
				fileInfo.Size() / (1024 * 1024))
			}
		}
	}
	
	html += `
    </ul>
    <p><em>Note: Videos are transcoded on first access which may take some time depending on the file size.</em></p>
</body>
</html>`
	
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// PlayerHandler serves a simple video player for a specific video
func (h *Handler) PlayerHandler(w http.ResponseWriter, r *http.Request) {
	// Extract the video file from the request path
	videoFile := strings.TrimPrefix(r.URL.Path, "/player/")
	if videoFile == "" {
		http.Error(w, "Video file not specified", http.StatusBadRequest)
		return
	}
	
	// Create a simple HTML page with video.js player
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>%s - Video Player</title>
    <link href="https://cdnjs.cloudflare.com/ajax/libs/video.js/7.11.4/video-js.min.css" rel="stylesheet">
    <script src="https://cdnjs.cloudflare.com/ajax/libs/video.js/7.11.4/video.min.js"></script>
    <style>
        body { margin: 0; padding: 20px; background-color: #f5f5f5; font-family: Arial, sans-serif; }
        .container { max-width: 900px; margin: 0 auto; }
        h1 { color: #333; }
        .video-container { background-color: #000; border-radius: 5px; overflow: hidden; }
    </style>
</head>
<body>
    <div class="container">
        <h1>%s</h1>
        <div class="video-container">
            <video id="my-player" class="video-js vjs-big-play-centered vjs-fluid" controls preload="auto">
                <source src="/video/%s" type="application/x-mpegURL">
                <p class="vjs-no-js">
                    To view this video please enable JavaScript, and consider upgrading to a
                    web browser that <a href="https://videojs.com/html5-video-support/" target="_blank">supports HTML5 video</a>
                </p>
            </video>
        </div>
    </div>

    <script>
        var player = videojs('my-player', {
            fluid: true,
            responsive: true,
            html5: {
                hls: {
                    overrideNative: true
                }
            }
        });
    </script>
</body>
</html>`, videoFile, videoFile, videoFile)
	
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}
