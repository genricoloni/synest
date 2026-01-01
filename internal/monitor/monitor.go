package monitor

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/genricoloni/synest/internal/domain"
	"github.com/godbus/dbus/v5"
	"go.uber.org/zap"
)

// MprisMonitor monitors media playback via D-Bus MPRIS interface
type MprisMonitor struct {
	logger          *zap.Logger
	events          chan domain.MediaMetadata
	mu              sync.RWMutex
	running         bool
	cancel          context.CancelFunc
	conn            DBusClient        // Interface for testability
	lastDropWarning time.Time         // Rate limiting for "channel full" warnings
	wg              sync.WaitGroup    // Tracks active producer goroutines
	playerNames     map[string]string // Maps unique bus names (:1.45) to well-known names (org.mpris.MediaPlayer2.spotify)
}

// NewMprisMonitor creates a new MPRIS monitor instance
func NewMprisMonitor(logger *zap.Logger) *MprisMonitor {
	return &MprisMonitor{
		logger:      logger,
		events:      make(chan domain.MediaMetadata, 10),
		playerNames: make(map[string]string),
	}
}

// Start begins monitoring for media events
func (m *MprisMonitor) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return nil
	}
	m.running = true

	monitorCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.mu.Unlock()

	m.logger.Info("MPRIS monitor started")

	// Connect to Session Bus (this may block)
	conn, err := NewStdDBusClient()
	if err != nil {
		m.logger.Error("Failed to connect to session bus", zap.Error(err))
		// Reset running state on failure
		m.mu.Lock()
		defer m.mu.Unlock()
		m.running = false
		m.cancel = nil
		return fmt.Errorf("session bus connection failed: %w", err)
	}

	// Check if we were stopped while connecting to D-Bus
	select {
	case <-monitorCtx.Done():
		m.logger.Info("Monitor stopped during D-Bus connection")
		if err := conn.Close(); err != nil {
			m.logger.Warn("Failed to close D-Bus connection", zap.Error(err))
		}
		return monitorCtx.Err()
	default:
	}

	// Protect connection assignment with mutex to avoid race with Stop()
	m.mu.Lock()
	m.conn = conn
	m.mu.Unlock()

	// Protect initial player detection with WaitGroup
	// This prevents race condition if Stop() is called during detection
	m.wg.Add(1)
	func() {
		defer m.wg.Done()
		if err := m.detectExistingPlayers(); err != nil {
			m.logger.Warn("Failed to detect existing players", zap.Error(err))
		}
	}()

	// Add match rule for PropertiesChanged signals on MPRIS interface
	matchRule := "type='signal',interface='org.freedesktop.DBus.Properties',member='PropertiesChanged',path='/org/mpris/MediaPlayer2'"
	if err := conn.AddMatchSignal(
		dbus.WithMatchObjectPath("/org/mpris/MediaPlayer2"),
		dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
		dbus.WithMatchMember("PropertiesChanged"),
	); err != nil {
		m.logger.Error("Failed to add match signal", zap.Error(err))
		return fmt.Errorf("failed to add match signal: %w", err)
	}

	m.logger.Info("D-Bus match rule added", zap.String("rule", matchRule))

	// Add match rule for NameOwnerChanged to track new/removed players dynamically
	if err := conn.AddMatchSignal(
		dbus.WithMatchInterface("org.freedesktop.DBus"),
		dbus.WithMatchMember("NameOwnerChanged"),
	); err != nil {
		m.logger.Warn("Failed to add NameOwnerChanged match signal", zap.Error(err))
		// Non-fatal, continue without dynamic tracking
	} else {
		m.logger.Info("Dynamic player tracking enabled via NameOwnerChanged")
	}

	// Start signal monitoring goroutine
	m.wg.Add(1)
	go m.monitorSignals(monitorCtx)

	// Block until context is cancelled
	<-monitorCtx.Done()

	m.logger.Info("MPRIS monitor stopped")
	return monitorCtx.Err()
}

