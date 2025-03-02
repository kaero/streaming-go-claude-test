package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Media    MediaConfig    `mapstructure:"media"`
	Database DatabaseConfig `mapstructure:"database"`
	Library  LibraryConfig  `mapstructure:"library"`
}

// ServerConfig holds server-specific configuration
type ServerConfig struct {
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	TranscodePreset string `mapstructure:"transcode_preset"`
	SegmentFormat   string `mapstructure:"segment_format"`
	SegmentDuration int    `mapstructure:"segment_duration"`
	PlaylistEntries int    `mapstructure:"playlist_entries"`
}

// MediaConfig holds media-specific configuration
type MediaConfig struct {
	MediaDir string `mapstructure:"media_dir"`
	CacheDir string `mapstructure:"cache_dir"`
}

// DatabaseConfig holds database-specific configuration
type DatabaseConfig struct {
	Path string `mapstructure:"path"`
}

// LibraryConfig holds library processing configuration
type LibraryConfig struct {
	ScanOnStart          bool  `mapstructure:"scan_on_start"`
	WatchForChanges      bool  `mapstructure:"watch_for_changes"`
	ScanIntervalMinutes  int   `mapstructure:"scan_interval_minutes"`
	ProcessingThreads    int   `mapstructure:"processing_threads"`
}

// Default configuration values
const (
	DefaultHost                   = "0.0.0.0"
	DefaultPort                   = 8080
	DefaultTranscodePreset        = "ultrafast"
	DefaultSegmentFormat          = "mpegts"
	DefaultSegmentDuration        = 10
	DefaultPlaylistEntries        = 6
	DefaultScanOnStart            = true
	DefaultWatchForChanges        = true
	DefaultScanIntervalMinutes    = 60
	DefaultProcessingThreads      = 2
)

// InitConfig initializes the configuration system
func InitConfig(cfgFile string) (*Config, error) {
	v := viper.New()

	// Set default values
	v.SetDefault("server.host", DefaultHost)
	v.SetDefault("server.port", DefaultPort)
	v.SetDefault("server.transcode_preset", DefaultTranscodePreset)
	v.SetDefault("server.segment_format", DefaultSegmentFormat)
	v.SetDefault("server.segment_duration", DefaultSegmentDuration)
	v.SetDefault("server.playlist_entries", DefaultPlaylistEntries)
	
	// Library config defaults
	v.SetDefault("library.scan_on_start", DefaultScanOnStart)
	v.SetDefault("library.watch_for_changes", DefaultWatchForChanges)
	v.SetDefault("library.scan_interval_minutes", DefaultScanIntervalMinutes)
	v.SetDefault("library.processing_threads", DefaultProcessingThreads)

	// Determine default paths based on executable location
	execDir, err := getExecutableDir()
	if err != nil {
		execDir = "."
	}

	v.SetDefault("media.media_dir", filepath.Join(execDir, "media"))
	v.SetDefault("media.cache_dir", filepath.Join(execDir, "cache"))
	v.SetDefault("database.path", filepath.Join(execDir, "library.db"))

	// Environment variables
	v.SetEnvPrefix("STREAMING")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Config file
	if cfgFile != "" {
		// Use config file from the flag
		v.SetConfigFile(cfgFile)
	} else {
		// Search for config in common locations
		v.AddConfigPath(".")
		v.AddConfigPath("$HOME/.streaming")
		v.AddConfigPath("/etc/streaming")
		v.SetConfigName("config")
		v.SetConfigType("toml")
	}

	// If a config file is found, read it in
	if err := v.ReadInConfig(); err != nil {
		// It's okay if the config file doesn't exist
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// Create configuration structure
	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unable to decode config: %w", err)
	}

	// Create directories if they don't exist
	dirs := []string{cfg.Media.MediaDir, cfg.Media.CacheDir}
	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
			}
		}
	}

	return cfg, nil
}

// WriteDefaultConfig writes a default configuration file
func WriteDefaultConfig(path string) error {
	v := viper.New()

	v.SetDefault("server.host", DefaultHost)
	v.SetDefault("server.port", DefaultPort)
	v.SetDefault("server.transcode_preset", DefaultTranscodePreset)
	v.SetDefault("server.segment_format", DefaultSegmentFormat)
	v.SetDefault("server.segment_duration", DefaultSegmentDuration)
	v.SetDefault("server.playlist_entries", DefaultPlaylistEntries)
	
	// Library config defaults
	v.SetDefault("library.scan_on_start", DefaultScanOnStart)
	v.SetDefault("library.watch_for_changes", DefaultWatchForChanges)
	v.SetDefault("library.scan_interval_minutes", DefaultScanIntervalMinutes)
	v.SetDefault("library.processing_threads", DefaultProcessingThreads)

	// Determine default paths based on executable location
	execDir, err := getExecutableDir()
	if err != nil {
		execDir = "."
	}

	v.SetDefault("media.media_dir", filepath.Join(execDir, "media"))
	v.SetDefault("media.cache_dir", filepath.Join(execDir, "cache"))
	v.SetDefault("database.path", filepath.Join(execDir, "library.db"))

	// Create the directory if it doesn't exist
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	v.SetConfigFile(path)
	return v.WriteConfig()
}

// getExecutableDir returns the directory of the current executable
func getExecutableDir() (string, error) {
	ex, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(ex), nil
}

// DefaultConfig returns a Config with default values (deprecated, use InitConfig instead)
func DefaultConfig() *Config {
	cfg, _ := InitConfig("")
	return cfg
}