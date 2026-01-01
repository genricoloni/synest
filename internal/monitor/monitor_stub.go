//go:build !linux
// +build !linux

package monitor

import (
	"context"
	"fmt"

	"github.com/genricoloni/synest/internal/domain"
	"go.uber.org/zap"
)

// MprisMonitor stub for non-Linux platforms
type MprisMonitor struct {
	logger *zap.Logger
}

// NewMprisMonitor creates a stub monitor that returns an error on non-Linux platforms
func NewMprisMonitor(logger *zap.Logger) *MprisMonitor {
	return &MprisMonitor{logger: logger}
}

// Start returns an error indicating MPRIS monitoring is not supported on this platform
func (m *MprisMonitor) Start(ctx context.Context) error {
	return fmt.Errorf("MPRIS monitoring is only supported on Linux systems")
}

// Events returns a closed channel since monitoring is not available
func (m *MprisMonitor) Events() <-chan domain.MediaMetadata {
	ch := make(chan domain.MediaMetadata)
	close(ch)
	return ch
}

// Stop is a no-op on non-Linux platforms
func (m *MprisMonitor) Stop() error {
	return nil
}