// Stop gracefully stops the monitor
func (m *MprisMonitor) Stop(ctx context.Context) error {
	m.mu.Lock()

	if !m.running {
		m.mu.Unlock()
		return nil
	}

	if m.cancel != nil {
		m.cancel()
	}

	m.running = false
	m.mu.Unlock()

	// Wait for all producer goroutines to terminate before closing channel
	// This prevents "send on closed channel" panic
	m.logger.Debug("Waiting for monitoring goroutines to finish")
	m.wg.Wait()

	// Now safe to close the channel
	close(m.events)

	// Close D-Bus connection
	m.mu.Lock()
	if m.conn != nil {
		if err := m.conn.Close(); err != nil {
			m.logger.Warn("Failed to close D-Bus connection", zap.Error(err))
		}
	}
	m.mu.Unlock()

	m.logger.Info("MPRIS monitor shutdown complete")
	return nil
}

// Events returns a read-only channel that emits MediaMetadata
func (m *MprisMonitor) Events() <-chan domain.MediaMetadata {
	return m.events
}

// detectExistingPlayers queries D-Bus for currently running MPRIS players
func (m *MprisMonitor) detectExistingPlayers() error {
	names, err := m.conn.ListNames()
	if err != nil {
		return fmt.Errorf("failed to list bus names: %w", err)
	}

	// Filter for MPRIS player names (org.mpris.MediaPlayer2.*)
	playerCount := 0
	for _, name := range names {
		if strings.HasPrefix(name, "org.mpris.MediaPlayer2.") {
			playerCount++
			m.logger.Info("Detected MPRIS player", zap.String("name", name))

			// Get the unique bus name for this well-known name
			uniqueName, err := m.conn.GetNameOwner(name)
			if err == nil {
				m.mu.Lock()
				m.playerNames[uniqueName] = name
				m.mu.Unlock()
				m.logger.Debug("Mapped player name",
					zap.String("unique", uniqueName),
					zap.String("wellKnown", name))
			}

			// Fetch initial metadata for this player
			if err := m.fetchPlayerMetadata(name); err != nil {
				m.logger.Warn("Failed to fetch initial metadata",
					zap.String("player", name),
					zap.Error(err))
			}
		}
	}

	m.logger.Info("Player detection complete", zap.Int("count", playerCount))
	return nil
}

// fetchPlayerMetadata retrieves and emits metadata from a specific player
func (m *MprisMonitor) fetchPlayerMetadata(playerName string) error {
	// Get Metadata property
	variant, err := m.conn.GetProperty(playerName, "/org/mpris/MediaPlayer2", "org.mpris.MediaPlayer2.Player.Metadata")
	if err != nil {
		return fmt.Errorf("failed to get metadata: %w", err)
	}

	// SAFE CAST: Some players may return nil or unexpected types if not playing anything
	metadata, ok := variant.Value().(map[string]dbus.Variant)
	if !ok {
		m.logger.Debug("Metadata variant is not a map, skipping", zap.String("player", playerName))
		return nil // Skip gracefully instead of failing
	}

	// Get PlaybackStatus
	statusVariant, err := m.conn.GetProperty(playerName, "/org/mpris/MediaPlayer2", "org.mpris.MediaPlayer2.Player.PlaybackStatus")
	if err != nil {
		return fmt.Errorf("failed to get playback status: %w", err)
	}

	status, ok := statusVariant.Value().(string)
	if !ok {
		return fmt.Errorf("invalid playback status format")
	}

	// Parse metadata into domain model
	mediaMeta := m.parseMetadata(metadata, status)

	// Emit event (non-blocking)
	// NOTE: For wallpaper generation, dropping intermediate events during rapid
	// track changes is acceptable and acts as implicit debouncing. The consumer
	// should implement proper debouncing to avoid unnecessary wallpaper regeneration.
	select {
	case m.events <- mediaMeta:
		m.logger.Debug("Emitted initial metadata", zap.String("title", mediaMeta.Title))
	default:
		m.logChannelFullWarning()
	}

	return nil
}

