<!-- cSpell: words testutils depguard runlevels softlevel iface testhelpers stretchr coverprofile godoc -->

# Plan: Make Iknite Unit-Test Friendly (1.5% → 80% Coverage)

The iknite project has significant testability challenges due to tight coupling
with system resources (Alpine OpenRC, Kubernetes APIs, process execution). The
existing `testutils.MockExecutor` (`pkg/testutils/executor.go`) pattern provides
a foundation, but ~95% of the codebase bypasses abstraction layers and calls
system APIs directly. This plan systematically introduces dependency injection
through interfaces to enable comprehensive unit testing.

## Current State Analysis

### Existing Test Coverage (~4 test files, <10% coverage)

1. **pkg/k8s/runtime_environment_test.go** (2 tests)
   - Tests: `PreventKubeletServiceFromStarting`, file manipulation
   - Pattern: testify/suite, MockExecutor for command execution
   - Coverage: File I/O operations only, not Alpine/OpenRC integration

2. **pkg/alpine/ip_test.go** (3 tests)
   - Tests: `CheckIpExists`, `AddIpAddress`, `AddIpMapping`
   - Pattern: testify/suite, MockExecutor, afero for filesystem
   - Coverage: IP management, partial hosts file manipulation

3. **pkg/provision/base_test.go** (1 test)
   - Tests: Kustomization template rendering
   - Pattern: Table-driven tests, no mocking needed
   - Coverage: Manifest generation only

### Test Patterns Already In Use ✅

- **testify/suite** for stateful fixtures and Setup/Teardown lifecycle
- **MockExecutor** abstraction for command execution (pkg/testutils/executor.go)
- **afero** for filesystem mocking in tests
- Table-driven test approach for pure functions

### Critical Problems Blocking Testing ❌

#### 1. Command Execution - Partially Abstracted

**Good**: `utils.Executor` interface exists with `MockExecutor` implementation
**Bad**: 15+ locations bypass the abstraction:

- `pkg/provision/common.go:107` - direct `exec.CommandContext`
- `pkg/k8s/kubelet.go:148` - direct `exec.CommandContext`
- `pkg/k8s/kubelet.go:137` - `StartKubelet()` returns raw `*exec.Cmd` handle

#### 2. Filesystem Operations - No Abstraction

45+ direct `os.*` calls across the codebase:

- `pkg/config/config.go:198` - `os.Create()`
- `pkg/cmd/reset.go:162` - `os.Stat()`
- `pkg/apis/iknite/v1alpha1/types.go:116,131` - `os.WriteFile()`,
  `os.ReadFile()`
- `pkg/k8s/checkers.go:35,103` - `os.ReadFile()`, `os.Stat()`
- `pkg/k8s/kubelet.go:54,57,93,176,204` - Multiple file operations

**Tests use afero** but production code doesn't.

#### 3. HTTP/Network Operations - No Abstraction

All code uses `http.DefaultClient` or raw kubernetes clients:

- `pkg/k8s/kubelet.go:328-361` - `CheckKubeletRunning()` with hardcoded HTTP
  client
- `pkg/k8s/config.go:119-157` - `CheckClusterRunning()` with REST client
- **0 tests exist** for any health check functions

#### 4. Alpine Service Management - No Abstraction

`pkg/alpine/service.go` - **0 tests** for 7 functions:

- `EnsureOpenRC()` - calls `/sbin/openrc`
- `StartService()` - calls `/sbin/rc-service`
- `StopService()` - calls `/sbin/rc-service`
- `EnableService()` - uses `os.Symlink()`
- `IsServiceStarted()` - filesystem checks

All depend on real OpenRC installation.

#### 5. Global State & No Dependency Injection

**Global variables** requiring test mutation:

```go
pkg/utils/executor.go:57           var Exec Executor = &CommandExecutor{}
pkg/alpine/service.go:40           var startedServicesDir = path.Join(...)
```

**All command constructors** have no injection points:

```go
func NewStartCmd(ikniteConfig *v1alpha1.IkniteClusterSpec) *cobra.Command {
    return &cobra.Command{
        Run: func(_ *cobra.Command, _ []string) {
            performStart(ikniteConfig)  // Directly calls with hardcoded deps
        },
    }
}
```

#### 6. Complex Functions Violating SRP

- `pkg/cmd/init.go:349` - `newInitData()` (150+ lines): validation + config
  loading + temp dirs + defaults
- `pkg/k8s/runtime_environment.go:75` - `PrepareKubernetesEnvironment()` (60+
  lines): writes /proc + modules + IP + hosts + OpenRC
- `pkg/provision/common.go:97` - `applyResmap()` (25 lines): YAML conversion +
  kubectl spawn + env vars + parsing
- `pkg/cmd/status.go:89` - `performStatus()` (200+ lines, gocyclo warning):
  creates 50+ check objects inline

## Steps

### Step 1: Establish Baseline Abstractions (Week 1-2)

**Goal**: Enforce existing patterns and create filesystem abstraction

#### 1.1 Enforce Executor Pattern Everywhere

