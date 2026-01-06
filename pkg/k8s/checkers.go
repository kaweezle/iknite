package k8s

// cSpell: words apiclient lipgloss
// cSpell: disable
import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	kubeadmConstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"k8s.io/kubernetes/cmd/kubeadm/app/util/apiclient"
	kubeConfigUtil "k8s.io/kubernetes/cmd/kubeadm/app/util/kubeconfig"
	staticPodUtil "k8s.io/kubernetes/cmd/kubeadm/app/util/staticpod"

	"github.com/kaweezle/iknite/pkg/alpine"
	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
)

// cSpell: enable

// SystemFileCheck checks if a file exists and has specific content.
func SystemFileCheck(name, description, path, expectedContent string) *Check {
	return &Check{
		Name:        name,
		Description: description,
		CheckFn: func(ctx context.Context, data CheckData) (bool, string, error) {
			content, err := os.ReadFile(path)
			if err != nil {
				return false, "", fmt.Errorf("failed to read %s: %w", path, err)
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
			return false, "", fmt.Errorf("failed to check if service %s is started: %w", serviceName, err)
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

// ServiceCheck checks if a service is running.
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

// KubernetesFileCheck checks if kubernetes configuration files exist.
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
				return false, "", fmt.Errorf("error checking %s: %w", path, err)
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

// FileTreeDifference computes the difference between an actual path and an expected file tree.
func FileTreeDifference(path string, expectedFiles []string) ([]string, []string, error) {
	foundFiles := []string{}
	actualPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to evaluate symlinks for %s: %w", path, err)
	}
	err = filepath.Walk(actualPath, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relativePath, err := filepath.Rel(actualPath, filePath)
			if err != nil {
				return fmt.Errorf("failed to get relative path: %w", err)
			}
			foundFiles = append(foundFiles, relativePath)
		}
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to walk file tree: %w", err)
	}
	missingFiles := difference(expectedFiles, foundFiles)
	extraFiles := difference(foundFiles, expectedFiles)
	return missingFiles, extraFiles, nil
}

// FileTreeCheck checks if a file tree exists.
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
		return false, "", fmt.Errorf("failed to create client set for kubelet health check: %w", err)
	}

	waiter := apiclient.NewKubeWaiter(client, timeout, io.Discard)
	err = waiter.WaitForKubelet("127.0.0.1", 10248)
	if err != nil {
		return false, "", fmt.Errorf("kubelet health check failed: %w", err)
	}
	return true, "Kubelet is healthy", nil
}

func CheckApiServerHealth(timeout time.Duration, checkData CheckData) (bool, string, error) {
	data, ok := checkData.(*checkWorkloadData)
	if !ok {
		return false, "", fmt.Errorf("wait-control-plane phase invoked with an invalid data struct %T", checkData)
	}

	client, err := kubeConfigUtil.ClientSetFromFile(kubeadmConstants.GetAdminKubeConfigPath())
	if err != nil {
		return false, "", fmt.Errorf("failed to create client set for API server health check: %w", err)
	}

	waiter := apiclient.NewKubeWaiter(client, timeout, io.Discard)
	var podMap map[string]*v1.Pod
	podMap, err = staticPodUtil.ReadMultipleStaticPodsFromDisk(data.ManifestDir(),
		kubeadmConstants.ControlPlaneComponents...)
	if err == nil {
		err = waiter.WaitForControlPlaneComponents(podMap, data.ApiAdvertiseAddress())
	}

	if err != nil {
		return false, "", fmt.Errorf("control plane health check failed: %w", err)
	}
	return true, "API Server is healthy", nil
}

type CheckWorkloadData interface {
	IsOk() bool
	WorkloadCount() int
	ReadyWorkloads() []*v1alpha1.WorkloadState
	NotReadyWorkloads() []*v1alpha1.WorkloadState
	Iteration() int
	Duration() time.Duration
	SetOk(bool)
	SetWorkloadCount(int)
	SetReadyWorkloads([]*v1alpha1.WorkloadState)
	SetNotReadyWorkloads([]*v1alpha1.WorkloadState)
	SetIteration(int)
	Start()
	ApiAdvertiseAddress() string
	ManifestDir() string
}

