// cSpell: words sirupsen samber sloglogrus
package utils

import (
	"io"
	"log/slog"
	"os"

	sloglogrus "github.com/samber/slog-logrus/v2"
	"github.com/sirupsen/logrus"
)

var DefaultLogLevel = slog.LevelDebug

const ErrorKey = "error"

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

func NewLogger(out io.Writer) *slog.Logger {
	l := logrus.New()
	l.SetOutput(out)
	return slog.New(sloglogrus.Option{
		Level:  DefaultLogLevel,
		Logger: l,
	}.NewLogrusHandler())
}

func (le *LogEnabled) Logger() *slog.Logger {
	if le.LogEntry == nil {
		le.LogEntry = NewLogger(os.Stdout)
	}
	return le.LogEntry
}

func (le *LogEnabled) SetLogger(logger *slog.Logger) {
	le.LogEntry = logger
}