**Changes needed:**

- Replace all `exec.CommandContext()` direct calls with `utils.Exec`
- Add golangci-lint rule to forbid direct exec usage
- Create `ProcessManager` interface for long-running processes

**Files to update** (15+):

- `pkg/provision/common.go:107`
- `pkg/k8s/kubelet.go:148`
- All other direct exec usage

**Example refactor** for `pkg/k8s/kubelet.go`:

```go
// Before:
func StartKubelet() (*exec.Cmd, error) {
    cmd := exec.CommandContext(context.Background(), "/usr/bin/kubelet")
    cmd.Start()
    return cmd, nil
}

// After:
type ProcessHandle interface {
    Wait() error
    Signal(sig os.Signal) error
    Pid() int
}

type ProcessManager interface {
    StartKubelet(args ...string) (ProcessHandle, error)
}

type processManager struct {
    executor Executor
}

func NewProcessManager(exec Executor) ProcessManager {
    return &processManager{executor: exec}
}

func (pm *processManager) StartKubelet(args ...string) (ProcessHandle, error) {
    // Use executor interface
    return pm.executor.StartLongRunning("/usr/bin/kubelet", args...)
}
```

**Add linter rule** to `.golangci.yml`:

```yaml
linters:
  - depguard

depguard:
  rules:
    main:
      deny:
        - pkg: "os/exec"
          desc: "Use utils.Exec interface instead"
```

**Expected coverage gain**: +15%

#### 1.2 Create FileSystem Abstraction

**Create** `pkg/utils/filesystem.go`:

```go
package utils

import (
    "os"
    "github.com/spf13/afero"
)

type FileSystem interface {
    ReadFile(path string) ([]byte, error)
    WriteFile(path string, data []byte, perm os.FileMode) error
    Stat(path string) (os.FileInfo, error)
    Create(path string) (afero.File, error)
    Open(path string) (afero.File, error)
    OpenFile(path string, flag int, perm os.FileMode) (afero.File, error)
    Remove(path string) error
    RemoveAll(path string) error
    MkdirAll(path string, perm os.FileMode) error
    Symlink(oldName, newName string) error
    ReadDir(dirname string) ([]os.FileInfo, error)
    Exists(path string) (bool, error)
}

type aferoFS struct {
    fs afero.Fs
}

// Default to real filesystem
var FS FileSystem = &aferoFS{fs: afero.NewOsFs()}

func (a *aferoFS) ReadFile(path string) ([]byte, error) {
    return afero.ReadFile(a.fs, path)
}

func (a *aferoFS) WriteFile(path string, data []byte, perm os.FileMode) error {
    return afero.WriteFile(a.fs, path, data, perm)
}

func (a *aferoFS) Stat(path string) (os.FileInfo, error) {
    return a.fs.Stat(path)
}

func (a *aferoFS) Create(path string) (afero.File, error) {
    return a.fs.Create(path)
}

func (a *aferoFS) Open(path string) (afero.File, error) {
    return a.fs.Open(path)
}

func (a *aferoFS) OpenFile(path string, flag int, perm os.FileMode) (afero.File, error) {
    return a.fs.OpenFile(path, flag, perm)
}

func (a *aferoFS) Remove(path string) error {
    return a.fs.Remove(path)
}

func (a *aferoFS) RemoveAll(path string) error {
    return a.fs.RemoveAll(path)
}

func (a *aferoFS) MkdirAll(path string, perm os.FileMode) error {
    return a.fs.MkdirAll(path, perm)
}

func (a *aferoFS) Symlink(oldName, newName string) error {
    linker, ok := a.fs.(afero.Linker)
    if !ok {
        return fmt.Errorf("filesystem does not support symlinks")
    }
    return linker.SymlinkIfPossible(oldName, newName)
}

func (a *aferoFS) ReadDir(dirname string) ([]os.FileInfo, error) {
    return afero.ReadDir(a.fs, dirname)
}

func (a *aferoFS) Exists(path string) (bool, error) {
    return afero.Exists(a.fs, path)
}

// For tests
func NewMemMapFS() FileSystem {
    return &aferoFS{fs: afero.NewMemMapFs()}
}
```

**Update files** (20+) to use `utils.FS` instead of `os.*`

**Expected coverage gain**: +20%

### Step 2: Extract Service Layer Interfaces (Week 3-4)

**Goal**: Abstract Alpine OpenRC and network management

#### 2.1 Create ServiceManager Interface

**Create** `pkg/alpine/service_manager.go`:

