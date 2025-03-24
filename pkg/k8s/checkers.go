package k8s

// cSpell: words apiclient termenv
import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/kaweezle/iknite/pkg/alpine"
	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/muesli/termenv"
	"github.com/pkg/errors"
	kubeadmConstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"k8s.io/kubernetes/cmd/kubeadm/app/util/apiclient"
	kubeConfigUtil "k8s.io/kubernetes/cmd/kubeadm/app/util/kubeconfig"
)

// SystemFileCheck checks if a file exists and has specific content
func SystemFileCheck(name, description, path, expectedContent string) *Check {
	return &Check{
		Name:        name,
		Description: description,
		CheckFn: func(ctx context.Context, data CheckData) (bool, string, error) {
			content, err := os.ReadFile(path)
			if err != nil {
				return false, "", fmt.Errorf("failed to read %s: %v", path, err)
			}
			contentMessage := ""
			if expectedContent != "" {
				if strings.TrimSpace(string(content)) != strings.TrimSpace(expectedContent) {
					return false, "", fmt.Errorf("unexpected content in %s", path)
				} else {
					contentMessage = " with expected content"
				}
			}
			return true, fmt.Sprintf("%s exists%s", path, contentMessage), nil
		},
	}
}

func CheckService(serviceName string, checkOpenRC, checkPidFile bool) (bool, string, error) {
	pid := 0
	if checkOpenRC {
		started, err := alpine.IsServiceStarted(serviceName)
		if err != nil {
			return false, "", err
		}
		if !started {
			return false, "", fmt.Errorf("service %s is not running", serviceName)
		}
	}
	if checkPidFile {
		var err error
		pid, err = CheckPidFile(serviceName, nil)
		if err != nil {
			return false, "", err
		}
		if pid == 0 {
			return false, "", fmt.Errorf("service %s is not running", serviceName)
		}
	}
	return true, fmt.Sprintf("Service %s is running with pid %d", serviceName, pid), nil
}

// ServiceCheck checks if a service is running
func ServiceCheck(name, serviceName string) *Check {
	return &Check{
		Name:        name,
		DependsOn:   []string{"openrc"},
		Description: fmt.Sprintf("Check if %s service is running", serviceName),
		CheckFn: func(ctx context.Context, data CheckData) (bool, string, error) {
			return CheckService(serviceName, true, true)
		},
	}
}

// KubernetesFileCheck checks if kubernetes configuration files exist
func KubernetesFileCheck(name, path string) *Check {
	return &Check{
		Name:        name,
		Description: fmt.Sprintf("Check %s", path),
		DependsOn:   []string{},
		CheckFn: func(ctx context.Context, data CheckData) (bool, string, error) {
			// Check if file at path exists
			info, err := os.Stat(path)
			if os.IsNotExist(err) {
				return false, "", fmt.Errorf("%s does not exist", path)
			} else if err != nil {
				return false, "", fmt.Errorf("error checking %s: %v", path, err)
			}

			// Check if path is a file and not a directory
			if info.IsDir() {
				return false, "", fmt.Errorf("%s is a directory, not a file", path)
			}

			return true, fmt.Sprintf("%s exists and is a file", path), nil
		},
	}
}

// difference returns the elements in `a` that are not in `b`.
func difference(a, b []string) []string {
	m := make(map[string]bool)
	for _, item := range b {
		m[item] = true
	}
	var diff []string
	for _, item := range a {
		if !m[item] {
			diff = append(diff, item)
		}
	}
	return diff
}

// FileTreeDifference computes the difference between an actual path and an expected file tree
func FileTreeDifference(path string, expectedFiles []string) ([]string, []string, error) {
	foundFiles := []string{}
	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relativePath, err := filepath.Rel(path, filePath)
			if err != nil {
				return err
			}
			foundFiles = append(foundFiles, relativePath)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	missingFiles := difference(expectedFiles, foundFiles)
	extraFiles := difference(foundFiles, expectedFiles)
	return missingFiles, extraFiles, nil
}

// FileTreeCheck checks if a file tree exists
func FileTreeCheck(name, description, path string, expectedFiles []string) *Check {
	return &Check{
		Name:        name,
		Description: description,
		CheckFn: func(ctx context.Context, data CheckData) (bool, string, error) {
			missingFiles, extraFiles, err := FileTreeDifference(path, expectedFiles)
			if err != nil {
				return false, "", err
			}
			if len(missingFiles) > 0 {
				return false, fmt.Sprintf("Missing files: %s", strings.Join(missingFiles, ", ")), nil
			}
			if len(extraFiles) > 0 {
				return false, fmt.Sprintf("Extra files: %s", strings.Join(extraFiles, ", ")), nil
			}
			return true, fmt.Sprintf("All expected %d files found in %s", len(expectedFiles), path), nil
		},
	}
}

func CheckKubeletHealth(timeout time.Duration) (bool, string, error) {

	client, err := kubeConfigUtil.ClientSetFromFile(kubeadmConstants.GetAdminKubeConfigPath())
	if err != nil {
		return false, "", err
	}

	waiter := apiclient.NewKubeWaiter(client, timeout, nil)
	err = waiter.WaitForKubelet("127.0.0.1", 10248)
	if err != nil {
		return false, "", err
	}
	return true, "Kubelet is healthy", nil

}

