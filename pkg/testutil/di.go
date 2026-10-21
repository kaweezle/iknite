package testutil

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/dig"

	"github.com/kaweezle/iknite/pkg/cmd/types"
	"github.com/kaweezle/iknite/pkg/host"
)

func TestContainer(t *testing.T) *dig.Container {
	t.Helper()
	req := require.New(t)

	c := dig.New()
	req.NoError(c.Provide(func() *testing.T { return t }))
	req.NoError(c.Provide(host.NewMemMapFS, dig.As(new(host.FileSystem))))
	req.NoError(c.Provide(func() *DummyHostOptions {
		return &DummyHostOptions{Platform: PlatformLinux, Username: DefaultUsername}
	}))
	req.NoError(c.Provide(NewDummyHost))
	req.NoError(c.Provide(func(dh *DelegateHost) host.Host { return dh }))
	req.NoError(c.Provide(func(dh *DelegateHost) host.System { return dh }))
	req.NoError(c.Provide(func(dh *DelegateHost) host.Environment { return dh }))
	req.NoError(c.Provide(func(dh *DelegateHost) host.FileEnvironment { return dh }))
	req.NoError(c.Provide(func(dh *DelegateHost) host.Executor { return dh }))
	req.NoError(c.Provide(func(dh *DelegateHost) host.FileExecutor { return dh }))
	req.NoError(c.Provide(func(dh *DelegateHost) host.NetworkHost { return dh }))
	req.NoError(c.Provide(func(dh *DelegateHost) host.UserHost { return dh }))

	req.NoError(c.Provide(func() *bytes.Buffer { return &bytes.Buffer{} }))
	req.NoError(c.Provide(func(b *bytes.Buffer) types.CmdOut { return b }))

	req.NoError(c.Provide(func(o types.CmdOut) *slog.Logger { return NewLogger(o) }))
	req.NoError(c.Provide(t.Context))

	// Cleanup hosts modification
	t.Cleanup(func() {
		req.NoError(c.Invoke(func(dh *DelegateHost) error {
			if networkHost, ok := dh.Net.(*DummyNetworkHost); ok {
				return networkHost.Cleanup()
			}
			return nil
		}))
	})
	return c
}

type Injector interface {
	Invoke(function any, options ...dig.InvokeOption) error
}

func Resolve[T any](t *testing.T, s Injector) T {
	t.Helper()
	var result T
	err := s.Invoke(func(value T) {
		result = value
	})
	require.NoError(t, err)
	return result
}
