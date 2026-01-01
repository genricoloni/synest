package engine

import (
	"context"
	"time"

	"github.com/genricoloni/synest/internal/domain"
	"go.uber.org/zap"
)

// Engine orchestrates the wallpaper generation pipeline.
// It listens to media events, fetches artwork, processes it, and sets the wallpaper.
type Engine struct {
	logger            *zap.Logger
	cfg               domain.Config
	monitor           domain.Monitor
	fetcher           domain.Fetcher
	processor         domain.Processor
	executor          domain.Executor
	originalWallpaper string // Path to wallpaper captured at startup
}

// NewEngine creates a new orchestration engine
func NewEngine(
	logger *zap.Logger,
	cfg domain.Config,
	mon domain.Monitor,
	fetch domain.Fetcher,
	proc domain.Processor,
	exec domain.Executor,
) *Engine {
	return &Engine{
		logger:    logger,
		cfg:       cfg,
		monitor:   mon,
		fetcher:   fetch,
		processor: proc,
		executor:  exec,
	}
}

// Start launches the engine's event processing loop in a goroutine.
// It returns immediately (non-blocking).
func (e *Engine) Start(ctx context.Context) error {
	e.logger.Info("Engine starting...")

	// Try to capture current wallpaper before we start changing it
	if wallpaper, err := e.executor.GetCurrentWallpaper(ctx); err == nil {
		e.originalWallpaper = wallpaper
		e.logger.Info("Captured original wallpaper for restoration",
			zap.String("path", wallpaper))
	} else {
		e.logger.Warn("Could not capture current wallpaper, restore on exit will be disabled",
			zap.Error(err))
	}

	go e.runLoop(ctx)
	return nil
}

// runLoop is the main event processing loop with debouncing.
// Debouncing prevents excessive wallpaper updates when users skip through tracks quickly.
func (e *Engine) runLoop(ctx context.Context) {
	events := e.monitor.Events()

	// Debouncing: wait for 500ms of silence before processing
	// This prevents generating wallpapers for every track during rapid skipping
	debounceDuration := 500 * time.Millisecond
	timer := time.NewTimer(debounceDuration)
	timer.Stop() // Start with stopped timer

	var pendingMeta *domain.MediaMetadata

	for {
		select {
		case <-ctx.Done():
			e.logger.Info("Engine loop stopped")
			return

		case meta, ok := <-events:
			if !ok {
				e.logger.Info("Monitor events channel closed")
				return
			}
			e.logger.Debug("Event received, debouncing...",
				zap.String("title", meta.Title),
				zap.String("artist", meta.Artist))

			// Save the latest event and reset the debounce timer
			pendingMeta = &meta
			timer.Reset(debounceDuration)

		case <-timer.C:
			// Timer expired: user stopped skipping, process the last event
			if pendingMeta != nil {
				e.processMetadata(ctx, *pendingMeta)
				pendingMeta = nil
			}
		}
	}
}

// processMetadata handles the complete wallpaper generation pipeline for a single track
func (e *Engine) processMetadata(ctx context.Context, meta domain.MediaMetadata) {
	// Skip if music is paused or stopped
	if meta.Status != domain.StatusPlaying {
		e.logger.Info("Music paused or stopped, skipping wallpaper update",
			zap.String("status", string(meta.Status)))
		return
	}

	// Skip if no artwork URL is available
	if meta.ArtUrl == "" {
		e.logger.Warn("No artwork URL found",
			zap.String("track", meta.Title),
			zap.String("artist", meta.Artist))
		return
	}

	e.logger.Info("Processing wallpaper",
		zap.String("track", meta.Title),
		zap.String("artist", meta.Artist),
		zap.String("album", meta.Album))

	// 1. Fetch artwork
	imgData, err := e.fetcher.Fetch(ctx, meta.ArtUrl)
	if err != nil {
		e.logger.Error("Failed to fetch artwork", zap.Error(err))
		return
	}

	// 2. Process image and save to disk
	mode := e.cfg.GetMode()
	wallpaperPath, err := e.processor.Generate(imgData, mode)
	if err != nil {
		e.logger.Error("Failed to generate wallpaper", zap.Error(err))
		return
	}

	// 3. Set wallpaper
	if err := e.executor.SetWallpaper(ctx, wallpaperPath); err != nil {
		e.logger.Error("Failed to set wallpaper", zap.Error(err))
		return
	}

	e.logger.Info("Wallpaper updated successfully",
		zap.String("path", wallpaperPath),
		zap.String("mode", mode))
}

// Stop gracefully stops the engine and restores the original wallpaper
func (e *Engine) Stop(ctx context.Context) error {
	e.logger.Info("Engine stopping...")

	// Restore original wallpaper if we captured one
	if e.originalWallpaper != "" {
		e.logger.Info("Restoring original wallpaper",
			zap.String("path", e.originalWallpaper))

		if err := e.executor.SetWallpaper(ctx, e.originalWallpaper); err != nil {
			e.logger.Error("Failed to restore original wallpaper", zap.Error(err))
			return err
		}

		e.logger.Info("Original wallpaper restored successfully")
	} else {
		e.logger.Info("No original wallpaper to restore")
	}

	return nil
}
