// cSpell: words viper logrus sirupsen sloglogrus samber
package util

import (
	"io"
	"log/slog"

	"github.com/spf13/pflag"

	"github.com/kaweezle/iknite/pkg/utils"
)

const (
	// LogLevelFlag is the name of the flag used to set the logging level.
	LogLevelFlag = "verbosity"
	// JSONLogsFlag is the name of the flag used to enable JSON formatted logs.
	JSONLogsFlag = "json"
)

type BaseOptions struct {
	Verbosity slog.Level
	JSONLogs  bool
}

func DefaultBaseOptions() *BaseOptions {
	return &BaseOptions{
		Verbosity: slog.LevelInfo,
		JSONLogs:  false,
	}
}

func (opts *BaseOptions) AddFlags(flags *pflag.FlagSet) {
	flags.VarP(
		NewLogLevelValue(&opts.Verbosity), LogLevelFlag, "v", "Log level (trace, debug, info, warn, error)")
	flags.BoolVar(&opts.JSONLogs, JSONLogsFlag, opts.JSONLogs, "Emit log messages as JSON")
}

func (opts *BaseOptions) Logger(out io.Writer) *slog.Logger {
	return utils.NewLogger(out, opts.Verbosity, opts.JSONLogs)
}

// setUpLogs configures logrus output and level.
func (opts *BaseOptions) SetUpLogs(out io.Writer, cmdIf CmdInterface) {
	if setLogger, ok := cmdIf.(utils.LoggerHolder); ok {
		setLogger.SetLogger(opts.Logger(out))
		return
	} else {
		cmdIf.Logger().Warn("cmdIf does not implement loggerHolder, using default logger")
	}
}
