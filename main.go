package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// Configuration
	PORT             = 8080
	MEDIA_DIR        = "/var/home/kaero/Code/streaming/media"       // Source media files
	CACHE_DIR        = "/var/home/kaero/Code/streaming/cache"       // Transcoded segments
	SEGMENT_DURATION = 10              // Duration of each segment in seconds
	TRANSCODE_PRESET = "ultrafast"     // FFmpeg preset (ultrafast, superfast, veryfast, faster, fast, medium, slow, slower, veryslow)
	SEGMENT_FORMAT   = "mpegts"        // Format for the segments
	PLAYLIST_ENTRIES = 6               // Number of segments to keep in the playlist
)

// VideoJob represents a transcoding task
type VideoJob struct {
	SourceFile     string
	OutputPath     string
	Width          int
	Height         int
	Bitrate        string
	SegmentDuration int
}

// TranscodingManager handles the transcoding operations
type TranscodingManager struct {
	activeJobs map[string]bool
	mutex      sync.Mutex
}

func NewTranscodingManager() *TranscodingManager {
	return &TranscodingManager{
		activeJobs: make(map[string]bool),
	}
}

// IsJobActive checks if a transcoding job is already in progress
func (tm *TranscodingManager) IsJobActive(jobKey string) bool {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()
	return tm.activeJobs[jobKey]
}

// SetJobActive marks a transcoding job as active or inactive
func (tm *TranscodingManager) SetJobActive(jobKey string, active bool) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()
	if active {
		tm.activeJobs[jobKey] = true
	} else {
		delete(tm.activeJobs, jobKey)
	}
}

// CreateDirectories ensures all required directories exist
func CreateDirectories() error {
	dirs := []string{MEDIA_DIR, CACHE_DIR}
	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return err
			}
		}
	}
	return nil
}

// TranscodeToHLS transcodes a video file to HLS format
func (tm *TranscodingManager) TranscodeToHLS(job VideoJob) error {
	// Create a unique key for this job
	jobKey := fmt.Sprintf("%s_%d_%d_%s", job.SourceFile, job.Width, job.Height, job.Bitrate)
	
	// Check if this job is already in progress
	if tm.IsJobActive(jobKey) {
		return nil
	}
	
	// Mark job as active
	tm.SetJobActive(jobKey, true)
	defer tm.SetJobActive(jobKey, false)
	
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(job.OutputPath), 0755); err != nil {
		return err
	}
	
	// Build FFmpeg command for HLS transcoding
	args := []string{
		"-i", job.SourceFile,
		"-c:v", "libx264",
		"-crf", "23",
		"-preset", TRANSCODE_PRESET,
		"-c:a", "aac",
		"-b:a", "128k",
	}
	
	// Add resolution parameters if specified
	if job.Width > 0 && job.Height > 0 {
		args = append(args, "-vf", fmt.Sprintf("scale=%d:%d", job.Width, job.Height))
	}
	
	// Add bitrate if specified
	if job.Bitrate != "" {
		args = append(args, "-b:v", job.Bitrate)
	}
	
	// Add HLS specific parameters
	args = append(args, 
		"-f", "hls",
		"-hls_time", strconv.Itoa(job.SegmentDuration),
		"-hls_segment_type", SEGMENT_FORMAT,
		"-hls_list_size", strconv.Itoa(PLAYLIST_ENTRIES),
		"-hls_playlist_type", "event",
		"-hls_segment_filename", fmt.Sprintf("%s%%03d.ts", strings.TrimSuffix(job.OutputPath, ".m3u8")),
		job.OutputPath,
	)
	
	// Execute FFmpeg command
	cmd := exec.Command("ffmpeg", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("FFmpeg error: %v\nOutput: %s\n", err, output)
		return fmt.Errorf("transcoding failed: %v", err)
	}
	
	return nil
}

// GenerateHLSMasterPlaylist creates a master playlist for adaptive streaming
func GenerateHLSMasterPlaylist(videoFile, outputDir string, qualities []map[string]string) (string, error) {
	// Create master playlist
	masterPlaylist := "#EXTM3U\n"
	masterPlaylist += "#EXT-X-VERSION:3\n"
	
	// Add each quality variant
	for _, quality := range qualities {
		width := quality["width"]
		height := quality["height"]
		bitrate := quality["bitrate"]
		
		bandwidthKbps, _ := strconv.Atoi(strings.TrimSuffix(bitrate, "k"))
		bandwidthBps := bandwidthKbps * 1000
		
		masterPlaylist += fmt.Sprintf("#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%s,NAME=\"%sp\"\n", 
			bandwidthBps, width+"x"+height, height)
		
		variantFile := fmt.Sprintf("%s_%s.m3u8", filepath.Base(videoFile), height)
		masterPlaylist += variantFile + "\n"
	}
	
	// Write master playlist file
	masterPath := filepath.Join(outputDir, filepath.Base(videoFile)+".m3u8")
	err := ioutil.WriteFile(masterPath, []byte(masterPlaylist), 0644)
	if err != nil {
		return "", err
	}
	
	return masterPath, nil
}

