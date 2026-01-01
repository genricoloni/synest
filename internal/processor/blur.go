package processor

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/jpeg" // JPEG format support
	_ "image/png"  // PNG format support
	"os"
	"path/filepath"

	"github.com/disintegration/imaging"
	"github.com/genricoloni/synest/internal/domain"
	"go.uber.org/zap"
)

const (
	defaultBlurRadius   = 15.0
	coverHeightRatio    = 0.40 // Cover size as percentage of screen height
	wallpaperFilename   = "current_wallpaper.jpg"
)

// ProcessorConfig holds configuration for image processing
type ProcessorConfig struct {
	BlurRadius       float64
	CoverSizePercent float64 // Cover size as percentage of screen height (0.0-1.0)
}

// BlurProcessor applies Gaussian blur and resizing to album art images
type BlurProcessor struct {
	logger *zap.Logger
	res    *domain.ScreenResolution // Injected automatically by Fx
	config ProcessorConfig
	appCfg domain.Config // Application configuration for output dir
}

// NewBlurProcessor creates a new blur-based image processor
func NewBlurProcessor(logger *zap.Logger, res *domain.ScreenResolution, appCfg domain.Config) *BlurProcessor {
	return &BlurProcessor{
		logger: logger,
		res:    res,
		appCfg: appCfg,
		config: ProcessorConfig{
			BlurRadius:       defaultBlurRadius,
			CoverSizePercent: coverHeightRatio,
		},
	}
}

// Process transforms image data by creating a blurred background with centered original cover
func (p *BlurProcessor) Process(ctx context.Context, imageData []byte) ([]byte, error) {
	// 1. Decode image from bytes
	img, _, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	// Validate image dimensions to prevent division by zero
	bounds := img.Bounds()
	if bounds.Dy() == 0 || bounds.Dx() == 0 {
		return nil, fmt.Errorf("invalid image dimensions: %dx%d", bounds.Dx(), bounds.Dy())
	}

	// 2. Create blurred background
	// Resize (Fill) to cover entire resolution and apply blur
	p.logger.Debug("Creating blurred background", zap.Int("w", p.res.Width), zap.Int("h", p.res.Height))
	background := imaging.Fill(img, p.res.Width, p.res.Height, imaging.Center, imaging.Lanczos)
	background = imaging.Blur(background, p.config.BlurRadius)

	// 3. Calculate centered cover dimensions (configurable % of screen height, maintaining aspect ratio)
	coverHeight := int(float64(p.res.Height) * p.config.CoverSizePercent)
	coverWidth := coverHeight * bounds.Dx() / bounds.Dy()

	// Resize original cover (sharp, no blur)
	p.logger.Debug("Resizing centered cover", zap.Int("w", coverWidth), zap.Int("h", coverHeight))
	cover := imaging.Resize(img, coverWidth, coverHeight, imaging.Lanczos)

	// 4. Composite: paste sharp cover at center of blurred background
	centerX := (p.res.Width - coverWidth) / 2
	centerY := (p.res.Height - coverHeight) / 2
	result := imaging.Paste(background, cover, image.Pt(centerX, centerY))

	// 5. Encode result to JPEG (in-memory buffer)
	buf := new(bytes.Buffer)
	err = jpeg.Encode(buf, result, &jpeg.Options{Quality: 90})
	if err != nil {
		return nil, fmt.Errorf("failed to encode result: %w", err)
	}

	p.logger.Debug("Image processed successfully", zap.Int("bytes", buf.Len()))
	return buf.Bytes(), nil
}

// Generate creates a wallpaper from album art data and saves it to disk
// This method satisfies the domain.Processor interface
func (p *BlurProcessor) Generate(imgData []byte, mode string) (string, error) {
	// 1. Process image (existing logic)
	processedData, err := p.Process(context.Background(), imgData)
	if err != nil {
		return "", fmt.Errorf("failed to process image: %w", err)
	}

	// 2. Ensure output directory exists
	outputDir := p.appCfg.GetOutputDir()
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// 3. Generate output file path
	outputPath := filepath.Join(outputDir, wallpaperFilename)

	// 4. Write processed image to disk
	if err := os.WriteFile(outputPath, processedData, 0644); err != nil {
		return "", fmt.Errorf("failed to write wallpaper file: %w", err)
	}

	p.logger.Info("Wallpaper generated successfully",
		zap.String("path", outputPath),
		zap.Int("size", len(processedData)),
		zap.String("mode", mode))

	// 5. Return absolute path
	absPath, err := filepath.Abs(outputPath)
	if err != nil {
		return outputPath, nil // Return relative path if abs fails
	}

	return absPath, nil
}
