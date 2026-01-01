package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"
)

func main() {
	app := fx.New(
		// Logger configuration
		fx.WithLogger(func(log *zap.Logger) fxevent.Logger {
			return &fxevent.ZapLogger{Logger: log}
		}),
		
		// Provide dependencies
		fx.Provide(
			newLogger,
		),
		
		// Lifecycle hooks
		fx.Invoke(registerHooks),
	)

	// Handle graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Start the application
	if err := app.Start(ctx); err != nil {
		panic(err)
	}

	// Wait for interrupt signal
	<-ctx.Done()

	// Stop the application gracefully
	if err := app.Stop(context.Background()); err != nil {
		panic(err)
	}
}

// newLogger creates a new zap logger instance
func newLogger() (*zap.Logger, error) {
	logger, err := zap.NewProduction()
	if err != nil {
		return nil, err
	}
	return logger, nil
}

// registerHooks sets up application lifecycle hooks
func registerHooks(lc fx.Lifecycle, logger *zap.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			logger.Info("Synest Daemon Started")
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("Shutting down")
			return nil
		},
	})
}
