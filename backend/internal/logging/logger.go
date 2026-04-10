package logging

import (
	"log/slog"
	"os"

	"github.com/natet/honeygen/backend/internal/config"
)

func NewLogger(cfg config.Config) *slog.Logger {
	level := slog.LevelInfo
	if cfg.AppEnv == "development" || cfg.AppEnv == "test" {
		level = slog.LevelDebug
	}

	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))
}
