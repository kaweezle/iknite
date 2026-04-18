package checkers

// cSpell: words apiclient lipgloss clientcmd charmbracelet staticpod wrapcheck
import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	v1 "k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	kubeadmConstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"k8s.io/kubernetes/cmd/kubeadm/app/util/apiclient"
	kubeConfigUtil "k8s.io/kubernetes/cmd/kubeadm/app/util/kubeconfig"
	staticPodUtil "k8s.io/kubernetes/cmd/kubeadm/app/util/staticpod"

	"github.com/kaweezle/iknite/pkg/alpine"
	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/check"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/k8s"
	"github.com/kaweezle/iknite/pkg/utils"
)

// checkFileAndContent checks if a file exists and optionally if it has the expected content. It returns a boolean
// indicating if the check passed, a message describing the result, and an error if any occurred during the check.
func checkFileAndContent(h host.FileSystem, path, expectedContent string) (bool, string, error) {
	// Check if file at path exists
	info, err := h.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, "", fmt.Errorf("%s does not exist", path)
	} else if err != nil {
		return false, "", fmt.Errorf("error checking %s: %w", path, err)
	}

	// Check if path is a file and not a directory
	if info.IsDir() {
		return false, "", fmt.Errorf("%s is a directory, not a file", path)
	}

	if expectedContent == "" {
		return true, fmt.Sprintf("%s exists and is a file", path), nil
	}

	content, err := h.ReadFile(path)
	if err != nil {
		return false, "", fmt.Errorf("failed to read %s: %w", path, err)
	}
	if strings.TrimSpace(string(content)) != strings.TrimSpace(expectedContent) {
		return false, "", fmt.Errorf("unexpected content in %s", path)
	}
	return true, fmt.Sprintf("%s exists with expected content", path), nil
}

// FileCheck checks if a file exists and has specific content.
func FileCheck(name, description, path, expectedContent string) *check.Check {
	return &check.Check{
		Name:        name,
		Description: description,
		CheckFn: func(_ context.Context, checkData check.CheckData) (bool, string, error) {
			data, ok := checkData.(host.HostProvider)
			if !ok {
				return false, "", fmt.Errorf("invalid check data type")
			}
			return checkFileAndContent(data.Host(), path, expectedContent)
		},
	}
}

type ServiceType int

const (
	ServiceTypeOpenRC ServiceType = iota
	ServiceTypePidFile
)

