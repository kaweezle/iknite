// cSpell: words paralleltest
package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/utils"
)

func TestInheritsFlags(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	source := pflag.NewFlagSet("source", pflag.ContinueOnError)
	target := pflag.NewFlagSet("target", pflag.ContinueOnError)

	var first bool
	var second bool
	source.BoolVar(&first, "first", false, "")
	source.BoolVar(&second, "second", false, "")

	inheritsFlags(source, target, "second")
	req.NotNil(target.Lookup("second"))
	req.Nil(target.Lookup("first"))
}

//nolint:paralleltest // mutates viper
func TestCommandConstructors(t *testing.T) {
	req := require.New(t)

	spec := &v1alpha1.IkniteClusterSpec{Ip: []byte{127, 0, 0, 1}}

	root := NewRootCmd(nil)
	req.NotNil(root)
	req.Equal("iknite", root.Name())

	expectedSubcommands := []string{
		"kustomize",
		"init",
		"reset",
		"clean",
		"kubelet",
		"mdns",
		"prepare",
		"start",
		"status",
		"info",
	}
	for _, name := range expectedSubcommands {
		t.Run("root has "+name, func(t *testing.T) {
			req := require.New(t)
			cmd, _, err := root.Find([]string{name})
			req.NoError(err)
			req.NotNil(cmd)
			req.Equal(name, cmd.Name())
		})
	}

	constructors := []struct {
		fn   func() *cobra.Command
		name string
	}{
		{name: "start", fn: func() *cobra.Command { return NewStartCmd(spec, nil) }},
		{name: "status", fn: func() *cobra.Command { return NewStatusCmd(spec, nil) }},
		{name: "prepare", fn: func() *cobra.Command { return NewPrepareCommand(spec) }},
		{name: "kubelet", fn: func() *cobra.Command { return NewKubeletCmd(spec, nil, nil) }},
		{name: "mdns", fn: func() *cobra.Command { return NewMdnsCmd(spec) }},
		{name: "info", fn: func() *cobra.Command { return NewInfoCmd(spec) }},
		{name: "kustomize", fn: func() *cobra.Command { return NewKustomizeCmd(nil, nil, nil) }},
		{
			name: "print-kustomize",
			fn: func() *cobra.Command {
				return NewPrintKustomizeCmd(host.NewMemMapFS(), utils.NewKustomizeOptions())
			},
		},
		{name: "clean", fn: func() *cobra.Command { return NewCmdClean(spec, nil, nil) }},
	}

	for _, tt := range constructors {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)
			cmd := tt.fn()
			req.NotNil(cmd)
			req.NotEmpty(cmd.Use)
		})
	}
}

func TestCleanOptionsBehavior(t *testing.T) {
	t.Parallel()

	tests := []struct {
		customize      func(*cleanOptions)
		name           string
		wantHasWork    bool
		wantAllEnabled bool
	}{
		{
			name: "defaults have work",
			customize: func(_ *cleanOptions) {
			},
			wantHasWork: true,
		},
		{
			name: "no flags means no work",
			customize: func(o *cleanOptions) {
				o.stopContainers = false
				o.stopContainerd = false
				o.unmountPaths = false
				o.cleanCni = false
				o.cleanIptables = false
				o.cleanAPIBackend = false
				o.cleanClusterConfig = false
				o.cleanIpAddress = false
				o.cleanAll = false
			},
			wantHasWork: false,
		},
		{
			name: "clean all enables all flags",
			customize: func(o *cleanOptions) {
				o.cleanAll = true
				o.stopContainers = false
				o.stopContainerd = false
				o.unmountPaths = false
				o.cleanCni = false
				o.cleanIptables = false
				o.cleanAPIBackend = false
				o.cleanClusterConfig = false
				o.cleanIpAddress = false
			},
			wantHasWork:    true,
			wantAllEnabled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)

			o := newCleanOptions()
			tt.customize(o)
			err := o.validate()
			req.NoError(err)
			req.Equal(tt.wantHasWork, o.hasActualWorkToDo())
			if tt.wantAllEnabled {
				req.True(o.stopContainers)
				req.True(o.stopContainerd)
				req.True(o.unmountPaths)
				req.True(o.cleanCni)
				req.True(o.cleanIptables)
				req.True(o.cleanAPIBackend)
				req.True(o.cleanClusterConfig)
				req.True(o.cleanIpAddress)
			}
		})
	}
}
