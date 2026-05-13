// cSpell: words errgroup wrapcheck
package init

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	kubeadmApi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"

	mockHost "github.com/kaweezle/iknite/mocks/pkg/host"
	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/utils"
)

//nolint:containedctx // context is provided by the workflow.RunData
type proxyAPIPhaseData struct {
	host        host.Host
	ctx         context.Context
	cluster     *v1alpha1.IkniteCluster
	cfg         *kubeadmApi.InitConfiguration
	errGroup    *errgroup.Group
	hookManager utils.HookManager
}

func (d *proxyAPIPhaseData) IkniteCluster() *v1alpha1.IkniteCluster {
	return d.cluster
}

func (d *proxyAPIPhaseData) Host() host.Host {
	return d.host
}

func (d *proxyAPIPhaseData) Cfg() *kubeadmApi.InitConfiguration {
	return d.cfg
}

func (d *proxyAPIPhaseData) Context() context.Context {
	return d.ctx
}

func (d *proxyAPIPhaseData) ErrGroup() *errgroup.Group {
	return d.errGroup
}

func (d *proxyAPIPhaseData) RegisterShutdownHook(name string, fn func() error) {
	d.hookManager.Register(name, fn)
}

func (d *proxyAPIPhaseData) RunShutdownHooks() error {
	if err := d.hookManager.Run(); err != nil {
		return fmt.Errorf("run shutdown hooks: %w", err)
	}
	return nil
}

type stubListener struct {
	addr      net.Addr
	acceptCh  chan net.Conn
	acceptErr error
	closeErr  error
	closed    chan struct{}
}

func newStubListener(addr net.Addr) *stubListener {
	return &stubListener{
		addr:     addr,
		acceptCh: make(chan net.Conn),
		closed:   make(chan struct{}),
	}
}

func (l *stubListener) Accept() (net.Conn, error) {
	if l.acceptErr != nil {
		return nil, l.acceptErr
	}
	select {
	case conn := <-l.acceptCh:
		return conn, nil
	case <-l.closed:
		return nil, net.ErrClosed
	}
}

func (l *stubListener) Close() error {
	select {
	case <-l.closed:
	default:
		close(l.closed)
	}
	return l.closeErr
}

func (l *stubListener) Addr() net.Addr {
	return l.addr
}

func TestProxyAPIPorts_DeduplicatesPositivePorts(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	req.Equal([]int{11443}, proxyAPIPorts(11443, 11443))
	req.Equal([]int{6443}, proxyAPIPorts(0, 6443))
	req.Equal([]int{11443, 6443}, proxyAPIPorts(11443, 6443))
}

func TestRunProxyAPI_SkipsWhenOutboundMatchesClusterIP(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockH := mockHost.NewMockHost(t)
	mockH.EXPECT().GetOutboundIP().Return(net.ParseIP("127.0.0.1"), nil).Once()

	data := &proxyAPIPhaseData{
		cluster: &v1alpha1.IkniteCluster{
			Spec: v1alpha1.IkniteClusterSpec{
				Ip:               net.ParseIP("127.0.0.1"),
				StatusServerPort: 11443,
			},
		},
		host:     mockH,
		cfg:      &kubeadmApi.InitConfiguration{LocalAPIEndpoint: kubeadmApi.APIEndpoint{BindPort: 6443}},
		errGroup: &errgroup.Group{},
	}

	req.NoError(runProxyAPI(data))
	req.NoError(data.RunShutdownHooks())
	req.NoError(data.ErrGroup().Wait())
}

func TestRunProxyAPI_FailsWhenOutboundIPLookupFails(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockH := mockHost.NewMockHost(t)
	mockH.EXPECT().GetOutboundIP().Return(nil, errors.New("no route")).Once()

	data := &proxyAPIPhaseData{
		cluster:  &v1alpha1.IkniteCluster{Spec: v1alpha1.IkniteClusterSpec{Ip: net.ParseIP("127.0.0.2")}},
		host:     mockH,
		cfg:      &kubeadmApi.InitConfiguration{LocalAPIEndpoint: kubeadmApi.APIEndpoint{BindPort: 6443}},
		errGroup: &errgroup.Group{},
	}

	err := runProxyAPI(data)
	req.ErrorContains(err, "failed to get outbound IP")
}

func TestRunProxyAPI_FailsWhenListenerCannotStart(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockH := mockHost.NewMockHost(t)
	mockH.EXPECT().GetOutboundIP().Return(net.ParseIP("127.0.0.1"), nil).Once()
	mockH.EXPECT().Listen(mock.Anything, "tcp", mock.Anything).Return(nil, fmt.Errorf("boom on %s", "tcp")).Once()

	data := &proxyAPIPhaseData{
		cluster: &v1alpha1.IkniteCluster{
			Spec: v1alpha1.IkniteClusterSpec{Ip: net.ParseIP("127.0.0.2"), StatusServerPort: 11443},
		},
		host:     mockH,
		cfg:      &kubeadmApi.InitConfiguration{LocalAPIEndpoint: kubeadmApi.APIEndpoint{BindPort: 6443}},
		errGroup: &errgroup.Group{},
		ctx:      t.Context(),
	}

	err := runProxyAPI(data)
	req.ErrorContains(err, "failed to start proxy on port 11443")
}

func TestServeProxyAPIListener_ReturnsAcceptError(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	listener := newStubListener(&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345})
	listener.acceptErr = errors.New("accept failed")
	nh := host.NewDefaultNetworkHost()

	err := serveProxyAPIListener(t.Context(), nh, listener, "127.0.0.2:12345")
	req.ErrorContains(err, "accept on 127.0.0.1:12345")
}