```go
package alpine

import (
    "github.com/kaweezle/iknite/pkg/utils"
)

type ServiceManager interface {
    EnsureOpenRC(level string) error
    Start(name string) error
    Stop(name string) error
    Enable(name string) error
    Disable(name string) error
    IsStarted(name string) (bool, error)
}

type OpenRCConfig struct {
    RunDir      string  // /run/openrc
    ServicesDir string  // /etc/init.d
    RunLevelDir string  // /etc/runlevels/default
    SoftLevelPath string // /run/openrc/softlevel
}

type openRCManager struct {
    executor utils.Executor
    fs       utils.FileSystem
    config   OpenRCConfig
}

func NewServiceManager(exec utils.Executor, fs utils.FileSystem, cfg OpenRCConfig) ServiceManager {
    return &openRCManager{
        executor: exec,
        fs:       fs,
        config:   cfg,
    }
}

func DefaultServiceManager() ServiceManager {
    return NewServiceManager(
        utils.Exec,
        utils.FS,
        OpenRCConfig{
            RunDir:        "/run/openrc",
            ServicesDir:   "/etc/init.d",
            RunLevelDir:   "/etc/runlevels/default",
            SoftLevelPath: "/run/openrc/softlevel",
        },
    )
}

func (m *openRCManager) Start(name string) error {
    started, err := m.IsStarted(name)
    if err != nil {
        return err
    }
    if started {
        return nil
    }

    _, err = m.executor.Run(false, "/sbin/rc-service", name, "start")
    return err
}

func (m *openRCManager) IsStarted(name string) (bool, error) {
    path := filepath.Join(m.config.RunDir, "started", name)
    return m.fs.Exists(path)
}

func (m *openRCManager) Enable(name string) error {
    source := filepath.Join(m.config.ServicesDir, name)
    target := filepath.Join(m.config.RunLevelDir, name)
    return m.fs.Symlink(source, target)
}

// ... implement other methods
```

**Refactor** `pkg/alpine/service.go` to use the interface

**Write tests** in `pkg/alpine/service_manager_test.go`:

```go
type ServiceManagerTestSuite struct {
    suite.Suite
    manager ServiceManager
    mockExec *testutils.MockExecutor
    mockFS   utils.FileSystem
}

func (s *ServiceManagerTestSuite) SetupTest() {
    s.mockExec = &testutils.MockExecutor{}
    s.mockFS = utils.NewMemMapFS()

    s.manager = NewServiceManager(
        s.mockExec,
        s.mockFS,
        OpenRCConfig{
            RunDir: "/run/openrc",
            ServicesDir: "/etc/init.d",
            RunLevelDir: "/etc/runlevels/default",
        },
    )
}

func (s *ServiceManagerTestSuite) TestStartService() {
    // Setup filesystem state
    s.mockFS.MkdirAll("/run/openrc/started", 0755)

    // Mock executor
    s.mockExec.On("Run", false, "/sbin/rc-service", "containerd", "start").
        Return([]byte("ok"), nil)

    // Test
    err := s.manager.Start("containerd")
    s.NoError(err)
    s.mockExec.AssertExpectations(s.T())
}

func (s *ServiceManagerTestSuite) TestEnableService() {
    s.mockFS.MkdirAll("/etc/init.d", 0755)
    s.mockFS.MkdirAll("/etc/runlevels/default", 0755)

    err := s.manager.Enable("iknite")
    s.NoError(err)

    // Verify symlink created
    exists, _ := s.mockFS.Exists("/etc/runlevels/default/iknite")
    s.True(exists)
}

// ... 7+ test functions total
```

**Expected coverage gain**: +10%

#### 2.2 Create NetworkManager Interface

**Create** `pkg/alpine/network_manager.go`:

```go
package alpine

import (
    "net"
    "github.com/kaweezle/iknite/pkg/utils"
)

type NetworkManager interface {
    CheckIPExists(ip net.IP) (bool, error)
    AddIPAddress(iface string, addr net.IP) error
    AddHostMapping(ip net.IP, hostname string) error
    RemoveHostMapping(hostname string) error
}

type alpineNetworkManager struct {
    executor  utils.Executor
    fs        utils.FileSystem
    hostsFile string
}

func NewNetworkManager(exec utils.Executor, fs utils.FileSystem, hostsPath string) NetworkManager {
    return &alpineNetworkManager{
        executor:  exec,
        fs:        fs,
        hostsFile: hostsPath,
    }
}

func DefaultNetworkManager() NetworkManager {
    return NewNetworkManager(utils.Exec, utils.FS, "/etc/hosts")
}

func (nm *alpineNetworkManager) CheckIPExists(ip net.IP) (bool, error) {
    // Implementation using executor to call ip command
    out, err := nm.executor.Run(true, "/sbin/ip", "addr", "show")
    if err != nil {
        return false, err
    }
    return strings.Contains(string(out), ip.String()), nil
}

func (nm *alpineNetworkManager) AddIPAddress(iface string, addr net.IP) error {
    cidr := addr.String() + "/24"
    _, err := nm.executor.Run(true, "/sbin/ip", "addr", "add", cidr, "broadcast", "+", "dev", iface)
    return err
}

// ... implement other methods
```

**Expected coverage gain**: +5%

### Step 3: Wrap HTTP & Kubernetes Integration (Week 5)

**Goal**: Abstract health checks and K8s clients

#### 3.1 Create HealthChecker Interface

**Create** `pkg/k8s/health_checker.go`:

