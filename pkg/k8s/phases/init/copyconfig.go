package init

// cSpell: disable
import (
	"fmt"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/workflow"

	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/k8s"
	"github.com/kaweezle/iknite/pkg/server"
)

// cSpell: enable

// NewCopyConfigPhase creates a phase that copies the admin.conf and iknite.conf
// files to /root/.kube/ so that kubectl and iknite CLI commands work without
// specifying explicit paths. It should run after the serve phase (which creates
// iknite.conf) and before the workloads phase.
func NewCopyConfigPhase() workflow.Phase {
	return workflow.Phase{
		Name:  "copy-config",
		Short: "Copy admin.conf and iknite.conf to /root/.kube/.",
		Run:   runCopyConfig,
	}
}

// runCopyConfig copies admin.conf (renamed to the cluster name) to
// /root/.kube/config and copies iknite.conf to /root/.kube/iknite.conf.
func runCopyConfig(c workflow.RunData) error {
	data, ok := c.(IkniteInitData)
	if !ok {
		return fmt.Errorf("copy-config phase invoked with an invalid data struct")
	}

	ikniteConfig := data.IkniteCluster().Spec

	// Copy admin.conf to /root/.kube/config (renamed to the cluster name).
	if err := copyAdminConf(data.Host(), ikniteConfig.ClusterName); err != nil {
		return fmt.Errorf("failed to copy admin.conf to %s: %w", constants.KubernetesRootConfig, err)
	}

	// Ensure the iknite configuration actually exists
	// TODO: This should be done where all the certificates are done.
	if err := server.EnsureIkniteServerConfiguration(constants.KubernetesPKIDir, &ikniteConfig); err != nil {
		return fmt.Errorf("failed to ensure iknite server configuration: %w", err)
	}

	// Copy iknite.conf to /root/.kube/iknite.conf.
	if err := copyFile(data.Host(), constants.IkniteConfPath, constants.IkniteLocalConfPath); err != nil {
		return fmt.Errorf("failed to copy iknite.conf to %s: %w", constants.IkniteLocalConfPath, err)
	}

	return nil
}

// copyAdminConf loads the admin kubeconfig from /etc/kubernetes/admin.conf,
// renames the cluster/context/user to clusterName, and writes the result to
// /root/.kube/config.
func copyAdminConf(fs host.FileSystem, clusterName string) error {
	k8sConfig, err := k8s.LoadFromDefault(fs)
	if err != nil {
		return fmt.Errorf("failed to load admin kubeconfig: %w", err)
	}

	k8sConfig = k8s.RenameConfig(k8sConfig, clusterName)
	if err := k8s.WriteToFile(k8sConfig, fs, constants.KubernetesRootConfig); err != nil {
		return fmt.Errorf("failed to write kubeconfig to %s: %w", constants.KubernetesRootConfig, err)
	}

	log.WithField("dest", constants.KubernetesRootConfig).Info("admin.conf copied")
	return nil
}

// copyFile copies the file at src to dst, creating parent directories as needed.
func copyFile(fs host.FileSystem, src, dst string) error {
	data, err := fs.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", src, err)
	}

	if err := fs.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", dst, err)
	}

	if err := fs.WriteFile(dst, data, 0o600); err != nil {
		return fmt.Errorf("failed to write %s: %w", dst, err)
	}

	log.WithFields(log.Fields{"src": src, "dest": dst}).Info("config file copied")
	return nil
}
