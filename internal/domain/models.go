package domain

// PlayerStatus represents the current state of the media player
type PlayerStatus string

const (
	// StatusPlaying indicates the media is currently playing
	StatusPlaying PlayerStatus = "Playing"
	// StatusPaused indicates the media is paused
	StatusPaused PlayerStatus = "Paused"
	// StatusStopped indicates the media is stopped
	StatusStopped PlayerStatus = "Stopped"
)

// MediaMetadata contains information about the currently playing media
type MediaMetadata struct {
	// Title of the currently playing track
	Title string
	// Artist name
	Artist string
	// Album name
	Album string
	// ArtUrl is the URL or local path to the album artwork
	ArtUrl string
	// Status is the current playback status
	Status PlayerStatus
}

// ScreenResolution holds the display dimensions
type ScreenResolution struct {
	Width  int
	Height int
}
