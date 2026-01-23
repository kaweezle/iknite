<!-- cspell: words chainguard vhdx gofmt softlevel covermode coverprofile testutils configmap devenv mkdocs livereload -->

# Iknite Development Guide for AI Agents

## Project Overview

Iknite is a Go-based cluster orchestrator that manages initialization and
startup of a single-node Kubernetes cluster on Alpine Linux (WSL2/VM). It wraps
`kubeadm` and `kubelet` with Alpine-specific integration (OpenRC services) and
pre-provisions essential cluster components.

### Core Workflow

`iknite start` → OpenRC init → containerd → kubeadm init (if needed) → apply
kustomizations → wait for workloads

### Project Components

The project provides five main deliverables:

1. **Iknite CLI tool** (`cmd/iknite/iknite.go`)
   - Cobra-based commands: `start`, `init`, `reset`, `status`, `clean`, `info`
   - Wraps kubeadm with Alpine/OpenRC integration

2. **Iknite APK package** (built with [goreleaser](../.goreleaser.yaml))
   - Installs `iknite` binary to `/sbin/iknite`
   - Includes Alpine service files (`/etc/init.d/iknite`, `/etc/conf.d/iknite`)
   - Default kustomizations in `/etc/iknite.d/`
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
   - Exported as tarball for WSL2 import
   - Single-layer Docker image
     ([Dockerfile](../packaging/rootfs/with-images/Dockerfile)) for container
     registries
5. **VM images** (built with
   [packaging/scripts/build-vm-image.sh](../packaging/scripts/build-vm-image.sh))
   - QCOW2 format for QEMU/KVM/OpenStack
   - VHDX format for Hyper-V
   - Pre-configured with iknite ready to start on first boot

### Build & Infrastructure Tools

- **[Goreleaser](../.goreleaser.yaml)**: APK package creation and versioning
- **[GitHub Actions](../.github/workflows/release.yml)**: CI/CD pipeline for
  releases
- **[Pre-commit](../.pre-commit-config.yaml)**: Code quality (gofmt,
  golangci-lint, shellcheck, cspell)
- **[packaging/scripts/build-helper.sh](../packaging/scripts/build-helper.sh)**:
  Developer-friendly build script (full pipeline locally)
- **[Devcontainer](../hack/devcontainer/Dockerfile)**: Alpine-based development
  environment with all dependencies
- **[Terraform/Terragrunt](../deploy/iac/iknite/root.hcl)**: APK repository
  hosting (Cloudflare) and VM testing (OpenStack)

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
   initialization phases and workload readiness

### Development Workflows

#### Build & Test

```bash
# Local build (single target)
goreleaser build --auto-snapshot --clean # Or use VS Code "goreleaser-build" task

# Build iknite package
goreleaser build --snapshot --skip=publish --clean # Or use VS Code "goreleaser" task

# Full test with coverage
go test -v -race -covermode=atomic -coverprofile=coverage.out ./... # Or use VS Code "test with coverage" task

# Lint & format
pre-commit run --all-files
```

#### Testing Patterns

- Use `testify/suite` for stateful test fixtures (see
  [pkg/k8s/runtime_environment_test.go](../pkg/k8s/runtime_environment_test.go))
- Mock executors via `pkg/testutils.MockExecutor` to avoid shell dependencies
- Afero filesystem mocking for file I/O tests (see
  [pkg/utils/filesystem_test.go](../pkg/utils/filesystem_test.go))

#### Adding Commands

Follow [pkg/cmd/status.go](../pkg/cmd/status.go) pattern:

