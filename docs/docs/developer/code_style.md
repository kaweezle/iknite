<!-- cSpell: words sirupsen iface gofmt mycommand -->

!!! wip "Work in progress"

    This documentation is in draft form and may change frequently.

# Code Style

This page describes the code style guidelines for the Iknite project.

## Language

Iknite is written in **Go**. Follow the official Go style guides:

- [Effective Go](https://go.dev/doc/effective_go)
- [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)
- [Google's Go Style Guide](https://google.github.io/styleguide/go/)

## Naming Conventions

### Packages

- Lowercase, single-word package names
- No underscores, hyphens, or mixedCaps
- Descriptive names: `alpine`, `config`, `k8s` — not `utils_common`

```go
package alpine  // ✅
package alpineUtils  // ❌
package alpine_utils  // ❌
```

### Variables and Functions

- Use `mixedCaps` (camelCase) for unexported names
- Use `MixedCaps` (PascalCase) for exported names
- Use descriptive names; avoid single-letter names except in short loops

```go
// ✅ Good
func EnsureOpenRC(runlevel string) error { ... }
func performStart(ikniteConfig *v1alpha1.IkniteClusterSpec) { ... }

// ❌ Avoid
func EnsureORC(rl string) error { ... }
func ps(c *v1alpha1.IkniteClusterSpec) { ... }
```

### Interfaces

- Use `-er` suffix for single-method interfaces
- Keep interfaces small and focused

```go
// ✅ Good
type Executor interface {
    Run(name string, args ...string) error
}

// ✅ Good
type Reader interface {
    Read(p []byte) (n int, err error)
}
```

## Error Handling

- Always check errors immediately after the call
- Wrap errors with context using `fmt.Errorf("context: %w", err)`
- Use `cobra.CheckErr()` for command-level errors

```go
// ✅ Good
err := alpine.StartService("containerd")
if err != nil {
    return fmt.Errorf("failed to start containerd: %w", err)
}

// ❌ Avoid
alpine.StartService("containerd")  // ignoring error

// ❌ Avoid (no context)
return err
```

## Logging

Use `logrus` (imported as `log`) for logging:

```go
import log "github.com/sirupsen/logrus"

// Info-level log with fields
log.WithField("phase", "init").Info("Starting initialization")

// Warning with error
log.WithError(err).Warn("Failed to start buildkitd, continuing")

// Debug log
log.WithFields(log.Fields{
    "ip":    config.Ip.String(),
    "iface": config.NetworkInterface,
}).Debug("Adding IP address")
```

**Log level guidelines:**

- `Debug`: Detailed technical information for debugging
- `Info`: Normal operational messages
- `Warn`: Unexpected but recoverable situations
- `Error`: Failures that affect functionality

## Code Organization

### Return Early

Prefer early return to reduce nesting:

```go
// ✅ Good
func processFile(path string) error {
    if path == "" {
        return errors.New("path cannot be empty")
    }
    content, err := os.ReadFile(path)
    if err != nil {
        return fmt.Errorf("reading %s: %w", path, err)
    }
    return process(content)
}

// ❌ Avoid deep nesting
func processFile(path string) error {
    if path != "" {
        content, err := os.ReadFile(path)
        if err == nil {
            return process(content)
        } else {
            return fmt.Errorf("reading %s: %w", path, err)
        }
    } else {
        return errors.New("path cannot be empty")
    }
}
```

### Defer for Cleanup

Use `defer` for resource cleanup:

```go
func readConfig(path string) error {
    f, err := os.Open(path)
    if err != nil {
        return err
    }
    defer f.Close()
    // ... read file
}
```

## Comments

- Comment exported symbols with a description starting with the symbol name
- Add comments for non-obvious logic only
- Avoid redundant comments

```go
// ✅ Good
// EnsureOpenRC starts the specified OpenRC runlevel and ensures all
// required services are running.
func EnsureOpenRC(runlevel string) error { ... }

// ✅ Good - explains why
// Prevent kubelet from auto-starting via OpenRC, as iknite manages it
// as a subprocess to maintain precise lifecycle control.
const RcConfPreventKubeletRunning = `rc_kubelet_need="non-existing-service"`

// ❌ Avoid - redundant
// startService starts the service
func startService() { ... }
```

## Using `any` vs `interface{}`

Use `any` (Go 1.18+) instead of `interface{}` for unconstrained types:

```go
// ✅ Good
func process(data any) { ... }

// ❌ Outdated
func process(data interface{}) { ... }
```

## Generics

Use generics with type constraints when a function works on multiple related
types:

```go
// ✅ Good
func Contains[T comparable](slice []T, item T) bool {
    for _, v := range slice {
        if v == item {
            return true
        }
    }
    return false
}
```

## Concurrency

- Use channels for communication, mutexes for protecting shared state
- Always know how goroutines exit
- Use `context.Context` for cancellation

```go
// ✅ Good - proper context usage
func waitForCluster(ctx context.Context, timeout time.Duration) error {
    return wait.PollUntilContextTimeout(
        ctx,
        2*time.Second,
        timeout,
        true,
        isClusterReady,
    )
}
```

## Linting

The project uses `golangci-lint` with the configuration in `.golangci.yml`.

Run before committing:

```bash
golangci-lint run --fix
```

Common lint rules enforced:

- `gofmt` – Code formatting
- `govet` – Suspicious constructs
- `errcheck` – Unchecked errors
- `gocyclo` – Cyclomatic complexity
- `unparam` – Unused function parameters
- `containedctx` – Context stored in structs (warn)

## Spell Checking

The project uses [CSpell](https://cspell.org/) for spell checking via pre-commit
hooks. To suppress false positives:

```go
// Add unknown words to the local dictionary
// cSpell: words kubeadm kubeconfig containerd

// Or disable for a block
// cSpell: disable
import "k8s.io/kubernetes/cmd/kubeadm/..."
// cSpell: enable
```

## Commit Messages

Use [gitmoji](https://gitmoji.dev/) for commit messages:

```
✨ add new status server endpoint
🐛 fix IP assignment in WSL2 environments
📝 update configuration documentation
♻️ refactor clean command to use runner pattern
✅ add tests for certificate generation
```

Keep the first line under 50 characters when possible. Reference issues with
`#`:

```
🐛 fix race condition in workload watcher (#42)
```

## Adding New Commands

Follow the pattern from `pkg/cmd/status.go`:

```go
// pkg/cmd/mycommand.go
package cmd

func NewMyCommand(ikniteConfig *v1alpha1.IkniteClusterSpec) *cobra.Command {
    cmd := &cobra.Command{
        Use:              "mycommand",
        Short:            "Short description",
        Long:             "Long description...",
        PersistentPreRun: config.StartPersistentPreRun,
        Run: func(_ *cobra.Command, _ []string) {
            performMyCommand(ikniteConfig)
        },
    }
    config.ConfigureClusterCommand(cmd.Flags(), ikniteConfig)
    return cmd
}

func performMyCommand(ikniteConfig *v1alpha1.IkniteClusterSpec) {
    cobra.CheckErr(config.DecodeIkniteConfig(ikniteConfig))
    // ... implementation
}
```

Register in `pkg/cmd/root.go`:

```go
rootCmd.AddCommand(NewMyCommand(ikniteConfig))
```
