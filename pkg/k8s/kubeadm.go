package k8s

import (
	"fmt"
	"net"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

func RunKubeadm(parameters []string) (err error) {
	log.Info("Running", "/usr/bin/kubeadm ", strings.Join(parameters, " "), "...")
	if out, err := exec.Command("/usr/bin/kubeadm", parameters...).CombinedOutput(); err != nil {
		return errors.Wrap(err, string(out))
	} else {
		log.Trace(string(out))
	}
	return
}

func RunKubeadmInit(ip net.IP) error {
	parameters := []string{
		"init",
		fmt.Sprintf("--apiserver-advertise-address=%v", ip),
		"--kubernetes-version=1.22.4",
		"--pod-network-cidr=10.244.0.0/16",
		"--control-plane-endpoint=kaweezle.local",
		"--ignore-preflight-errors=DirAvailable--var-lib-etcd,Swap",
		"--skip-phases=mark-control-plane",
	}
	return RunKubeadm(parameters)
}
