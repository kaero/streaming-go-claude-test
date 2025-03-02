# Go Video Streaming Server

A complete solution for video streaming with transcoding support using FFmpeg, featuring a server component and background processing.

## Features

- Separate streaming server and library processor components
- Background video transcoding to HLS format
- Adaptive streaming with multiple quality levels
- Built-in video player with video.js
- Automatic cache management
- Simple web UI for browsing videos
- Library management with status tracking
- File system watching for automatic processing
- SQLite database for library state
- Configurable via CLI, environment variables, and TOML config file

## Requirements

- Go 1.24 or later
- FFmpeg installed and available in PATH
- SQLite3

## Installation

```bash
go build -o streaming ./cmd/streaming
```

## Command Structure

The application has two main components that can be run separately:

```
streaming - Main command (shows help when run without subcommands)
  ├── streaming - Start the HTTP streaming server
  └── librarian - Start the library processing service
```

### Streaming Server

The streaming server handles HTTP requests and serves videos to users:

```bash
./streaming streaming [flags]
```

Flags:
```
--host string         host to listen on
--port int            port to listen on
```

### Librarian

The librarian processes videos in the background and manages the media library:

```bash
./streaming librarian [flags]
```

Flags:
```
--scan-on-start       scan for new videos on start (default true)
--scan-interval int   interval between scans in minutes (default 60)
--threads int         number of processing threads (default 2)
--watch               watch for file system changes (default true)
```

### Global Flags

These flags apply to both subcommands:

```
--cache-dir string    directory for cached transcoded files
--config string       config file (default is ./config.toml)
--db-path string      path to the SQLite database file
--gen-config          generate a default config file
--media-dir string    directory containing media files
```

## Configuration

The application can be configured in several ways (in order of precedence):

1. Command-line flags
2. Environment variables
3. Configuration file
4. Default values

### Environment Variables

All configuration parameters can be set with environment variables using the prefix `STREAMING_` followed by the parameter name. For example:

```bash
STREAMING_SERVER_HOST=127.0.0.1 STREAMING_SERVER_PORT=9000 ./streaming streaming
```

### Configuration File

The application looks for a configuration file in the following locations:
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

[database]
path = "/path/to/library.db"

[library]
scan_on_start = true
watch_for_changes = true
scan_interval_minutes = 60
processing_threads = 2
```

## Typical Usage

1. Start the librarian service in background:

```bash
./streaming librarian &
```

2. Start the streaming server:

```bash
./streaming streaming
```

3. Access the server at http://localhost:8080

For production use, consider using a process manager like systemd to keep both services running.

## Project Structure

- `/cmd/streaming`: Main application entry point with subcommands
- `/config`: Application configuration
- `/internal/handlers`: HTTP handlers
- `/internal/transcoder`: Video transcoding logic
- `/internal/utils`: Utility functions
- `/internal/templates`: HTML templates
- `/internal/database`: SQLite database operations
- `/internal/library`: Library management

## License

MIT