package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/natet/honeygen/backend/internal/config"
	"github.com/natet/honeygen/backend/internal/decoy"
	"github.com/natet/honeygen/backend/internal/logging"
)

const defaultDecoyHTTPAddr = ":8081"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := loadConfig("")
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	logger := logging.NewLogger(cfg)

	if err := run(ctx, cfg, logger); err != nil {
		logger.Error("decoy-web exited", "error", err)
		os.Exit(1)
	}
}

func loadConfig(configPath string) (config.Config, error) {
	return config.LoadWithDefaults(configPath, config.Config{HTTPAddr: defaultDecoyHTTPAddr})
}

func run(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.New(slog.NewJSONHandler(io.Discard, nil))
	}

	handler, err := decoy.NewHandler(cfg, logger)
	if err != nil {
		return err
	}

	listener, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return err
	}
	defer listener.Close()

	server := &http.Server{
		Handler:  handler,
		ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	logger.Info("decoy-web listening", "addr", listener.Addr().String(), "generated_assets_dir", cfg.GeneratedAssetsDir, "internal_api_base_url", cfg.InternalAPIBaseURL)

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.Serve(listener)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}

		err := <-serverErr
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case err := <-serverErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
