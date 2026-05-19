package utils

import (
	"io"
	"log/slog"
	"os"
)

var DefaultLogLevel = slog.LevelDebug

const (
	ErrorKey   = "error"
	LevelTrace = slog.Level(-8)
)

type LoggerProvider interface {
	Logger() *slog.Logger
}

type LoggerHolder interface {
	SetLogger(*slog.Logger)
}

type LogEnabled struct {
	LogEntry *slog.Logger
}

var _ LoggerProvider = (*LogEnabled)(nil)

var _ LoggerHolder = (*LogEnabled)(nil)

func NewLogger(out io.Writer, level slog.Level, jsonLogs bool) *slog.Logger {
	var handler slog.Handler
	options := &slog.HandlerOptions{
		Level: level,
	}
	if jsonLogs {
		handler = slog.NewJSONHandler(out, options)
	} else {
		handler = slog.NewTextHandler(out, options)
	}
	return slog.New(handler)
}

func (le *LogEnabled) Logger() *slog.Logger {
	if le.LogEntry == nil {
		le.LogEntry = NewLogger(os.Stdout, DefaultLogLevel, false)
	}
	return le.LogEntry
}

func (le *LogEnabled) SetLogger(logger *slog.Logger) {
	le.LogEntry = logger
}
