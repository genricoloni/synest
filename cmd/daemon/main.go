package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/genricoloni/synest/internal/config"
	"github.com/genricoloni/synest/internal/domain"
	"github.com/genricoloni/synest/internal/engine"
	"github.com/genricoloni/synest/internal/executor"
	"github.com/genricoloni/synest/internal/fetcher"
	"github.com/genricoloni/synest/internal/monitor"
	"github.com/genricoloni/synest/internal/processor"
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
		monitor.NewScreenResolution, // Detects screen resolution at startup
		fx.Annotate(
			config.NewAppConfig,
			fx.As(new(domain.Config)),
		),
		fx.Annotate(
			monitor.NewMprisMonitor,
			fx.As(new(domain.Monitor)),
		),
		fx.Annotate(
			fetcher.NewHTTPFetcher,
			fx.As(new(domain.Fetcher)),
		),
		fx.Annotate(
			processor.NewBlurProcessor,
			fx.As(new(domain.ImageProcessor)),
			fx.As(new(domain.Processor)),
		),
		fx.Annotate(
			executor.NewExecutor,
			fx.As(new(domain.Executor)),
		),
		engine.NewEngine, // Orchestrator
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
func registerHooks(lc fx.Lifecycle, logger *zap.Logger, eng *engine.Engine, mon domain.Monitor) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			logger.Info("Starting Synest Daemon...")

			// 1. Start the MPRIS monitor (event producer)
			// Runs in goroutine because monitor.Start is blocking
			go func() {
				if err := mon.Start(ctx); err != nil && ctx.Err() == nil {
					logger.Error("Monitor stopped with error", zap.Error(err))
				}
			}()

			// 2. Start the Engine (event consumer and orchestrator)
			if err := eng.Start(ctx); err != nil {
				return err
			}

			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("Shutting down Synest Daemon...")

			// 1. Stop the engine and restore original wallpaper
			if err := eng.Stop(ctx); err != nil {
				logger.Error("Failed to stop engine", zap.Error(err))
				// Don't return, try to stop monitor anyway
			}

			// 2. Stop the monitor gracefully
			if err := mon.Stop(ctx); err != nil {
				logger.Error("Failed to stop monitor", zap.Error(err))
				return err
			}

			return nil
		},
	})
}
