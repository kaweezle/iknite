// cSpell: words zerolog
package utils

import (
	"io"
	"log/slog"
	"os"

	"github.com/rs/zerolog"
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
	zLevel, err := zerolog.ParseLevel(level.String())
	if err != nil {
		zLevel = zerolog.DebugLevel
	}
	zl := zerolog.New(out).Level(zLevel)
	if !jsonLogs {
		output := zerolog.ConsoleWriter{Out: os.Stdout}
		zl = zl.Output(output)
	}
	handler := zerolog.NewSlogHandler(zl)
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
