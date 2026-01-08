/*
Copyright 2019 The Kubernetes Authors.

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

package reset

// cSpell:words klog cleanupconfig
// cSpell:disable
import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"k8s.io/klog/v2"
	kubeadmApiV1 "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta3"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/options"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/workflow"
	kubeadmConstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"k8s.io/kubernetes/cmd/kubeadm/app/features"
	"k8s.io/kubernetes/cmd/kubeadm/app/util/users"
)

// cSpell:enable

// NewCleanupNodePhase creates a kubeadm workflow phase that cleanup the node.
func NewCleanupConfigPhase() workflow.Phase {
	return workflow.Phase{
		Name:    "cleanup-config",
		Aliases: []string{"cleanupconfig"},
		Short:   "Run cleanup config.",
		Run:     runCleanupConfig,
		InheritFlags: []string{
			options.CertificatesDir,
			options.NodeCRISocket,
			options.CleanupTmpDir,
			options.DryRun,
		},
	}
}

func runCleanupConfig(c workflow.RunData) error {
	dirsToClean := []string{
		filepath.Join(kubeadmConstants.KubernetesDir, kubeadmConstants.ManifestsSubDirName),
	}
	r, ok := c.(IkniteResetData)
	if !ok {
		return errors.New("cleanup-config phase invoked with an invalid data struct")
	}

	if !r.DryRun() {
		// In case KubeletRunDirectory holds a symbolic link, evaluate it.
		// This would also throw an error if the directory does not exist.
		kubeletRunDirectory, err := filepath.EvalSymlinks(kubeadmConstants.KubeletRunDirectory)
		if err != nil {
			klog.Warningf("[reset] Skipping cleaning of directory %q: %v\n",
				kubeadmConstants.KubeletRunDirectory, err)
		} else {
			dirsToClean = append(dirsToClean, kubeletRunDirectory)
		}
	} else {
		log.Infof("[reset] Would clean directory %q\n", kubeadmConstants.KubeletRunDirectory)
	}

	certsDir := r.CertificatesDir()

	// Remove contents from the config and pki directories
	if certsDir != kubeadmApiV1.DefaultCertificatesDir {
		klog.Warningf(
			"[reset] WARNING: Cleaning a non-default certificates directory: %q\n",
			certsDir,
		)
	}

	dirsToClean = append(dirsToClean, certsDir)
	if r.CleanupTmpDir() {
		tempDir := path.Join(kubeadmConstants.KubernetesDir, kubeadmConstants.TempDir)
		dirsToClean = append(dirsToClean, tempDir)
	}
	resetConfigDir(kubeadmConstants.KubernetesDir, dirsToClean, r.DryRun())

	if r.Cfg() != nil && features.Enabled(r.Cfg().FeatureGates, features.RootlessControlPlane) {
		if !r.DryRun() {
			klog.Infoln("[reset] Removing users and groups created for rootless control-plane")
			if err := users.RemoveUsersAndGroups(); err != nil {
				klog.Warningf("[reset] Failed to remove users and groups: %v\n", err)
			}
		} else {
			klog.Infoln("[reset] Would remove users and groups created for rootless control-plane")
		}
	}

	return nil
}

func CleanConfig(isDryRun bool) {
	dirsToClean := []string{
		filepath.Join(kubeadmConstants.KubernetesDir, kubeadmConstants.ManifestsSubDirName),
		kubeadmConstants.KubeletRunDirectory,
		kubeadmApiV1.DefaultCertificatesDir,
	}

	resetConfigDir(kubeadmConstants.KubernetesDir, dirsToClean, isDryRun)
}

func resetConfigDir(configPathDir string, dirsToClean []string, isDryRun bool) {
	if !isDryRun {
		log.Infof("[reset] Deleting contents of directories: %v\n", dirsToClean)
		for _, dir := range dirsToClean {
			if err := CleanDir(dir); err != nil {
				klog.Warningf("[reset] Failed to delete contents of %q directory: %v", dir, err)
			}
		}
	} else {
		log.Infof("[reset] Would delete contents of directories: %v\n", dirsToClean)
	}

	filesToClean := []string{
		filepath.Join(configPathDir, kubeadmConstants.AdminKubeConfigFileName),
		filepath.Join(configPathDir, kubeadmConstants.SuperAdminKubeConfigFileName),
		filepath.Join(configPathDir, kubeadmConstants.KubeletKubeConfigFileName),
		filepath.Join(configPathDir, kubeadmConstants.KubeletBootstrapKubeConfigFileName),
		filepath.Join(configPathDir, kubeadmConstants.ControllerManagerKubeConfigFileName),
		filepath.Join(configPathDir, kubeadmConstants.SchedulerKubeConfigFileName),
	}

	if !isDryRun {
		log.Infof("[reset] Deleting files: %v\n", filesToClean)
		for _, path := range filesToClean {
			if err := os.RemoveAll(path); err != nil {
				klog.Warningf("[reset] Failed to remove file: %q [%v]\n", path, err)
			}
		}
	} else {
		log.Infof("[reset] Would delete files: %v\n", filesToClean)
	}
}

// CleanDir removes everything in a directory, but not the directory itself.
func CleanDir(filePath string) error {
	// If the directory doesn't even exist there's nothing to do, and we do
	// not consider this an error
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil
	}

	d, err := os.Open(filePath) //nolint:gosec // Controlled file path
	if err != nil {
		return fmt.Errorf("failed to open directory for cleanup: %w", err)
	}
	defer func() {
		err = d.Close()
	}()
	names, err := d.Readdirnames(-1)
	if err != nil {
		return fmt.Errorf("failed to read directory names: %w", err)
	}
	for _, name := range names {
		if err = os.RemoveAll(filepath.Join(filePath, name)); err != nil {
			return fmt.Errorf("failed to remove %s: %w", name, err)
		}
	}
	return nil
}

// IsDirEmpty returns true if a directory is empty.
func IsDirEmpty(dir string) (bool, error) {
	d, err := os.Open(dir) //nolint:gosec // Just checking directory contents
	if err != nil {
		return false, fmt.Errorf("failed to open directory: %w", err)
	}
	defer func() {
		err = d.Close()
	}()
	_, err = d.Readdirnames(1)
	if errors.Is(err, io.EOF) {
		return true, nil
	}
	return false, nil
}
