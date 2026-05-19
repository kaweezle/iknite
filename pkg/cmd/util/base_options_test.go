// cSpell: words paralleltest
package util_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/cmd/util"
)

func TestDefaultBaseOptions_returnsExpectedDefaults(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	opts := util.DefaultBaseOptions()

	req.NotNil(opts, "DefaultBaseOptions should not return nil")
	req.Equal(slog.LevelInfo, opts.Verbosity, "expected default verbosity to be InfoLevel")
	req.False(opts.JSONLogs, "expected JSONLogs to be false by default")
}

func TestBaseOptions_AddFlags_registersAndParsesFlags(t *testing.T) {
	t.Parallel()
	opts := util.DefaultBaseOptions()
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)

	opts.AddFlags(flags)

	if flags.Lookup(util.LogLevelFlag) == nil {
		t.Fatalf("expected %q flag to be registered", util.LogLevelFlag)
	}
	if flags.Lookup(util.JSONLogsFlag) == nil {
		t.Fatalf("expected %q flag to be registered", util.JSONLogsFlag)
	}

	if err := flags.Parse([]string{"--verbosity", "debug", "--json"}); err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if opts.Verbosity != slog.LevelDebug {
		t.Fatalf("expected verbosity %q, got %q", slog.LevelDebug, opts.Verbosity)
	}
	if !opts.JSONLogs {
		t.Fatal("expected JSONLogs to be true after parsing --json")
	}
}

func TestBaseOptions_AddFlags_returnsErrorOnInvalidVerbosity(t *testing.T) {
	t.Parallel()
	opts := util.DefaultBaseOptions()
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.SetOutput(io.Discard)

	opts.AddFlags(flags)

	err := flags.Parse([]string{"--verbosity", "not-a-level"})
	if err == nil {
		t.Fatal("expected parse error for invalid verbosity value")
	}
}

func TestBaseOptions_SetUpLogs_configuresLogger(t *testing.T) {
	t.Parallel()
	tests := []struct {
		opts   *util.BaseOptions
		assert func(req *require.Assertions, logger *slog.Logger, out *bytes.Buffer)
		name   string
	}{
		//nolint:dupl // Similar test cases for different log levels and JSON settings.
		{
			name: "sets output and level without changing formatter when json disabled",
			opts: &util.BaseOptions{
				Verbosity: slog.LevelWarn,
				JSONLogs:  false,
			},
			assert: func(req *require.Assertions, logger *slog.Logger, out *bytes.Buffer) {
				ctx := context.Background()
				req.True(logger.Handler().Enabled(ctx, slog.LevelWarn))
				req.False(logger.Handler().Enabled(ctx, slog.LevelInfo))
				logger.Warn("Test warning message")
				outStr := out.String()
				req.Contains(outStr, "Test warning message", "Expected log output to contain the warning message")
				req.NotContains(outStr, "{", "Expected log output to not be in JSON format")
			},
		},
		//nolint:dupl // Similar test cases for different log levels and JSON settings.
		{
			name: "sets json formatter when json enabled",
			opts: &util.BaseOptions{
				Verbosity: slog.LevelError,
				JSONLogs:  true,
			},
			assert: func(req *require.Assertions, logger *slog.Logger, out *bytes.Buffer) {
				ctx := context.Background()
				req.True(logger.Handler().Enabled(ctx, slog.LevelError))
				req.False(logger.Handler().Enabled(ctx, slog.LevelWarn))
				logger.Error("Test error message")
				outStr := out.String()
				req.Contains(outStr, "Test error message", "Expected log output to contain the error message")
				req.Contains(outStr, "{", "Expected log output to be in JSON format")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)

			var out bytes.Buffer
			cmdIf := util.NewCmdInterface(nil)
			tt.opts.SetUpLogs(&out, cmdIf)

			logger := cmdIf.Logger()
			tt.assert(req, logger, &out)
		})
	}
}
