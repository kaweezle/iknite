// cSpell: words sirupsen sloglogrus samber
package util

import (
	"context"
	"log/slog"

	sloglogrus "github.com/samber/slog-logrus/v2"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/kaweezle/iknite/pkg/utils"
)

type loggerHolder interface {
	SetLogger(*logrus.Entry)
}

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

var _ loggerHolder = (*cmdStruct)(nil)

func (c *cmdStruct) Viper() *viper.Viper {
	return c.v
}

func (c *cmdStruct) SetLogger(logger *logrus.Entry) {
	c.LogEntry = logger
}

type _logger struct {
	*logrus.Logger
}

func (l _logger) Level() slog.Level {
	switch l.GetLevel() {
	case logrus.TraceLevel:
		return slog.LevelDebug
	case logrus.DebugLevel:
		return slog.LevelDebug
	case logrus.InfoLevel:
		return slog.LevelInfo
	case logrus.WarnLevel:
		return slog.LevelWarn
	case logrus.ErrorLevel:
		return slog.LevelError
	case logrus.FatalLevel:
		return slog.LevelError
	case logrus.PanicLevel:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func NewCmdInterface() CmdInterface {
	l := logrus.New()
	e := logrus.NewEntry(l)
	v := viper.NewWithOptions(viper.WithLogger(slog.New(sloglogrus.Option{
		Level:  _logger{l},
		Logger: l,
	}.NewLogrusHandler())))
	return &cmdStruct{
		LogEnabled: utils.LogEnabled{LogEntry: e},
		v:          v,
	}
}

var defaultEntry = logrus.NewEntry(logrus.StandardLogger())

type cmdInterfaceKey struct{}

func WithCmdInterface(ctx context.Context, cmdInterface CmdInterface) context.Context {
	if holder, ok := cmdInterface.(loggerHolder); ok {
		logger := cmdInterface.Logger()
		if logEntry, ok := logger.(*logrus.Entry); ok {
			holder.SetLogger(logEntry.WithContext(ctx))
		}
	}
	return context.WithValue(ctx, cmdInterfaceKey{}, cmdInterface)
}

func CmdInterfaceFromContext(ctx context.Context) (CmdInterface, bool) {
	value := ctx.Value(cmdInterfaceKey{})
	if value == nil {
		return nil, false
	}
	cmdInterface, ok := value.(CmdInterface)
	return cmdInterface, ok
}

func CmdInterfaceFromCommand(cmd *cobra.Command) (CmdInterface, bool) {
	ctx := cmd.Context()
	if ctx == nil {
		return nil, false
	}
	return CmdInterfaceFromContext(ctx)
}

func GetLoggerFromContext(ctx context.Context) logrus.FieldLogger {
	cmdInterface, ok := CmdInterfaceFromContext(ctx)
	if !ok {
		return defaultEntry.WithContext(ctx) // Return a new logger if no CmdInterface is found in the context
	}
	return cmdInterface.Logger()
}

func GetLoggerFromCommand(cmd *cobra.Command) logrus.FieldLogger {
	ctx := cmd.Context()
	if ctx == nil {
		return defaultEntry
	}
	return GetLoggerFromContext(ctx)
}

func GetViperFromContext(ctx context.Context) *viper.Viper {
	cmdInterface, ok := CmdInterfaceFromContext(ctx)
	if !ok {
		return viper.New() // Return a new Viper instance if no CmdInterface is found in the context
	}
	return cmdInterface.Viper()
}

func GetViperFromCommand(cmd *cobra.Command) *viper.Viper {
	ctx := cmd.Context()
	if ctx == nil {
		return viper.New() // Return a new Viper instance if the command context is nil
	}
	return GetViperFromContext(ctx)
}
