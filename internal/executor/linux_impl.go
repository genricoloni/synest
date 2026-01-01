//go:build linux
// +build linux

package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"go.uber.org/zap"
)

// WallpaperCommand represents a detected wallpaper setter command
type WallpaperCommand struct {
	Name    string
	Binary  string
	Args    []string // %s will be replaced with image path
	UsesURI bool     // If true, path will be prefixed with file://
}

var (
	// Ordered list of wallpaper commands to try (highest priority first)
	wallpaperCommands = []WallpaperCommand{
		// Hyprland - swww (recommended)
		{Name: "swww", Binary: "swww", Args: []string{"img", "%s"}},
		// Hyprland - hyprpaper
		{Name: "hyprpaper", Binary: "hyprctl", Args: []string{"hyprpaper", "wallpaper", ",%s"}},
		// swaybg (Sway/Wayland)
		{Name: "swaybg", Binary: "swaybg", Args: []string{"-i", "%s", "-m", "fill"}},
		// GNOME (dark theme)
		{Name: "gnome", Binary: "gsettings", Args: []string{"set", "org.gnome.desktop.background", "picture-uri-dark", "file://%s"}, UsesURI: true},
		// Generic X11 - feh
		{Name: "feh", Binary: "feh", Args: []string{"--bg-fill", "%s"}},
		// Generic X11 - nitrogen
		{Name: "nitrogen", Binary: "nitrogen", Args: []string{"--set-zoom-fill", "%s"}},
	}
)

// LinuxExecutor handles wallpaper setting on Linux systems
type LinuxExecutor struct {
	logger  *zap.Logger
	command WallpaperCommand
}

// NewExecutor creates a new platform-specific wallpaper executor (Linux implementation)
func NewExecutor(logger *zap.Logger) (*LinuxExecutor, error) {
	cmd := detectCommand(logger)
	if cmd.Binary == "" {
		return nil, fmt.Errorf("no supported wallpaper command found on this system")
	}

	logger.Info("Wallpaper setter detected",
		zap.String("name", cmd.Name),
		zap.String("binary", cmd.Binary))

	return &LinuxExecutor{
		logger:  logger,
		command: cmd,
	}, nil
}

// NewLinuxExecutor is deprecated, use NewExecutor instead
// Kept for backward compatibility
func NewLinuxExecutor(logger *zap.Logger) (*LinuxExecutor, error) {
	return NewExecutor(logger)
}

// detectCommand analyzes the environment to choose the best wallpaper command
func detectCommand(logger *zap.Logger) WallpaperCommand {
	// Check environment variables for hints
	desktop := os.Getenv("XDG_CURRENT_DESKTOP")
	session := os.Getenv("XDG_SESSION_TYPE")
	wayland := os.Getenv("WAYLAND_DISPLAY")
	hyprland := os.Getenv("HYPRLAND_INSTANCE_SIGNATURE")

	logger.Debug("Detecting wallpaper command",
		zap.String("desktop", desktop),
		zap.String("session", session),
		zap.String("wayland", wayland),
		zap.String("hyprland", hyprland))

	// Priority-based detection
	if hyprland != "" {
		// Running on Hyprland - prefer swww or hyprpaper
		for _, cmd := range wallpaperCommands {
			if (cmd.Name == "swww" || cmd.Name == "hyprpaper") && commandExists(cmd.Binary) {
				return cmd
			}
		}
	}

	if strings.Contains(strings.ToLower(desktop), "gnome") {
		// GNOME desktop
		for _, cmd := range wallpaperCommands {
			if cmd.Name == "gnome" && commandExists(cmd.Binary) {
				return cmd
			}
		}
	}

	if wayland != "" || session == "wayland" {
		// Wayland session - prefer Wayland-native tools
		for _, cmd := range wallpaperCommands {
			if (cmd.Name == "swww" || cmd.Name == "swaybg") && commandExists(cmd.Binary) {
				return cmd
			}
		}
	}

	// Fallback: try all commands in order
	for _, cmd := range wallpaperCommands {
		if commandExists(cmd.Binary) {
			logger.Info("Using fallback wallpaper command", zap.String("name", cmd.Name))
			return cmd
		}
	}

	return WallpaperCommand{} // No command found
}

