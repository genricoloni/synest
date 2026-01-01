package monitor

import (
	"fmt"
	"testing"

	"github.com/genricoloni/synest/internal/domain"
	"github.com/genricoloni/synest/internal/monitor/mocks"
	"github.com/godbus/dbus/v5"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

// TestFetchPlayerMetadata unifies all scenarios regarding metadata fetching:
// 1. Success (Happy Path)
// 2. DBus Errors (Connection fail)
// 3. Invalid Data types (Robustness)
func TestFetchPlayerMetadata(t *testing.T) {
	playerName := "org.mpris.MediaPlayer2.spotify"
	metaPath := "org.mpris.MediaPlayer2.Player.Metadata"
	statusPath := "org.mpris.MediaPlayer2.Player.PlaybackStatus"
	objPath := "/org/mpris/MediaPlayer2"

	tests := []struct {
		name          string
		setupMock     func(*mocks.MockDBusClient)
		expectError   bool
		expectedEvent *domain.MediaMetadata
	}{
		{
			name: "Success - Valid Metadata",
			setupMock: func(m *mocks.MockDBusClient) {
				// Metadata
				m.EXPECT().GetProperty(playerName, objPath, metaPath).
					Return(dbus.MakeVariant(map[string]dbus.Variant{
						"xesam:title":  dbus.MakeVariant("Stairway to Heaven"),
						"xesam:artist": dbus.MakeVariant([]string{"Led Zeppelin"}),
					}), nil)
				// Status
				m.EXPECT().GetProperty(playerName, objPath, statusPath).
					Return(dbus.MakeVariant("Playing"), nil)
			},
			expectError: false,
			expectedEvent: &domain.MediaMetadata{
				Title:  "Stairway to Heaven",
				Artist: "Led Zeppelin",
				Status: domain.StatusPlaying,
			},
		},
		{
			name: "DBus Error - Connection Fail",
			setupMock: func(m *mocks.MockDBusClient) {
				m.EXPECT().GetProperty(playerName, objPath, metaPath).
					Return(dbus.MakeVariant(""), fmt.Errorf("connection timeout"))
			},
			expectError:   true,
			expectedEvent: nil,
		},
		{
			name: "Invalid Data - Metadata is Int not Map",
			setupMock: func(m *mocks.MockDBusClient) {
				m.EXPECT().GetProperty(playerName, objPath, metaPath).
					Return(dbus.MakeVariant(12345), nil) // Wrong type
			},
			expectError:   false, // Should handle gracefully, no error returned
			expectedEvent: nil,   // But no event emitted
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mocks.NewMockDBusClient(ctrl)
			tt.setupMock(mockClient)

			mon := NewMprisMonitor(zap.NewNop())
			mon.conn = mockClient
			mon.running = true

			err := mon.fetchPlayerMetadata(playerName)

			// Verify Error Return
			if tt.expectError && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Verify Event Emission
			select {
			case event := <-mon.Events():
				if tt.expectedEvent == nil {
					t.Errorf("Unexpected event emitted: %+v", event)
				} else {
					if event.Title != tt.expectedEvent.Title {
						t.Errorf("Title mismatch: want %s, got %s", tt.expectedEvent.Title, event.Title)
					}
					if event.Status != tt.expectedEvent.Status {
						t.Errorf("Status mismatch: want %v, got %v", tt.expectedEvent.Status, event.Status)
					}
				}
			default:
				if tt.expectedEvent != nil {
					t.Error("Expected event was not emitted")
				}
			}
		})
	}
}

// TestDetectExistingPlayers verifies the initial scan of DBus names.
func TestDetectExistingPlayers(t *testing.T) {
	tests := []struct {
		name             string
		setupMock        func(*mocks.MockDBusClient)
		expectError      bool
		expectedPlayers  int
		expectedMappings map[string]string
	}{
		{
			name: "Success - Detects Spotify and VLC",
			setupMock: func(m *mocks.MockDBusClient) {
				// 1. ListNames
				m.EXPECT().ListNames().Return([]string{
					"org.freedesktop.DBus",
					"org.mpris.MediaPlayer2.spotify",
					"org.mpris.MediaPlayer2.vlc",
					"com.example.OtherApp",
				}, nil)

				// 2. GetNameOwner (Mapping)
				m.EXPECT().GetNameOwner("org.mpris.MediaPlayer2.spotify").Return(":1.100", nil)
				m.EXPECT().GetNameOwner("org.mpris.MediaPlayer2.vlc").Return(":1.200", nil)

				// 3. Fetch Metadata for Spotify
				m.EXPECT().GetProperty("org.mpris.MediaPlayer2.spotify", gomock.Any(), gomock.Any()).
					Return(dbus.MakeVariant(map[string]dbus.Variant{"xesam:title": dbus.MakeVariant("Song A")}), nil)
				m.EXPECT().GetProperty("org.mpris.MediaPlayer2.spotify", gomock.Any(), gomock.Any()).
					Return(dbus.MakeVariant("Playing"), nil)

				// 4. Fetch Metadata for VLC
				m.EXPECT().GetProperty("org.mpris.MediaPlayer2.vlc", gomock.Any(), gomock.Any()).
					Return(dbus.MakeVariant(map[string]dbus.Variant{"xesam:title": dbus.MakeVariant("Video B")}), nil)
				m.EXPECT().GetProperty("org.mpris.MediaPlayer2.vlc", gomock.Any(), gomock.Any()).
					Return(dbus.MakeVariant("Paused"), nil)
			},
			expectError:     false,
			expectedPlayers: 2,
			expectedMappings: map[string]string{
				":1.100": "org.mpris.MediaPlayer2.spotify",
				":1.200": "org.mpris.MediaPlayer2.vlc",
			},
		},
		{
			name: "Failure - ListNames fails",
			setupMock: func(m *mocks.MockDBusClient) {
				m.EXPECT().ListNames().Return(nil, fmt.Errorf("bus error"))
			},
			expectError:     true,
			expectedPlayers: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mocks.NewMockDBusClient(ctrl)
			tt.setupMock(mockClient)

			mon := NewMprisMonitor(zap.NewNop())
			mon.conn = mockClient
			mon.running = true

			err := mon.detectExistingPlayers()

			// Check Error
			if tt.expectError && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Check Mappings
			if len(mon.playerNames) != len(tt.expectedMappings) {
				t.Errorf("Mapping count mismatch: want %d, got %d", len(tt.expectedMappings), len(mon.playerNames))
			}
			for k, v := range tt.expectedMappings {
				if mon.playerNames[k] != v {
					t.Errorf("Mapping mismatch for %s: want %s, got %s", k, v, mon.playerNames[k])
				}
			}

			// Check Events Emitted (only relevant if success)
			if !tt.expectError {
				eventsFound := 0
				// Drain channel
				for len(mon.Events()) > 0 {
					<-mon.Events()
					eventsFound++
				}
				if eventsFound != tt.expectedPlayers {
					t.Errorf("Expected %d events, got %d", tt.expectedPlayers, eventsFound)
				}
			}
		})
	}
}