func CheckApiServerHealth(timeout time.Duration) (bool, string, error) {

	client, err := kubeConfigUtil.ClientSetFromFile(kubeadmConstants.GetAdminKubeConfigPath())
	if err != nil {
		return false, "", err
	}

	waiter := apiclient.NewKubeWaiter(client, timeout, nil)
	err = waiter.WaitForAPI()
	if err != nil {
		return false, "", err
	}
	return true, "API Server is healthy", nil
}

type CheckWorkloadData interface {
	IsOk() bool
	WorkloadCount() int
	ReadyWorkloads() []*v1alpha1.WorkloadState
	NotReadyWorkloads() []*v1alpha1.WorkloadState
	Iteration() int
	SetOk(bool)
	SetWorkloadCount(int)
	SetReadyWorkloads([]*v1alpha1.WorkloadState)
	SetNotReadyWorkloads([]*v1alpha1.WorkloadState)
	SetIteration(int)
}

type checkWorkloadData struct {
	ok                bool
	workloadCount     int
	readyWorkloads    []*v1alpha1.WorkloadState
	notReadyWorkloads []*v1alpha1.WorkloadState
	iteration         int
}

func (c *checkWorkloadData) IsOk() bool {
	return c.ok
}

func (c *checkWorkloadData) WorkloadCount() int {
	return c.workloadCount
}

func (c *checkWorkloadData) ReadyWorkloads() []*v1alpha1.WorkloadState {
	return c.readyWorkloads
}

func (c *checkWorkloadData) NotReadyWorkloads() []*v1alpha1.WorkloadState {
	return c.notReadyWorkloads
}

func (c *checkWorkloadData) Iteration() int {
	return c.iteration
}

func (c *checkWorkloadData) SetOk(ok bool) {
	c.ok = ok
}

func (c *checkWorkloadData) SetWorkloadCount(count int) {
	c.workloadCount = count
}

func (c *checkWorkloadData) SetReadyWorkloads(ready []*v1alpha1.WorkloadState) {
	c.readyWorkloads = ready
}

func (c *checkWorkloadData) SetNotReadyWorkloads(unready []*v1alpha1.WorkloadState) {
	c.notReadyWorkloads = unready
}

func (c *checkWorkloadData) SetIteration(iteration int) {
	c.iteration = iteration
}

func CreateCheckWorkloadData() (CheckData, error) {
	return &checkWorkloadData{}, nil
}

func PrettyPrintWorkloadState(prefix string, r *v1alpha1.WorkloadState, output *termenv.Output) {
	p := output.Profile
	var statusColor termenv.Style
	if r.Ok {
		statusColor = output.String("✓").Foreground(p.Color("46")) // Green
	} else {
		statusColor = output.String("✗").Foreground(p.Color("196")) // Red
	}

	output.WriteString(fmt.Sprintf("%s%s %-20s %-54s %s\n", prefix, statusColor, r.Namespace, r.Name, r.Message))
}

func CheckWorkloadResultPrinter(result *CheckResult, prefix string, output *termenv.Output) {
	data := result.CheckData.(CheckWorkloadData)
	p := output.Profile

	ready := data.ReadyWorkloads()
	unready := data.NotReadyWorkloads()
	count := data.WorkloadCount()

	prettyCount := output.String(strconv.Itoa(count)).Foreground(p.Color("33")).Bold()           // Blue
	prettyReady := output.String(strconv.Itoa(len(ready))).Foreground(p.Color("46")).Bold()      // Green
	prettyUnready := output.String(strconv.Itoa(len(unready))).Foreground(p.Color("227")).Bold() // Yellow

	result.Message = ""
	if len(ready) > 0 {
		result.Message += fmt.Sprintf("%s / %s workloads ready", prettyReady, prettyCount)
	}
	if len(unready) > 0 {
		if len(ready) > 0 {
			result.Message += ", "
		}
		result.Message += fmt.Sprintf("%s / %s workloads unready", prettyUnready, prettyCount)
	}

	// Print the global result
	result.PrettyPrint(prefix, output)
	if result.Status == StatusSkipped {
		return
	}
	prefix = prefix + "  "

	if len(ready) > 0 {
		for _, state := range ready {
			PrettyPrintWorkloadState(prefix, state, output)
		}
	}

	if len(unready) > 0 {
		for _, state := range unready {
			PrettyPrintWorkloadState(prefix, state, output)
		}
	}
}

func CheckWorkloads(ctx context.Context, data CheckData) (bool, string, error) {
	workloadData := data.(CheckWorkloadData)
	config, err := LoadFromFile(constants.KubernetesRootConfig)
	if err != nil {
		return false, "", errors.Wrap(err, "While loading local cluster configuration")
	}

	err = config.WaitForWorkloads(context.Background(), 0, func(state bool, total int, ready, unready []*v1alpha1.WorkloadState, iteration int) bool {
		workloadData.SetOk(state)
		workloadData.SetWorkloadCount(total)
		workloadData.SetReadyWorkloads(ready)
		workloadData.SetNotReadyWorkloads(unready)
		workloadData.SetIteration(iteration)
		if iteration > 5 {
			return state
		}
		return false
	})
	if err != nil {
		return false, "", errors.Wrap(err, "While waiting for workloads")
	}
	return workloadData.IsOk(), "", nil
}
