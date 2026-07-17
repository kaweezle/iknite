package iknitectl_test

import (
	"bytes"
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/dig"

	iknitectl "github.com/kaweezle/iknite/pkg/cmd/iknitectl"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/iknitectl/config"
	"github.com/kaweezle/iknite/pkg/testutil"
)

func createMemFSContainer(t *testing.T) *dig.Container {
	t.Helper()
	req := require.New(t)

	fs := host.NewMemMapFS()
	h, err := testutil.NewDummyHost(fs, &testutil.DummyHostOptions{})
	req.NoError(err)

	out := &bytes.Buffer{}
	options := iknitectl.NewRootOptions(h)
	options.Output = out
	c, err := iknitectl.NewContainer(options)
	req.NoError(err)
	return c
}

func TestContainer(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	c, err := iknitectl.NewContainer(nil)
	req.NoError(err)
	req.NoError(c.Invoke(func(h host.FileSystem) error {
		if h == nil {
			return fmt.Errorf("host is nil")
		}
		return nil
	}))
	req.NoError(c.Invoke(func(h host.System) error {
		if h == nil {
			return fmt.Errorf("host is nil")
		}
		return nil
	}))
	req.NoError(c.Invoke(func(h host.Host) error {
		if h == nil {
			return fmt.Errorf("host is nil")
		}
		return nil
	}))
}

func TestContainerWithOptions(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	opts := iknitectl.NewRootOptions(nil)
	c, err := iknitectl.NewContainer(opts)
	req.NoError(err)
	req.NoError(c.Invoke(func(h host.FileSystem) error {
		if h == nil {
			return fmt.Errorf("host is nil")
		}
		return nil
	}))

	req.NoError(c.Invoke(func(h host.Host) error {
		if h == nil {
			return fmt.Errorf("host is nil")
		}
		return nil
	}))
}

func TestContainerWithTestHost(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	c := createMemFSContainer(t)
	req.NoError(c.Invoke(func(h host.Host) error {
		if h == nil {
			return fmt.Errorf("host is nil")
		}
		return nil
	}))

	req.NoError(c.Invoke(func(c *config.ConfigOptions) error {
		if c == nil {
			return fmt.Errorf("config options is nil")
		}
		if c.ConfigDir != "/home/alpine/.config/iknite" {
			return fmt.Errorf("unexpected config path: %s", c.ConfigDir)
		}
		return nil
	}))

	// Outputting something in the logs
	req.NoError(c.Invoke(func(logger *slog.Logger) error {
		if logger == nil {
			return fmt.Errorf("logger is nil")
		}
		logger.Info("Test log message")
		return nil
	}))

	// Ensure that the log message was captured in the output buffer
	req.NoError(c.Invoke(func(options *iknitectl.RootOptions) error {
		if options == nil {
			return fmt.Errorf("options is nil")
		}
		if options.Output == nil {
			return fmt.Errorf("output is nil")
		}
		outputBuffer, ok := options.Output.(*bytes.Buffer)
		if !ok {
			return fmt.Errorf("output is not a bytes.Buffer")
		}
		outputBytes := outputBuffer.Bytes()
		if !bytes.Contains(outputBytes, []byte("Test log message")) {
			return fmt.Errorf("expected log message not found in output: %s", outputBytes)
		}
		return nil
	}))
}

func TestContainerDecorateAfterTheFact(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	c := createMemFSContainer(t)

	// Outputting something in the logs
	req.NoError(c.Invoke(func(logger *slog.Logger) error {
		if logger == nil {
			return fmt.Errorf("logger is nil")
		}
		logger.Info("Test log message")
		return nil
	}))

	// Now decorate the logger after the fact
	req.NoError(c.Decorate(func(logger *slog.Logger) *slog.Logger {
		return logger.With("decorated", "true")
	}))

	// Log another message and ensure the decorator is applied
	req.NoError(c.Invoke(func(logger *slog.Logger) error {
		if logger == nil {
			return fmt.Errorf("logger is nil")
		}
		logger.Info("Test log message after decoration")
		return nil
	}))

	// Ensure that the decorated log message was captured in the output buffer
	req.NoError(c.Invoke(func(options *iknitectl.RootOptions) error {
		if options == nil {
			return fmt.Errorf("options is nil")
		}
		if options.Output == nil {
			return fmt.Errorf("output is nil")
		}
		outputBuffer, ok := options.Output.(*bytes.Buffer)
		if !ok {
			return fmt.Errorf("output is not a bytes.Buffer")
		}
		outputBytes := outputBuffer.Bytes()
		if !bytes.Contains(outputBytes, []byte(`decorated=true`)) {
			return fmt.Errorf("expected decorated log message not found in output: %s", string(outputBytes))
		}
		return nil
	}))
}
