package utils

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/kaero/streaming/config"
)

// CreateDirectories ensures all required directories exist
func CreateDirectories(cfg *config.Config) error {
	dirs := []string{cfg.Media.MediaDir, cfg.Media.CacheDir}
	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return err
			}
		}
	}
	return nil
}

// CleanupCache periodically removes old cache files
func CleanupCache(cfg *config.Config) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	
	for range ticker.C {
		// Get all directories in cache
		dirs, err := os.ReadDir(cfg.Media.CacheDir)
		if err != nil {
			log.Printf("Error reading cache directory: %v", err)
			continue
		}
		
		// Check modification time of each directory
		for _, dir := range dirs {
			if !dir.IsDir() {
				continue
			}
			
			dirPath := filepath.Join(cfg.Media.CacheDir, dir.Name())
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