func TestCloseProxyAPIListeners_CollectsErrors(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	closedListener := newStubListener(&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 1})
	req.NoError(closedListener.Close())

	failingListener := newStubListener(&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 2})
	failingListener.closeErr = errors.New("close failed")

	err := closeProxyAPIListeners([]net.Listener{closedListener, failingListener})
	req.ErrorContains(err, "close failed")
}

func TestProxyAPIConnection_DialFailureClosesClient(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockH := mockHost.NewMockHost(t)
	mockH.EXPECT().DialTimeout(mock.Anything, "tcp", "127.0.0.2:6443", proxyAPIDialTimeout).
		Return(nil, errors.New("dial failed")).Once()

	clientConn, peerConn := net.Pipe()
	defer func() {
		req.NoError(peerConn.Close())
	}()

	done := make(chan struct{})
	go func() {
		proxyAPIConnection(t.Context(), mockH, clientConn, "127.0.0.2:6443")
		close(done)
	}()

	req.NoError(peerConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond)))
	buf := make([]byte, 1)
	_, err := peerConn.Read(buf)
	req.Error(err)
	req.True(errors.Is(err, net.ErrClosed) || err.Error() == "EOF")
	<-done
}

func TestRunProxyAPI_ForwardsTrafficAndStops(t *testing.T) {
	t.Parallel()

	req := require.New(t)

	statusBackend, statusPort := startProxyAPITestBackend(t, "127.0.0.2", 0)
	defer func() {
		req.NoError(statusBackend.Close())
	}()
	const apiPort int32 = 34443
	apiBackend, _ := startProxyAPITestBackend(t, "127.0.0.2", int(apiPort))
	defer func() {
		req.NoError(apiBackend.Close())
	}()

	mockH := mockHost.NewMockHost(t)
	mockH.EXPECT().GetOutboundIP().Return(net.ParseIP("127.0.0.1"), nil).Once()
	mockH.EXPECT().Listen(mock.Anything, "tcp", mock.Anything).
		RunAndReturn(func(ctx context.Context, network, address string) (net.Listener, error) {
			lc := &net.ListenConfig{}
			return lc.Listen(ctx, network, address)
		}).Twice()
	mockH.EXPECT().DialTimeout(mock.Anything, "tcp", mock.Anything, proxyAPIDialTimeout).
		RunAndReturn(func(ctx context.Context, network, address string, timeout time.Duration) (net.Conn, error) {
			dialer := &net.Dialer{Timeout: timeout}
			return dialer.DialContext(ctx, network, address)
		}).Twice()

	data := &proxyAPIPhaseData{
		cluster: &v1alpha1.IkniteCluster{
			Spec: v1alpha1.IkniteClusterSpec{
				Ip:               net.ParseIP("127.0.0.2"),
				StatusServerPort: statusPort,
			},
		},
		host: mockH,
		cfg: &kubeadmApi.InitConfiguration{
			LocalAPIEndpoint: kubeadmApi.APIEndpoint{BindPort: apiPort},
		},
		errGroup: &errgroup.Group{},
		ctx:      t.Context(),
	}

	req.NoError(runProxyAPI(data))
	req.Equal("echo:status", proxyAPITestRoundTrip(t, net.JoinHostPort("127.0.0.1", fmt.Sprint(statusPort)), "status"))
	req.Equal("echo:api", proxyAPITestRoundTrip(t, net.JoinHostPort("127.0.0.1", fmt.Sprint(apiPort)), "api"))
	req.NoError(data.RunShutdownHooks())
	req.NoError(data.ErrGroup().Wait())

	_, err := (&net.Dialer{Timeout: 200 * time.Millisecond}).DialContext(
		context.Background(),
		"tcp",
		net.JoinHostPort("127.0.0.1", fmt.Sprint(statusPort)),
	)
	req.Error(err)
	_, err = (&net.Dialer{Timeout: 200 * time.Millisecond}).DialContext(
		context.Background(),
		"tcp",
		net.JoinHostPort("127.0.0.1", fmt.Sprint(apiPort)),
	)
	req.Error(err)
}

func startProxyAPITestBackend(t *testing.T, ip string, port int) (net.Listener, int) {
	t.Helper()

	listener, err := (&net.ListenConfig{}).Listen(
		context.Background(),
		"tcp",
		net.JoinHostPort(ip, strconv.Itoa(port)),
	)
	require.NoError(t, err)

	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				if errors.Is(acceptErr, net.ErrClosed) {
					return
				}
				return
			}

			go func(conn net.Conn) {
				defer func() {
					if err := conn.Close(); err != nil {
						return
					}
				}()
				buf := make([]byte, 32)
				n, readErr := conn.Read(buf)
				if readErr != nil {
					return
				}
				_, writeErr := conn.Write([]byte("echo:" + string(buf[:n])))
				if writeErr != nil {
					return
				}
			}(conn)
		}
	}()

	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	require.True(t, ok)

	return listener, tcpAddr.Port
}

func proxyAPITestRoundTrip(t *testing.T, address, payload string) string {
	t.Helper()

	conn, err := (&net.Dialer{Timeout: time.Second}).DialContext(context.Background(), "tcp", address)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, conn.Close())
	}()

	require.NoError(t, conn.SetDeadline(time.Now().Add(time.Second)))
	_, err = conn.Write([]byte(payload))
	require.NoError(t, err)

	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	require.NoError(t, err)

	return string(buf[:n])
}
