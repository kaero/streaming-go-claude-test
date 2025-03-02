package transcoder

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/kaero/streaming/config"
)

// VideoJob represents a transcoding task
type VideoJob struct {
	SourceFile      string
	OutputPath      string
	Width           int
	Height          int
	Bitrate         string
	SegmentDuration int
}

// Manager handles the transcoding operations
type Manager struct {
	activeJobs map[string]bool
	mutex      sync.Mutex
	config     *config.Config
}

// NewManager creates a new transcoding manager
func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		activeJobs: make(map[string]bool),
		config:     cfg,
	}
}

// IsJobActive checks if a transcoding job is already in progress
func (tm *Manager) IsJobActive(jobKey string) bool {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()
	return tm.activeJobs[jobKey]
}

// SetJobActive marks a transcoding job as active or inactive
func (tm *Manager) SetJobActive(jobKey string, active bool) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()
	if active {
		tm.activeJobs[jobKey] = true
	} else {
		delete(tm.activeJobs, jobKey)
	}
}

// TranscodeToHLS transcodes a video file to HLS format
func (tm *Manager) TranscodeToHLS(job VideoJob) error {
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
		"-preset", tm.config.TranscodePreset,
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
		"-hls_segment_type", tm.config.SegmentFormat,
		"-hls_list_size", strconv.Itoa(tm.config.PlaylistEntries),
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
	err := os.WriteFile(masterPath, []byte(masterPlaylist), 0644)
	if err != nil {
		return "", err
	}
	
	return masterPath, nil
}

// PrepareVideo prepares a video for HLS streaming
func (tm *Manager) PrepareVideo(videoPath string) (string, error) {
	// Create destination directory
	videoFileName := filepath.Base(videoPath)
	outputDir := filepath.Join(tm.config.CacheDir, strings.TrimSuffix(videoFileName, filepath.Ext(videoFileName)))
	
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
				SegmentDuration: tm.config.SegmentDuration,
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