// PrepareVideo prepares a video for HLS streaming
func (tm *TranscodingManager) PrepareVideo(videoPath string) (string, error) {
	// Create destination directory
	videoFileName := filepath.Base(videoPath)
	outputDir := filepath.Join(CACHE_DIR, strings.TrimSuffix(videoFileName, filepath.Ext(videoFileName)))
	
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", err
	}
	
	// Define quality variants
	qualities := []map[string]string{
		{"width": "1280", "height": "720", "bitrate": "2500k"},
		//{"width": "854", "height": "480", "bitrate": "1000k"},
		//{"width": "640", "height": "360", "bitrate": "500k"},
	}
	
	// Start transcoding for each quality
	var wg sync.WaitGroup
	for _, quality := range qualities {
		wg.Add(1)
		go func(q map[string]string) {
			defer wg.Done()
			
			width, _ := strconv.Atoi(q["width"])
			height, _ := strconv.Atoi(q["height"])
			
			outputFile := filepath.Join(outputDir, 
				fmt.Sprintf("%s_%s.m3u8", videoFileName, q["height"]))
			
			job := VideoJob{
				SourceFile:      videoPath,
				OutputPath:      outputFile,
				Width:           width,
				Height:          height,
				Bitrate:         q["bitrate"],
				SegmentDuration: SEGMENT_DURATION,
			}
			
			if err := tm.TranscodeToHLS(job); err != nil {
				log.Printf("Error transcoding %s to %s: %v", videoPath, outputFile, err)
			}
		}(quality)
	}
	
	// Wait for all transcoding jobs to complete
	wg.Wait()
	
	// Generate master playlist
	masterPath, err := GenerateHLSMasterPlaylist(videoFileName, outputDir, qualities)
	if err != nil {
		return "", err
	}
	
	return masterPath, nil
}

// VideoHandler handles requests for video streaming
func VideoHandler(w http.ResponseWriter, r *http.Request, tm *TranscodingManager) {
	// Extract the video file from the request path
	videoFile := strings.TrimPrefix(r.URL.Path, "/video/")
	if videoFile == "" {
		http.Error(w, "Video file not specified", http.StatusBadRequest)
		return
	}
	
	// Check if the requested file exists
	videoPath := filepath.Join(MEDIA_DIR, videoFile)
	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		http.Error(w, "Video file not found", http.StatusNotFound)
		return
	}
	
	// Create the output directory path
	outputDir := filepath.Join(CACHE_DIR, strings.TrimSuffix(videoFile, filepath.Ext(videoFile)))
	masterPlaylist := filepath.Join(outputDir, videoFile+".m3u8")
	
	// Check if master playlist already exists
	if _, err := os.Stat(masterPlaylist); os.IsNotExist(err) {
		// Prepare video for streaming (transcoding)
		var err error
		masterPlaylist, err = tm.PrepareVideo(videoPath)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error preparing video: %v", err), http.StatusInternalServerError)
			return
		}
	}
	
	// Redirect to the master playlist
	relativePlaylist := strings.TrimPrefix(masterPlaylist, CACHE_DIR+"/")
	http.Redirect(w, r, "/stream/"+relativePlaylist, http.StatusFound)
}

// StreamHandler serves HLS files
func StreamHandler(w http.ResponseWriter, r *http.Request) {
	// Extract the file path from the request
	filePath := strings.TrimPrefix(r.URL.Path, "/stream/")
	fullPath := filepath.Join(CACHE_DIR, filePath)
	
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

// CleanupCache periodically removes old cache files
func CleanupCache() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	
	for range ticker.C {
		// Get all directories in cache
		dirs, err := ioutil.ReadDir(CACHE_DIR)
		if err != nil {
			log.Printf("Error reading cache directory: %v", err)
			continue
		}
		
		// Check modification time of each directory
		for _, dir := range dirs {
			if !dir.IsDir() {
				continue
			}
			
			dirPath := filepath.Join(CACHE_DIR, dir.Name())
			info, err := os.Stat(dirPath)
			if err != nil {
				continue
			}
			
			// Remove directories older than 24 hours
			if time.Since(info.ModTime()) > 24*time.Hour {
				log.Printf("Removing old cache: %s", dirPath)
				os.RemoveAll(dirPath)
			}
		}
	}
}

// ListVideosHandler serves a simple UI listing available videos
func ListVideosHandler(w http.ResponseWriter, r *http.Request) {
	files, err := ioutil.ReadDir(MEDIA_DIR)
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
				file.Size() / (1024 * 1024))
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
func PlayerHandler(w http.ResponseWriter, r *http.Request) {
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

func main() {
	// Create required directories
	if err := CreateDirectories(); err != nil {
		log.Fatalf("Error creating directories: %v", err)
	}
	
	// Create transcoding manager
	tm := NewTranscodingManager()
	
	// Setup HTTP handlers
	http.HandleFunc("/", ListVideosHandler)
	http.HandleFunc("/video/", func(w http.ResponseWriter, r *http.Request) {
		VideoHandler(w, r, tm)
	})
	http.HandleFunc("/stream/", StreamHandler)
	http.HandleFunc("/player/", PlayerHandler)
	
	// Start cache cleanup goroutine
	go CleanupCache()
	
	// Start the server
	serverAddr := fmt.Sprintf(":%d", PORT)
	log.Printf("Starting server on http://localhost%s", serverAddr)
	log.Fatal(http.ListenAndServe(serverAddr, nil))
}