func CheckService(
	fileExec host.FileExecutor,
	serviceName string,
	serviceType ServiceType,
) (bool, string, error) {
	pid := 0
	if serviceType == ServiceTypeOpenRC {
		success, err := alpine.IsServiceStarted(fileExec, serviceName)
		if err != nil {
			return false, "", fmt.Errorf(
				"failed to check if service %s is started: %w",
				serviceName,
				err,
			)
		}
		if !success {
			return false, "", fmt.Errorf("service %s is not running", serviceName)
		}
	}
	if serviceType == ServiceTypePidFile {
		var err error
		pid, _, err = alpine.CheckPidFile(fileExec, serviceName)
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
func ServiceCheck(name, serviceName string, serviceType ServiceType, parents ...string) *check.Check {
	if len(parents) == 0 {
		parents = []string{"openrc"}
	}
	return &check.Check{
		Name:        name,
		DependsOn:   parents,
		Description: fmt.Sprintf("Check if %s service is running", serviceName),
		CheckFn: func(_ context.Context, checkData check.CheckData) (bool, string, error) {
			data, ok := checkData.(host.HostProvider)
			if !ok {
				return false, "", fmt.Errorf("invalid check data type")
			}
			return CheckService(data.Host(), serviceName, serviceType)
		},
	}
}

// SimpleFileCheck checks if a file exists.
func SimpleFileCheck(name, path string) *check.Check {
	return &check.Check{
		Name:        name,
		Description: fmt.Sprintf("Check %s", path),
		DependsOn:   []string{},
		CheckFn: func(_ context.Context, checkData check.CheckData) (bool, string, error) {
			data, ok := checkData.(host.HostProvider)
			if !ok {
				return false, "", fmt.Errorf("invalid check data type")
			}
			return checkFileAndContent(data.Host(), path, "")
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
func FileTreeDifference(
	fs host.FileSystem,
	path string,
	expectedFiles []string,
) ([]string, []string, error) {
	foundFiles := []string{}
	actualPath, err := fs.EvalSymlinks(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to evaluate symlinks for %s: %w", path, err)
	}
	err = fs.Walk(actualPath, func(filePath string, info os.FileInfo, err error) error {
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

// checkFileTree checks if the file tree at the given path matches the expected files. It returns a boolean indicating
// if the check passed, a message describing the result, and an error if any occurred during the check.
func checkFileTree(fs host.FileSystem, path string, expectedFiles []string) (bool, string, error) {
	missingFiles, extraFiles, err := FileTreeDifference(fs, path, expectedFiles)
	if err != nil {
		return false, "", err
	}
	if len(missingFiles) > 0 {
		return false, fmt.Sprintf(
			"Missing files: %s",
			strings.Join(missingFiles, ", "),
		), nil
	}
	if len(extraFiles) > 0 {
		return false, fmt.Sprintf("Extra files: %s", strings.Join(extraFiles, ", ")), nil
	}
	return true, fmt.Sprintf(
		"All expected %d files found in %s",
		len(expectedFiles),
		path,
	), nil
}

// FileTreeCheck checks if a file tree exists.
func FileTreeCheck(name, description, path string, expectedFiles []string) *check.Check {
	return &check.Check{
		Name:        name,
		Description: description,
		CheckFn: func(_ context.Context, checkData check.CheckData) (bool, string, error) {
			data, ok := checkData.(host.HostProvider)
			if !ok {
				return false, "", fmt.Errorf("invalid check data type")
			}
			return checkFileTree(data.Host(), path, expectedFiles)
		},
	}
}

func CheckKubeletHealth(timeout time.Duration) (bool, string, error) {
	var client clientset.Interface
	client, err := kubeConfigUtil.ClientSetFromFile(kubeadmConstants.GetAdminKubeConfigPath())
	if err != nil {
		return false, "", fmt.Errorf(
			"failed to create client set for kubelet health check: %w",
			err,
		)
	}

	waiter := apiclient.NewKubeWaiter(client, timeout, io.Discard)
	err = waiter.WaitForKubelet("127.0.0.1", 10248)
	if err != nil {
		return false, "", fmt.Errorf("kubelet health check failed: %w", err)
	}
	return true, "Kubelet is healthy", nil
}

func CheckApiServerHealth(
	timeout time.Duration,
	checkData check.CheckData,
) (bool, string, error) {
	var data *checkWorkloadData
	data, success := checkData.(*checkWorkloadData)
	if !success {
		return false, "", fmt.Errorf(
			"wait-control-plane phase invoked with an invalid data struct %T",
			checkData,
		)
	}

	client, err := kubeConfigUtil.ClientSetFromFile(kubeadmConstants.GetAdminKubeConfigPath())
	if err != nil {
		return false, "", fmt.Errorf(
			"failed to create client set for API server health check: %w",
			err,
		)
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
		statusStyle = check.SuccessStyle
	} else {
		status = "✗"
		statusStyle = check.ErrorStyle
	}

	return fmt.Sprintf("%s%s %s %s %s\n",
		prefix,
		statusStyle.Render(status),
		workloadLabelStyle.Render(r.Namespace),
		workloadNameStyle.Render(r.Name),
		r.Message,
	)
}

func CheckWorkloadResultPrinter(result *check.CheckResult, prefix, spinView string) string {
	data, ok := result.CheckData.(CheckWorkloadData)
	if !ok {
		return result.FormatResult(prefix, spinView)
	}

	ready := data.ReadyWorkloads()
	unready := data.NotReadyWorkloads()
	count := data.WorkloadCount()

	prettyCount := blueStyle.Render(fmt.Sprintf("%d", count))
	prettyReady := check.SuccessStyle.Render(fmt.Sprintf("%d", len(ready)))
	prettyUnready := check.ErrorStyle.Render(fmt.Sprintf("%d", len(unready)))

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
	if result.Status == check.StatusSkipped {
		return output
	}
	prefix += "  "

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

func CheckWorkloads(ctx context.Context, data check.CheckData) (bool, string, error) {
	workloadData, ok := (data).(CheckWorkloadData)
	if !ok {
		return false, "", errors.New("invalid check data type")
	}
	config, err := k8s.LoadFromDefault()
	if err != nil {
		return false, "", fmt.Errorf("while loading local cluster configuration: %w", err)
	}
	workloadData.Start()

	err = workloadData.WaitOptions().Poll(ctx, config.RESTClient().WorkloadsReadyConditionWithContextFunc(
		func(allReady bool, total int, ready, unready []*v1alpha1.WorkloadState, iteration, okIterations int) bool {
			workloadData.SetOk(allReady)
			workloadData.SetWorkloadCount(total)
			workloadData.SetReadyWorkloads(ready)
			workloadData.SetNotReadyWorkloads(unready)
			workloadData.SetIteration(iteration)
			workloadData.SetOkIterations(okIterations)
			return allReady
		}))
	if err != nil {
		return false, "", fmt.Errorf("while waiting for workloads: %w", err)
	}
	return workloadData.IsOk(), "", nil
}

// CheckIkniteServerHealth checks the /healthz endpoint of the iknite status
// server using the mTLS client configuration stored in constants.IkniteLocalConfPath
// (/root/.kube/iknite.conf). It returns true when the server responds with "ok".
func CheckIkniteServerHealth(ctx context.Context, waitOptions *utils.WaitOptions) (bool, string, error) {
	kubeConfig, err := k8s.LoadFromFile(constants.IkniteLocalConfPath)
	if err != nil {
		return false, "", fmt.Errorf("failed to load iknite config from %s: %w", constants.IkniteLocalConfPath, err)
	}

	var restClient rest.Interface
	if restClient, err = kubeConfig.NewRESTClient(); err != nil {
		return false, "", fmt.Errorf("failed to create REST client: %w", err)
	}

	err = waitOptions.Poll(ctx, func(ctx context.Context) (bool, error) {
		body, healthzErr := restClient.Get().AbsPath("/healthz").Timeout(waitOptions.CheckTimeout).DoRaw(ctx)
		if healthzErr != nil {
			return false, fmt.Errorf("failed to call /healthz endpoint: %w", healthzErr)
		}
		if string(body) != "ok" {
			return false, fmt.Errorf("iknite status server returned unexpected response: %s", string(body))
		}
		return true, nil
	})
	if err != nil {
		return false, "", fmt.Errorf("iknite status server health check failed: %w", err)
	}
	return true, "Iknite status server is healthy", nil
}

// checkOpenRCStarted checks if OpenRC is started by verifying the existence of the SoftLevelPath file.
func checkOpenRCStarted(fs host.FileSystem) (bool, string, error) {
	exists, err := fs.Exists(constants.SoftLevelPath)
	if err != nil {
		return false, "", fmt.Errorf(
			"failed to check if OpenRC is started: %w",
			err,
		)
	}
	if !exists {
		return false, "OpenRC is not started", nil
	}
	return true, "OpenRC is started", nil
}

// OpenRCCheck returns a check that verifies if OpenRC is started.
func OpenRCCheck() *check.Check {
	return &check.Check{
		Name:        "openrc",
		Description: "Check that OpenRC is started",
		CheckFn: func(_ context.Context, checkData check.CheckData) (bool, string, error) {
			data, ok := checkData.(host.HostProvider)
			if !ok {
				return false, "", fmt.Errorf("invalid check data type")
			}
			return checkOpenRCStarted(data.Host())
		},
	}
}

func checkApiBackendData(fs host.FileSystem, apiBackendName string) (bool, string, error) {
	var expectedFiles []string
	if apiBackendName == constants.EtcdBackendName {
		expectedFiles = []string{"member/snap/db"}
	} else {
		expectedFiles = []string{"kine.db"}
	}
	missingFiles, _, err := FileTreeDifference(
		fs,
		fmt.Sprintf("/var/lib/%s", apiBackendName),
		expectedFiles,
	)
	if err != nil {
		return false, "", fmt.Errorf(
			"failed to check %s file tree: %w",
			apiBackendName,
			err,
		)
	}
	if len(missingFiles) > 0 {
		return false, fmt.Sprintf(
			"/var/lib/%s has no data file",
			apiBackendName,
		), nil
	}
	return true, fmt.Sprintf("/var/lib/%s has data files", apiBackendName), nil
}

func APIBackendDataCheck(apiBackendName string) *check.Check {
	return &check.Check{
		Name: fmt.Sprintf("api_backend_%s_data", apiBackendName),
		Description: fmt.Sprintf(
			"Check that the %s data directory (/var/lib/%s) is present and contains data",
			apiBackendName,
			apiBackendName,
		),
		CheckFn: func(_ context.Context, checkData check.CheckData) (bool, string, error) {
			data, ok := checkData.(host.HostProvider)
			if !ok {
				return false, "", fmt.Errorf("invalid check data type")
			}
			return checkApiBackendData(data.Host(), apiBackendName)
		},
	}
}

// checkDomainName checks if the given domain name is mapped to the given IP address.
func checkDomainName(ctx context.Context, nh host.NetworkHost, domainName string, ip net.IP) (bool, string, error) {
	if domainName == "" {
		return true, "Domain name is not set", nil
	}
	ipString := ip.String()
	if contains, ips := nh.IsHostMapped(
		ctx,
		ip,
		domainName,
	); contains {
		mapped := func() bool {
			for _, ip := range ips {
				if ip.String() == ipString {
					return true
				}
			}
			return false
		}()
		if mapped {
			return true, fmt.Sprintf(
				"Domain name %s is mapped to IP %s",
				domainName,
				ipString,
			), nil
		}
	}
	return false, fmt.Sprintf(
		"Domain name %s is not mapped to IP %s",
		domainName,
		ipString,
	), nil
}

// DomainNameCheck returns a check that verifies if the given domain name is mapped to the given IP address.
func DomainNameCheck(domainName string, ip net.IP) *check.Check {
	return &check.Check{
		Name:        "domain_name",
		Description: "Check if the domain name is set",
		CheckFn: func(ctx context.Context, checkData check.CheckData) (bool, string, error) {
			data, ok := checkData.(host.HostProvider)
			if !ok {
				return false, "", fmt.Errorf("invalid check data type")
			}
			return checkDomainName(ctx, data.Host(), domainName, ip)
		},
	}
}
