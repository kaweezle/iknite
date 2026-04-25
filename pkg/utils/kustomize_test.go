package utils_test

import (
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/cmd/options"
	"github.com/kaweezle/iknite/pkg/config"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/utils"
)

func TestKustomizeOptions_DefaultsAndFlags(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	options := utils.NewKustomizeOptions()
	req.False(options.ForceConfig)
	req.False(options.ForceEmbedded)
	req.Equal(constants.DefaultKustomization, options.Kustomization)

	flags := pflag.NewFlagSet("kustomize", pflag.ContinueOnError)
	utils.AddKustomizeOptionsFlags(flags, options)
	err := flags.Parse([]string{"--force-config", "--force-embedded", "--kustomization=custom"})
	req.NoError(err)

	req.True(options.ForceConfig)
	req.True(options.ForceEmbedded)
	req.Equal("custom", options.Kustomization)
}

func TestKustomizationValue(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	opts := utils.NewKustomizeOptions()
	req.Equal(constants.DefaultKustomization, opts.Kustomization)

	ikniteSpec := &v1alpha1.IkniteClusterSpec{
		Kustomization: constants.DefaultKustomization,
	}
	flags := pflag.NewFlagSet("kustomize", pflag.ContinueOnError)
	config.AddIkniteClusterFlags(flags, ikniteSpec)
	utils.AddKustomizeOptionsFlags(flags, opts)
	err := flags.Parse([]string{"--kustomization=custom"})
	req.NoError(err)
	req.Equal("custom", opts.Kustomization)
	req.Equal("custom", ikniteSpec.Kustomization)

	// Getting to 100% code coverage for the MultiStringValue type
	f := flags.Lookup(options.Kustomization)
	req.NotNil(f)
	req.Equal("custom", f.Value.String())
	req.IsType(&utils.MultiStringValue{}, f.Value)
	req.Equal("string", f.Value.Type())

	v := &utils.MultiStringValue{}
	req.Empty(v.String())
}
