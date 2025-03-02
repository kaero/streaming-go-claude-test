package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/kaero/streaming/config"
)

var (
	cfgFile            string
	mediaDir           string
	cacheDir           string
	dbPath             string
	listenHost         string
	listenPort         int
	genConfig          bool
	scanOnStart        bool
	watchForChanges    bool
	scanIntervalMinutes int
	processingThreads  int
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "streaming",
	Short: "Video streaming server and library manager",
	Long: `A complete solution for video streaming with transcoding support.

This application has two main components:
1. 'streaming' server - Serves videos and handles user requests
2. 'librarian' - Processes videos in the background
    
It can be configured using command line flags, environment variables,
or a TOML configuration file.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Generate config file if requested
		if genConfig {
			configPath := cfgFile
			if configPath == "" {
				configPath = "./config.toml"
			}
			if err := config.WriteDefaultConfig(configPath); err != nil {
				fmt.Printf("Error generating config file: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Config file generated at: %s\n", configPath)
			return
		}
        
		// If no subcommand is specified, show help
		cmd.Help()
	},
}

// streamingCmd represents the streaming subcommand
var streamingCmd = &cobra.Command{
	Use:   "streaming",
	Short: "Start the HTTP streaming server",
	Long: `Starts the HTTP streaming server that serves videos.
The streaming server serves preprocessed videos from the library
and handles user requests.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runServer(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	},
}

// librarianCmd represents the librarian subcommand
var librarianCmd = &cobra.Command{
	Use:   "librarian",
	Short: "Start the library processing service",
	Long: `Starts the library processing service that scans for new videos
and processes them in the background.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runLibrarian(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	// Load configuration from file, environment, and flags
	cobra.OnInitialize(initConfig)

	// Define global persistent flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./config.toml)")
	rootCmd.PersistentFlags().StringVar(&mediaDir, "media-dir", "", "directory containing media files")
	rootCmd.PersistentFlags().StringVar(&cacheDir, "cache-dir", "", "directory for cached transcoded files")
	rootCmd.PersistentFlags().StringVar(&dbPath, "db-path", "", "path to the SQLite database file")
	rootCmd.PersistentFlags().BoolVar(&genConfig, "gen-config", false, "generate a default config file")

	// Streaming server specific flags
	streamingCmd.Flags().StringVar(&listenHost, "host", "", "host to listen on")
	streamingCmd.Flags().IntVar(&listenPort, "port", 0, "port to listen on")

	// Librarian specific flags
	librarianCmd.Flags().BoolVar(&scanOnStart, "scan-on-start", true, "scan for new videos on start")
	librarianCmd.Flags().BoolVar(&watchForChanges, "watch", true, "watch for file system changes")
	librarianCmd.Flags().IntVar(&scanIntervalMinutes, "scan-interval", 60, "interval between scans (minutes)")
	librarianCmd.Flags().IntVar(&processingThreads, "threads", 2, "number of processing threads")

	// Add subcommands
	rootCmd.AddCommand(streamingCmd)
	rootCmd.AddCommand(librarianCmd)
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	// This is just to prepare for reading the config.
	// Actual config loading happens in command functions.
}

// Configuration variable used globally
var cfg *config.Config