// Package logging настраивает структурированное логирование через slog
// для всего приложения.
package logging

import (
	"log/slog"
	"os"
	"strings"
)

// New возвращает slog.Logger с JSON-выводом в stdout и уровнем level
// ("debug", "info", "warn", "error"; неизвестное значение → info).
func New(level string) *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: parseLevel(level),
	})
	return slog.New(handler)
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
