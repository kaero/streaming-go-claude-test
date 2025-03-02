package config

// Config holds all configuration for the application
type Config struct {
	Port            int
	MediaDir        string
	CacheDir        string
	SegmentDuration int
	TranscodePreset string
	SegmentFormat   string
	PlaylistEntries int
}

// DefaultConfig returns a Config with default values
func DefaultConfig() *Config {
	return &Config{
		Port:            8080,
		MediaDir:        "/var/home/kaero/Code/streaming/media",
		CacheDir:        "/var/home/kaero/Code/streaming/cache",
		SegmentDuration: 10,
		TranscodePreset: "ultrafast",
		SegmentFormat:   "mpegts",
		PlaylistEntries: 6,
	}
}