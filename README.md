# Go Video Streaming Server

A simple HTTP video streaming server written in Go with on-the-fly transcoding support using FFmpeg.

## Features

- On-demand video transcoding to HLS format
- Adaptive streaming with multiple quality levels
- Built-in video player with video.js
- Automatic cache management
- Simple web UI for browsing videos

## Requirements

- Go 1.24 or later
- FFmpeg installed and available in PATH

## Usage

1. Build the server:

```bash
go build -o streaming ./cmd/server
```

2. Run the server:

```bash
./streaming
```

3. Place your video files in the `/var/home/kaero/Code/streaming/media` directory (or change the path in config)

4. Access the server at http://localhost:8080

## Configuration

Configuration values are defined in `config/config.go`. You can modify these values to change:

- Server port
- Media and cache directories
- Transcoding parameters
- Segment duration and format

## Project Structure

- `/cmd/server`: Main application entry point
- `/config`: Application configuration
- `/internal/handlers`: HTTP handlers
- `/internal/transcoder`: Video transcoding logic
- `/internal/utils`: Utility functions

## License

MIT