// commandExists checks if a binary exists in PATH
func commandExists(binary string) bool {
	_, err := exec.LookPath(binary)
	return err == nil
}

// SetWallpaper sets the desktop wallpaper to the specified image
func (e *LinuxExecutor) SetWallpaper(ctx context.Context, imagePath string) error {
	// Build command arguments
	args := make([]string, len(e.command.Args))
	for i, arg := range e.command.Args {
		if strings.Contains(arg, "%s") {
			path := imagePath
			if e.command.UsesURI {
				// GNOME requires file:// URI
				path = imagePath // %s template already includes file://
			}
			args[i] = strings.ReplaceAll(arg, "%s", path)
		} else {
			args[i] = arg
		}
	}

	e.logger.Debug("Setting wallpaper",
		zap.String("command", e.command.Binary),
		zap.Strings("args", args),
		zap.String("path", imagePath))

	// Execute command
	cmd := exec.CommandContext(ctx, e.command.Binary, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set wallpaper with %s: %w (output: %s)",
			e.command.Name, err, string(output))
	}

	e.logger.Info("Wallpaper set successfully",
		zap.String("command", e.command.Name),
		zap.String("path", imagePath))

	return nil
}

// GetCurrentWallpaper retrieves the path to the currently set wallpaper
func (e *LinuxExecutor) GetCurrentWallpaper(ctx context.Context) (string, error) {
	switch e.command.Name {
	case "swww":
		return e.getCurrentWallpaperSwww(ctx)
	case "hyprpaper":
		return "", fmt.Errorf("hyprpaper does not support querying current wallpaper")
	case "gnome":
		return e.getCurrentWallpaperGnome(ctx)
	case "feh", "swaybg", "nitrogen":
		// These tools don't provide easy ways to query current wallpaper
		return "", fmt.Errorf("%s does not support querying current wallpaper", e.command.Name)
	default:
		return "", fmt.Errorf("wallpaper query not supported for %s", e.command.Name)
	}
}

// getCurrentWallpaperSwww queries swww for the current wallpaper
func (e *LinuxExecutor) getCurrentWallpaperSwww(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "swww", "query")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to query swww: %w (output: %s)", err, string(output))
	}

	// Parse output: "eDP-1: image: /path/to/wallpaper.jpg"
	// Format can vary, but typically contains "image:" followed by the path
	outputStr := strings.TrimSpace(string(output))
	lines := strings.Split(outputStr, "\n")

	for _, line := range lines {
		// Look for "image:" in the line
		if idx := strings.Index(line, "image:"); idx != -1 {
			// Extract path after "image:"
			path := strings.TrimSpace(line[idx+6:])
			if path != "" {
				e.logger.Debug("Captured current wallpaper from swww",
					zap.String("path", path))
				return path, nil
			}
		}
	}

	return "", fmt.Errorf("could not parse wallpaper path from swww query output: %s", outputStr)
}

// getCurrentWallpaperGnome queries gsettings for the current wallpaper
func (e *LinuxExecutor) getCurrentWallpaperGnome(ctx context.Context) (string, error) {
	// Try dark theme first (as we set it)
	cmd := exec.CommandContext(ctx, "gsettings", "get", "org.gnome.desktop.background", "picture-uri-dark")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Fallback to light theme
		cmd = exec.CommandContext(ctx, "gsettings", "get", "org.gnome.desktop.background", "picture-uri")
		output, err = cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("failed to query gnome wallpaper: %w", err)
		}
	}

	// Parse output: 'file:///path/to/wallpaper.jpg'
	uri := strings.TrimSpace(string(output))
	// Remove quotes
	uri = strings.Trim(uri, "'\"")
	// Remove file:// prefix if present
	path := strings.TrimPrefix(uri, "file://")

	if path == "" {
		return "", fmt.Errorf("empty wallpaper path from gsettings")
	}

	e.logger.Debug("Captured current wallpaper from gnome",
		zap.String("path", path))

	return path, nil
}
