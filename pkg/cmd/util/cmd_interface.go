// cSpell: words sirupsen sloglogrus samber
package util

import (
	"context"
	"log/slog"
	"os"

	sloglogrus "github.com/samber/slog-logrus/v2"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/utils"
)

type ViperProvider interface {
	Viper() *viper.Viper
}

// TODO: Replace these concrete dependencies with interfaces to allow for better testability and separation of concerns.
type CmdInterface interface {
	utils.LoggerProvider
	ViperProvider
}

type cmdStruct struct {
	utils.LogEnabled
	v *viper.Viper
}

var _ CmdInterface = (*cmdStruct)(nil)

var _ utils.LoggerHolder = (*cmdStruct)(nil)

func (c *cmdStruct) Viper() *viper.Viper {
	return c.v
}

func (c *cmdStruct) SetLogger(logger *slog.Logger) {
	c.LogEntry = logger
}

func NewCmdInterface() CmdInterface {
	le := utils.NewLogger(os.Stdout)
	v := viper.NewWithOptions(viper.WithLogger(le))
	return &cmdStruct{
		LogEnabled: utils.LogEnabled{LogEntry: le},
		v:          v,
	}
}

var defaultLogger = slog.New(sloglogrus.Option{
	Level:  slog.LevelDebug,
	Logger: logrus.New(),
}.NewLogrusHandler())

func WithCmdInterface(ctx context.Context, cmdInterface CmdInterface) context.Context {
	newCtx := context.WithValue(ctx, constants.LoggerContextKey{}, cmdInterface.Logger())
	return context.WithValue(newCtx, constants.ViperContextKey{}, cmdInterface.Viper())
}

func SetCmdInterface(cmd *cobra.Command, cmdInterface CmdInterface) {
	ctx := cmd.Context()
	if ctx == nil {
		cmdInterface.Logger().Warn("Command context is nil, creating a new context for the command")
		return
	}
	cmd.SetContext(WithCmdInterface(ctx, cmdInterface))
}

func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, constants.LoggerContextKey{}, logger)
}

func WithViper(ctx context.Context, v *viper.Viper) context.Context {
	return context.WithValue(ctx, constants.ViperContextKey{}, v)
}

func CmdInterfaceFromContext(ctx context.Context) CmdInterface {
	l := LoggerFromContext(ctx)
	v := ViperFromContext(ctx)
	return &cmdStruct{
		LogEnabled: utils.LogEnabled{LogEntry: l},
		v:          v,
	}
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
		if logger, ok := value.(*slog.Logger); ok {
			return logger
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
		if v, ok := value.(*viper.Viper); ok {
			return v
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
