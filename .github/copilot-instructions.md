<!-- cspell: words chainguard vhdx gofmt softlevel covermode coverprofile testutils configmap devenv mkdocs -->
<!-- cspell: words oras subtests livereload -->

# Iknite Development Guide for AI Agents

### Responses, Documentation and comments style rules (Required)

Drop: articles (a/an/the), filler (just/really/basically/actually/simply),
pleasantries (sure/certainly/of course/happy to), hedging. Fragments OK. Short
synonyms (big not extensive, fix not "implement a solution for"). Technical
terms exact. Code blocks unchanged. Errors quoted exact.

Pattern: `[thing] [action] [reason]. [next step].`

Not: "Sure! I'd be happy to help you with that. The issue you're experiencing is
likely caused by..." Yes: "Bug in auth middleware. Token expiry check use `<`
not `<=`. Fix:"

## General Guidelines (Required)

- UPDATE THIS FILE WITH NEW LEARNINGS.
- Be concise. Focus on key information and avoid unnecessary details, especially
  in code comments and documentation.
- When a task is finished, add the added or modified files to the git staging
  area and run `pre-commit run`. If any issues are found, fix them and repeat
  until all checks pass.
- The project uses cSpell. When `pre-commit` report cSpell errors, the preferred
  way to fix them is to add the missing words in a `cSpell: words` comment at
  the beginning of the offending file. Example:

  ```go
  // cSpell: words chainguard vhdx gofmt softlevel covermode cover
  ```

  ```md
  <!-- cSpell: words devenv mkdocs livereload -->
  ```

  When terms are found to be used across multiple files, they can be added to
  the global `cspell.json` file in the project root.

- When more than 10 files are modified, or when the modified files are in more
  than 3 different directories, it's recommended to run
  `pre-commit run --all-files` to ensure all changes are properly checked.

## Project Overview

Iknite is a Go-based cluster orchestrator that manages initialization and
startup of a single-node Kubernetes cluster on Alpine Linux (WSL2/VM/Incus
Container). It wraps `kubeadm` and `kubelet` with Alpine-specific integration
(OpenRC services) and pre-provisions essential cluster components.

### Core Workflow

`iknite start` → OpenRC init → containerd → kubeadm init (if needed) → apply
kustomizations → wait for workloads

### Project Components

The project provides the following deliverables:

1. **Iknite CLI tool** (`cmd/iknite/iknite.go`)
   - Cobra-based commands: `start`, `init`, `reset`, `status`, `clean`, `info`
   - Wraps kubeadm with Alpine/OpenRC integration

2. **Iknite APK package** (built with [goreleaser](../.goreleaser.yaml))
   - Installs `iknite` binary to `/sbin/iknite`
   - Includes Alpine service files (`/etc/init.d/iknite`, `/etc/conf.d/iknite`)
   - Default kustomization in `/etc/iknite.d/`
   - Depends on: kubelet, kubeadm, kubectl, containerd, cni-plugins, buildkit

3. **Iknite-images APK package** (built with
   [melange](../packaging/apk/iknite-images/iknite-images.yaml))
   - Pre-pulls container images for faster startup
   - Includes: kubeadm images, flannel, metrics-server, kube-vip,
     local-path-provisioner
   - Reduces first-boot time by 2-5 minutes

4. **Root filesystem image** (built with
   [Dockerfile](../packaging/rootfs/base/Dockerfile))
   - Alpine Linux base with `iknite` and `iknite-images` pre-installed
   - Exported as tarball for WSL2/Incus import
   - Single-layer Docker image
     ([Dockerfile](../packaging/rootfs/with-images/Dockerfile)) for container
     registries

5. **VM images** (built with
   [packaging/scripts/build-vm-image.sh](../packaging/scripts/build-vm-image.sh))
   - QCOW2 format for QEMU/KVM/Incus/OpenStack
   - VHDX format for Hyper-V
   - Pre-configured with iknite ready to start on first boot
   - Includes cloud-init for easy customization

### Build & Infrastructure Tools

