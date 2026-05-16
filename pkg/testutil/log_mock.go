package testutil

import (
	"testing"

	"github.com/sirupsen/logrus"
	logTest "github.com/sirupsen/logrus/hooks/test"
)

func TestLogger(t *testing.T) (*logrus.Entry, *logTest.Hook) {
	t.Helper()
	logger, hook := logTest.NewNullLogger()
	t.Cleanup(func() {
		hook.Reset()
	})
	return logger.WithContext(t.Context()), hook
}
