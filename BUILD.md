<!-- cSpell: words libstdc doas socat skopeo tenv rootlesskit slirp devenv signingkey gpgsign sopsdiffer textconv -->
<!-- cSpell: words covermode coverprofile vhdx mycluster gofmt rootfull -->

# Building Iknite

This guide explains how to build Iknite from source, including all dependencies,
build tasks, and workflow steps.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Development Environment](#development-environment)
- [Build Tasks](#build-tasks)
- [Full Build Pipeline](#full-build-pipeline)
- [Starting Iknite](#starting-iknite)
- [Testing](#testing)

## Prerequisites

### System Requirements

- **Operating System**: Linux (Alpine Linux recommended for development)
- **Architecture**: x86_64 (amd64)
- **Container Runtime**: Docker or Containerd with buildkit support

### Required Tools

#### Core Development Tools

```bash
# Base utilities
apk add zsh tzdata git libstdc++ doas iproute2 gnupg socat openssh openrc curl tar zstd

# Development tools
apk add pipx uv go jq skopeo tenv kubectl k9s golangci-lint sops age nodejs npm openssl abuild
```

#### Kubernetes Tools

```bash
apk add ip6tables containerd kubelet kubeadm cni-plugins cni-plugin-flannel \
        util-linux-misc buildkit buildctl nerdctl rootlesskit slirp4netns
```

#### Build Tools

- **goreleaser**: Latest version (automatically fetched)

  ```bash
  GORELEASER_VERSION=$(curl --silent https://api.github.com/repos/goreleaser/goreleaser/releases/latest | jq -r .tag_name | sed -e 's/^v//')
  wget -O /tmp/goreleaser.apk "https://github.com/goreleaser/goreleaser/releases/download/v${GORELEASER_VERSION}/goreleaser_${GORELEASER_VERSION}_x86_64.apk"
  apk add --allow-untrusted /tmp/goreleaser.apk
  ```

- **tflint** v0.60.0

  ```bash
  wget -O /tmp/tflint.zip "https://github.com/terraform-linters/tflint/releases/download/v0.60.0/tflint_linux_amd64.zip"
  unzip /tmp/tflint.zip -d /usr/local/bin/
  ```

- **terraform-docs** v0.21.0

  ```bash
  wget -O /tmp/terraform-docs.tar.gz "https://github.com/terraform-docs/terraform-docs/releases/download/v0.21.0/terraform-docs-v0.21.0-linux-amd64.tar.gz"
  tar -xzf /tmp/terraform-docs.tar.gz -C /usr/local/bin/ terraform-docs
  ```

- **pre-commit**
  ```bash
  pipx install pre-commit
  ```

### Signing Key

The project requires a signing key for APK packages. Ensure you have
`kaweezle-devel@kaweezle.com-c9d89864.rsa` in the project root.

To install the signing key:

```bash
go run hack/iknitedev/ install signing-key deploy/iac/iknite/secrets.sops.yaml .
```

### SOPS/Age Configuration

For encrypted secrets, install the age key:

```bash
mkdir -p ~/.config/sops/age
# Copy your age key to ~/.config/sops/age/keys.txt
```

## Development Environment

### Using the Devcontainer

The easiest way to get started is using the provided devcontainer:

```bash
# Open the project in VS Code with the Remote-Containers extension
# The devcontainer will automatically set up all dependencies
```

The devcontainer is based on Alpine Linux and includes all necessary tools
pre-installed.

### Manual Setup on Alpine

If you're setting up manually on Alpine Linux, run:

```bash
./hack/make-rootfs-devenv.sh
```

This script will:

1. Install all required packages
2. Configure the development environment
3. Install pre-commit hooks

### Git Configuration

Configure git for signed commits:

```gitconfig
[user]
    signingkey = YOUR_GPG_KEY
    name = Your Name
    email = your.email@example.com
[gpg]
    program = /usr/bin/gpg
[init]
    defaultBranch = main
[commit]
    gpgsign = true
[core]
    editor = "code -r --wait"
[diff "sopsdiffer"]
    textconv = "sops --decrypt --config /dev/null"
```

## Build Tasks

### Quick Build Tasks

The project provides several VS Code tasks (defined in `.vscode/tasks.json`):

#### 1. Format and Lint

```bash
# Run golangci-lint with auto-fix
golangci-lint run --fix

# Format code
golangci-lint fmt

# Tidy go modules
go mod tidy

# Run all pre-commit hooks
pre-commit run --all-files
```

**VS Code Tasks:**

- `golangci-lint` - Lint with auto-fix (Test task)
- `golangci-fmt` - Format code (Build task)
- `go mod tidy` - Tidy dependencies (Build task)
- `pre-commit` - Run pre-commit hooks (Default test task)

#### 2. Build Binary

```bash
# Build single target binary (local architecture)
goreleaser build --single-target --auto-snapshot --clean

# Build all architectures (snapshot mode)
goreleaser build --snapshot --skip=publish --clean
```

**VS Code Tasks:**

- `goreleaser-build` - Build single target (Default build task - `Ctrl+Shift+B`)
- `goreleaser` - Build all architectures

#### 3. Run Tests

```bash
# Run tests with coverage
go test -v -race -covermode=atomic -coverprofile=coverage.out ./...
```

**VS Code Task:**

- `test with coverage` - Run full test suite with coverage

### Running Individual Tasks

In VS Code:

1. Press `Ctrl+Shift+P`
2. Type "Tasks: Run Task"
3. Select the desired task

Or use keyboard shortcuts:

- `Ctrl+Shift+B` - Run default build task (goreleaser-build)
- `Ctrl+Shift+T` - Run default test task (pre-commit)

## Full Build Pipeline

The project uses `packaging/scripts/build-helper.sh` for the complete build
pipeline. This script orchestrates building the APK packages, root filesystem
images, and VM images.

### Build Helper Usage

```bash
./packaging/scripts/build-helper.sh [OPTIONS]
```

#### Options

- `-h, --help` - Show help message
- `--rootless` - Use rootless containerd (skip doas/sudo)
- `--skip-<step>` - Skip a specific build step
- `--only-<step>` - Run only the specified step (skip all others)
- `--with-cache` - Use cache for docker builds (default: no cache)
- `--release` - Build release version (default: snapshot)

#### Build Steps

1. **goreleaser** - Build Iknite APK package with goreleaser
2. **build** - Build Iknite rootfs base image
3. **images** - Build iknite-images APK package (pre-pulled container images)
4. **add-images** - Add images to rootfs container
5. **export** - Export rootfs tarball
6. **rootfs-image** - Build final rootfs Docker image
7. **fetch-krmfnbuiltin** - Fetch krmfnbuiltin APKs
8. **make-apk-repo** - Create APK repository in dist/repo
9. **upload-repo** - Upload APK repository to https://static.iknite.app/
10. **vm-image** - Build VM images (qcow2, vhdx)
11. **clean** - Cleanup temporary files

### Common Build Workflows

#### Full Build (Recommended for Development)

```bash
./packaging/scripts/build-helper.sh --with-cache --skip-clean
```

This runs all steps with caching enabled and skips cleanup for faster iteration.

#### Build Only the Binary

```bash
./packaging/scripts/build-helper.sh --only-goreleaser
```

#### Build Root Filesystem Image

```bash
./packaging/scripts/build-helper.sh --only-build --with-cache
```

#### Skip Specific Steps

```bash
# Skip image building (faster for testing)
./packaging/scripts/build-helper.sh --skip-images --with-cache
```

#### Build for Release

```bash
./packaging/scripts/build-helper.sh --release
```

This builds without snapshot mode and sets the repository to `release` instead
of `test`.

### Environment Variables

- `KUBERNETES_VERSION` - Override Kubernetes version (default: from go.mod)
- `KEY_NAME` - Signing key filename (default:
  kaweezle-devel@kaweezle.com-c9d89864.rsa)
- `BUILDKIT_HOST` - For rootless builds:
  `unix:///run/user/$UID/buildkit/buildkitd.sock`

### Build Artifacts

After a successful build, artifacts are located in:

- `dist/` - Main output directory
  - `iknite-<version>.x86_64.apk` - Main APK package
  - `iknite-images-<k8s-version>.x86_64.apk` - Pre-pulled images APK
  - `iknite-<version>-<k8s-version>.rootfs.tar.gz` - Root filesystem tarball
  - `iknite-vm.<version>-<k8s-version>.qcow2` - QEMU/KVM VM image
  - `iknite-vm.<version>-<k8s-version>.vhdx` - Hyper-V VM image
  - `repo/` - APK repository with index
  - `SHA256SUMS` - Checksums for all artifacts

## Starting Iknite

Once built, you can start an Iknite cluster in several ways:

### From APK Package

```bash
# Add the Kaweezle APK repository
KEY_URL=$(curl -s https://api.github.com/repos/kaweezle/iknite/releases/latest | \
          grep "browser_download_url.*rsa.pub" | cut -d '"' -f 4 | sed 's/%40/@/g')
wget -q -P /etc/apk/keys "${KEY_URL}"

# For testing pre-releases
echo https://static.iknite.app/test/ >> /etc/apk/repositories

# For stable releases
echo https://static.iknite.app/release/ >> /etc/apk/repositories

# Install dependencies
apk --update add krmfnbuiltin k9s openssl nerdctl

# Install iknite
apk add iknite iknite-images
```

### From Local Build

```bash
# Install the locally built APK
apk add --allow-untrusted --no-cache ./dist/iknite-*.x86_64.apk
apk add --allow-untrusted --no-cache ./dist/iknite-images-*.x86_64.apk
```

### Initialize and Start

```bash
# Start OpenRC (if not already running)
openrc -n default

# Start iknite with debug logging and 120-second workload wait timeout
iknite -v debug -w 120 start
```

The `iknite start` command will:

1. Initialize the host environment (IP addresses, networking, OpenRC services)
2. Start the iknite service that will:
   1. Run the equivalent (embedded) of `kubeadm init` if needed
   2. Start the kubelet
   3. Apply default kustomizations (networking, storage, metrics)
   4. Start the mdns responder on WSL
   5. Daemonize and monitor the cluster state
3. Wait for all workloads to be ready
4. Keep running to maintain the kubelet process

### Common Options

```bash
# Specify custom IP address
iknite --ip 192.168.99.2 start

# Use custom domain name
iknite --domain-name mycluster.local start

# Custom kustomization directory
iknite --kustomization /path/to/kustomizations start

# See all available options
iknite start --help
```

## Testing

### Unit Tests

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests with race detection and coverage
go test -v -race -covermode=atomic -coverprofile=coverage.out ./...

# View coverage report
go tool cover -html=coverage.out
```

### Integration Tests

Integration tests require a running containerd and buildkit:

```bash
# Ensure containerd is running
rc-service containerd start

# Ensure buildkit is running
rc-service buildkitd start

# Run the full build pipeline as a test
./packaging/scripts/build-helper.sh --with-cache --skip-upload-repo
```

### Pre-commit Hooks

Run all quality checks before committing:

```bash
# Run all pre-commit hooks
pre-commit run --all-files

# Install hooks for automatic checks
pre-commit install
```

The hooks include:

- Code formatting (gofmt, golangci-lint)
- Shell script validation (shellcheck)
- YAML validation
- Spell checking (cspell)
- Terraform validation

## Troubleshooting

### Build Fails with "No available nbd device"

If building VM images fails, ensure the `nbd` kernel module is loaded:

```bash
modprobe nbd max_part=16
```

### buildkit Not Available

For rootless builds on systemd based systems, ensure buildkit is running:

```bash
# Check if buildkit is running
systemctl --user status buildkit

# Start buildkit
systemctl --user start buildkit

# Set environment variable
export BUILDKIT_HOST=unix:///run/user/$UID/buildkit/buildkitd.sock
```

### Signing Key Issues

Ensure the signing key is present:

```bash
ls -l kaweezle-devel@kaweezle.com-c9d89864.rsa
```

If missing, install it from the secrets file:

```bash
go run hack/iknitedev/ install signing-key deploy/iac/iknite/secrets.sops.yaml .
```

### Container Image Pull Failures

If building iknite-images fails with image pull errors, check:

1. Internet connectivity
2. DNS resolution
3. Containerd configuration: `/etc/containerd/config.toml`

### Permission Denied Errors

If you see permission errors:

```bash
# For rootless builds, ensure proper permissions
chown -R $(id -u):$(id -g) dist/ build/

# For rootfull builds, use sudo/doas
doas ./packaging/scripts/build-helper.sh
```

## Additional Resources

- [Contributing Guide](CONTRIBUTING.md) - Guidelines for contributing to the
  project
- [Project Structure](STRUCTURE.md) - Detailed project structure documentation
- [Copilot Instructions](.github/copilot-instructions.md) - AI agent development
  guide
- [Release Workflow](.github/workflows/release.yml) - CI/CD pipeline
  documentation

## Quick Reference

| Command                                                               | Description                      |
| --------------------------------------------------------------------- | -------------------------------- |
| `goreleaser build --single-target --auto-snapshot --clean`            | Build binary only                |
| `go test -v -race -covermode=atomic -coverprofile=coverage.out ./...` | Run tests with coverage          |
| `pre-commit run --all-files`                                          | Run all quality checks           |
| `./packaging/scripts/build-helper.sh --with-cache --skip-clean`       | Full build with caching          |
| `./packaging/scripts/build-helper.sh --only-goreleaser`               | Build APK package only           |
| `iknite -v debug start`                                               | Start cluster with debug logging |
