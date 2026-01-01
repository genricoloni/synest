package monitor

import (
	"fmt"
	"testing"
	"time"

	"github.com/genricoloni/synest/internal/domain"
	"github.com/godbus/dbus/v5"
	"go.uber.org/zap"
)

// TestHandleSignal_HappyPath verifies the standard scenario: a valid signal produces a valid event.
func TestHandleSignal_HappyPath(t *testing.T) {
	logger := zap.NewNop()
	mon := NewMprisMonitor(logger)
	mon.conn = &noopDBusClient{} // Prevent panic if code tries to call DBus
	mon.running = true
	mon.playerNames = map[string]string{":1.100": "org.mpris.MediaPlayer2.spotify"}

	expectedTitle := "Bohemian Rhapsody"
	expectedArtist := "Queen"
	expectedArtUrl := "https://example.com/cover.jpg"

	// Simulate complete D-Bus signal
	signal := &dbus.Signal{
		Name:   "org.freedesktop.DBus.Properties.PropertiesChanged",
		Sender: ":1.100",
		Body: []interface{}{
			"org.mpris.MediaPlayer2.Player",
			map[string]dbus.Variant{
				"Metadata": dbus.MakeVariant(map[string]dbus.Variant{
					"xesam:title":  dbus.MakeVariant(expectedTitle),
					"xesam:artist": dbus.MakeVariant([]string{expectedArtist}),
					"mpris:artUrl": dbus.MakeVariant(expectedArtUrl),
				}),
				"PlaybackStatus": dbus.MakeVariant("Playing"),
			},
			[]string{},
		},
	}

	go mon.handleSignal(signal)

	select {
	case event := <-mon.Events():
		if event.Title != expectedTitle {
			t.Errorf("Title: expected '%s', got '%s'", expectedTitle, event.Title)
		}
		if event.Artist != expectedArtist {
			t.Errorf("Artist: expected '%s', got '%s'", expectedArtist, event.Artist)
		}
		if event.Status != domain.StatusPlaying {
			t.Errorf("Status: expected Playing, got %v", event.Status)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout: Event was not emitted")
	}
}

// TestHandleSignal_EdgeCases consolidates all invalid/ignored scenarios into a table test.
func TestHandleSignal_EdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		signal *dbus.Signal
	}{
		{
			name: "Wrong Signal Name",
			signal: &dbus.Signal{
				Name: "org.freedesktop.DBus.SomeOtherSignal",
				Body: []interface{}{},
			},
		},
		{
			name: "Wrong Interface",
			signal: &dbus.Signal{
				Name: "org.freedesktop.DBus.Properties.PropertiesChanged",
				Body: []interface{}{"org.mpris.MediaPlayer2", map[string]dbus.Variant{}, []string{}},
			},
		},
		{
			name: "Short Body",
			signal: &dbus.Signal{
				Name: "org.freedesktop.DBus.Properties.PropertiesChanged",
				Body: []interface{}{"org.mpris.MediaPlayer2.Player"}, // Missing props
			},
		},
		{
			name: "Invalid Metadata Type (Int instead of Map)",
			signal: &dbus.Signal{
				Name: "org.freedesktop.DBus.Properties.PropertiesChanged",
				Body: []interface{}{
					"org.mpris.MediaPlayer2.Player",
					map[string]dbus.Variant{"Metadata": dbus.MakeVariant(12345)},
					[]string{},
				},
			},
		},
		{
			name: "Invalid PlaybackStatus Type (Array instead of String)",
			signal: &dbus.Signal{
				Name: "org.freedesktop.DBus.Properties.PropertiesChanged",
				Body: []interface{}{
					"org.mpris.MediaPlayer2.Player",
					map[string]dbus.Variant{"PlaybackStatus": dbus.MakeVariant([]string{"Playing"})},
					[]string{},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mon := NewMprisMonitor(zap.NewNop())
			mon.conn = &noopDBusClient{}
			mon.running = true

			// Non-blocking call or goroutine
			mon.handleSignal(tt.signal)

			select {
			case <-mon.Events():
				t.Error("Should NOT emit event for invalid input")
			case <-time.After(50 * time.Millisecond):
				// Pass
			}
		})
	}
}

