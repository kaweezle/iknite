// cSpell: words paralleltest
package cmd

import (
	"bytes"
	"context"
	"net"
	"net/netip"
	"sync"
	"testing"

	"github.com/pion/mdns/v2"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/dns/dnsmessage"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
)

type fakeMdnsServer struct {
	source    netip.Addr
	queryName string
	answer    dnsmessage.ResourceHeader
	called    bool
}

func (s *fakeMdnsServer) QueryAddr(_ context.Context, name string) (dnsmessage.ResourceHeader, netip.Addr, error) {
	s.called = true
	s.queryName = name
	return s.answer, s.source, nil
}

func (s *fakeMdnsServer) Close() error {
	return nil
}

//nolint:paralleltest // overrides package-level test hook
func TestNewMdnsCmdTestSubcommand(t *testing.T) {
	req := require.New(t)

	spec := &v1alpha1.IkniteClusterSpec{DomainName: "cluster.iknite"}
	fakeServer := &fakeMdnsServer{
		answer: dnsmessage.ResourceHeader{Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET},
		source: netip.MustParseAddr("192.0.2.1"),
	}

	originalNewMdnsServerFn := newMdnsServerFn
	t.Cleanup(func() {
		newMdnsServerFn = originalNewMdnsServerFn
	})

	var factoryCalls int
	newMdnsServerFn = func(cfg *mdns.Config) (mdnsServer, net.Addr, net.Addr, error) {
		factoryCalls++
		req.NotNil(cfg)
		req.Empty(cfg.LocalNames)
		return fakeServer, nil, nil, nil
	}

	command := NewMdnsCmd(spec)
	testCommand, _, err := command.Find([]string{"test"})
	req.NoError(err)
	req.NotNil(testCommand)
	req.Equal("test", testCommand.Name())

	out := &bytes.Buffer{}
	command.SetOut(out)
	command.SetErr(out)
	testCommand.SetOut(out)
	testCommand.SetErr(out)
	command.SetArgs([]string{"test"})

	err = command.ExecuteContext(t.Context())
	req.NoError(err)
	req.Equal(1, factoryCalls)
	req.Equal(spec.DomainName, fakeServer.queryName)
	req.True(fakeServer.called)
	req.Contains(out.String(), "192.0.2.1")
}

func TestMdnsCmd(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	spec := &v1alpha1.IkniteClusterSpec{CreateIp: true, DomainName: "cluster.iknite"}
	v1alpha1.SetDefaults_IkniteClusterSpec(spec)

	command := NewMdnsCmd(spec)
	testCommand, _, err := command.Find([]string{"test"})
	req.NoError(err)
	req.NotNil(testCommand)
	req.Equal("test", testCommand.Name())

	dnsCommand := NewMdnsCmd(spec)
	var dnsErrMu sync.Mutex
	var dnsErr error
	dnsCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() {
		dnsErrMu.Lock()
		defer dnsErrMu.Unlock()
		dnsErr = dnsCommand.ExecuteContext(dnsCtx)
	}()

	out := &bytes.Buffer{}
	command.SetOut(out)
	command.SetErr(out)
	testCommand.SetOut(out)
	testCommand.SetErr(out)
	command.SetArgs([]string{"test"})

	err = command.ExecuteContext(t.Context())
	req.NoError(err)
	req.Contains(out.String(), "TypeA")
	cancel()
	dnsErrMu.Lock()
	defer dnsErrMu.Unlock()
	req.NoError(dnsErr)
}