1. Create `NewXxxCmd(*v1alpha1.IkniteClusterSpec)` returning `*cobra.Command`
2. Implement `performXxx(ikniteConfig)` with main logic
3. Register in [pkg/cmd/root.go](../pkg/cmd/root.go#L46-L89)'s `NewRootCmd()`
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
  [pkg/cmd/root.go](../pkg/cmd/root.go#L113-L131))

#### Configuration

- Viper binds flags to environment variables (prefix: `IKNITE_`)
- Priority: CLI flags > ENV vars > config file > defaults
- Cluster config loaded from `/run/iknite/status.json` (see
  [pkg/apis/iknite/v1alpha1/types.go](../pkg/apis/iknite/v1alpha1/types.go#L133-L151))

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
(see [.goreleaser.yaml](../.goreleaser.yaml#L67-L79)). On first run, the iknite
OpenRC service ([/etc/init.d/iknite](../packaging/apk/iknite/init.d/iknite)) is
started which in turn runs `iknite init` to initialize the cluster. The service
depends on `containerd` service to ensure container runtime is running before
starting the cluster. It _wants_ the `buildkitd` service in order to provide
container image building capabilities inside the cluster. `iknite` will launch
and manage the `kubelet` command as part of its workflow. In consequence the
`kubelet` service is disabled in `/etc/rc.conf` to prevent the kubeadm included
auto-start logic from interfering with iknite.

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
restarts.

Iknite makes the following kustomizations to the kubeadm initialization process
(in [pkg/cmd/init.go](../pkg/cmd/init.go)):

- Ensure that the containerd environment is clean.
- Inject Kube-VIP as a control plane component.
- Prevent coredns deployment. coredns will be deployed as part of the
  kustomization applied after kubeadm init.
- Launch the kubelet as a subprocess instead of invoking the OpenRC service.
  This allows better control over the kubelet lifecycle.
- Disable the node taint to allow scheduling workloads on the control plane
  node.
- Launch the MDNS responder on WSL2 environments.
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

A single layer Docker images is built using
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
a QCOW2 image for QEMU/KVM and converts it into a VHDX image.

### Development Workflows

The main script to build the iknite images is
[packaging/scripts/build-helper.sh](../packaging/scripts/build-helper.sh). It is
a developer friendly version of the
[release workflow](../.github/workflows/release.yml) that can be run locally.

It performs the following steps:

```bash
STEPS:
    goreleaser          Build Iknite package with goreleaser
    build               Build Iknite rootfs base image
    images              Build iknite-images APK package
    add-images          Add images to rootfs container
    export              Export rootfs tarball
    rootfs-image        Build final rootfs image
    fetch-krmfnbuiltin  Fetch krmfnbuiltin APKs
    make-apk-repo       Create APK repository in dist/repo
    upload-repo         Upload APK repository to https://static.iknite.app/<repo>/
    vm-image            Build VM images (qcow2, vhdx)
    clean               Cleanup temporary files
```

A single step can be run:

```bash
./packaging/scripts/build-helper.sh --only-goreleaser # Several other --only-<step> can be added
```

Or one or more steps can be skipped:

```bash
./packaging/scripts/build-helper.sh --skip-images
```

The `--with-cache` flag can be used to speed up docker builds by reusing
previous layers.

In general, the full build can be run with:

```bash
./packaging/scripts/build-helper.sh --with-cache --skip-clean
```

And the the focus on one specific step by skipping all the others:

```bash
./packaging/scripts/build-helper.sh --only-build --skip-clean --with-cache
```

The script assumes a Linux host with Docker or Containerd (preferred) installed.
The main development environment is an Alpine based devcontainer
([.devcontainer/devcontainer.json](../.devcontainer/devcontainer.json)) or WSL2
distribution with Alpine installed via the rootfs image. The
[hack/make-rootfs-devenv.sh](../hack/make-rootfs-devenv.sh) script adds the
appropriate packages to the rootfs image to make it suitable as a development
environment.

## General Project Guidelines

### Commit Style

Use [gitmoji](https://gitmoji.dev/) conventions:

- ✨ New features
- 🐛 Bug fixes
- 📝 Documentation
- ♻️ Refactoring
- ✅ Tests

Keep commits atomic and descriptive (see
[CONTRIBUTING.md](../CONTRIBUTING.md#L146-L174)).

## Common Pitfalls

1. **OpenRC timing**: Ensure `/run/openrc` symlink exists before service
   operations (see `EnsureOpenRCDirectory`)
2. **Kubeadm config drift**: Always load cluster spec via
   `config.DecodeIkniteConfig()` to merge flags/config
3. **Test isolation**: Reset `utils.Exec` in TearDown to avoid cross-test
   pollution
4. **Alpine vs standard Linux**: Use `/sbin/openrc`, not systemd; paths differ
   (`/lib/rc/init.d`)

## CI/CD

- **PR checks**: Go build, test, pre-commit hooks
  ([.github/workflows/go.yaml](../.github/workflows/go.yaml))
- **Releases**: Tag push → full goreleaser build → APK + rootfs + VM images
  ([.github/workflows/release.yml](../.github/workflows/release.yml))
- **Versioning**: Semantic versioning, Kubernetes version extracted from
  `go.mod`

## Documentation

Iknite documentation is built with **MkDocs** using Material for MkDocs theme.
The documentation source is in `docs/docs/` and is managed with
[awesome-nav](https://github.com/lukasgeiter/mkdocs-awesome-nav) for automatic
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

- **Configuration**: [docs/mkdocs.yaml](../docs/mkdocs.yaml) - MkDocs
  configuration
- **Content**: `docs/docs/` - Markdown documentation files
- **Navigation**: [`docs/docs/.nav.yml`](../docs/docs/.nav.yml) - Manual
  navigation overrides
- **Dependencies**: [docs/pyproject.toml](../docs/pyproject.toml) - Python
  dependencies managed with `uv`

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

### Go Development

| Task                     | Command                                                               |
| ------------------------ | --------------------------------------------------------------------- |
| Run iknite locally       | `go run cmd/iknite/iknite.go start -v debug`                          |
| Run all tests            | `go test ./...`                                                       |
| Run tests with coverage  | `go test -v -race -covermode=atomic -coverprofile=coverage.out ./...` |
| Build single target APK  | `goreleaser build --single-target --snapshot`                         |
| Build all APKs           | `goreleaser build --snapshot --skip=publish --clean`                  |
| Update generated code    | `./hack/update-codegen.sh`                                            |
| Verify code generation   | `./hack/verify-codegen.sh`                                            |
| Run linters              | `golangci-lint run --fix`                                             |
| Run all pre-commit hooks | `pre-commit run --all-files`                                          |

### Image Building

| Task                     | Command                                                          |
| ------------------------ | ---------------------------------------------------------------- |
| Full image build (local) | `./packaging/scripts/build-helper.sh --with-cache --skip-clean`  |
| Build only APK packages  | `./packaging/scripts/build-helper.sh --only-goreleaser`          |
| Build rootfs base image  | `./packaging/scripts/build-helper.sh --only-build --with-cache`  |
| Build iknite-images APK  | `./packaging/scripts/build-helper.sh --only-images`              |
| Build rootfs tarball     | `./packaging/scripts/build-helper.sh --only-export`              |
| Build VM images          | `./packaging/scripts/build-vm-image.sh`                          |
| Skip specific step       | `./packaging/scripts/build-helper.sh --skip-<step> --with-cache` |
| Build APK repository     | `./packaging/scripts/build-helper.sh --only-make-apk-repo`       |

Available steps: `goreleaser`, `build`, `images`, `add-images`, `export`,
`rootfs-image`, `fetch-krmfnbuiltin`, `make-apk-repo`, `upload-repo`,
`vm-image`, `clean`

### Documentation

| Task                | Command                                           |
| ------------------- | ------------------------------------------------- |
| Build documentation | `cd docs && uv run mkdocs build --clean --strict` |
| Serve docs locally  | `cd docs && uv run mkdocs serve --livereload`     |
| Install/update deps | `cd docs && uv sync`                              |

### Cluster Management

| Task                  | Command/Path                                                                                                      |
| --------------------- | ----------------------------------------------------------------------------------------------------------------- |
| View cluster state    | `cat /run/iknite/status.json`                                                                                     |
| View kubeconfig       | `cat /root/.kube/config`                                                                                          |
| Check service status  | `rc-status`                                                                                                       |
| View service logs     | `cat /var/log/iknite.log`                                                                                         |
| APK dependencies      | [.goreleaser.yaml](../.goreleaser.yaml#L67-L79)                                                                   |
| Default kustomization | [packaging/apk/iknite/iknite.d/base/kustomization.yaml](../packaging/apk/iknite/iknite.d/base/kustomization.yaml) |
