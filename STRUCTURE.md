<!-- cSpell:words argocd checkmake codegen devcontainer iknitedev releaserepo testutils vhdx -->
<!-- cSpell:words overlayfs testrepo devenv prds venv dockerimage -->

# Directory Structure

This document describes the current repository layout of the iknite project. The
repository follows a typical Go CLI structure, with additional directories for
Alpine packaging, Kubernetes assets, documentation, and infrastructure.

## Overview

At the top level, the repository is organized around the CLI source code,
packaging and image-building assets, infrastructure definitions, documentation,
and development tooling.

## Directory Layout

```
iknite/
├── .devcontainer/    # Root dev container entrypoint for VS Code
├── .github/          # Workflows, issue templates, and agent instructions
├── .vscode/          # Workspace tasks and editor settings
├── build/            # Generated build inputs and staging assets
├── cmd/              # Application entry points
├── deploy/           # Kubernetes assets and Terraform/Terragrunt config
├── docs/             # MkDocs site, specs, and doc tooling
├── hack/             # Developer utilities and code generation helpers
├── packaging/        # APK packaging, rootfs images, and VM assets
├── pkg/              # Main reusable Go packages
├── test/             # Test scripts and supporting fixtures
└── dist/             # Generated release artifacts
```

## Core Directories

### `cmd/`

Application entry points.

- `cmd/iknite/` - main CLI binary entrypoint

### `pkg/`

Reusable Go packages organized by domain:

- `pkg/alpine/` - Alpine-specific helpers such as networking and services
- `pkg/apis/` - API types and generated Kubernetes-style objects
- `pkg/cmd/` - Cobra command implementations for the CLI
- `pkg/config/` - configuration loading and defaults
- `pkg/constants/` - project-wide constants
- `pkg/cri/` - container runtime helpers
- `pkg/k8s/` - Kubernetes runtime, readiness, and lifecycle logic
- `pkg/provision/` - provisioning helpers and embedded base assets
- `pkg/server/` - HTTP or service-facing helpers used by the project
- `pkg/testutils/` - testing support code and mocks
- `pkg/utils/` - general shared utilities

Notable subtrees:

- `pkg/apis/iknite/v1alpha1/` - versioned API types and generated files
- `pkg/k8s/phases/init/` - custom kubeadm init phase logic
- `pkg/cmd/options/` - shared Cobra option helpers

## Packaging And Deployment

### `packaging/`

Assets used to build Alpine packages, root filesystems, and VM images.

#### `packaging/apk/`

APK package definitions and contents:

- `packaging/apk/iknite/` - main iknite package contents
- `packaging/apk/iknite-images/` - image preload APK definition
- `packaging/apk/incus-agent/` - Incus agent packaging assets

The main iknite APK currently includes:

- `buildkit/` - BuildKit-related configuration
- `conf.d/` - OpenRC service configuration files
- `flannel/` - Flannel networking assets
- `iknite.d/` - default kustomization content
- `init.d/` - OpenRC init scripts
- `crictl.yaml` - CRI client configuration

#### `packaging/rootfs/`

Root filesystem image definitions:

- `packaging/rootfs/base/` - base Alpine rootfs image setup
- `packaging/rootfs/with-images/` - rootfs image with imported Kubernetes images

#### `packaging/scripts/`

Packaging and image build scripts:

- `build-vm-image.sh` - VM image creation
- `configure-vm-image.sh` - VM image customization in chroot
- `install_images.sh` - image import helper
- `test-overlayfs.sh` - overlay filesystem validation helper

#### `packaging/vm/`

VM metadata and templates:

- `metadata.yaml.tmpl` - base metadata template
- `templates/` - supporting VM template files

### `deploy/`

Deployment assets for both Kubernetes manifests and infrastructure.

#### `deploy/k8s/`

Cluster-side assets:

- `argocd/` - Argo CD-related manifests
- `container-images/` - image-related deployment assets
- `hack/` - support scripts or helpers for deployment workflows

#### `deploy/iac/`

Infrastructure as code definitions.

- `deploy/iac/iknite/` - environment-specific Terragrunt stacks
- `deploy/iac/modules/` - reusable Terraform and Terragrunt modules
- `deploy/iac/README.md` - infrastructure documentation

Current `deploy/iac/iknite/` subdirectories include:

- `acme/` - ACME-related configuration
- `apkrepo/` - APK repository publishing setup
- `dns_iknite_app/` - DNS configuration for the iknite domain
- `github-configuration/` - GitHub repository automation/config
- `iknite-argocd/` - Argo CD deployment stack
- `iknite-argocd-state/` - Argo CD state storage/config
- `iknite-image/` - image publishing or image metadata stack
- `iknite-kubeconfig-fetcher/` - kubeconfig retrieval stack
- `iknite-kubernetes-state/` - Kubernetes state storage/config
- `iknite-public-images/` - public image publication stack
- `iknite-vm/` - VM deployment stack
- `releaserepo/` - release repository publication
- `testrepo/` - test repository publication
- `root.hcl` - shared Terragrunt root configuration

Current `deploy/iac/modules/` subdirectories include:

- `acme/`
- `dns_cloudflare/`
- `github-configuration/`
- `helmfile-deploy/`
- `kubeconfig-fetcher/`
- `kubernetes-state/`
- `object-store-sync/`
- `openstack-image/`
- `openstack-vm/`
- `public-object-store/`

## Development Tooling

### `hack/`

Development helpers, generator scripts, and local tooling:

- `boilerplate.go.txt` - code generation boilerplate
- `custom-boilerplate.go.txt` - project-specific boilerplate
- `build-container-image.sh` - local container image build helper
- `make-rootfs-devenv.sh` - rootfs-based development environment helper
- `tools.go` - Go tool dependency tracking
- `update-codegen.sh` - regenerate Kubernetes-style code
- `verify-codegen.sh` - verify generated code is up to date
- `devcontainer/` - devcontainer build context and supporting config
- `iknitedev/` - auxiliary development CLI module

### `test/`

Test scripts and supporting fixtures:

- `test/e2e/` - end-to-end test helpers such as `argocd-checker.sh`
- `test/ops/` - operational examples such as `nginx/`
- `test/vm/` - VM testing resources, including `cloud-init/` and `scripts/`

### `docs/`

Documentation sources and tooling.

- `docs/mkdocs.yaml` - MkDocs configuration
- `docs/pyproject.toml` - Python dependency manifest for docs tooling
- `docs/uv.lock` - locked dependency set for docs tooling
- `docs/docs/` - published Markdown content and site assets
- `docs/specs/` - documentation specifications, currently `prds/`
- `docs/src/` - additional documentation source material
- `docs/.venv/` - local virtual environment for docs work

## Repository Metadata And Configuration

### Root-Level Files And Folders

Key repository-wide configuration currently present at the root includes:

- `.editorconfig` - editor defaults
- `.golangci.yml` - Go lint configuration
- `.goreleaser.yaml` - release and package build configuration
- `.pre-commit-config.yaml` - pre-commit hooks
- `.prettierrc.json` - formatting configuration
- `.shellcheckrc` - shell lint configuration
- `.sops.yaml` - SOPS configuration
- `aqua.yaml` - tool installation manifest
- `checkmake.ini` - `checkmake` configuration
- `cspell.json` - spell checker configuration
- `go.mod`, `go.sum` - main Go module metadata
- `go.work`, `go.work.sum` - Go workspace metadata
- `GNUmakefile` - make targets
- `README.md`, `BUILD.md`, `CONTRIBUTING.md`, `RELEASE.md`, `STRUCTURE.md` -
  top-level documentation

### `.github/`

GitHub-specific automation and contributor support files:

- `ISSUE_TEMPLATE/` - issue templates
- `actions/` - reusable GitHub Actions
- `instructions/` - agent instruction files used in this repository
- `workflows/` - CI and release workflows
- `copilot-instructions.md` - repository guidance for AI agents
- `dependabot.yaml` - dependency update automation
- `pull_request_template.md` - PR template
- `release.yml` - release metadata or automation configuration

Current workflows in `.github/workflows/` include:

- `build_push_dockerimage.yaml`
- `devcontainer.yaml`
- `docs.yaml`
- `go.yaml`
- `release.yml`
- `test-e2e.yml`

## Build Artifacts

The following paths are used for generated or local build output:

- `build/` - staged build assets, including `build/e2e/`
- `dist/` - generated packages, images, and release artifacts
- `coverage.out` - local Go coverage output when tests are run with coverage

## Getting Started

1. See [BUILD.md](BUILD.md) for build instructions.
2. See [CONTRIBUTING.md](CONTRIBUTING.md) for contribution guidelines.
3. See [README.md](README.md) for a high-level project overview.
4. See [docs/](docs/) for the documentation source tree.

## Related Files

- [README.md](README.md) - project overview
- [BUILD.md](BUILD.md) - build and packaging workflows
- [CONTRIBUTING.md](CONTRIBUTING.md) - contribution process
- [RELEASE.md](RELEASE.md) - release checklist and notes
