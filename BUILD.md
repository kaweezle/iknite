# Building Iknite

This document describes how to build, test, and release iknite.

## Prerequisites

### Required Tools

- [Go](https://golang.org/) 1.24.0 or later
- [GoReleaser](https://goreleaser.com/) for releases
- [Docker](https://www.docker.com/) for building container images
- [Git](https://git-scm.com/) for version control

### Optional Tools

- [Pre-commit](https://pre-commit.com/) for git hooks
- [golangci-lint](https://golangci-lint.run/) for code linting
- [shellcheck](https://www.shellcheck.net/) for shell script linting

## Quick Start

### Building the Binary

Build the iknite binary for your current platform:

```bash
go build -o iknite ./cmd/iknite
```

### Running Tests

Run unit tests:

```bash
go test ./...
```

Run tests with coverage:

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Running Locally

Run iknite in development mode:

```bash
go run ./cmd/iknite start --help
```

## Build Process

### Local Development Build

For local development and testing:

```bash
# Build for current OS/architecture
go build -o dist/iknite ./cmd/iknite

# Run the binary
./dist/iknite --version
```

### Cross-Platform Builds with GoReleaser

Build for all platforms defined in `.goreleaser.yaml`:

```bash
# Test the build without publishing
goreleaser build --snapshot --clean

# Build with specific Kubernetes version
KUBERNETES_VERSION=1.34.3 goreleaser build --snapshot --clean
```

The build produces:

- Binary archives in `dist/`
- Alpine APK packages in `dist/`
- Checksums in `dist/SHA256SUMS`

### Build Targets

GoReleaser builds the following targets (see `.goreleaser.yaml`):

- **Linux 386**: `iknite_Linux_i386.tar.gz`
- **Linux amd64**: `iknite_Linux_x86_64.tar.gz`
- **APK packages**: `iknite-<version>.x86_64.apk`, `iknite-<version>.i386.apk`

### Build Configuration

The build is configured via `.goreleaser.yaml`:

- **Binary name**: `iknite`
- **Install path**: `/sbin/iknite` (for APK)
- **Build flags**: `-trimpath` for reproducible builds
- **LDflags**: Embeds version, commit, and build date

Version information is injected at build time:

```go
// pkg/cmd/version.go
IkniteVersion = "v1.2.3"          // From git tag
Commit = "abc123"                 // Git commit hash
BuildDate = "2024-01-01"          // Build timestamp
KubernetesVersionDefault = "1.34.3" // From KUBERNETES_VERSION env
```

## APK Package

### Package Contents

The Alpine APK package includes:

- Binary: `/sbin/iknite`
- Configuration files from `apk/`:
  - `/etc/crictl.yaml` - CRI control config
  - `/etc/iknite.d/` - Kustomization files
  - `/etc/init.d/iknite` - OpenRC init script
  - `/etc/conf.d/iknite` - Service configuration
  - `/etc/cni/net.d/10-flannel.conflist` - Flannel CNI config
  - `/lib/iknite/flannel/subnet.env` - Flannel subnet config
  - `/etc/buildkit/buildkitd.toml` - BuildKit config

### Package Dependencies

The APK depends on the following Alpine packages:

- `openrc` - Init system
- `containerd` - Container runtime
- `kubelet`, `kubeadm`, `kubectl` - Kubernetes components
- `cni-plugins`, `cni-plugin-flannel` - Network plugins
- `buildkit`, `buildctl`, `nerdctl` - Build tools
- `git`, `openssh`, `util-linux-misc` - Utilities

### Building APK Locally

The APK is built as part of the GoReleaser process:

```bash
goreleaser release --snapshot --clean
```

APK packages will be in `dist/`:
- `iknite-<version>.x86_64.apk`
- `iknite-<version>.i386.apk`

### APK Signing

APKs are signed with the key: `kaweezle-devel@kaweezle.com-c9d89864.rsa`

The public key is included in releases for APK repository setup.

## Root Filesystem Images

### Building the Base Image

Build the base Alpine image with iknite:

```bash
cd rootfs
docker build -f Dockerfile -t iknite-base .
```

### Building the Complete Rootfs

Build the complete WSL root filesystem:

```bash
cd rootfs
docker build -f Dockerfile.rootfs -t kaweezle-rootfs .
```

This includes:
- Alpine Linux base
- iknite APK package
- Pre-pulled Kubernetes images
- Development tools (zsh, p10k)

### Exporting the Rootfs

Export for WSL distribution:

```bash
docker run --rm kaweezle-rootfs tar -czf - / > kaweezle.rootfs.tar.gz
```

## Development Scripts

The `hack/` directory contains development and build helper scripts:

### Build Scripts

- `build-helper.sh` - General build automation
- `build-vm-image.sh` - Build VM images for testing
- `configure-vm-image.sh` - Configure built VM images

### Development Scripts

- `make-rootfs-devenv.sh` - Create a development environment rootfs
- `install-signing-key.sh` - Install APK signing key for local testing

### Testing Scripts

- `test_start.sh` - Test cluster startup
- `test/` - Test fixtures and Kubernetes manifests

### Code Generation

- `update-codegen.sh` - Generate Kubernetes client code
- `verify-codegen.sh` - Verify generated code is up-to-date

Run code generation:

```bash
./hack/update-codegen.sh
```

Verify generated code:

```bash
./hack/verify-codegen.sh
```

## Testing

### Unit Tests

Run all unit tests:

```bash
go test ./...
```

Run tests with verbose output:

```bash
go test -v ./...
```

Run specific package tests:

```bash
go test ./pkg/alpine/...
go test ./pkg/k8s/...
```

### Integration Tests

Test cluster startup (requires Alpine Linux or WSL):

```bash
./hack/test_start.sh
```

### Test Coverage

Generate and view coverage:

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

View coverage by package:

```bash
go test -coverprofile=coverage.out ./... && \
go tool cover -func=coverage.out
```

## Linting and Code Quality

### Go Linting

Run golangci-lint:

```bash
golangci-lint run
```

Auto-fix issues:

```bash
golangci-lint run --fix
```

Configuration: `.golangci.yml`

### Shell Script Linting

Check shell scripts with shellcheck:

```bash
find . -name "*.sh" -exec shellcheck {} +
```

Configuration: `.shellcheckrc`

### Pre-commit Hooks

Install pre-commit hooks:

```bash
pre-commit install
```

Run manually:

```bash
pre-commit run --all-files
```

Configuration: `.pre-commit-config.yaml`

## Release Process

### Automated Releases

Releases are automated via GitHub Actions (`.github/workflows/release.yml`):

1. Tag a new version: `git tag v1.2.3`
2. Push the tag: `git push origin v1.2.3`
3. GitHub Actions runs GoReleaser
4. Binaries, APKs, and rootfs are published to GitHub Releases

### Manual Release

Create a release locally:

```bash
# Ensure clean working directory
git status

# Create and push tag
git tag -a v1.2.3 -m "Release v1.2.3"
git push origin v1.2.3

# Build locally (optional, for testing)
GITHUB_TOKEN=<token> goreleaser release --clean
```

### Release Artifacts

Each release includes:

- Binary archives for Linux (386, amd64)
- Alpine APK packages (signed)
- Root filesystem tarball (`kaweezle.rootfs.tar.gz`)
- APK signing public key
- SHA256 checksums
- Changelog

### Version Management

Version is determined from git tags:

- Release: `v1.2.3` (from tag)
- Development: `v1.2.4-devel` (incremented with `-devel` suffix)

## CI/CD Workflows

### GitHub Actions

The project uses several workflows:

- **go.yaml** - Build and test on every push
- **release.yml** - Build and publish releases on tags
- **docs.yaml** - Build and deploy documentation
- **devcontainer.yaml** - Build development container

### Environment Variables

Key environment variables:

- `KUBERNETES_VERSION` - Kubernetes version to embed (default: 1.34.3)
- `GITHUB_TOKEN` - GitHub API token (for releases)
- `CGO_ENABLED=0` - Static binary compilation

## Troubleshooting

### Build Failures

**Module checksum errors:**
```bash
go clean -modcache
go mod download
```

**GoReleaser errors:**
```bash
goreleaser check
goreleaser build --snapshot --clean
```

### Test Failures

**Missing test dependencies:**
```bash
go mod download
```

**Permission errors in tests:**
```bash
# Tests may require root/sudo on Linux
sudo go test ./...
```

### APK Build Issues

**Missing signing key:**
```bash
# Generate a new key pair
openssl genrsa -out key.rsa 2048
openssl rsa -in key.rsa -pubout -out key.rsa.pub
```

## Additional Resources

- [STRUCTURE.md](STRUCTURE.md) - Project directory structure
- [CONTRIBUTING.md](CONTRIBUTING.md) - Contribution guidelines
- [GoReleaser Documentation](https://goreleaser.com/)
- [Alpine APK Format](https://wiki.alpinelinux.org/wiki/Alpine_package_format)

## Getting Help

- Open an issue: [GitHub Issues](https://github.com/kaweezle/iknite/issues)
- Discussion: [GitHub Discussions](https://github.com/kaweezle/iknite/discussions)