- **[Goreleaser](../.goreleaser.yaml)**: APK package creation and versioning
- **[GitHub Actions](../.github/workflows/release.yml)**: CI/CD pipeline for
  releases
- **[Pre-commit](../.pre-commit-config.yaml)**: Code quality (gofmt,
  golangci-lint, shellcheck, cspell)
- **[GNUmakefile](../GNUmakefile)**: All build and development commands
- **[Devcontainer](../hack/devcontainer/Dockerfile)**: Alpine-based development
  environment with all dependencies
- **[Terragrunt/Opentofu](../deploy/iac/)**: APK repository hosting (Cloudflare)
  and VM testing (OpenStack, Incus) - see [IaC README](../deploy/iac/README.md)
  for detailed conventions and getting started guide

## Golang CLI (iknite)

### Architecture

#### Package Structure

- **`cmd/iknite/`**: Entry point calling [pkg/cmd/root.go](../pkg/cmd/root.go)
- **`pkg/cmd/`**: Cobra command implementations (`start`, `init`, `reset`,
  `status`, `clean`, `info`)
  - `init.go` (~850 lines): Wraps kubeadm init workflow using **unsafe package
    linking** (`//go:linkname`) to access unexported kubeadm internals
  - Commands accept `*v1alpha1.IkniteClusterSpec` for configuration
- **`pkg/k8s/`**: Kubernetes interaction layer
  - `runtime_environment.go`: Manages Alpine environment setup (OpenRC symlinks,
    rc.conf patches, cgroup requirements)
  - `phases/init/`: Custom kubeadm phases (kube-vip setup)
- **`pkg/alpine/`**: Alpine Linux system integration
  - `service.go`: OpenRC service management (`EnsureOpenRC`, `StartOpenRC`)
  - `ip.go`: Network interface/IP address management
- **`pkg/provision/`**: Kustomization orchestration
  - Embedded base manifests in `base/` via `//go:embed`
  - Falls back to `/etc/iknite.d` if available
- **`pkg/apis/iknite/v1alpha1/`**: Custom types with k8s code-generation
  - `IkniteCluster`, `IkniteClusterSpec`, `IkniteClusterStatus`
  - Persists cluster state to `/run/iknite/status.json`

#### Build System

- **goreleaser** (`.goreleaser.yaml`): Produces APK packages with Alpine
  dependencies (kubelet, kubeadm, containerd, etc.)
- **Tasks** (`.vscode/tasks.json`): `goreleaser-build`, `golangci-lint`,
  `pre-commit`, `test with coverage`
- **Code generation**: `hack/update-codegen.sh` uses k8s.io/code-generator for
  deepcopy/client generation
- **Versioning**: Kubernetes version embedded at build time via ldflags from
  `go.mod` dependency

#### Key Integration Points

1. **Kubeadm wrapping**: [pkg/cmd/init.go](../pkg/cmd/init.go) uses
   `//go:linkname` to access unexported kubeadm functions, allowing deep
   customization of init phases
2. **OpenRC coordination**: Kubelet prevented from auto-starting via rc.conf
   patches (see ` pkg/k8s/runtime_environment.go`)
3. **State tracking**: Custom `IkniteCluster` CR persisted as JSON to track
   initialization phases and workload readiness. HTTPs server with mTLS on port
   11443 for status queries (see `pkg/server/server.go`).

### Development Workflows

#### Build & Test

Results of `make help` in the project root:

