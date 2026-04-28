/*
Copyright © 2025 Antoine Martin <antoine@openance.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
// cSpell: words paralleltest testutil

package kubewait

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/cli-utils/pkg/object"

	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/testutil"
)

func TestNewOptionsAndFlags(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	opts := NewOptions()
	req.NotNil(opts)
	req.Equal(defaultBootstrapDir, opts.BootstrapDir)
	req.Equal(defaultBootstrapScript, opts.BootstrapScript)
	req.Equal(defaultTimeout, opts.Timeout)

	flags := pflag.NewFlagSet("kubewait", pflag.ContinueOnError)
	opts.AddFlags(flags)
	req.NotNil(flags.Lookup("timeout"))
	req.NotNil(flags.Lookup("resource-types"))
	req.NotNil(flags.Lookup("bootstrap-dir"))
	req.NotNil(flags.Lookup("skip-wait"))
	req.NotNil(flags.Lookup("skip-bootstrap"))
	req.NotNil(flags.Lookup("all-namespaces"))
}

func TestResourceAndBootstrapOptionFactories(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	resources := NewResourcesOptions()
	req.Equal(defaultTimeout, resources.Timeout)
	req.Equal(defaultStatusUpdateInterval, resources.StatusUpdateInterval)
	req.Equal(defaultResourcesUpdateInterval, resources.ResourcesUpdateInterval)
	req.Equal(defaultSettlePeriod, resources.SettlePeriod)

	bootstrap := NewBootstrapOptions()
	req.Equal(defaultBootstrapDir, bootstrap.BootstrapDir)
	req.Equal(defaultBootstrapScript, bootstrap.BootstrapScript)
}

//nolint:paralleltest // mutates environment
func TestReadEnvFile(t *testing.T) {
	req := require.New(t)

	fs := host.NewMemMapFS()
	dir := "base"
	envPath := filepath.Join(dir, ".env")
	req.NoError(fs.WriteFile(envPath, []byte("TEST_KUBEWAIT_ENV=enabled\n"), 0o600))
	oldValue, hadValue := os.LookupEnv("TEST_KUBEWAIT_ENV")
	req.NoError(os.Unsetenv("TEST_KUBEWAIT_ENV"))
	t.Cleanup(func() {
		if hadValue {
			req.NoError(os.Setenv("TEST_KUBEWAIT_ENV", oldValue))
			return
		}
		req.NoError(os.Unsetenv("TEST_KUBEWAIT_ENV"))
	})

	opts := &BootstrapOptions{BootstrapDir: dir}
	ok, err := opts.ReadEnvFile(fs)
	req.NoError(err)
	req.True(ok)
	req.Equal("enabled", os.Getenv("TEST_KUBEWAIT_ENV"))

	missingOpts := &BootstrapOptions{BootstrapDir: filepath.Join(dir, "missing")}
	ok, err = missingOpts.ReadEnvFile(fs)
	req.NoError(err)
	req.True(ok)
}

func TestExtractDomain(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		url  string
		want string
	}{
		{name: "git ssh style", url: "git@github.com:owner/repo.git", want: "github.com"},
		{name: "invalid no at", url: "https://github.com/owner/repo", want: ""},
		{name: "invalid missing repo", url: "git@github.com", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)
			req.Equal(tt.want, extractDomain(tt.url))
		})
	}
}

func TestRunBootstrapAndRunKubewaitSkipPaths(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	fs := host.NewMemMapFS()
	dir := "base"
	script := filepath.Join(dir, "iknite-bootstrap.sh")
	req.NoError(fs.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0o755))

	hostOpts := &testutil.DummyHostOptions{}
	h, err := testutil.NewDummyHost(fs, hostOpts)
	req.NoError(err)

	opts := &Options{BootstrapOptions: BootstrapOptions{BootstrapDir: dir, BootstrapScript: "iknite-bootstrap.sh"}}
	err = runBootstrap(t.Context(), h, opts)
	req.NoError(err)

	err = RunKubewait(t.Context(), h, &Options{SkipWaitingForResources: true, SkipBootstrap: true}, nil)
	req.NoError(err)
}

func TestResourceWaiterStateHelpers(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	w := &resourceWaiter{
		opts:       &Options{ResourcesOptions: ResourcesOptions{SettlePeriod: 10 * time.Millisecond}},
		endChannel: make(chan error, 1),
	}

	req.False(w.hasSettleTimer())
	req.NoError(w.StartSettleTimer())
	req.True(w.hasSettleTimer())
	req.NoError(w.StopSettleTimer())
	req.False(w.hasSettleTimer())

	set := object.ObjMetadataSet{{Namespace: "ns", Name: "a"}}
	w.setCurrentDataSet(set)
	req.Equal(set, w.getCurrentDataSet())
}