```go
package k8s

import (
    "context"
    "net/http"
    "time"
)

type HTTPClient interface {
    Do(req *http.Request) (*http.Response, error)
}

type HealthChecker interface {
    CheckKubelet(ctx context.Context, retries, okResponses, waitSec int) error
    CheckAPIServer(ctx context.Context, retries, okResponses, waitSec int) error
}

type httpHealthChecker struct {
    client       HTTPClient
    kubeletURL   string
    apiServerURL string
}

func NewHealthChecker(client HTTPClient, kubeletURL, apiURL string) HealthChecker {
    return &httpHealthChecker{
        client:       client,
        kubeletURL:   kubeletURL,
        apiServerURL: apiURL,
    }
}

func DefaultHealthChecker() HealthChecker {
    return NewHealthChecker(
        &http.Client{Timeout: 2 * time.Second},
        "http://localhost:10248/healthz",
        "https://localhost:6443/readyz",
    )
}

func (h *httpHealthChecker) CheckKubelet(ctx context.Context, retries, okResponses, waitSec int) error {
    waitTime := time.Duration(waitSec) * time.Second
    successCount := 0

    for i := 0; i < retries; i++ {
        req, err := http.NewRequestWithContext(ctx, "GET", h.kubeletURL, nil)
        if err != nil {
            return err
        }

        resp, err := h.client.Do(req)
        if err == nil && resp.StatusCode == http.StatusOK {
            successCount++
            if successCount >= okResponses {
                return nil
            }
        } else {
            successCount = 0
        }

        time.Sleep(waitTime)
    }

    return fmt.Errorf("kubelet not healthy after %d retries", retries)
}

// ... implement CheckAPIServer similarly
```

**Refactor** `pkg/k8s/kubelet.go` and `pkg/k8s/config.go` to use interface

**Write tests** with mock HTTP client:

```go
type mockHTTPClient struct {
    responses map[string]*http.Response
    errors    map[string]error
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
    url := req.URL.String()
    if err, ok := m.errors[url]; ok {
        return nil, err
    }
    return m.responses[url], nil
}

type HealthCheckerTestSuite struct {
    suite.Suite
    checker HealthChecker
    mockClient *mockHTTPClient
}

func (s *HealthCheckerTestSuite) SetupTest() {
    s.mockClient = &mockHTTPClient{
        responses: make(map[string]*http.Response),
        errors:    make(map[string]error),
    }
    s.checker = NewHealthChecker(
        s.mockClient,
        "http://localhost:10248/healthz",
        "https://localhost:6443/readyz",
    )
}

func (s *HealthCheckerTestSuite) TestCheckKubeletHealthy() {
    s.mockClient.responses["http://localhost:10248/healthz"] = &http.Response{
        StatusCode: http.StatusOK,
        Body: io.NopCloser(strings.NewReader("ok")),
    }

    err := s.checker.CheckKubelet(context.Background(), 3, 2, 0)
    s.NoError(err)
}

func (s *HealthCheckerTestSuite) TestCheckKubeletUnhealthy() {
    s.mockClient.errors["http://localhost:10248/healthz"] = errors.New("connection refused")

    err := s.checker.CheckKubelet(context.Background(), 3, 2, 0)
    s.Error(err)
}
```

**Expected coverage gain**: +15%

#### 3.2 Create KustomizationApplier Interface

**Create** `pkg/provision/applier.go`:

```go
package provision

import (
    "sigs.k8s.io/kustomize/api/resmap"
    "sigs.k8s.io/kustomize/api/resource"
    "github.com/kaweezle/iknite/pkg/utils"
)

type KustomizationApplier interface {
    Apply(resources resmap.ResMap) error
    ApplyFromDirectory(dir string) ([]*resource.Resource, error)
}

type kubectlApplier struct {
    executor   utils.Executor
    kubeconfig string
}

func NewKubectlApplier(exec utils.Executor, kubeconfigPath string) KustomizationApplier {
    return &kubectlApplier{
        executor:   exec,
        kubeconfig: kubeconfigPath,
    }
}

func DefaultKubectlApplier() KustomizationApplier {
    return NewKubectlApplier(utils.Exec, "/root/.kube/config")
}

func (ka *kubectlApplier) Apply(resources resmap.ResMap) error {
    yaml, err := resources.AsYaml()
    if err != nil {
        return err
    }

    // Use executor interface instead of direct exec
    _, err = ka.executor.Pipe(
        bytes.NewReader(yaml),
        true,
        "/usr/bin/kubectl",
        "--kubeconfig", ka.kubeconfig,
        "apply", "-f", "-",
    )
    return err
}
```

**Expected coverage gain**: +8%

### Step 4: Refactor Command Constructors (Week 6-7)

**Goal**: Inject dependencies into all commands

#### 4.1 Create Dependencies Struct

**Create** `pkg/cmd/dependencies.go`:

