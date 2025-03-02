# Go Video Streaming Server

A simple HTTP video streaming server written in Go with on-the-fly transcoding support using FFmpeg.

## Features

- On-demand video transcoding to HLS format
- Adaptive streaming with multiple quality levels
- Built-in video player with video.js
- Automatic cache management
- Simple web UI for browsing videos
- Configurable via CLI, environment variables, and TOML config file

## Requirements

- Go 1.24 or later
- FFmpeg installed and available in PATH

## Installation

```bash
go build -o streaming ./cmd/streaming
```

## Configuration

The server can be configured in several ways (in order of precedence):

1. Command-line flags
2. Environment variables
3. Configuration file
4. Default values

### Command-line Flags

```
Usage:
  streaming [flags]

Flags:
      --cache-dir string   directory for cached transcoded files
      --config string      config file (default is ./config.toml)
      --gen-config         generate a default config file
  -h, --help               help for streaming
      --host string        host to listen on
      --media-dir string   directory containing media files
      --port int           port to listen on
```

### Environment Variables

All configuration parameters can be set with environment variables using the prefix `STREAMING_` followed by the parameter name. For example:

```bash
STREAMING_SERVER_HOST=127.0.0.1 STREAMING_SERVER_PORT=9000 ./streaming
```

### Configuration File

The server looks for a configuration file in the following locations:
- Path specified by the `--config` flag
- `./config.toml`
- `$HOME/.streaming/config.toml`
- `/etc/streaming/config.toml`

You can generate a default configuration file with:

```bash
./streaming --gen-config [--config=your-config-path.toml]
```

Example TOML configuration:

```toml
[server]
host = "0.0.0.0"
port = 8080
transcode_preset = "ultrafast"
segment_format = "mpegts"
segment_duration = 10
playlist_entries = 6

[media]
media_dir = "/path/to/media"
cache_dir = "/path/to/cache"
```

## Usage

1. Start the server:

```bash
./streaming
```

2. Access the server at http://localhost:8080

## Project Structure

- `/cmd/streaming`: Main application entry point
- `/config`: Application configuration
- `/internal/handlers`: HTTP handlers
- `/internal/transcoder`: Video transcoding logic
- `/internal/utils`: Utility functions
- `/internal/templates`: HTML templates

## License

MIT