package monitor

import (
	"github.com/genricoloni/synest/internal/domain"
	"github.com/kbinani/screenshot"
	"go.uber.org/zap"
)

// NewScreenResolution detects the primary screen resolution at startup
func NewScreenResolution(logger *zap.Logger) *domain.ScreenResolution {
	n := screenshot.NumActiveDisplays()
	if n <= 0 {
		logger.Warn("No active displays detected, falling back to 1920x1080")
		return &domain.ScreenResolution{Width: 1920, Height: 1080}
	}

	// Use primary monitor (index 0)
	bounds := screenshot.GetDisplayBounds(0)
	res := &domain.ScreenResolution{
		Width:  bounds.Dx(),
		Height: bounds.Dy(),
	}

	logger.Info("Screen resolution detected",
		zap.Int("width", res.Width),
		zap.Int("height", res.Height))

	return res
}
