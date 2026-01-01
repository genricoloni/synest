package domain

import "context"

// Monitor defines the interface for monitoring media playback events
// Implementations should handle D-Bus/MPRIS communication
type Monitor interface {
	// Start begins monitoring for media events
	// It should block until context is cancelled or an error occurs
	Start(ctx context.Context) error

	// Stop gracefully stops the monitor
	Stop(ctx context.Context) error

	// Events returns a read-only channel that emits MediaMetadata
	// when media playback state changes
	Events() <-chan MediaMetadata
}

// Processor defines the interface for image processing operations
// Implementations should handle album art transformations
type Processor interface {
	// Generate creates a wallpaper from album art data
	// mode specifies the processing type (e.g., "blur", "gradient", "lyrics")
	// Returns the file path to the generated wallpaper or an error
	Generate(imgData []byte, mode string) (string, error)
}

// ImageProcessor defines the interface for in-memory image processing
// This is OS-agnostic and works purely with byte streams
type ImageProcessor interface {
	// Process transforms image data (e.g., blur, resize, gradient)
	// Returns the processed image bytes or an error
	Process(ctx context.Context, imageData []byte) ([]byte, error)
}

// Fetcher defines the interface for retrieving album artwork
type Fetcher interface {
	// Fetch downloads or reads image data from a URL or local path
	// Returns the raw image bytes or an error
	Fetch(ctx context.Context, url string) ([]byte, error)
}

// Executor defines the interface for executing system commands
type Executor interface {
	// SetWallpaper sets the desktop wallpaper to the specified image path
	SetWallpaper(ctx context.Context, imagePath string) error

	// GetCurrentWallpaper retrieves the path to the currently set wallpaper
	// Returns an error if the operation is not supported or fails
	GetCurrentWallpaper(ctx context.Context) (string, error)
}

// Config defines the interface for application configuration
type Config interface {
	// GetMode returns the current wallpaper generation mode
	GetMode() string

	// GetOutputDir returns the directory for generated wallpapers
	GetOutputDir() string
}