```go
package cmd

import (
    "github.com/kaweezle/iknite/pkg/alpine"
    "github.com/kaweezle/iknite/pkg/k8s"
    "github.com/kaweezle/iknite/pkg/provision"
    "github.com/kaweezle/iknite/pkg/utils"
)

type Dependencies struct {
    Executor       utils.Executor
    FileSystem     utils.FileSystem
    ServiceManager alpine.ServiceManager
    NetworkManager alpine.NetworkManager
    HealthChecker  k8s.HealthChecker
    Applier        provision.KustomizationApplier
    ProcessManager k8s.ProcessManager
}

func DefaultDependencies() *Dependencies {
    return &Dependencies{
        Executor:       utils.Exec,
        FileSystem:     utils.FS,
        ServiceManager: alpine.DefaultServiceManager(),
        NetworkManager: alpine.DefaultNetworkManager(),
        HealthChecker:  k8s.DefaultHealthChecker(),
        Applier:        provision.DefaultKubectlApplier(),
        ProcessManager: k8s.DefaultProcessManager(),
    }
}

func TestDependencies(
    exec utils.Executor,
    fs utils.FileSystem,
    svc alpine.ServiceManager,
    net alpine.NetworkManager,
    health k8s.HealthChecker,
    applier provision.KustomizationApplier,
    proc k8s.ProcessManager,
) *Dependencies {
    return &Dependencies{
        Executor:       exec,
        FileSystem:     fs,
        ServiceManager: svc,
        NetworkManager: net,
        HealthChecker:  health,
        Applier:        applier,
        ProcessManager: proc,
    }
}
```

#### 4.2 Refactor Command Constructors

**Update** `pkg/cmd/start.go`:

```go
// Before:
func NewStartCmd(ikniteConfig *v1alpha1.IkniteClusterSpec) *cobra.Command {
    return &cobra.Command{
        Run: func(_ *cobra.Command, _ []string) {
            performStart(ikniteConfig)
        },
    }
}

// After:
func NewStartCmd(ikniteConfig *v1alpha1.IkniteClusterSpec, deps *Dependencies) *cobra.Command {
    if deps == nil {
        deps = DefaultDependencies()
    }

    return &cobra.Command{
        Run: func(_ *cobra.Command, _ []string) {
            performStart(ikniteConfig, deps)
        },
    }
}

func performStart(ikniteConfig *v1alpha1.IkniteClusterSpec, deps *Dependencies) error {
    // Use injected dependencies
    if err := deps.ServiceManager.Start("containerd"); err != nil {
        return err
    }

    if err := deps.NetworkManager.AddIPAddress("eth0", ikniteConfig.Ip); err != nil {
        return err
    }

    // ... rest of implementation
}
```

**Update** `pkg/cmd/root.go` to pass dependencies:

```go
func NewRootCmd() *cobra.Command {
    rootCmd := &cobra.Command{...}

    ikniteConfig := &v1alpha1.IkniteClusterSpec{}
    deps := DefaultDependencies()

    rootCmd.AddCommand(NewKustomizeCmd())
    rootCmd.AddCommand(newCmdInit(os.Stdout, nil, deps))
    rootCmd.AddCommand(newCmdReset(os.Stdin, os.Stdout, nil, deps))
    rootCmd.AddCommand(NewCmdClean(ikniteConfig, nil, deps))
    rootCmd.AddCommand(NewStartCmd(ikniteConfig, deps))
    rootCmd.AddCommand(NewStatusCmd(ikniteConfig, deps))
    // ...
}
```

**Expected coverage gain**: +10%

#### 4.3 Split Init Workflow

**Extract** validation logic from `pkg/cmd/init.go`:

```go
// Create pkg/cmd/init_validator.go
package cmd

type InitValidator struct {
    fs utils.FileSystem
}

func NewInitValidator(fs utils.FileSystem) *InitValidator {
    return &InitValidator{fs: fs}
}

// Pure validation functions - easily testable
func (v *InitValidator) ValidatePKI(certsDir string) error {
    requiredFiles := []string{
        "ca.crt", "ca.key",
        "sa.pub", "sa.key",
        // ... all PKI files
    }

    for _, file := range requiredFiles {
        path := filepath.Join(certsDir, file)
        exists, err := v.fs.Exists(path)
        if err != nil {
            return err
        }
        if !exists {
            return fmt.Errorf("missing required certificate: %s", file)
        }
    }
    return nil
}

func (v *InitValidator) ValidateKubeconfig(path string) error {
    exists, err := v.fs.Exists(path)
    if err != nil {
        return err
    }
    if !exists {
        return fmt.Errorf("kubeconfig not found: %s", path)
    }
    return nil
}

// ... more validation functions
```

**Create** `pkg/cmd/init_workflow.go`:

