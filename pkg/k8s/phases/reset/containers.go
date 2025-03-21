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

// cSpell:words klog cleanupnode
// cSpell:disable
import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"

	"k8s.io/klog/v2"
	utilsExec "k8s.io/utils/exec"

	utilErrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/options"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/workflow"
	kubeadmConstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	initSystem "k8s.io/kubernetes/cmd/kubeadm/app/util/initsystem"
	utilRuntime "k8s.io/kubernetes/cmd/kubeadm/app/util/runtime"

	kubeadmApi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
)

// cSpell:enable

// NewCleanupNodePhase creates a kubeadm workflow phase that cleanup the node
func NewCleanupNodePhase() workflow.Phase {
	return workflow.Phase{
		Name:    "cleanup-node",
		Aliases: []string{"cleanupnode"},
		Short:   "Run cleanup node.",
		Run:     runCleanupNode,
		InheritFlags: []string{
			options.CertificatesDir,
			options.NodeCRISocket,
			options.CleanupTmpDir,
			options.DryRun,
		},
	}
}

func runCleanupNode(c workflow.RunData) error {
	r, ok := c.(IkniteResetData)
	if !ok {
		return errors.New("cleanup-node phase invoked with an invalid data struct")
	}

	// Try to stop the kubelet service
	klog.V(1).Infoln("[reset] Getting init system")
	initSystem, err := initSystem.GetInitSystem()
	if err != nil {
		klog.Warningln("[reset] The iknite service could not be stopped by kubeadm. Unable to detect a supported init system!")
		klog.Warningln("[reset] Please ensure iknite is stopped manually")
	} else {
		if !r.DryRun() {
			fmt.Println("[reset] Stopping the iknite service")
			if err := initSystem.ServiceStop("iknite"); err != nil {
				klog.Warningf("[reset] The iknite service could not be stopped by kubeadm: [%v]\n", err)
				klog.Warningln("[reset] Please ensure iknite is stopped manually")
			}
		} else {
			fmt.Println("[reset] Would stop the iknite service")
		}
	}

	if !r.DryRun() {
		// In case KubeletRunDirectory holds a symbolic link, evaluate it.
		// This would also throw an error if the directory does not exist.
		kubeletRunDirectory, err := filepath.EvalSymlinks(kubeadmConstants.KubeletRunDirectory)
		if err != nil {
			klog.Warningf("[reset] Skipping unmount of directories in %q: %v\n",
				kubeadmConstants.KubeletRunDirectory, err)
		} else {
			// Unmount all mount paths under kubeletRunDirectory.
			fmt.Printf("[reset] Unmounting mounted directories in %q\n", kubeadmConstants.KubeletRunDirectory)
			if err := unmountKubeletDirectory(kubeletRunDirectory, r.ResetCfg().UnmountFlags); err != nil {
				return err
			}
		}
	} else {
		fmt.Printf("[reset] Would unmount mounted directories in %q\n", kubeadmConstants.KubeletRunDirectory)
	}

	if !r.DryRun() {
		klog.V(1).Info("[reset] Removing Kubernetes-managed containers")
		if err := removeContainers(utilsExec.New(), r.CRISocketPath()); err != nil {
			klog.Warningf("[reset] Failed to remove containers: %v\n", err)
		}
	} else {
		fmt.Println("[reset] Would remove Kubernetes-managed containers")
	}

	return nil
}

func removeContainers(execer utilsExec.Interface, criSocketPath string) error {
	containerRuntime, err := utilRuntime.NewContainerRuntime(execer, criSocketPath)
	if err != nil {
		return err
	}
	containers, err := containerRuntime.ListKubeContainers()
	if err != nil {
		return err
	}
	return containerRuntime.RemoveContainers(containers)
}

var flagMap = map[string]int{
	kubeadmApi.UnmountFlagMNTForce:       unix.MNT_FORCE,
	kubeadmApi.UnmountFlagMNTDetach:      unix.MNT_DETACH,
	kubeadmApi.UnmountFlagMNTExpire:      unix.MNT_EXPIRE,
	kubeadmApi.UnmountFlagUmountNoFollow: unix.UMOUNT_NOFOLLOW,
}

func flagsToInt(flags []string) int {
	res := 0
	for _, f := range flags {
		res |= flagMap[f]
	}
	return res
}

// unmountKubeletDirectory unmounts all paths that contain KubeletRunDirectory
func unmountKubeletDirectory(kubeletRunDirectory string, flags []string) error {
	raw, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return err
	}

	if !strings.HasSuffix(kubeletRunDirectory, "/") {
		// trailing "/" is needed to ensure that possibly mounted /var/lib/kubelet is skipped
		kubeletRunDirectory += "/"
	}

	var errList []error
	mounts := strings.Split(string(raw), "\n")
	flagsInt := flagsToInt(flags)
	for _, mount := range mounts {
		m := strings.Split(mount, " ")
		if len(m) < 2 || !strings.HasPrefix(m[1], kubeletRunDirectory) {
			continue
		}
		klog.V(5).Infof("[reset] Unmounting %q", m[1])
		if err := syscall.Unmount(m[1], flagsInt); err != nil {
			errList = append(errList, errors.WithMessagef(err, "failed to unmount %q", m[1]))
		}
	}
	return errors.Wrapf(utilErrors.NewAggregate(errList),
		"encountered the following errors while unmounting directories in %q", kubeletRunDirectory)
}
