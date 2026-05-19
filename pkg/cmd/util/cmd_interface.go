// cSpell: words
package util

import (
	"context"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/utils"
)

// TODO: Replace these concrete dependencies with interfaces to allow for better testability and separation of concerns.
type CmdInterface interface {
	utils.LoggerProvider
	utils.ViperProvider
}

type cmdStruct struct {
	utils.LogEnabled
	utils.ViperEnabled
}

var _ CmdInterface = (*cmdStruct)(nil)

var _ utils.LoggerHolder = (*cmdStruct)(nil)

func (c *cmdStruct) Viper() *viper.Viper {
	return c.ViperEnabled.Viper()
}

func (c *cmdStruct) SetLogger(logger *slog.Logger) {
	c.LogEntry = logger
}

func NewCmdInterface(opts *BaseOptions) CmdInterface {
	if opts == nil {
		opts = DefaultBaseOptions()
	}
	le := opts.Logger(os.Stdout)
	viperLogger := utils.NewLogger(os.Stdout, slog.LevelWarn, opts.JSONLogs)
	v := viper.NewWithOptions(viper.WithLogger(viperLogger))
	return &cmdStruct{
		LogEnabled:   utils.LogEnabled{LogEntry: le},
		ViperEnabled: *utils.NewViperEnabled(v),
	}
}

var defaultLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
	Level: slog.LevelDebug,
}))

func WithCmdInterface(ctx context.Context, cmdInterface CmdInterface) context.Context {
	return WithViper(WithLogger(ctx, cmdInterface), cmdInterface)
}

func SetCmdInterface(cmd *cobra.Command, cmdInterface CmdInterface) {
	ctx := cmd.Context()
	if ctx == nil {
		cmdInterface.Logger().Warn("Command context is nil, creating a new context for the command")
		return
	}
	cmd.SetContext(WithCmdInterface(ctx, cmdInterface))
}

func WithLogger(ctx context.Context, loggerProvider utils.LoggerProvider) context.Context {
	return context.WithValue(ctx, constants.LoggerContextKey{}, loggerProvider)
}

func WithViper(ctx context.Context, viperProvider utils.ViperProvider) context.Context {
	return context.WithValue(ctx, constants.ViperContextKey{}, viperProvider)
}

func CmdInterfaceFromContext(ctx context.Context) CmdInterface {
	value := ctx.Value(constants.LoggerContextKey{})
	if value != nil {
		if cmdIf, ok := value.(CmdInterface); ok {
			return cmdIf
		}
	}

	return NewCmdInterface(nil)
}

func CmdInterfaceFromCommand(cmd *cobra.Command) (CmdInterface, bool) {
	ctx := cmd.Context()
	if ctx == nil {
		return nil, false
	}
	return CmdInterfaceFromContext(ctx), true
}

func LoggerFromContext(ctx context.Context) *slog.Logger {
	value := ctx.Value(constants.LoggerContextKey{})
	if value != nil {
		if logger, ok := value.(utils.LoggerProvider); ok {
			return logger.Logger()
		}
	}
	return defaultLogger
}

func LoggerFromCommand(cmd *cobra.Command) *slog.Logger {
	ctx := cmd.Context()
	if ctx == nil {
		return defaultLogger
	}
	return LoggerFromContext(ctx)
}

func ViperFromContext(ctx context.Context) *viper.Viper {
	value := ctx.Value(constants.ViperContextKey{})
	if value != nil {
		if viperProvider, ok := value.(utils.ViperProvider); ok {
			return viperProvider.Viper()
		}
	}
	// Return a new Viper instance if no CmdInterface is found in the context
	return viper.New()
}

func ViperFromCommand(cmd *cobra.Command) *viper.Viper {
	ctx := cmd.Context()
	if ctx == nil {
		return viper.New() // Return a new Viper instance if the command context is nil
	}
	return ViperFromContext(ctx)
}