```go
package cmd

type InitWorkflow struct {
    validator      *InitValidator
    serviceManager alpine.ServiceManager
    networkManager alpine.NetworkManager
    healthChecker  k8s.HealthChecker
    applier        provision.KustomizationApplier
    processManager k8s.ProcessManager
}

func NewInitWorkflow(deps *Dependencies) *InitWorkflow {
    return &InitWorkflow{
        validator:      NewInitValidator(deps.FileSystem),
        serviceManager: deps.ServiceManager,
        networkManager: deps.NetworkManager,
        healthChecker:  deps.HealthChecker,
        applier:        deps.Applier,
        processManager: deps.ProcessManager,
    }
}

func (w *InitWorkflow) Execute(ctx context.Context, cfg *kubeadmApi.InitConfiguration) error {
    // Orchestrate phases using injected dependencies

    // Phase 1: Prepare environment
    if err := w.serviceManager.Start("containerd"); err != nil {
        return err
    }

    // Phase 2: Validate certificates
    if err := w.validator.ValidatePKI(cfg.CertificatesDir); err != nil {
        return err
    }

    // Phase 3: Start kubelet
    handle, err := w.processManager.StartKubelet()
    if err != nil {
        return err
    }
    defer handle.Signal(syscall.SIGTERM)

    // Phase 4: Wait for health
    if err := w.healthChecker.CheckKubelet(ctx, 30, 3, 1); err != nil {
        return err
    }

    // ... continue with remaining phases

    return nil
}
```

**Expected coverage gain**: +20%

### Step 5: Build Test Infrastructure (Week 8)

**Goal**: Reusable test helpers and fixtures

#### 5.1 Create Test Helpers Package

**Create** `pkg/testhelpers/fixtures.go`:

```go
package testhelpers

import (
    "github.com/spf13/afero"
    "github.com/kaweezle/iknite/pkg/alpine"
    "github.com/kaweezle/iknite/pkg/k8s"
    "github.com/kaweezle/iknite/pkg/provision"
    "github.com/kaweezle/iknite/pkg/testutils"
    "github.com/kaweezle/iknite/pkg/utils"
)

// Common test fixtures
func NewMockExecutor() *testutils.MockExecutor {
    return &testutils.MockExecutor{}
}

func NewTestFileSystem() utils.FileSystem {
    return utils.NewMemMapFS()
}

func NewMockServiceManager() *MockServiceManager {
    return &MockServiceManager{}
}

func NewMockNetworkManager() *MockNetworkManager {
    return &MockNetworkManager{}
}

func NewMockHealthChecker() *MockHealthChecker {
    return &MockHealthChecker{}
}

func NewMockKustomizationApplier() *MockKustomizationApplier {
    return &MockKustomizationApplier{}
}

// Setup helpers
func SetupTestKubernetesEnvironment(fs utils.FileSystem) error {
    // Create typical K8s directory structure
    dirs := []string{
        "/etc/kubernetes/pki",
        "/etc/kubernetes/manifests",
        "/var/lib/kubelet",
        "/var/lib/etcd",
    }

    for _, dir := range dirs {
        if err := fs.MkdirAll(dir, 0755); err != nil {
            return err
        }
    }
    return nil
}

func SetupTestPKI(fs utils.FileSystem) error {
    // Create minimal PKI files for testing
    files := map[string]string{
        "/etc/kubernetes/pki/ca.crt": "fake-ca-cert",
        "/etc/kubernetes/pki/ca.key": "fake-ca-key",
        "/etc/kubernetes/pki/sa.pub": "fake-sa-pub",
        "/etc/kubernetes/pki/sa.key": "fake-sa-key",
    }

    for path, content := range files {
        if err := fs.WriteFile(path, []byte(content), 0600); err != nil {
            return err
        }
    }
    return nil
}

func SetupTestConfig() *v1alpha1.IkniteClusterSpec {
    cfg := &v1alpha1.IkniteClusterSpec{}
    v1alpha1.SetDefaults_IkniteClusterSpec(cfg)
    return cfg
}
```

**Create** `pkg/testhelpers/mocks.go`:

```go
package testhelpers

import (
    "github.com/stretchr/testify/mock"
    "github.com/kaweezle/iknite/pkg/alpine"
    "github.com/kaweezle/iknite/pkg/k8s"
)

type MockServiceManager struct {
    mock.Mock
}

func (m *MockServiceManager) Start(name string) error {
    args := m.Called(name)
    return args.Error(0)
}

func (m *MockServiceManager) Stop(name string) error {
    args := m.Called(name)
    return args.Error(0)
}

func (m *MockServiceManager) Enable(name string) error {
    args := m.Called(name)
    return args.Error(0)
}

func (m *MockServiceManager) IsStarted(name string) (bool, error) {
    args := m.Called(name)
    return args.Bool(0), args.Error(1)
}

// ... implement other interface methods

type MockNetworkManager struct {
    mock.Mock
}

func (m *MockNetworkManager) CheckIPExists(ip net.IP) (bool, error) {
    args := m.Called(ip)
    return args.Bool(0), args.Error(1)
}

func (m *MockNetworkManager) AddIPAddress(iface string, addr net.IP) error {
    args := m.Called(iface, addr)
    return args.Error(0)
}

// ... implement other interface methods

type MockHealthChecker struct {
    mock.Mock
}

func (m *MockHealthChecker) CheckKubelet(ctx context.Context, retries, okResponses, waitSec int) error {
    args := m.Called(ctx, retries, okResponses, waitSec)
    return args.Error(0)
}

func (m *MockHealthChecker) CheckAPIServer(ctx context.Context, retries, okResponses, waitSec int) error {
    args := m.Called(ctx, retries, okResponses, waitSec)
    return args.Error(0)
}

type MockKustomizationApplier struct {
    mock.Mock
}

func (m *MockKustomizationApplier) Apply(resources resmap.ResMap) error {
    args := m.Called(resources)
    return args.Error(0)
}

func (m *MockKustomizationApplier) ApplyFromDirectory(dir string) ([]*resource.Resource, error) {
    args := m.Called(dir)
    return args.Get(0).([]*resource.Resource), args.Error(1)
}
```