```bash
make help
Iknite build targets

Step targets:
  make all                            Run full pipeline (extract key, build packages, rootfs, VM images, etc.)
  make apk-iknite-build               Build iknite package (goreleaser)
  make apk-images-build               Build iknite-images APK
  make apk-incus-agent-build          Build incus-agent APK
  make apk-karmafun-fetch             Fetch karmafun dependencies
  make apk-repo-build                 Set up Alpine Linux package repository
  make apk-repo-publish               Upload APK repository with terragrunt
  make check-prerequisites            Verify all prerequisites are installed
  make ci-cache-rotate                Rotate build cache
  make ci-check-argocd                Validate ArgoCD configuration
  make ci-extract-key                 Extract cryptographic keys
  make ci-release-files               Prepare release files
  make ci-vm-known-hosts              Extract VM SSH host public key to ~/.ssh/iknite_known_hosts
  make ci-vm-ssh                      Connect to the E2E test VM using the fixed host key
  make clean                          Remove build artifacts and temporary files
  make container-ci-build             Build CI container image
  make container-dev-build            Build development container
  make container-login                Log in to container registry
  make e2e                            Run end-to-end tests
  make e2e-check-argocd               Check ArgoCD during e2e tests
  make e2e-tg-apply                   Apply Terraform configuration for e2e tests
  make e2e-tg-apply-vm                Apply VM Terraform configuration for e2e tests
  make e2e-tg-destroy                 Destroy Terraform infrastructure for e2e tests
  make e2e-tg-init                    Initialize Terraform for e2e tests
  make e2e-tg-refresh                 Refresh Terraform state for e2e tests
  make generate-vm-host-key           Generate VM SSH host key
  make help                           Show this help message
  make incus-metadata-build           Build Incus metadata tarball
  make info                           Show build configuration information
  make rootfs                         Build rootfs
  make rootfs-base-image              Build rootfs base image
  make rootfs-container               Create rootfs container and add preloaded images to it
  make rootfs-image                   Build final rootfs image from rootfs container
  make rootfs-image-incus-attachment  Attach Incus metadata to rootfs image in container registry with oras
  make ssh-key                        Generate SSH key
  make test                           Run go tests with coverage
  make vm-images-push                 Publish VM images to registry with oras
  make vm-images-build                Build VM images (qcow2, vhdx)
  make vm-images-publish              Publish VM images to public static object storage

File targets (examples):
  make dist/iknite-<version>.x86_64.apk
  make dist/iknite-images-<k8s-version>.x86_64.apk
  make dist/karmafun-<version>.x86_64.apk
  make dist/SHA256SUMS

Common variables (override with VAR=value):
  ARCH=x86_64
  KUBERNETES_VERSION=1.35.2
  IKNITE_RELEASE_TAG=
  IKNITE_VERSION=0.6.4-devel
  IKNITE_REPO_NAME=test
  CACHE_FLAG=
  VM_STACK=openstack
  SNAPSHOT=--snapshot
  PUSH_IMAGES=false
```

#### Testing Patterns

- Use `testify` for assertions and stateful test fixtures (see
  [pkg/k8s/runtime_environment_test.go](../pkg/k8s/runtime_environment_test.go))
- Mock executors via `pkg/testutils.MockExecutor` to avoid shell dependencies
- Afero filesystem mocking for file I/O tests (see
  [pkg/utils/filesystem_test.go](../pkg/utils/filesystem_test.go))

### Test Duplication Guardrails (Required)

When adding or modifying tests, avoid copy-paste test bodies. Treat this as a
hard requirement because `golangci-lint` (`dupl`) fails the build.

1. Prefer table-driven tests when only inputs, expected output, or expected
   error differ.
2. Extract shared setup into helpers (plugin creation, config loading,
   ResourceMap construction, transform execution).
3. Move repeated YAML/config snippets into named constants near the top of the
   test file.
4. Keep one-off tests only for truly unique logic branches; if two tests share
   the same control flow, merge them into one table-driven test with subtests.
5. For error-path testing, use a single table with `wantErrContains`-style
   assertions instead of one function per error variant.
6. Before finishing, run `golangci-lint run --fix` and verify no `dupl` findings
   remain in edited test files.

Quick check before adding a new test function:

- Can this be a new row in an existing test table?
- Is setup duplicated from another test in the same file?
- Is any multiline YAML/config string repeated more than once?

#### Adding Commands

Follow [pkg/cmd/status.go](../pkg/cmd/status.go) pattern:

