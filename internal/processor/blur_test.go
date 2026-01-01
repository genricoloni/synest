package processor

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"strings"
	"testing"

	"github.com/genricoloni/synest/internal/domain"
	"go.uber.org/zap"
)

func TestBlurProcessor_Process(t *testing.T) {
	tests := []struct {
		name          string
		imageData     []byte
		resolution    *domain.ScreenResolution
		expectedError string
		validateFunc  func(t *testing.T, result []byte)
	}{
		{
			name:       "Success - Valid JPEG 1920x1080",
			imageData:  createTestJPEG(100, 100, color.RGBA{R: 255, G: 0, B: 0, A: 255}),
			resolution: &domain.ScreenResolution{Width: 1920, Height: 1080},
			validateFunc: func(t *testing.T, result []byte) {
				if len(result) == 0 {
					t.Error("expected non-empty result")
				}
				// Verify it's a valid JPEG
				img, _, err := image.Decode(bytes.NewReader(result))
				if err != nil {
					t.Errorf("result is not a valid image: %v", err)
				}
				// Verify dimensions
				bounds := img.Bounds()
				if bounds.Dx() != 1920 || bounds.Dy() != 1080 {
					t.Errorf("expected 1920x1080, got %dx%d", bounds.Dx(), bounds.Dy())
				}
			},
		},
		{
			name:       "Success - Different Resolution 800x600",
			imageData:  createTestJPEG(200, 150, color.RGBA{R: 0, G: 255, B: 0, A: 255}),
			resolution: &domain.ScreenResolution{Width: 800, Height: 600},
			validateFunc: func(t *testing.T, result []byte) {
				img, _, err := image.Decode(bytes.NewReader(result))
				if err != nil {
					t.Errorf("failed to decode result: %v", err)
				}
				bounds := img.Bounds()
				if bounds.Dx() != 800 || bounds.Dy() != 600 {
					t.Errorf("expected 800x600, got %dx%d", bounds.Dx(), bounds.Dy())
				}
			},
		},
		{
			name:          "Error - Invalid Image Data",
			imageData:     []byte("not-an-image"),
			resolution:    &domain.ScreenResolution{Width: 1920, Height: 1080},
			expectedError: "failed to decode image",
		},
		{
			name:          "Error - Empty Data",
			imageData:     []byte{},
			resolution:    &domain.ScreenResolution{Width: 1920, Height: 1080},
			expectedError: "failed to decode image",
		},
		{
			name:          "Error - Corrupted JPEG",
			imageData:     []byte{0xFF, 0xD8, 0xFF, 0x00, 0x00}, // Partial JPEG header
			resolution:    &domain.ScreenResolution{Width: 1920, Height: 1080},
			expectedError: "failed to decode image",
		},
		{
			name:       "Edge Case - Very Small Image",
			imageData:  createTestJPEG(1, 1, color.RGBA{R: 128, G: 128, B: 128, A: 255}),
			resolution: &domain.ScreenResolution{Width: 1920, Height: 1080},
			validateFunc: func(t *testing.T, result []byte) {
				img, _, err := image.Decode(bytes.NewReader(result))
				if err != nil {
					t.Errorf("failed to decode result: %v", err)
				}
				bounds := img.Bounds()
				if bounds.Dx() != 1920 || bounds.Dy() != 1080 {
					t.Errorf("expected 1920x1080, got %dx%d", bounds.Dx(), bounds.Dy())
				}
			},
		},
		{
			name:       "Edge Case - 4K Resolution",
			imageData:  createTestJPEG(100, 100, color.RGBA{R: 255, G: 255, B: 0, A: 255}),
			resolution: &domain.ScreenResolution{Width: 3840, Height: 2160},
			validateFunc: func(t *testing.T, result []byte) {
				img, _, err := image.Decode(bytes.NewReader(result))
				if err != nil {
					t.Errorf("failed to decode result: %v", err)
				}
				bounds := img.Bounds()
				if bounds.Dx() != 3840 || bounds.Dy() != 2160 {
					t.Errorf("expected 3840x2160, got %dx%d", bounds.Dx(), bounds.Dy())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock config
			mockCfg := &mockConfig{outputDir: "/tmp/synest-test"}
			processor := NewBlurProcessor(zap.NewNop(), tt.resolution, mockCfg)
			result, err := processor.Process(context.Background(), tt.imageData)

			// Verify error
			if tt.expectedError != "" {
				if err == nil {
					t.Fatalf("expected error containing '%s', got nil", tt.expectedError)
				}
				if !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("expected error '%s' to contain '%s'", err.Error(), tt.expectedError)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Run validation function if provided
			if tt.validateFunc != nil {
				tt.validateFunc(t, result)
			}
		})
	}
}

// TestBlurProcessor_Process_ContextCancellation tests context cancellation handling
func TestBlurProcessor_Process_ContextCancellation(t *testing.T) {
	res := &domain.ScreenResolution{Width: 1920, Height: 1080}
	mockCfg := &mockConfig{outputDir: "/tmp/synest-test"}
	processor := NewBlurProcessor(zap.NewNop(), res, mockCfg)
	imageData := createTestJPEG(100, 100, color.RGBA{R: 255, G: 0, B: 0, A: 255})

	// Note: The current implementation doesn't check context during processing
	// because image operations are CPU-bound and complete quickly.
	// This test verifies that processing completes even with a cancelled context.
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Processing should still succeed (no ctx checks in synchronous operations)
	result, err := processor.Process(ctx, imageData)
	if err != nil {
		t.Errorf("processing failed: %v", err)
	}
	if len(result) == 0 {
		t.Error("expected non-empty result")
	}
}

// createTestJPEG generates a simple JPEG image for testing
func createTestJPEG(width, height int, col color.Color) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, col)
		}
	}

	buf := new(bytes.Buffer)
	err := jpeg.Encode(buf, img, &jpeg.Options{Quality: 80})
	if err != nil {
		panic("failed to create test JPEG: " + err.Error())
	}
	return buf.Bytes()
}

// mockConfig is a simple mock implementation of domain.Config for testing
type mockConfig struct {
	outputDir string
	mode      string
}

func (m *mockConfig) GetOutputDir() string {
	return m.outputDir
}

func (m *mockConfig) GetMode() string {
	if m.mode == "" {
		return "blur"
	}
	return m.mode
}