### Step 6: Write Comprehensive Unit Tests (Week 9-10)

**Goal**: Achieve 80%+ coverage

#### 6.1 Priority 1: Alpine Service Management (+10%)

**Create** `pkg/alpine/service_manager_test.go` with 7+ tests:

- `TestStartService`
- `TestStartServiceAlreadyStarted`
- `TestStopService`
- `TestEnableService`
- `TestEnableServiceAlreadyEnabled`
- `TestIsServiceStarted`
- `TestEnsureOpenRC`

#### 6.2 Priority 2: Health Checks (+15%)

**Create** `pkg/k8s/health_checker_test.go`:

- `TestCheckKubeletHealthy`
- `TestCheckKubeletUnhealthy`
- `TestCheckKubeletTimeout`
- `TestCheckAPIServerHealthy`
- `TestCheckAPIServerUnhealthy`

#### 6.3 Priority 3: Kustomization (+8%)

**Create** `pkg/provision/applier_test.go`:

- `TestApplyResources`
- `TestApplyFromDirectory`
- `TestApplyWithError`

#### 6.4 Priority 4: Init Validation (+20%)

**Create** `pkg/cmd/init_validator_test.go`:

- `TestValidatePKI`
- `TestValidatePKIMissing`
- `TestValidateKubeconfig`
- `TestValidateKubeconfigMissing`
- `TestValidateConfiguration`

**Create** `pkg/cmd/init_workflow_test.go`:

- `TestExecuteFullWorkflow`
- `TestExecuteServiceFailure`
- `TestExecuteHealthCheckFailure`

#### 6.5 Priority 5: Command Tests (+12%)

**Create** tests for all refactored commands:

- `pkg/cmd/start_test.go`
- `pkg/cmd/status_test.go`
- `pkg/cmd/prepare_test.go`

#### 6.6 Expand Existing Tests

**Enhance** `pkg/alpine/ip_test.go`:

- Add `TestCheckIPExistsMultipleIPs`
- Add `TestAddIPAddressError`
- Add `TestRemoveHostMapping`

**Enhance** `pkg/k8s/runtime_environment_test.go`:

- Add `TestPrepareKubernetesEnvironment`
- Add `TestPreventKubeletServiceWhenMissing`

## Further Considerations

### 1. Kubeadm Integration Strategy

**Problem**: `pkg/cmd/init.go` uses `//go:linkname` to access unexported kubeadm
functions:

```go
//go:linkname AddInitOtherFlags k8s.io/kubernetes/cmd/kubeadm/app/cmd.AddInitOtherFlags
func AddInitOtherFlags(flagSet *flag.FlagSet, initOptions *initOptions)

//go:linkname getDryRunClient k8s.io/kubernetes/cmd/kubeadm/app/cmd.getDryRunClient
func getDryRunClient(d *initData) (clientset.Interface, error)
```

**Options**:

**Option A: Facade Interface** ✅ Recommended

```go
type KubeadmFacade interface {
    AddInitFlags(flags *flag.FlagSet, opts interface{})
    GetDryRunClient(data interface{}) (clientset.Interface, error)
    InitCluster(cfg *kubeadmApi.InitConfiguration) error
}

type kubeadmFacadeImpl struct {
    // Real implementation using go:linkname
}

type mockKubeadmFacade struct {
    mock.Mock
}
```

- Pros: Testable, maintains abstraction
- Cons: Extra layer of indirection, ~1 week effort

**Option B: Integration Tests Only**

- Pros: Fast to implement, tests real behavior
- Cons: No unit tests for init workflow, slower tests

**Option C: Fork/Vendor Kubeadm**

- Pros: Full control, can export needed APIs
- Cons: High maintenance burden, version drift

**Recommendation**: Option A - create facade interface. The init workflow is
critical and worth the investment.

### 2. Process Lifecycle Testing

**Problem**: `StartKubelet()` returns `*exec.Cmd` for process tracking:

```go
func StartKubelet() (*exec.Cmd, error) {
    cmd := exec.CommandContext(context.Background(), "/usr/bin/kubelet")
    cmd.Start()
    return cmd, nil  // Real process handle
}

// Used in init workflow to wait/signal
kubeletCmd.Process.Signal(syscall.SIGTERM)
kubeletCmd.Wait()
```

**Options**:

**Option A: ProcessHandle Interface** ✅ Recommended

