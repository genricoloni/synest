package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/genricoloni/synest/internal/domain"
	"github.com/genricoloni/synest/internal/monitor"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"
)

// AppOptions definisce il grafo delle dipendenze dell'applicazione.
// Esportandolo, possiamo testare che il grafo sia valido senza lanciare il main.
var AppOptions = fx.Options(
	// Logger configuration
	fx.WithLogger(func(log *zap.Logger) fxevent.Logger {
		return &fxevent.ZapLogger{Logger: log}
	}),

	// Provide dependencies (Qui aggiungerai monitor.NewMprisMonitor, etc.)
	fx.Provide(
		newLogger,
		fx.Annotate(
			monitor.NewMprisMonitor,
			fx.As(new(domain.Monitor)),
		),
	),

	// Lifecycle hooks
	fx.Invoke(registerHooks),
)

func main() {
	app := fx.New(AppOptions)

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
func registerHooks(lc fx.Lifecycle, logger *zap.Logger, mon domain.Monitor) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			logger.Info("Synest Daemon Started")

			// Start the MPRIS monitor in a separate goroutine
			// (monitor.Start is blocking, it waits until context is cancelled)
			go func() {
				if err := mon.Start(ctx); err != nil && ctx.Err() == nil {
					logger.Error("Monitor stopped with error", zap.Error(err))
				}
			}()

			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("Shutting down")

			// Stop the monitor gracefully
			if err := mon.Stop(ctx); err != nil {
				logger.Error("Failed to stop monitor", zap.Error(err))
				return err
			}

			return nil
		},
	})
}