// TestHandleSignal_DataVariations tests valid parsing variations (Artist types, Status strings, etc.)
func TestHandleSignal_DataVariations(t *testing.T) {
	tests := []struct {
		name  string
		props map[string]dbus.Variant
		check func(*testing.T, domain.MediaMetadata)
	}{
		{
			name: "Artist as String (Non-compliant)",
			props: map[string]dbus.Variant{
				"Metadata": dbus.MakeVariant(map[string]dbus.Variant{
					"xesam:artist": dbus.MakeVariant("Single Artist"),
				}),
				"PlaybackStatus": dbus.MakeVariant("Playing"),
			},
			check: func(t *testing.T, e domain.MediaMetadata) {
				if e.Artist != "Single Artist" {
					t.Errorf("Expected 'Single Artist', got '%s'", e.Artist)
				}
			},
		},
		{
			name: "Empty Art URL",
			props: map[string]dbus.Variant{
				"Metadata": dbus.MakeVariant(map[string]dbus.Variant{
					"mpris:artUrl": dbus.MakeVariant(""),
					"xesam:title":  dbus.MakeVariant("Song"),
				}),
				"PlaybackStatus": dbus.MakeVariant("Playing"),
			},
			check: func(t *testing.T, e domain.MediaMetadata) {
				if e.ArtUrl != "" {
					t.Errorf("Expected empty ArtUrl, got '%s'", e.ArtUrl)
				}
			},
		},
		{
			name: "Status Paused",
			props: map[string]dbus.Variant{
				"PlaybackStatus": dbus.MakeVariant("Paused"),
			},
			check: func(t *testing.T, e domain.MediaMetadata) {
				if e.Status != domain.StatusPaused {
					t.Errorf("Expected Paused, got %v", e.Status)
				}
			},
		},
		{
			name: "Status Stopped",
			props: map[string]dbus.Variant{
				"PlaybackStatus": dbus.MakeVariant("Stopped"),
			},
			check: func(t *testing.T, e domain.MediaMetadata) {
				if e.Status != domain.StatusStopped {
					t.Errorf("Expected Stopped, got %v", e.Status)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mon := NewMprisMonitor(zap.NewNop())
			mon.conn = &noopDBusClient{}
			mon.running = true

			signal := &dbus.Signal{
				Name:   "org.freedesktop.DBus.Properties.PropertiesChanged",
				Sender: ":1.99",
				Body:   []interface{}{"org.mpris.MediaPlayer2.Player", tt.props, []string{}},
			}

			go mon.handleSignal(signal)

			select {
			case event := <-mon.Events():
				tt.check(t, event)
			case <-time.After(1 * time.Second):
				t.Fatal("Timeout waiting for event")
			}
		})
	}
}

// TestHandleNameOwnerChanged verifies player lifecycle tracking
func TestHandleNameOwnerChanged(t *testing.T) {
	tests := []struct {
		name         string
		signalBody   []interface{}
		expectMapped bool
		expectedName string
		targetUnique string
	}{
		{
			name: "New Player Appears",
			signalBody: []interface{}{
				"org.mpris.MediaPlayer2.spotify", // Name
				"",                               // Old Owner (Empty = New)
				":1.50",                          // New Owner
			},
			expectMapped: true,
			expectedName: "org.mpris.MediaPlayer2.spotify",
			targetUnique: ":1.50",
		},
		{
			name: "Player Disappears",
			signalBody: []interface{}{
				"org.mpris.MediaPlayer2.spotify",
				":1.50", // Old Owner
				"",      // New Owner (Empty = Deleted)
			},
			expectMapped: false,
			targetUnique: ":1.50",
		},
		{
			name: "Non-MPRIS Service Ignored",
			signalBody: []interface{}{
				"com.example.service",
				"",
				":1.99",
			},
			expectMapped: false,
			targetUnique: ":1.99",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mon := NewMprisMonitor(zap.NewNop())
			mon.conn = &noopDBusClient{} // Stub to avoid fetch panic

			// Pre-populate if testing disappearance
			if !tt.expectMapped && tt.targetUnique != "" {
				mon.playerNames[tt.targetUnique] = "org.mpris.MediaPlayer2.spotify"
			}

			signal := &dbus.Signal{
				Name: "org.freedesktop.DBus.NameOwnerChanged",
				Body: tt.signalBody,
			}

			mon.handleNameOwnerChanged(signal)

			mon.mu.RLock()
			val, exists := mon.playerNames[tt.targetUnique]
			mon.mu.RUnlock()

			if tt.expectMapped {
				if !exists {
					t.Error("Expected player to be mapped, but it wasn't")
				}
				if val != tt.expectedName {
					t.Errorf("Expected name %s, got %s", tt.expectedName, val)
				}
			} else {
				if exists && tt.name != "Non-MPRIS Service Ignored" {
					// For "Player Disappears" case, it should be gone.
					// For "Non-MPRIS", it simply shouldn't be added.
					if val == "org.mpris.MediaPlayer2.spotify" && tt.signalBody[2] == "" {
						t.Error("Expected player to be removed, but it still exists")
					}
				}
			}
		})
	}
}

func TestGetPlayerName(t *testing.T) {
	mon := NewMprisMonitor(zap.NewNop())
	mon.playerNames = map[string]string{
		":1.100": "org.mpris.MediaPlayer2.spotify",
	}

	tests := []struct {
		input    string
		expected string
	}{
		{":1.100", "org.mpris.MediaPlayer2.spotify"},
		{":1.999", ":1.999"}, // Fallback
	}

	for _, tt := range tests {
		if got := mon.getPlayerName(tt.input); got != tt.expected {
			t.Errorf("getPlayerName(%s): expected %s, got %s", tt.input, tt.expected, got)
		}
	}
}



// noopDBusClient is a stub to prevent panics during unit tests where
// we don't want to use full mocks but code calls GetProperty/ListNames.
type noopDBusClient struct{}

func (n *noopDBusClient) Close() error                             { return nil }
func (n *noopDBusClient) AddMatchSignal(...dbus.MatchOption) error { return nil }
func (n *noopDBusClient) Signal(chan<- *dbus.Signal)               {}
func (n *noopDBusClient) ListNames() ([]string, error)             { return []string{}, nil }
func (n *noopDBusClient) GetNameOwner(string) (string, error)      { return "", fmt.Errorf("noop") }
func (n *noopDBusClient) GetProperty(string, string, string) (dbus.Variant, error) {
	return dbus.MakeVariant(""), fmt.Errorf("noop")
}
