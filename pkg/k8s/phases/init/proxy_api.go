// cSpell: words errgroup
package init

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	kubeadmApi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/workflow"

	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/utils"
)

const (
	proxyAPIDialTimeout = 5 * time.Second
	proxyAPIPhaseName   = "proxy-api"
)

func NewProxyAPIPhase() workflow.Phase {
	return workflow.Phase{
		Name:  proxyAPIPhaseName,
		Short: "Proxy API and status traffic from outbound IP to iknite IP.",
		Run:   runProxyAPI,
	}
}

type proxyAPIData interface {
	IkniteClusterProvider
	host.HostProvider
	ErrGroupProvider
	ShutdownHookRegistrar
	Cfg() *kubeadmApi.InitConfiguration
	ContextProvider
	utils.LoggerProvider
}

func runProxyAPI(c workflow.RunData) error {
	data, ok := c.(proxyAPIData)
	if !ok {
		return fmt.Errorf("%s phase invoked with an invalid data struct", proxyAPIPhaseName)
	}

	ikniteConfig := data.IkniteCluster().Spec
	cfg := data.Cfg()
	alpineHost := data.Host()
	ctx := data.Context()
	logger := data.Logger()

	clusterIP := ikniteConfig.Ip
	if clusterIP == nil {
		return nil
	}

	outboundIP, err := alpineHost.GetOutboundIP()
	if err != nil {
		return fmt.Errorf("failed to get outbound IP: %w", err)
	}
	if outboundIP == nil || outboundIP.Equal(clusterIP) {
		logger.Info("No API proxy needed", "outboundIP", outboundIP, "clusterIP", clusterIP)
		return nil
	}

	ports := proxyAPIPorts(ikniteConfig.StatusServerPort, int(cfg.LocalAPIEndpoint.BindPort))
	listeners := make([]net.Listener, 0, len(ports))
	errGroup := data.ErrGroup()
	for _, port := range ports {
		listener, listenErr := startProxyAPIListener(ctx, alpineHost, errGroup, outboundIP, clusterIP, port, logger)
		if listenErr != nil {
			if closeErr := closeProxyAPIListeners(listeners, logger); closeErr != nil {
				logger.Warn("Failed to clean up proxy listeners", utils.ErrorKey, closeErr)
			}
			return fmt.Errorf("failed to start proxy on port %d: %w", port, listenErr)
		}
		listeners = append(listeners, listener)
	}

	logger.Info("API proxy started", "clusterIP", clusterIP.String(), "outboundIP", outboundIP.String(), "ports", ports)

	data.RegisterShutdownHook(proxyAPIPhaseName, func() error {
		return closeProxyAPIListeners(listeners, logger)
	})

	return nil
}

func proxyAPIPorts(sourcePorts ...int) []int {
	ports := make([]int, 0, 2)
	seen := make(map[int]struct{}, 2)
	for _, port := range sourcePorts {
		if port <= 0 {
			continue
		}
		if _, ok := seen[port]; ok {
			continue
		}
		seen[port] = struct{}{}
		ports = append(ports, port)
	}
	return ports
}

func startProxyAPIListener(
	ctx context.Context,
	networkHost host.NetworkHost,
	errGroup *errgroup.Group,
	outboundIP, clusterIP net.IP,
	port int,
	logger *slog.Logger,
) (net.Listener, error) {
	listenAddr := net.JoinHostPort(outboundIP.String(), strconv.Itoa(port))
	targetAddr := net.JoinHostPort(clusterIP.String(), strconv.Itoa(port))

	listener, err := networkHost.Listen(ctx, "tcp", listenAddr)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", listenAddr, err)
	}

	errGroup.Go(func() error {
		return serveProxyAPIListener(ctx, networkHost, listener, targetAddr, logger)
	})

	return listener, nil
}

func serveProxyAPIListener(
	ctx context.Context,
	networkHost host.NetworkHost,
	listener net.Listener,
	targetAddr string,
	logger *slog.Logger,
) error {
	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return fmt.Errorf("accept on %s: %w", listener.Addr().String(), err)
		}

		go proxyAPIConnection(ctx, networkHost, conn, targetAddr, logger)
	}
}

func proxyAPIConnection(
	ctx context.Context,
	networkHost host.NetworkHost,
	clientConn net.Conn,
	targetAddr string,
	logger *slog.Logger,
) {
	defer func() {
		closeProxyAPIConn(clientConn, "client", logger)
	}()

	targetConn, err := networkHost.DialTimeout(ctx, "tcp", targetAddr, proxyAPIDialTimeout)
	if err != nil {
		logger.Warn("Proxy dial failed", utils.ErrorKey, err, "target", targetAddr)
		return
	}
	defer func() {
		closeProxyAPIConn(targetConn, "target", logger)
	}()

	proxyAPICopy(clientConn, targetConn, logger)
}

func proxyAPICopy(left, right net.Conn, logger *slog.Logger) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		copyProxyStream(left, right, logger)
	}()
	go func() {
		defer wg.Done()
		copyProxyStream(right, left, logger)
	}()
	wg.Wait()
}

func copyProxyStream(dst, src net.Conn, logger *slog.Logger) {
	if _, err := io.Copy(dst, src); err != nil && !errors.Is(err, net.ErrClosed) {
		logger.Debug("Proxy stream stopped", utils.ErrorKey, err)
	}
	if closeWriter, ok := dst.(interface{ CloseWrite() error }); ok {
		if err := closeWriter.CloseWrite(); err != nil && !errors.Is(err, net.ErrClosed) {
			logger.Debug("Failed to close proxy write side", utils.ErrorKey, err)
		}
	}
}

func closeProxyAPIConn(conn net.Conn, side string, logger *slog.Logger) {
	if err := conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		logger.Debug("Failed to close proxy connection", utils.ErrorKey, err, "side", side)
	}
}

func closeProxyAPIListeners(listeners []net.Listener, logger *slog.Logger) error {
	var errs []error
	for _, listener := range listeners {
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			logger.Debug("Failed to close proxy listener", utils.ErrorKey, err, "listener", listener.Addr())
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
