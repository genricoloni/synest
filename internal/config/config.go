package config

import (
	"os"
	"path/filepath"

	"go.uber.org/zap"
)

const (
	defaultOutputDir = "/tmp/synest"
	defaultMode      = "blur"
)

// AppConfig holds application configuration
type AppConfig struct {
	logger    *zap.Logger
	outputDir string
	mode      string
}

// NewAppConfig creates a new application configuration instance
func NewAppConfig(logger *zap.Logger) *AppConfig {
	// Read from environment variables or use defaults
	outputDir := os.Getenv("SYNEST_OUTPUT_DIR")
	if outputDir == "" {
		outputDir = defaultOutputDir
	}

	mode := os.Getenv("SYNEST_MODE")
	if mode == "" {
		mode = defaultMode
	}

	// Expand path if it contains ~ or environment variables
	outputDir = os.ExpandEnv(outputDir)
	if len(outputDir) > 0 && outputDir[0] == '~' {
		home, err := os.UserHomeDir()
		if err == nil {
			outputDir = filepath.Join(home, outputDir[1:])
		}
	}

	logger.Info("Configuration loaded",
		zap.String("outputDir", outputDir),
		zap.String("mode", mode))

	return &AppConfig{
		logger:    logger,
		outputDir: outputDir,
		mode:      mode,
	}
}

// GetMode returns the current wallpaper generation mode
func (c *AppConfig) GetMode() string {
	return c.mode
}

// GetOutputDir returns the directory for generated wallpapers
func (c *AppConfig) GetOutputDir() string {
	return c.outputDir
}
