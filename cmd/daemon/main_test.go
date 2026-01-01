package main

import (
	"testing"

	"go.uber.org/fx"
)

// TestAppGraphValidity verifies that the dependency graph is resolvable.
// This test will fail if you forget an fx.Provide for a required interface.
func TestAppGraphValidity(t *testing.T) {
	// fx.ValidateApp checks that there are no missing or cyclic dependencies
	err := fx.ValidateApp(
		AppOptions,
		// In the future, when you have external dependencies (e.g., DBus),
		// you can use fx.Decorate or fx.Replace to swap them with Mocks here.
		// Example:
		// fx.Decorate(func() monitor.Monitor { return mocks.NewMockMonitor(...) }),
	)

	if err != nil {
		t.Errorf("Dependency graph is not valid: %v", err)
	}
}

// TestNewLogger specifically verifies the logger configuration
func TestNewLogger(t *testing.T) {
	logger, err := newLogger()
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	if logger == nil {
		t.Fatal("Logger should not be nil")
	}
	// We can verify it's a real logger by writing something (should not panic)
	logger.Info("Test logger initialization")
}

// TestEndToEndStartup (Optional) tries a real startup/stop in a controlled environment
// We use fx.NopLogger to avoid cluttering test output
func TestEndToEndStartup(t *testing.T) {
	app := fx.New(
		AppOptions,
		fx.NopLogger, // Silence Fx logs during tests
	)

	// Verify that the app can start without errors
	if err := app.Start(t.Context()); err != nil {
		t.Fatalf("App failed to start: %v", err)
	}

	// Verify that the app can stop without errors
	if err := app.Stop(t.Context()); err != nil {
		t.Fatalf("App failed to stop: %v", err)
	}
}
