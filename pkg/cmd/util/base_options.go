// cSpell: words viper logrus sirupsen
package util

import (
	"io"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

const (
	// LogLevelFlag is the name of the flag used to set the logging level.
	LogLevelFlag = "verbosity"
	// JSONLogsFlag is the name of the flag used to enable JSON formatted logs.
	JSONLogsFlag = "json"
)

type BaseOptions struct {
	Verbosity log.Level
	JSONLogs  bool
}

func DefaultBaseOptions() *BaseOptions {
	return &BaseOptions{
		Verbosity: log.InfoLevel,
		JSONLogs:  false,
	}
}

func (opts *BaseOptions) AddFlags(flags *pflag.FlagSet) {
	flags.VarP(
		NewLogLevelValue(&opts.Verbosity), LogLevelFlag, "v", "Log level (debug, info, warn, error, fatal, panic)")
	flags.BoolVar(&opts.JSONLogs, JSONLogsFlag, opts.JSONLogs, "Emit log messages as JSON")
}

// setUpLogs configures logrus output and level.
func (opts *BaseOptions) SetUpLogs(out io.Writer) error {
	log.SetOutput(out)
	if opts.JSONLogs {
		log.SetFormatter(&log.JSONFormatter{})
	}
	log.SetLevel(opts.Verbosity)
	return nil
}
