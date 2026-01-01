//go:build !linux && !windows
// +build !linux,!windows

package executor

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

// StubExecutor is a placeholder for unsupported platforms (macOS, BSD, etc.)
type StubExecutor struct {
	logger *zap.Logger
}

// NewExecutor creates a stub executor for unsupported platforms
func NewExecutor(logger *zap.Logger) (*StubExecutor, error) {
	logger.Warn("Wallpaper setting is not yet implemented for this platform")
	return &StubExecutor{logger: logger}, nil
}

// SetWallpaper returns an error indicating the platform is not supported
func (e *StubExecutor) SetWallpaper(ctx context.Context, imagePath string) error {
	return fmt.Errorf("wallpaper setting not implemented for this platform (macOS/BSD support coming soon)")
}

// GetCurrentWallpaper returns an error indicating the platform is not supported
func (e *StubExecutor) GetCurrentWallpaper(ctx context.Context) (string, error) {
	return "", fmt.Errorf("wallpaper query not implemented for this platform (macOS/BSD support coming soon)")
}