1. Create `NewXxxCmd(*v1alpha1.IkniteClusterSpec)` returning `*cobra.Command`
2. Implement `performXxx(ikniteConfig)` with main logic
3. Register in [pkg/cmd/root.go](../pkg/cmd/root.go)'s `NewRootCmd()`
4. Use `config.ConfigureClusterCommand(flags, ikniteConfig)` for standard flags

#### Modifying Kubernetes Components

Edit manifests in
[packaging/apk/iknite/iknite.d/base/](../packaging/apk/iknite/iknite.d/base/kustomization.yaml).

Verify with the following command after making changes:

```bash
kubectl kustomize packaging/apk/iknite/iknite.d
```

This can be done also with iknite:

```bash
 ./dist/iknite_linux_amd64_v1/iknite kustomize -d packaging/apk/iknite/iknite.d print
```

### Project-Specific Conventions

#### Error Handling

- Use `cobra.CheckErr()` for command-level errors
- Wrap errors with context: `fmt.Errorf("failed to start OpenRC: %w", err)`
- Log with structured fields: `log.WithField("phase", phase).Info(...)`

#### Logging

- Use `logrus` (aliased as `log`)
- Verbosity set via `-v` flag: `debug`, `info`, `warn`, `error`
- JSON output via `--json` flag (see
  [pkg/cmd/util/base_options.go](../pkg/cmd/util/base_options.go).

#### Configuration

- Viper binds flags to environment variables (prefix: `IKNITE_`)
- Priority: CLI flags > ENV vars > config file > defaults
- Cluster config loaded from `/run/iknite/status.json` (see
  [pkg/apis/iknite/v1alpha1/types.go](../pkg/apis/iknite/v1alpha1/types.go))

#### Code Generation

After modifying types in `pkg/apis/iknite/v1alpha1/`:

```bash
./hack/update-codegen.sh
```

Regenerates deepcopy/client code. Verify with `./hack/verify-codegen.sh`.

## Iknite Image

### Architecture

The iknite image is an Alpine linux image running with OpenRC as init system. On
WSL2/Docker, a custom [/etc/rc.conf](../packaging/rootfs/base/rc.conf) is used
to disable hardware and networking services not needed in container
environments. A file `/run/openrc/softlevel` is also created to allow OpenRC to
run as non init process.

`iknite` is installed via the iknite APK package, along with its dependencies
(see `.goreleaser.yaml`). On first run, the iknite OpenRC service
([/etc/init.d/iknite](../packaging/apk/iknite/init.d/iknite)) is started which
in turn runs `iknite init` to initialize the cluster. The service depends on
`containerd` service to ensure container runtime is running before starting the
cluster. It _wants_ the `buildkitd` service in order to provide container image
building capabilities inside the cluster. `iknite` will launch and manage the
`kubelet` command as part of its workflow. In consequence the `kubelet` service
is disabled in `/etc/rc.conf` to prevent the kubeadm included auto-start logic
from interfering with iknite.

As part of the startup process, iknite will apply the default kustomization
present in
[/etc/iknite.d/kustomization.yaml](../packaging/apk/iknite/iknite.d/base/kustomization.yaml)
unless overridden by user configuration (in `/etc/conf.d/iknite`). The APK
package installs a base set of kustomizations including networking (flannel),
metrics-server, local-path-provisioner and Kube-VIP. Iknite then creates a
configmap `iknite-config` in the `kube-system` namespace to avoid re-applying
the base kustomization on subsequent restarts.

On WSL2, the IP address of the WSL VM changes on each start. Kubernetes expects
the node IP to be stable. To solve this, iknite detects the WSL environment and
adds a new IP address (`192.168.99.2` by default) to the `eth0` interface. This
IP is then used as the node IP for kubeadm initialization and kubelet
registration. This ensures the node IP remains consistent across restarts.
Iknite also launches a small MDNS responder to allow `iknite.local` to resolve
to this IP from the Windows host.

Iknite persists cluster state in `/run/iknite/status.json` using the
[`IkniteCluster`](../pkg/apis/iknite/v1alpha1/types.go) custom resource format.
This allows tracking initialization phases and workload readiness across
restarts. It launches an HTTPS server with mTLS on port 11443 to allow querying
cluster status from external tools.

Iknite makes the following modifications to the kubeadm initialization process
(in [pkg/cmd/init.go](../pkg/cmd/init.go)):

- Ensure that the containerd environment is clean.
- Inject Kube-VIP as a control plane component.
- Prevent coredns deployment. coredns will be deployed as part of the
  kustomization applied after kubeadm init.
- Launch the kubelet as a subprocess instead of invoking the OpenRC service.
  This allows better control over the kubelet lifecycle.
- Disable the node taint to allow scheduling workloads on the control plane
  node.
- Launch the MDNS responder on WSL2 or Incus environments.
- Perform the Kustomization when the control plane is ready.
- Wait for all iknite managed workloads to be ready before marking the cluster
  as ready.
- Put the process on hold to keep the iknite service running waiting for OpenRC
  to stop it or the kubelet to exit.
- Clean the containerd environment by removing all existing pods and containers
  and unmounting any leftover mounts. This ensures a clean state for restarting
  the cluster.

To allow a faster startup, a companion APK package
([iknite-images](../packaging/apk/iknite-images/iknite-images.yaml)) pre-imports
all required container images (kubeadm, kubelet, pause, flannel, metrics-server,
etc.) during installation. This package is built using `chainguard-dev/melange`
to create a minimal APK package containing the images.

The iknite root filesystem image (`iknite-rootfs-base`) is built using a
Dockerfile
([packaging/rootfs/base/Dockerfile](../packaging/rootfs/base/Dockerfile)) that
installs the iknite APK package along with its dependencies on top of a minimal
Alpine base image

This image does not contain the controller plane images. In order to install
them, a container is created and the `iknite-images` APK package is installed
inside it. This container is then exported as as tarball and is the base root
filesystem image that is distributed.

A single layer Docker image is built using
([packaging/rootfs/with-images/Dockerfile](../packaging/rootfs/with-images/Dockerfile))
that adds metadata and configures the image for use as a root filesystem image
in WSL2 and Docker.

In addition to the root filesystem image, ready-to-use VM images in QCOW2 and
VHDX formats are built using the root filesystem image as base. A script
([packaging/scripts/build-vm-image.sh](../packaging/scripts/build-vm-image.sh))
automates the process of creating a VM image, installing the iknite APK packages
and configuring the VM for first use. The starting point of the script is the
built root filesystem image
(`dist/iknite-<iknite_version>-<kubernetes_version>.rootfs.tar.gz`). It produces
a QCOW2 image for QEMU/KVM and converts it into a VHDX image for Hyper-V.

### Development Workflows

The project build and release process is controlled by a central Makefile
([GNUmakefile](../GNUmakefile)) The targets are obtained by running `make help`.

It is used in the release workflow defined in
[.github/workflows/release.yml](../.github/workflows/release.yml) and can be
used locally to run the different steps of the build process.

Information about the build process:

- A container runtime is needed. The project supports both Docker (`docker`
  command used) and Containerd (`nerdctl` command used).
- Buildkit is used as the image builder via the `buildctl` command. When Docker
  is used, a buildx builder named `iknite` is created and enabled for buildctl
  by exporting the appropriate `BUILDKIT_HOST` (`"docker-container://..."`)
  variable.
- On Github Actions, docker is used.

## General Project Guidelines

### Commit Style

Use gitmoji conventions:

- ✨ New features
- 🐛 Bug fixes
- 📝 Documentation
- ♻️ Refactoring
- ✅ Tests

Keep commits atomic and descriptive (see [CONTRIBUTING.md](../CONTRIBUTING.md)).

## Common Pitfalls

1. **OpenRC timing**: Ensure `/run/openrc` symlink exists before service
   operations (see `EnsureOpenRCDirectory`)
2. **Kubeadm config drift**: Always load cluster spec via
   `config.DecodeIkniteConfig()` to merge flags/config
3. **Test isolation**: Reset `utils.Exec` in TearDown to avoid cross-test
   pollution
4. **Alpine vs standard Linux**: Use `/sbin/openrc`, not systemd; paths differ
   (`/lib/rc/init.d`)
5. **Race-prone globals in tests**: Avoid parallel tests when mutating global
   state (`viper`, `utils.Exec`, `utils.FS`, env vars). Prefer serialized tests
   with explicit cleanup, or refactor code to injected dependencies.
6. **Iknitectl filesystem plumbing**: Prefer `host.FileSystem` for commands that
   only touch files and `host.FileExecutor` only when command execution is
   required. Do not thread `afero.Fs` through `pkg/cmd/iknitectl` root options
   or command constructors.

## CI/CD

- **PR checks**: Go build, test, pre-commit hooks
  ([.github/workflows/go.yaml](../.github/workflows/go.yaml))
- **Releases**: Tag push → full goreleaser build → APK + rootfs + VM images
  ([.github/workflows/release.yml](../.github/workflows/release.yml))
- **Versioning**: Semantic versioning, Kubernetes version extracted from
  `go.mod`

## Documentation

Iknite documentation is built with **MkDocs** using Material for MkDocs theme.
The documentation source is in `docs/docs/` and is managed with automatic
navigation generation.

The project also includes the following markdown pages outside the `docs/`
folder:

- [README.md](../README.md): Project overview and quickstart
- [CONTRIBUTING.md](../CONTRIBUTING.md): Contribution guidelines
- [RELEASE.md](../RELEASE.md): Important information to include in releases.
- [STRUCTURE.md](../STRUCTURE.md): Project architecture
- [BUILD.md](../BUILD.md): Build instructions
- [pkg/README.md](../pkg/README.md): Golang package overview

### Documentation Structure

- **Configuration**: `docs/mkdocs.yaml` - MkDocs configuration
- **Content**: `docs/docs/` - Markdown documentation files
- **Navigation**: `docs/docs/.nav.yml` - Manual navigation overrides
- **Dependencies**: `docs/pyproject.toml` - Python dependencies managed with
  `uv`

### Documentation Commands

```bash
# Install dependencies (first time only)
cd docs
uv sync

# Build documentation
uv run mkdocs build --clean --strict

# Serve documentation locally with live reload
uv run mkdocs serve --livereload

# Access at http://localhost:8000/
```

### Adding Documentation

1. Create markdown files in `docs/docs/`
2. Use symbolic links for files outside `docs/docs/`:
   ```bash
   cd docs/docs/some/category
   ln -s ../../../../path/to/your/README.md .
   ```
3. Main categories defined in `docs/docs/.nav.yml`
4. GitHub Actions automatically builds and deploys to GitHub Pages on push to
   `main`

See [.github/workflows/docs.yaml](../.github/workflows/docs.yaml) for CI/CD
configuration.

## Quick Reference

IMPORTANT: Always check `make help` for the most up-to-date commands and
targets.

### Go Development

| Task                     | Command                                                                  |
| ------------------------ | ------------------------------------------------------------------------ |
| Run iknite locally       | `go run cmd/iknite/iknite.go start -v debug`                             |
| Run tests with coverage  | `go test -v -race -covermode=atomic -coverprofile=coverage.out ./...`    |
| Build single target APK  | `goreleaser build --single-target --snapshot` or `make apk-iknite-build` |
| Update generated code    | `./hack/update-codegen.sh`                                               |
| Verify code generation   | `./hack/verify-codegen.sh`                                               |
| Run linters              | `golangci-lint run --fix`                                                |
| Run all pre-commit hooks | `pre-commit run --all-files`                                             |

### Documentation

| Task                | Command                                           |
| ------------------- | ------------------------------------------------- |
| Build documentation | `cd docs && uv run mkdocs build --clean --strict` |
| Serve docs locally  | `cd docs && uv run mkdocs serve --livereload`     |
| Install/update deps | `cd docs && uv sync`                              |
