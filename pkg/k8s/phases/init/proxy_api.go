// cSpell: words errgroup sirupsen
package init

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	kubeadmApi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/workflow"

	"github.com/kaweezle/iknite/pkg/host"
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

	clusterIP := ikniteConfig.Ip
	if clusterIP == nil {
		return nil
	}

	outboundIP, err := alpineHost.GetOutboundIP()
	if err != nil {
		return fmt.Errorf("failed to get outbound IP: %w", err)
	}
	if outboundIP == nil || outboundIP.Equal(clusterIP) {
		log.WithField("outboundIP", outboundIP).WithField("clusterIP", clusterIP).Info("No API proxy needed")
		return nil
	}

	ports := proxyAPIPorts(ikniteConfig.StatusServerPort, int(cfg.LocalAPIEndpoint.BindPort))
	listeners := make([]net.Listener, 0, len(ports))
	errGroup := data.ErrGroup()
	for _, port := range ports {
		listener, listenErr := startProxyAPIListener(ctx, alpineHost, errGroup, outboundIP, clusterIP, port)
		if listenErr != nil {
			if closeErr := closeProxyAPIListeners(listeners); closeErr != nil {
				log.WithError(closeErr).Warn("Failed to clean up proxy listeners")
			}
			return fmt.Errorf("failed to start proxy on port %d: %w", port, listenErr)
		}
		listeners = append(listeners, listener)
	}

	log.WithFields(log.Fields{
		"clusterIP":  clusterIP.String(),
		"outboundIP": outboundIP.String(),
		"ports":      ports,
	}).Info("API proxy started")

	data.RegisterShutdownHook(proxyAPIPhaseName, func() error {
		return closeProxyAPIListeners(listeners)
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
) (net.Listener, error) {
	listenAddr := net.JoinHostPort(outboundIP.String(), strconv.Itoa(port))
	targetAddr := net.JoinHostPort(clusterIP.String(), strconv.Itoa(port))

	listener, err := networkHost.Listen(ctx, "tcp", listenAddr)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", listenAddr, err)
	}

	errGroup.Go(func() error {
		return serveProxyAPIListener(ctx, networkHost, listener, targetAddr)
	})

	return listener, nil
}

func serveProxyAPIListener(
	ctx context.Context,
	networkHost host.NetworkHost,
	listener net.Listener,
	targetAddr string,
) error {
	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return fmt.Errorf("accept on %s: %w", listener.Addr().String(), err)
		}

		go proxyAPIConnection(ctx, networkHost, conn, targetAddr)
	}
}

func proxyAPIConnection(ctx context.Context, networkHost host.NetworkHost, clientConn net.Conn, targetAddr string) {
	defer func() {
		closeProxyAPIConn(clientConn, "client")
	}()

	targetConn, err := networkHost.DialTimeout(ctx, "tcp", targetAddr, proxyAPIDialTimeout)
	if err != nil {
		log.WithError(err).WithField("target", targetAddr).Warn("Proxy dial failed")
		return
	}
	defer func() {
		closeProxyAPIConn(targetConn, "target")
	}()

	proxyAPICopy(clientConn, targetConn)
}

func proxyAPICopy(left, right net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		copyProxyStream(left, right)
	}()
	go func() {
		defer wg.Done()
		copyProxyStream(right, left)
	}()
	wg.Wait()
}

func copyProxyStream(dst, src net.Conn) {
	if _, err := io.Copy(dst, src); err != nil && !errors.Is(err, net.ErrClosed) {
		log.WithError(err).Debug("Proxy stream stopped")
	}
	if closeWriter, ok := dst.(interface{ CloseWrite() error }); ok {
		if err := closeWriter.CloseWrite(); err != nil && !errors.Is(err, net.ErrClosed) {
			log.WithError(err).Debug("Failed to close proxy write side")
		}
	}
}

func closeProxyAPIConn(conn net.Conn, side string) {
	if err := conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		log.WithError(err).WithField("side", side).Debug("Failed to close proxy connection")
	}
}

func closeProxyAPIListeners(listeners []net.Listener) error {
	var errs []error
	for _, listener := range listeners {
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
