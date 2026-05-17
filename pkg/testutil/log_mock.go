package testutil

import (
	"testing"

	"github.com/sirupsen/logrus"
	logTest "github.com/sirupsen/logrus/hooks/test"
)

func TestLogger(t *testing.T) *logrus.Entry {
	t.Helper()
	logger := logrus.New()
	logger.SetOutput(t.Output())
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
