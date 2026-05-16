// cSpell: words logrus sirupsen paralleltest
package util_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/cmd/util"
)

func TestDefaultBaseOptions_returnsExpectedDefaults(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	opts := util.DefaultBaseOptions()

	req.NotNil(opts, "DefaultBaseOptions should not return nil")
	req.Equal(logrus.InfoLevel, opts.Verbosity, "expected default verbosity to be InfoLevel")
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
	if opts.Verbosity != logrus.DebugLevel {
		t.Fatalf("expected verbosity %q, got %q", logrus.DebugLevel, opts.Verbosity)
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
		assert func(req *require.Assertions, std *logrus.Logger)
		name   string
	}{
		{
			name: "sets output and level without changing formatter when json disabled",
			opts: &util.BaseOptions{
				Verbosity: logrus.WarnLevel,
				JSONLogs:  false,
			},
			assert: func(req *require.Assertions, std *logrus.Logger) {
				req.Equal(logrus.WarnLevel, std.Level, "Expected logger level to be set to WarnLevel")
				req.IsType(
					&logrus.TextFormatter{},
					std.Formatter,
					"Expected formatter to be TextFormatter when JSONLogs is false",
				)
			},
		},
		{
			name: "sets json formatter when json enabled",
			opts: &util.BaseOptions{
				Verbosity: logrus.ErrorLevel,
				JSONLogs:  true,
			},
			assert: func(req *require.Assertions, std *logrus.Logger) {
				req.Equal(logrus.ErrorLevel, std.Level, "Expected logger level to be set to ErrorLevel")
				req.IsType(
					&logrus.JSONFormatter{},
					std.Formatter,
					"Expected formatter to be JSONFormatter when JSONLogs is true",
				)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			std := logrus.New()
			req := require.New(t)

			var out bytes.Buffer
			tt.opts.SetUpLogs(&out, std)

			req.Equal(&out, std.Out, "expected standard logger output to be set to provided writer")
			tt.assert(req, std)
		})
	}
}
