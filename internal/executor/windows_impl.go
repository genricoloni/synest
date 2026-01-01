//go:build windows
// +build windows

package executor

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

// WindowsExecutor handles wallpaper setting on Windows systems
type WindowsExecutor struct {
	logger *zap.Logger
}

// NewExecutor creates a new platform-specific wallpaper executor (Windows implementation)
func NewExecutor(logger *zap.Logger) (*WindowsExecutor, error) {
	logger.Info("Windows wallpaper setter initialized")
	return &WindowsExecutor{logger: logger}, nil
}

// SetWallpaper sets the desktop wallpaper using Windows API
func (e *WindowsExecutor) SetWallpaper(ctx context.Context, imagePath string) error {
	e.logger.Info("Setting wallpaper", zap.String("path", imagePath))

	// TODO: Implement Windows wallpaper setting
	// Options:
	// 1. Use syscall to call SystemParametersInfoW
	// 2. Use PowerShell: powershell -Command "Set-ItemProperty -Path 'HKCU:\Control Panel\Desktop' -Name Wallpaper -Value '$imagePath'"
	// 3. Use registry + SPIF_UPDATEINIFILE + SPIF_SENDCHANGE

	return fmt.Errorf("Windows wallpaper setting not yet implemented")
}
// GetCurrentWallpaper is not yet implemented for Windows
func (e *WindowsExecutor) GetCurrentWallpaper(ctx context.Context) (string, error) {
	return "", fmt.Errorf("wallpaper query not yet implemented for Windows")
}