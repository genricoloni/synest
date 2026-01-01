package monitor

import (
	"github.com/godbus/dbus/v5"
)

// DBusClient defines the interface for D-Bus operations.
// This abstraction allows us to mock D-Bus interactions in tests.
//
//go:generate mockgen -destination=mocks/dbus_client_mock.go -package=mocks github.com/genricoloni/synest/internal/monitor DBusClient
type DBusClient interface {
	// Close closes the D-Bus connection
	Close() error

	// AddMatchSignal adds a signal match rule
	AddMatchSignal(options ...dbus.MatchOption) error

	// Signal registers a channel to receive D-Bus signals
	Signal(ch chan<- *dbus.Signal)

	// ListNames returns all names on the bus
	ListNames() ([]string, error)

	// GetNameOwner returns the unique name that owns the given well-known name
	GetNameOwner(name string) (string, error)

	// GetProperty retrieves a property from a D-Bus object
	// player: The bus name (e.g., "org.mpris.MediaPlayer2.spotify")
	// path: The object path (e.g., "/org/mpris/MediaPlayer2")
	// prop: The property name (e.g., "org.mpris.MediaPlayer2.Player.Metadata")
	GetProperty(player, path, prop string) (dbus.Variant, error)
}

// StdDBusClient is the real implementation using godbus
type StdDBusClient struct {
	conn *dbus.Conn
}

// NewStdDBusClient creates a real D-Bus client connected to the session bus
func NewStdDBusClient() (*StdDBusClient, error) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return nil, err
	}
	return &StdDBusClient{conn: conn}, nil
}

// Close closes the D-Bus connection
func (c *StdDBusClient) Close() error {
	return c.conn.Close()
}

// AddMatchSignal adds a signal match rule
func (c *StdDBusClient) AddMatchSignal(options ...dbus.MatchOption) error {
	return c.conn.AddMatchSignal(options...)
}

// Signal registers a channel to receive D-Bus signals
func (c *StdDBusClient) Signal(ch chan<- *dbus.Signal) {
	c.conn.Signal(ch)
}

// ListNames returns all names on the bus
func (c *StdDBusClient) ListNames() ([]string, error) {
	var names []string
	err := c.conn.BusObject().Call("org.freedesktop.DBus.ListNames", 0).Store(&names)
	return names, err
}

// GetNameOwner returns the unique name that owns the given well-known name
func (c *StdDBusClient) GetNameOwner(name string) (string, error) {
	var owner string
	err := c.conn.BusObject().Call("org.freedesktop.DBus.GetNameOwner", 0, name).Store(&owner)
	return owner, err
}

// GetProperty retrieves a property from a D-Bus object
func (c *StdDBusClient) GetProperty(player, path, prop string) (dbus.Variant, error) {
	obj := c.conn.Object(player, dbus.ObjectPath(path))
	return obj.GetProperty(prop)
}
