package utils_test

import (
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/utils"
)

func TestKustomizeOptions_DefaultsAndFlags(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	options := utils.NewKustomizeOptions()
	req.False(options.ForceConfig)
	req.False(options.ForceEmbedded)

	flags := pflag.NewFlagSet("kustomize", pflag.ContinueOnError)
	utils.AddKustomizeOptionsFlags(flags, options)
	err := flags.Parse([]string{"--force-config", "--force-embedded"})
	req.NoError(err)

	req.True(options.ForceConfig)
	req.True(options.ForceEmbedded)
}
