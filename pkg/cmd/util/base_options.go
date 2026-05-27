// cSpell: words viper  samber
package util

import (
	"io"
	"log/slog"
	"os"

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
	Output    io.Writer
	Verbosity slog.Level
	JSONLogs  bool
}

func DefaultBaseOptions() *BaseOptions {
	return &BaseOptions{
		Verbosity: slog.LevelInfo,
		JSONLogs:  false,
		Output:    os.Stdout,
	}
}

func (opts *BaseOptions) AddFlags(flags *pflag.FlagSet) {
	flags.VarP(
		NewLogLevelValue(&opts.Verbosity), LogLevelFlag, "v", "Log level (trace, debug, info, warn, error)")
	flags.BoolVar(&opts.JSONLogs, JSONLogsFlag, opts.JSONLogs, "Emit log messages as JSON")
	// TODO: Add flag for log output file
}

func (opts *BaseOptions) Logger() *slog.Logger {
	return utils.NewLogger(opts.Output, opts.Verbosity, opts.JSONLogs)
}

// setUpLogs configures log output and level.
func (opts *BaseOptions) SetUpLogs(out io.Writer, cmdIf CmdInterface) {
	if setLogger, ok := cmdIf.(utils.LoggerHolder); ok {
		opts.Output = out
		setLogger.SetLogger(opts.Logger())
	} else {
		cmdIf.Logger().Warn("cmdIf does not implement loggerHolder, using default logger")
	}
}
