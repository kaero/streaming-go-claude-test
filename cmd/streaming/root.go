package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/kaero/streaming/config"
)

var (
	cfgFile    string
	mediaDir   string
	cacheDir   string
	listenHost string
	listenPort int
	genConfig  bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "streaming",
	Short: "HTTP video streaming server with transcoding",
	Long: `An HTTP video streaming server with on-the-fly transcoding
that converts videos to HLS format for streaming.

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

		// Run the server
		if err := runServer(); err != nil {
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

	// Define flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./config.toml)")
	rootCmd.PersistentFlags().StringVar(&mediaDir, "media-dir", "", "directory containing media files")
	rootCmd.PersistentFlags().StringVar(&cacheDir, "cache-dir", "", "directory for cached transcoded files")
	rootCmd.PersistentFlags().StringVar(&listenHost, "host", "", "host to listen on")
	rootCmd.PersistentFlags().IntVar(&listenPort, "port", 0, "port to listen on")
	rootCmd.PersistentFlags().BoolVar(&genConfig, "gen-config", false, "generate a default config file")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	// This is just to prepare for reading the config.
	// Actual config loading happens in runServer().
}

// Configuration variable used globally
var cfg *config.Config