<!-- cSpell:words testutils vhdx overlayfs releaserepo testrepo devenv -->

# Directory Structure

This document explains the directory organization of the iknite project.

## Overview

Iknite follows a standard Go project layout with some additional directories
specific to Kubernetes cluster management and Alpine Linux packaging.

## Directory Layout

```
iknite/
├── cmd/              # Application entry points
├── pkg/              # Reusable Go libraries
├── packaging/        # Alpine Linux packages and root filesystem
├── deploy/           # Deployment and infrastructure as code
├── docs/             # Documentation site
├── hack/             # Development utilities and code generation
├── test/             # Test fixtures and test scripts
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
- `provision/` - Cluster provisioning logic (Kustomize-based)
- `testutils/` - Testing utilities
- `utils/` - General purpose utilities

## Deployment & Packaging

### `packaging/`

Alpine Linux packages, root filesystem, and build scripts.

#### `packaging/apk/`

Alpine Linux package configuration files:

- `iknite/` - Main iknite package configuration
  - `buildkit/` - BuildKit configuration
  - `conf.d/` - Service configuration files
  - `flannel/` - Flannel CNI configuration
  - `iknite.d/` - Iknite kustomization files
  - `init.d/` - OpenRC init scripts
  - `crictl.yaml` - crictl to containerd socket mapping
- `iknite-images/` - Pre-pulled container images package
  - `iknite-images.yaml` - Melange configuration for images APK

#### `packaging/rootfs/`

Root filesystem build artifacts for creating WSL distributions and VM images:

- `base/` - Base Alpine rootfs with iknite
  - `Dockerfile` - Base image build configuration
  - `*.rsa.pub` - APK signing public keys
  - `rc.conf`, `p10k.zsh` - System configuration files
- `with-images/` - Complete rootfs with pre-pulled images
  - `Dockerfile` - Final image build with embedded tarball

#### `packaging/scripts/`

Build automation and helper scripts:

- `build-helper.sh` - Main build orchestration script
- `build-vm-image.sh` - VM image builder (QCOW2, VHDX)
- `configure-vm-image.sh` - VM image configuration script (chroot setup)
- `install_images.sh` - Container image installation helper
- `test-overlayfs.sh` - Filesystem testing utility

### `deploy/`

Deployment configurations and infrastructure as code:

- `iac/` - Infrastructure as Code (Terraform/Terragrunt)
  - `iknite/` - Iknite-specific infrastructure
    - `root.hcl` - Terragrunt root configuration
    - `secrets.sops.yaml` - Encrypted secrets
    - `apkrepo/` - Static APK repository website creation
    - `releaserepo/`, `testrepo/` - APK repository modules
    - `dns_iknite_app/` - DNS configuration
    - `iknite-lb/` - Load balancer configuration
    - `iknite-vm/` - VM deployment configuration
  - `modules/` - Reusable Terraform modules
    - `object-store-sync/` - Object storage synchronization
    - `openstack-vm/` - OpenStack VM provisioning
    - `public-object-store/` - Public object storage setup

## Development

### `hack/`

Development utilities and code generation:

- `make-rootfs-devenv.sh` - Development environment setup
- `tools.go` - Go tool dependencies
- `update-codegen.sh` - Kubernetes code generator
- `verify-codegen.sh` - Verify generated code
- `boilerplate.go.txt` - Go file header template
- `custom-boilerplate.go.txt` - Custom boilerplate template
- `devcontainer/` - VS Code devcontainer configuration
  - `Dockerfile` - Dev container image
  - `buildkitd.toml` - BuildKit configuration
  - `p10k.zsh`, `rc.conf` - Shell and system configuration
- `iknitedev/` - Development utilities package
  - `cmd/` - CLI commands for development tasks
    - `install_signing_key.go` - APK signing key installer

### `test/`

Test fixtures, resources, and test scripts:

- `ops/` - Operational test resources
  - `nginx/` - Example nginx deployment
- `vm/` - VM testing resources
  - `cloud-init/` - Cloud-init configuration templates
  - `scripts/` - VM test scripts

### `docs/`

Documentation website built with MkDocs:

- `mkdocs.yaml` - MkDocs configuration
- `pyproject.toml` - Python dependencies (for `uv install`)
- `docs/` - Documentation content (Markdown files)
- `src/` - Additional documentation source files

## Configuration Files

### Root-level Configuration

The project root contains various tool configuration files:

- `.golangci.yml` - Go linter configuration
- `.goreleaser.yaml` - Release automation
- `.pre-commit-config.yaml` - Pre-commit hooks
- `.sops.yaml` - Secrets encryption configuration
- `.prettierrc.json` - Code formatting
- `.shellcheckrc` - Shell script linting
- `.editorconfig` - Editor configuration
- `cspell.json` - Spell checking
- `go.mod`, `go.sum`, `go.work`, `go.work.sum` - Go module and workspace
  definitions
- `.vscode/` - VS Code workspace settings (optional)

### CI/CD

GitHub Actions workflows in `.github/workflows/`:

- `go.yaml` - Go build and test
- `docs.yaml` - Documentation build and deploy
- `release.yml` - Release automation
- `devcontainer.yaml` - Development container builds

## Build Artifacts

The following directories contain build artifacts and should not be committed:

- `build/` - Build artifacts
- `dist/` - Distribution packages - e.g., alpine packages, root filesystem
  tarballs, VM images

## Getting Started

1. **Building the project**: See [BUILD.md](BUILD.md) for build instructions
2. **Contributing**: See [CONTRIBUTING.md](CONTRIBUTING.md) for contribution
   guidelines
3. **Documentation**: Visit `docs/` or the
   [project website](https://kaweezle.github.io/iknite/)

## Related Files

- [BUILD.md](BUILD.md) - Build and release process
- [CONTRIBUTING.md](CONTRIBUTING.md) - Contribution guidelines
- [README.md](README.md) - Project overview
- [RELEASE.md](RELEASE.md) - Release notes