```go
type ProcessHandle interface {
    Wait() error
    Signal(sig os.Signal) error
    Pid() int
    ExitCode() (int, error)
}

type mockProcessHandle struct {
    waitErr error
    pid     int
    exitCode int
}

func (m *mockProcessHandle) Wait() error {
    return m.waitErr
}

func (m *mockProcessHandle) Signal(sig os.Signal) error {
    return nil
}
```

- Pros: Clean abstraction, easy to mock
- Cons: Requires refactoring all process code (~5 files)

**Option B: State Machine**

```go
type ProcessState int
const (
    StateStarting ProcessState = iota
    StateRunning
    StateStopped
)

type MockProcess struct {
    state ProcessState
    stateChanges chan ProcessState
}
```

- Pros: More realistic behavior simulation
- Cons: Complex state management

**Option C: Real Background Processes in Tests**

```go
func TestKubeletLifecycle(t *testing.T) {
    // Spawn real sleep/echo process
    cmd := exec.Command("sleep", "10")
    cmd.Start()
    defer cmd.Process.Kill()
    // ... test
}
```

- Pros: Tests real behavior
- Cons: Slower, platform-dependent, cleanup issues

**Recommendation**: Option A - ProcessHandle interface. Clean, fast,
predictable.

### 3. Test Parallelization Strategy

**Considerations**:

- Go test framework supports `t.Parallel()`
- afero MemMapFs is not thread-safe by default
- Global variable mutation (utils.Exec) requires synchronization
- testify/suite runs sequentially by default

**Options**:

**Option A: Parallel Pure Functions, Sequential Stateful** ✅ Recommended

```go
// Pure functions - run in parallel
func TestValidatePKI(t *testing.T) {
    t.Parallel()  // Safe - no global state
    fs := utils.NewMemMapFS()
    validator := NewInitValidator(fs)
    // ... test
}

// Stateful/global - run sequentially
type ServiceManagerTestSuite struct {
    suite.Suite
    // Don't call t.Parallel() in suite tests
}
```

- Pros: Best of both worlds, ~30% faster
- Cons: Requires discipline in test design

**Option B: Fully Parallel with Locks**

```go
var execMutex sync.Mutex

func (s *TestSuite) SetupTest() {
    execMutex.Lock()
    s.oldExec = utils.Exec
    utils.Exec = s.mockExec
}

func (s *TestSuite) TeardownTest() {
    utils.Exec = s.oldExec
    execMutex.Unlock()
}
```

- Pros: Maximum parallelism
- Cons: Complex, hard to debug deadlocks

**Option C: Fully Sequential**

```go
// No t.Parallel() anywhere
```

- Pros: Simple, no race conditions
- Cons: Slower CI (3-5x slower with 100+ tests)

**Recommendation**: Option A - mixed approach. Pure functions in parallel,
stateful tests sequential. Add `-race` flag to CI to catch issues.

## Expected Outcomes

### Coverage Progression

| Phase     | Target Files | Functions | Coverage Gain | Cumulative | Duration     |
| --------- | ------------ | --------- | ------------- | ---------- | ------------ |
| Phase 1   | 30+          | 50+       | +25%          | 26.5%      | 2 weeks      |
| Phase 2   | 2            | 11        | +15%          | 41.5%      | 2 weeks      |
| Phase 3   | 3            | 8         | +15%          | 56.5%      | 1 week       |
| Phase 4   | 5            | 12        | +18%          | 74.5%      | 2 weeks      |
| Phase 5   | 3            | 15        | +12%          | 86.5%      | 3 weeks      |
| **Total** | **43+**      | **96+**   | **+85%**      | **86.5%**  | **10 weeks** |

### Test Execution Performance

With parallelization:

- **Pure function tests**: ~50 tests, 2-3s total
- **Integration tests**: ~30 tests, 5-10s total (sequential)
- **Command workflow tests**: ~20 tests, 3-5s total
- **Total CI time**: <20s (from potential 60s+ sequential)

### Maintainability Improvements

1. **Dependency injection** makes code more modular
2. **Interface boundaries** clarify component responsibilities
3. **Mock infrastructure** speeds up new test creation
4. **Test helpers** reduce boilerplate from ~50 lines to ~10 per test
5. **Linter rules** prevent regression to direct system calls

## Risk Mitigation

### High Risk: Breaking Changes

**Mitigation**:

- Implement interfaces alongside existing code
- Use feature flags for new code paths
- Run both old and new implementations in parallel initially
- Extensive integration testing with real Alpine/K8s

### Medium Risk: Test Maintenance

**Mitigation**:

- Centralize mock setup in test helpers
- Use table-driven tests for combinatorial cases
- Document testing patterns in CONTRIBUTING.md
- CI checks for test coverage regressions

### Low Risk: Performance

**Mitigation**:

- Benchmark critical paths after refactoring
- Profile test execution time
- Cache mock setups for repeated tests

## Success Metrics

1. **Coverage**: 80%+ line coverage on `go test -coverprofile`
2. **Speed**: Full test suite <20s on CI
3. **Maintainability**: New contributors can add tests without system access
4. **Reliability**: Tests pass consistently without system dependencies
5. **Documentation**: All public interfaces have godoc and usage examples
