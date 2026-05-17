package testutil

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	logTest "github.com/sirupsen/logrus/hooks/test"

	"github.com/kaweezle/iknite/pkg/constants"
)

func TestLogger(t *testing.T) *logrus.Entry {
	t.Helper()
	logger := logrus.New()
	// FIXME: This provokes race coditions
	// logger.SetOutput(t.Output())
	logger.SetLevel(logrus.DebugLevel)
	return logger.WithContext(t.Context())
}

func TestLoggerWithHook(t *testing.T) (*logrus.Entry, *logTest.Hook) {
	t.Helper()
	logger := TestLogger(t)
	hook := logTest.NewLocal(logger.Logger)
	t.Cleanup(func() {
		hook.Reset()
	})
	return logger, hook
}

func WithTestLogger(t *testing.T, ctx context.Context) context.Context {
	t.Helper()
	logger := TestLogger(t)
	return context.WithValue(ctx, constants.LoggerContextKey{}, logger)
}
