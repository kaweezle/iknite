package testutil

import (
	"context"
	"io"
	"log/slog"
	"testing"

	sloglogrus "github.com/samber/slog-logrus/v2"
	"github.com/sirupsen/logrus"
	logTest "github.com/sirupsen/logrus/hooks/test"

	"github.com/kaweezle/iknite/pkg/constants"
)

func NewLogger(out io.Writer) *slog.Logger {
	l := logrus.New()
	l.SetOutput(out)
	return slog.New(sloglogrus.Option{
		Level:  slog.LevelDebug,
		Logger: l,
	}.NewLogrusHandler())
}

func TestLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(sloglogrus.Option{
		Level:  slog.LevelDebug,
		Logger: logrus.New(),
	}.NewLogrusHandler())
}

func TestLoggerWithHook(t *testing.T) (*slog.Logger, *logTest.Hook) {
	t.Helper()
	logger := logrus.New()
	hook := logTest.NewLocal(logger)
	t.Cleanup(func() {
		hook.Reset()
	})
	return slog.New(sloglogrus.Option{
		Level:  slog.LevelDebug,
		Logger: logger,
	}.NewLogrusHandler()), hook
}

func WithTestLogger(t *testing.T, ctx context.Context) context.Context {
	t.Helper()
	logger := TestLogger(t)
	return context.WithValue(ctx, constants.LoggerContextKey{}, logger)
}