// monitorSignals listens for D-Bus signals and processes them
func (m *MprisMonitor) monitorSignals(ctx context.Context) {
	defer m.wg.Done() // Signal completion when goroutine exits

	signals := make(chan *dbus.Signal, 10)
	m.conn.Signal(signals)

	m.logger.Info("Signal monitoring goroutine started")

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("Signal monitoring goroutine stopped")
			return
		case sig := <-signals:
			if sig == nil {
				continue
			}
			// Handle different signal types
			if sig.Name == "org.freedesktop.DBus.NameOwnerChanged" {
				m.handleNameOwnerChanged(sig)
			} else {
				m.handleSignal(sig)
			}
		}
	}
}

// handleNameOwnerChanged processes NameOwnerChanged signals to track player lifecycle
func (m *MprisMonitor) handleNameOwnerChanged(sig *dbus.Signal) {
	if len(sig.Body) < 3 {
		return
	}

	name, ok := sig.Body[0].(string)
	if !ok || !strings.HasPrefix(name, "org.mpris.MediaPlayer2.") {
		return // Not an MPRIS player
	}

	oldOwner, _ := sig.Body[1].(string)
	newOwner, _ := sig.Body[2].(string)

	if newOwner != "" && oldOwner == "" {
		// New player appeared
		m.mu.Lock()
		m.playerNames[newOwner] = name
		m.mu.Unlock()

		m.logger.Info("New MPRIS player detected",
			zap.String("player", name),
			zap.String("unique", newOwner))

		// Fetch initial metadata for the new player
		if err := m.fetchPlayerMetadata(name); err != nil {
			m.logger.Warn("Failed to fetch metadata from new player",
				zap.String("player", name),
				zap.Error(err))
		}
	} else if newOwner == "" && oldOwner != "" {
		// Player disappeared
		m.mu.Lock()
		delete(m.playerNames, oldOwner)
		m.mu.Unlock()

		m.logger.Info("MPRIS player removed",
			zap.String("player", name),
			zap.String("unique", oldOwner))
	}
	// If both oldOwner and newOwner are set, it's a transfer (rare), we update the mapping
	if newOwner != "" && oldOwner != "" {
		m.mu.Lock()
		delete(m.playerNames, oldOwner)
		m.playerNames[newOwner] = name
		m.mu.Unlock()

		m.logger.Debug("MPRIS player ownership changed",
			zap.String("player", name),
			zap.String("oldUnique", oldOwner),
			zap.String("newUnique", newOwner))
	}
}

// handleSignal processes a D-Bus signal
func (m *MprisMonitor) handleSignal(sig *dbus.Signal) {
	// PropertiesChanged signal has 3 arguments:
	// 1. Interface name (string)
	// 2. Changed properties (map[string]Variant)
	// 3. Invalidated properties ([]string)

	if sig.Name != "org.freedesktop.DBus.Properties.PropertiesChanged" {
		return
	}

	if len(sig.Body) < 2 {
		return
	}

	interfaceName, ok := sig.Body[0].(string)
	if !ok || interfaceName != "org.mpris.MediaPlayer2.Player" {
		return
	}

	changedProps, ok := sig.Body[1].(map[string]dbus.Variant)
	if !ok {
		return
	}

	// Resolve player name from unique bus name for better logging and future
	// player-specific logic (e.g., priority-based selection)
	playerName := m.getPlayerName(sig.Sender)

	m.logger.Debug("Received PropertiesChanged signal",
		zap.String("sender", sig.Sender),
		zap.String("player", playerName),
		zap.Int("properties", len(changedProps)))

	// Check if Metadata or PlaybackStatus changed
	metadataVariant, hasMetadata := changedProps["Metadata"]
	statusVariant, hasStatus := changedProps["PlaybackStatus"]

	if !hasMetadata && !hasStatus {
		return
	}

	// Get current values
	var metadata map[string]dbus.Variant
	var status string

	if hasMetadata {
		var ok bool
		metadata, ok = metadataVariant.Value().(map[string]dbus.Variant)
		if !ok {
			m.logger.Warn("Invalid metadata format in signal, ignoring")
			return
		}
	}

	if hasStatus {
		var ok bool
		status, ok = statusVariant.Value().(string)
		if !ok {
			m.logger.Warn("Invalid playback status format in signal, ignoring")
			return
		}
	} else {
		// Fetch current status from player
		variant, err := m.conn.GetProperty(sig.Sender, "/org/mpris/MediaPlayer2", "org.mpris.MediaPlayer2.Player.PlaybackStatus")
		if err == nil {
			if s, ok := variant.Value().(string); ok {
				status = s
			}
		}
	}

	// If we only got status change, fetch metadata
	if !hasMetadata && hasStatus {
		variant, err := m.conn.GetProperty(sig.Sender, "/org/mpris/MediaPlayer2", "org.mpris.MediaPlayer2.Player.Metadata")
		if err == nil {
			if m, ok := variant.Value().(map[string]dbus.Variant); ok {
				metadata = m
			}
		}
	}

	// Parse and emit
	mediaMeta := m.parseMetadata(metadata, status)

	// Non-blocking send: Prevents monitor from blocking on slow consumers.
	// The consumer (engine/processor) should implement debouncing to handle
	// rapid track changes gracefully (e.g., only process the last event within
	// a time window). Dropping intermediate events here is intentional.
	select {
	case m.events <- mediaMeta:
		m.logger.Info("Media change detected",
			zap.String("player", playerName),
			zap.String("title", mediaMeta.Title),
			zap.String("artist", mediaMeta.Artist),
			zap.String("status", string(mediaMeta.Status)))
	default:
		m.logChannelFullWarning()
	}
}

