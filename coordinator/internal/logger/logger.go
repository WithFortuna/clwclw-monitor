package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

var logFile *os.File

// Init configures the global slog logger.
// level: debug, info, warn, error (default: info)
// format: text or json (default: text)
// filePath: log file path; empty means stdout
func Init(level, format, filePath string) error {
	var lvl slog.Level
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		lvl = slog.LevelDebug
	case "info", "":
		lvl = slog.LevelInfo
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		return fmt.Errorf("unknown log level: %q", level)
	}

	var w io.Writer = os.Stdout
	if filePath = strings.TrimSpace(filePath); filePath != "" {
		f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("open log file %q: %w", filePath, err)
		}
		logFile = f
		w = f
	}

	opts := &slog.HandlerOptions{Level: lvl}

	var handler slog.Handler
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json":
		handler = slog.NewJSONHandler(w, opts)
	default:
		handler = slog.NewTextHandler(w, opts)
	}

	slog.SetDefault(slog.New(handler))
	return nil
}

// Close releases any open log file handle.
func Close() {
	if logFile != nil {
		_ = logFile.Close()
		logFile = nil
	}
}