type checkWorkloadData struct {
	startTime           time.Time
	apiAdvertiseAddress string
	readyWorkloads      []*v1alpha1.WorkloadState
	notReadyWorkloads   []*v1alpha1.WorkloadState
	workloadCount       int
	iteration           int
	ok                  bool
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

func (c *checkWorkloadData) Start() {
	c.startTime = time.Now()
}

func (c *checkWorkloadData) Duration() time.Duration {
	if c.startTime.IsZero() {
		return 0
	}
	return time.Since(c.startTime)
}

func (c *checkWorkloadData) ApiAdvertiseAddress() string {
	return c.apiAdvertiseAddress
}

func (c *checkWorkloadData) ManifestDir() string {
	return kubeadmConstants.GetStaticPodDirectory()
}

func CreateCheckWorkloadData(apiAdvertiseAddress string) CheckData {
	return &checkWorkloadData{
		apiAdvertiseAddress: apiAdvertiseAddress,
	}
}

var (
	workloadLabelStyle = lipgloss.NewStyle().Width(20)
	workloadNameStyle  = lipgloss.NewStyle().Width(54)
	blueStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("33")) // blue
)

func PrettyPrintWorkloadState(prefix string, r *v1alpha1.WorkloadState) string {
	var status string
	var statusStyle lipgloss.Style

	if r.Ok {
		status = "✓"
		statusStyle = successStyle
	} else {
		status = "✗"
		statusStyle = errorStyle
	}

	return fmt.Sprintf("%s%s %s %s %s\n",
		prefix,
		statusStyle.Render(status),
		workloadLabelStyle.Render(r.Namespace),
		workloadNameStyle.Render(r.Name),
		r.Message,
	)
}

func CheckWorkloadResultPrinter(result *CheckResult, prefix string, spinView string) string {
	data := result.CheckData.(CheckWorkloadData)

	ready := data.ReadyWorkloads()
	unready := data.NotReadyWorkloads()
	count := data.WorkloadCount()

	prettyCount := blueStyle.Render(fmt.Sprintf("%d", count))
	prettyReady := successStyle.Render(fmt.Sprintf("%d", len(ready)))
	prettyUnready := errorStyle.Render(fmt.Sprintf("%d", len(unready)))

	elapsed := data.Duration().Round(time.Millisecond)
	if elapsed == 0 {
		result.Message = ""
	} else {
		result.Message = fmt.Sprintf("%7.3fs - ", elapsed.Seconds())
	}
	if len(ready) > 0 {
		result.Message += fmt.Sprintf("%s / %s workloads ready", prettyReady, prettyCount)
	}
	if len(unready) > 0 {
		if len(ready) > 0 {
			result.Message += ", "
		}
		result.Message += fmt.Sprintf("%s / %s workloads unready", prettyUnready, prettyCount)
	}

	// Format the global result first
	output := result.FormatResult(prefix, spinView)
	if result.Status == StatusSkipped {
		return output
	}
	prefix = prefix + "  "

	if len(ready) > 0 {
		for _, state := range ready {
			output += PrettyPrintWorkloadState(prefix, state)
		}
	}

	if len(unready) > 0 {
		for _, state := range unready {
			output += PrettyPrintWorkloadState(prefix, state)
		}
	}

	return output
}

func CheckWorkloads(ctx context.Context, data CheckData) (bool, string, error) {
	workloadData := (data).(CheckWorkloadData)
	config, err := LoadFromFile(filepath.Join(kubeadmConstants.KubernetesDir, kubeadmConstants.AdminKubeConfigFileName))
	if err != nil {
		return false, "", errors.Wrap(err, "While loading local cluster configuration")
	}
	workloadData.Start()

	err = config.WaitForWorkloads(ctx, 0, func(state bool, total int, ready, unready []*v1alpha1.WorkloadState, iteration int) bool {
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
