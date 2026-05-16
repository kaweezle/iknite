// cSpell: words sirupsen
package utils

import "github.com/sirupsen/logrus"

var DefaultLogLevel = logrus.DebugLevel

type LoggerProvider interface {
	Logger() logrus.FieldLogger
}

type LogEnabled struct {
	LogEntry logrus.FieldLogger
}

var _ LoggerProvider = (*LogEnabled)(nil)

func (le *LogEnabled) Logger() logrus.FieldLogger {
	if le.LogEntry == nil {
		l := logrus.New()
		l.SetLevel(DefaultLogLevel)
		le.LogEntry = logrus.NewEntry(l)
	}
	return le.LogEntry
}
