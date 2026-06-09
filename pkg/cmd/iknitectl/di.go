package iknitectl

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/dig"

	"github.com/kaweezle/iknite/pkg/cmd/types"
	"github.com/kaweezle/iknite/pkg/cmd/util"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/iknitectl/config"
	"github.com/kaweezle/iknite/pkg/iknitectl/db"
	"github.com/kaweezle/iknite/pkg/utils"
)

func NewContainer(opts *RootOptions) (*dig.Container, error) {
	c := dig.New()

	if opts == nil {
		if err := provideDefaultHostAndOptions(c); err != nil {
			return nil, fmt.Errorf("failed to provide default host and options: %w", err)
		}
	} else {
		if err := provideOptionsAndHostFromOpts(c, opts); err != nil {
			return nil, fmt.Errorf("failed to provide options and host from opts: %w", err)
		}
	}

	if err := c.Provide(utils.NewHookManager); err != nil {
		return nil, fmt.Errorf("failed to provide HookManager: %w", err)
	}

	if err := c.Provide(func(opts *RootOptions) *config.ConfigOptions { return &opts.ConfigOptions }); err != nil {
		return nil, fmt.Errorf("failed to provide ConfigOptions: %w", err)
	}

	if err := c.Provide(configFromOptions); err != nil {
		return nil, fmt.Errorf("failed to provide Config: %w", err)
	}

	if err := c.Provide(newStore); err != nil {
		return nil, fmt.Errorf("failed to provide store: %w", err)
	}

	if err := c.Provide(func(opts *RootOptions) *util.BaseOptions { return &opts.BaseOptions }); err != nil {
		return nil, fmt.Errorf("failed to provide BaseOptions: %w", err)
	}

	if err := c.Provide(util.NewCmdInterface); err != nil {
		return nil, fmt.Errorf("failed to provide CmdInterface: %w", err)
	}

	if err := c.Provide(func(cmdIf util.CmdInterface) *slog.Logger { return cmdIf.Logger() }); err != nil {
		return nil, fmt.Errorf("failed to provide logger: %w", err)
	}

	return c, nil
}

func configFromOptions(h host.Host, opts *config.ConfigOptions) (*config.Config, error) {
	c := &config.Config{}
	err := opts.Resolve(h, c)
	if err != nil {
		return nil, fmt.Errorf("resolving config: %w", err)
	}
	return c, nil
}

func newStore(c *config.Config, hm *utils.HookManager) (*db.Store, error) {
	store, err := db.Open(c.Database)
	if err != nil {
		return nil, fmt.Errorf("opening store: %w", err)
	}
	hm.Register("store", func() error {
		return store.Close()
	})
	return store, nil
}

func provideDefaultHostAndOptions(c *dig.Container) error {
	if err := c.Provide(
		host.NewDefaultHost,
		dig.As(
			new(host.FileEnvironment),
			new(host.Environment),
			new(host.FileExecutor),
			new(host.NetworkHost),
			new(host.System),
			new(host.FileSystem),
		),
	); err != nil {
		return fmt.Errorf("failed to provide host: %w", err)
	}

	if err := c.Provide(func(fe host.FileEnvironment) (host.Host, error) {
		feAsHost, ok := fe.(host.Host)
		if !ok {
			return nil, fmt.Errorf("provided FileEnvironment does not implement Host")
		}
		return feAsHost, nil
	}); err != nil {
		return fmt.Errorf("failed to provide host as FileEnvironment: %w", err)
	}

	if err := c.Provide(NewRootOptions); err != nil {
		return fmt.Errorf("failed to provide RootOptions: %w", err)
	}

	return nil
}

func provideOptionsAndHostFromOpts(c *dig.Container, opts *RootOptions) error {
	if err := c.Provide(func() host.Host { return opts.host }); err != nil {
		return fmt.Errorf("failed to provide host: %w", err)
	}
	if err := c.Provide(func() host.Host { return opts.host },
		dig.As(
			new(host.Host),
			new(host.FileEnvironment),
			new(host.Environment),
			new(host.FileExecutor),
			new(host.NetworkHost),
			new(host.System),
			new(host.FileSystem),
		)); err != nil {
		return fmt.Errorf("failed to provide host: %w", err)
	}

	if err := c.Provide(func() *RootOptions { return opts }); err != nil {
		return fmt.Errorf("failed to provide RootOptions: %w", err)
	}
	return nil
}

type Injector interface {
	Invoke(function any, options ...dig.InvokeOption) error
}

// Resolve is a helper function to resolve a value of type T from the container or scope.
func Resolve[T any](s Injector) (T, error) {
	var result T
	err := s.Invoke(func(value T) {
		result = value
	})
	if err != nil {
		var zero T
		return zero, fmt.Errorf("failed to get value from scope: %w", err)
	}
	return result, nil
}

type Provider interface {
	Provide(function any, options ...dig.ProvideOption) error
	Decorate(function any, options ...dig.DecorateOption) error
}

func ProvideCommand(c Provider, cmd *cobra.Command, args []string) error {
	if err := c.Provide(func() *cobra.Command { return cmd }); err != nil {
		return fmt.Errorf("failed to provide command: %w", err)
	}
	if err := c.Provide(func() types.CmdArgs { return args }); err != nil {
		return fmt.Errorf("failed to provide args: %w", err)
	}
	if err := c.Provide(func(command *cobra.Command) types.CmdOut { return command.OutOrStdout() }); err != nil {
		if strings.Contains(errors.Unwrap(err).Error(), "already provided") {
			err = c.Decorate(func(_ types.CmdOut, command *cobra.Command) types.CmdOut { return command.OutOrStdout() })
		}
		if err != nil {
			return fmt.Errorf("failed to provide CmdOut: %w", err)
		}
	}
	if err := c.Provide(func() types.CmdIn { return cmd.InOrStdin() }); err != nil {
		return fmt.Errorf("failed to provide CmdIn: %w", err)
	}
	if err := c.Provide(
		func(command *cobra.Command) context.Context { return command.Context() },
	); err != nil {
		if strings.Contains(errors.Unwrap(err).Error(), "already provided") {
			err = c.Decorate(
				func(_ context.Context, command *cobra.Command) context.Context {
					return command.Context()
				},
			)
		}
		if err != nil {
			return fmt.Errorf("failed to provide context: %w", err)
		}
	}

	return nil
}
