# Synest

**Synest** is a lightweight, high-performance daemon written in Go that monitors media playback via D-Bus (MPRIS) and generates dynamic wallpapers based on album artwork. It is based on a previous project called [syncWall](https://github.com/genricoloni/spotifysyncwall), replacing Python with Go for improved performance and resource efficiency, and removing Spotify-specific dependencies.

## Features

- Real-time media playback monitoring via MPRIS/D-Bus
- Dynamic wallpaper generation with multiple modes:
  - **Blur**: Blurred album art backgrounds
  - **Gradient**: Color gradient extraction from artwork
  - **Lyrics**: Lyrics overlay on artwork (planned)
- Resource-efficient Go implementation
- Clean architecture with dependency injection (Fx)

## Tech Stack

- **Language**: Go 1.22+
- **Dependency Injection**: [Fx](https://github.com/uber-go/fx)
- **Logging**: [Zap](https://github.com/uber-go/zap)

## Project Structure

```
synest/
├── cmd/
│   └── daemon/          # Main entry point
├── internal/
│   ├── domain/          # Core interfaces and models (ports)
│   ├── monitor/         # D-Bus/MPRIS adapter
│   ├── fetcher/         # HTTP/File fetcher adapter
│   ├── processor/       # Image processing adapter
│   ├── executor/        # Shell command adapter
│   ├── config/          # Configuration adapter
│   └── engine/          # Business logic orchestration
├── Makefile             # Build automation
└── README.md
```

## Getting Started

### Prerequisites

- Go 1.22 or higher
- D-Bus (available on most Linux distributions)
- golangci-lint (for development)

### Installation

```bash
# Clone the repository
git clone https://github.com/genricoloni/synest.git
cd synest

# Download dependencies
make deps

# Build the binary
make build

# Install (optional)
sudo make install
```

### Running

```bash
# Run directly
make run

# Or run the built binary
./bin/synest
```

## Development

### Building

```bash
make build
```

### Running Tests

```bash
make test

# With coverage
make test-coverage
```

### Linting

```bash
make lint
```

### Available Make Commands

- `make build` - Build the binary
- `make run` - Run the application
- `make test` - Run all tests
- `make test-coverage` - Run tests with coverage report
- `make clean` - Remove build artifacts
- `make lint` - Run golangci-lint
- `make tidy` - Tidy go.mod
- `make deps` - Download dependencies
- `make install` - Install binary to /usr/local/bin

## Contributing

Contributions are welcome! Please ensure:

1. All code passes `golangci-lint` checks
2. Tests are included for new features
3. Code follows the established architecture patterns

## License

MIT License - see LICENSE file for details

## Acknowledgments

Built with:
- [Fx](https://github.com/uber-go/fx) - Dependency injection framework
- [Zap](https://github.com/uber-go/zap) - Blazing fast, structured logging
