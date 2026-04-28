<!-- cSpell: words kubeadm kubelet kustomize iknite kubewait gosec restclient -->

# Testability Refactoring Recommendations

## Context

Current repository-wide test coverage is constrained by side effects in core
packages:

- hardcoded absolute paths (`/etc`, `/run`, `/var/lib`, `/etc/kubernetes`)
- direct process control (`os/exec`, `os.Exit`, `cobra.CheckErr`)
- direct global state mutation (`viper`, package globals, `utils.Exec`)
- live Kubernetes/OpenRC integration in command logic

Result: large code regions are only integration-testable, not unit-testable.

## Refactoring Goals

1. Make critical orchestration code unit-testable without root privileges.
2. Isolate side effects behind interfaces.
3. Keep behavior unchanged while enabling deterministic tests.
4. Reduce race-prone global state usage.

## Priority Refactorings

### 1. Introduce System Abstractions in `pkg/k8s` and `pkg/alpine`

Action:

- add interfaces for filesystem, clock, process signaling, and OS environment
  access
- inject these dependencies into functions that now use `os.*` and `time.*`
  directly

Example interfaces:

- `FileReaderWriter` (`ReadFile`, `WriteFile`, `Stat`, `Remove`, `MkdirAll`)
- `ProcessManager` (`FindProcess`, `Signal`, `Wait`)
- `SleeperClock` (`Now`, `Sleep`, `After`)

Benefit:

- unit-test `kubelet.go`, `service.go`, `clean.go` without touching host
  paths/processes.

### 2. Replace Global `utils.Exec` and `utils.FS` with Constructor Injection

Action:

- migrate packages from implicit globals to explicit dependencies in structs
- provide default constructors for production (`NewRuntimeEnvironment()` etc.)

Benefit:

- eliminate test races from shared mutable globals
- improve parallel test execution reliability

### 3. Extract Command Runners from Cobra Commands

Action:

- split `NewXxxCmd` (CLI wiring) from pure `Runner` types (business logic)
- `Runner.Run(ctx, options)` should return errors instead of calling
  `cobra.CheckErr` or `os.Exit`

Benefit:

- unit-test command behavior without spawning Cobra runtime or process exits
- significantly improve `pkg/cmd` coverage

### 4. Encapsulate Kubeadm/OpenRC Integration Behind Interfaces

Action:

- define adapters:
  - `KubeadmService` (`Init`, `Reset`, `LoadConfig`)
  - `OpenRCService` (`Ensure`, `StartService`, `StopService`, `IsStarted`)
- keep current implementation as default adapter

Benefit:

- mock high-cost integration points in `pkg/cmd/init.go` and `pkg/cmd/reset.go`
- unlock table-driven unit coverage for branch-heavy orchestration

### 5. Remove Hidden Global Config Coupling (`viper`)

Action:

- pass config provider interface through command constructors/runners
- avoid direct package-level `viper.Get*` in business logic

Benefit:

- deterministic tests for config precedence and flag handling
- fewer race issues in parallel tests

### 6. Separate Kubernetes Client Creation from Workload Logic

Action:

- split methods that both build clients and perform checks
- create pure functions that consume prebuilt interfaces
  (`kubernetes.Interface`, `rest.Interface`)

Benefit:

- test readiness and status computation with fake clients only
- reduce complexity in `pkg/k8s/config.go`, `pkg/k8s/restclient.go`,
  `pkg/kubewait/resources.go`

### 7. Introduce Polling Strategy Interfaces

Action:

- abstract retry loops (`Poll`, intervals, retries, settle timers) behind
  strategy interfaces
- inject test strategy with zero sleep

Benefit:

- faster and deterministic tests for `kubewait` and kubelet health checks
- avoid long-running flaky tests

## Suggested Implementation Plan

1. Phase A: infrastructure abstractions (`Exec`, `FS`, clock, process) and
   runner extraction in `pkg/cmd`.
2. Phase B: refactor `pkg/k8s/kubelet.go`, `pkg/k8s/clean.go`,
   `pkg/alpine/service.go` to injected dependencies.
3. Phase C: adapter pattern for kubeadm/OpenRC integrations in
   `init/reset/start` flows.
4. Phase D: convert command logic tests to table-driven runner tests with full
   `t.Parallel()` support.

## Test Design Guidelines

- use table-driven tests for all branching logic
- isolate mutable global state; prefer local fixtures per test case
- use `require` assertions for fail-fast behavior
- keep integration tests separate from unit tests with clear naming and tags

## Expected Impact

After these refactorings:

- large currently untestable files become unit-testable
- race conditions in tests decrease
- realistic path to high coverage targets across orchestration-heavy packages
- improved maintainability and change safety for cluster lifecycle code
