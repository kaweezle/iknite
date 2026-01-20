# Directory Structure

This document explains the directory organization of the iknite project.

## Overview

Iknite follows a standard Go project layout with some additional directories specific to Kubernetes cluster management and Alpine Linux packaging.

## Directory Layout

```
iknite/
├── cmd/              # Application entry points
├── pkg/              # Reusable Go libraries
├── apk/              # Alpine Linux package configuration
├── rootfs/           # Root filesystem build artifacts
├── support/          # Supporting infrastructure
├── docs/             # Documentation site
├── hack/             # Development and build scripts
└── .github/          # GitHub workflows and CI/CD
```

## Core Directories

### `cmd/`
Application entry points and main packages.

- `cmd/iknite/` - Main iknite CLI application

### `pkg/`
Reusable Go packages and libraries organized by functionality:

- `alpine/` - Alpine Linux specific utilities
- `apis/` - Kubernetes API definitions
- `cmd/` - CLI command implementations
- `config/` - Configuration management
- `constants/` - Project-wide constants
- `cri/` - Container Runtime Interface (CRI) utilities
- `k8s/` - Kubernetes cluster management
- `provision/` - Cluster provisioning logic
- `testutils/` - Testing utilities
- `utils/` - General purpose utilities

## Deployment & Packaging

### `apk/`
Alpine Linux package configuration files. These files are included in the APK package:

- `buildkit/` - BuildKit configuration
- `conf.d/` - Service configuration files
- `flannel/` - Flannel CNI configuration
- `iknite.d/` - Iknite kustomization files
- `init.d/` - OpenRC init scripts
- `crictl.yaml` - CRI control tool configuration

### `rootfs/`
Root filesystem build artifacts for creating WSL distributions:

- `Dockerfile` - Base image build
- `Dockerfile.rootfs` - Complete rootfs build with iknite
- `install_images.sh` - Container image installation script
- `test-overlayfs.sh` - Filesystem testing script
- `*.rsa.pub` - APK signing public keys
- `rc.conf`, `p10k.zsh` - System configuration files

### `support/`
Supporting infrastructure and deployment configurations:

- `apk/` - APK package image manifests
  - `iknite-images.yaml` - List of container images to pre-pull
- `cloud-init/` - Cloud-init configuration templates
- `iac/` - Infrastructure as Code (Terraform, etc.)

## Development

### `hack/`
Development scripts and tools:

- `build-helper.sh` - Build automation helper
- `build-vm-image.sh` - VM image builder
- `configure-vm-image.sh` - VM image configuration
- `create-vm-nocloud-seed.sh` - Cloud-init seed creator
- `install-signing-key.sh` - APK signing key installer
- `make-rootfs-devenv.sh` - Development environment setup
- `test_start.sh` - Test cluster startup
- `tools.go` - Go tool dependencies
- `update-codegen.sh` - Kubernetes code generator
- `verify-codegen.sh` - Verify generated code
- `devcontainer/` - VS Code devcontainer configuration
- `test/` - Test fixtures and resources

### `docs/`
Documentation website built with MkDocs:

- `mkdocs.yaml` - MkDocs configuration
- `pyproject.toml` - Python dependencies
- `docs/` - Documentation content (Markdown files)
- `src/` - Additional documentation source files

## Configuration Files

### Root-level Configuration
The project root contains various tool configuration files:

- `.golangci.yml` - Go linter configuration
- `.goreleaser.yaml` - Release automation
- `.pre-commit-config.yaml` - Pre-commit hooks
- `.sops.yaml` - Secrets encryption
- `.prettierrc.json` - Code formatting
- `.shellcheckrc` - Shell script linting
- `.editorconfig` - Editor configuration
- `cspell.json` - Spell checking
- `go.mod`, `go.sum` - Go module definitions

### CI/CD
GitHub Actions workflows in `.github/workflows/`:

- `go.yaml` - Go build and test
- `docs.yaml` - Documentation build and deploy
- `release.yml` - Release automation
- `devcontainer.yaml` - Development container builds

## Build Artifacts

The following directories contain build artifacts and should not be committed:

- `dist/` - GoReleaser output
- `build/` - Build artifacts
- `.vscode/` - VS Code workspace settings (optional)

## Getting Started

1. **Building the project**: See [BUILD.md](BUILD.md) for build instructions
2. **Contributing**: See [CONTRIBUTING.md](CONTRIBUTING.md) for contribution guidelines
3. **Documentation**: Visit `docs/` or the [project website](https://kaweezle.github.io/iknite/)

## Related Files

- [BUILD.md](BUILD.md) - Build and release process
- [CONTRIBUTING.md](CONTRIBUTING.md) - Contribution guidelines
- [README.md](README.md) - Project overview
- [RELEASE.md](RELEASE.md) - Release notes