// parseMetadata converts MPRIS metadata to domain model
func (m *MprisMonitor) parseMetadata(metadata map[string]dbus.Variant, status string) domain.MediaMetadata {
	var meta domain.MediaMetadata

	// Parse status
	switch status {
	case "Playing":
		meta.Status = domain.StatusPlaying
	case "Paused":
		meta.Status = domain.StatusPaused
	case "Stopped":
		meta.Status = domain.StatusStopped
	default:
		meta.Status = domain.StatusStopped
	}

	if metadata == nil {
		return meta
	}

	// Extract title
	if titleVar, ok := metadata["xesam:title"]; ok {
		if title, ok := titleVar.Value().(string); ok {
			meta.Title = title
		}
	}

	// Extract artist (can be an array)
	if artistVar, ok := metadata["xesam:artist"]; ok {
		switch artists := artistVar.Value().(type) {
		case []string:
			if len(artists) > 0 {
				meta.Artist = artists[0]
			}
		case string:
			meta.Artist = artists
		default:
			// Some non-compliant players may use unexpected types
			m.logger.Debug("Unexpected artist type in metadata",
				zap.String("type", fmt.Sprintf("%T", artistVar.Value())))
		}
	}

	// Extract album
	if albumVar, ok := metadata["xesam:album"]; ok {
		if album, ok := albumVar.Value().(string); ok {
			meta.Album = album
		}
	}

	// Extract art URL
	if artVar, ok := metadata["mpris:artUrl"]; ok {
		if artUrl, ok := artVar.Value().(string); ok {
			if artUrl == "" {
				// Some players (browsers, local files) may send empty artUrl
				m.logger.Debug("Empty artUrl received",
					zap.String("title", meta.Title),
					zap.String("artist", meta.Artist))
			} else {
				meta.ArtUrl = artUrl
			}
		}
	}

	return meta
}

// getPlayerName returns the well-known player name for a unique bus name
// Falls back to the unique name if no mapping exists
func (m *MprisMonitor) getPlayerName(uniqueName string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if wellKnown, ok := m.playerNames[uniqueName]; ok {
		return wellKnown
	}
	return uniqueName
}

// logChannelFullWarning logs a warning about channel being full, but rate-limited
// to avoid log spam during rapid track changes (e.g., fast skipping)
func (m *MprisMonitor) logChannelFullWarning() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Rate limit to max one warning per 5 seconds
	const warningInterval = 5 * time.Second
	now := time.Now()

	if now.Sub(m.lastDropWarning) >= warningInterval {
		m.logger.Warn("Events channel full, dropping metadata (consumer may be slow or fast track changes occurring)",
			zap.String("note", "This is expected during rapid track skipping. Consumer should implement debouncing."))
		m.lastDropWarning = now
	}
}
