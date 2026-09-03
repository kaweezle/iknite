<!-- cSpell: words libstdc socat skopeo tenv rootlesskit slirp devenv testutils -->

!!! wip "Work in progress"

    This documentation is in draft form and may change frequently.

# Developer Getting Started

This page explains how to set up a development environment for contributing to
Iknite.

## Prerequisites

### System Requirements

- **Operating System**: Linux (Alpine Linux recommended) or Windows with WSL2
- **Architecture**: x86_64 (amd64)
- **RAM**: 8 GB minimum (16 GB recommended for running the cluster locally)
- **Disk**: 20 GB+ free space

### Required Tools

#### Core Tools

```bash
# Alpine Linux
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

```bash
# Install goreleaser (latest version)
GORELEASER_VERSION=$(curl --silent \
  https://api.github.com/repos/goreleaser/goreleaser/releases/latest | \
  jq -r .tag_name | sed -e 's/^v//')
wget -O /tmp/goreleaser.apk \
  "https://github.com/goreleaser/goreleaser/releases/download/v${GORELEASER_VERSION}/goreleaser_${GORELEASER_VERSION}_x86_64.apk"
apk add --allow-untrusted /tmp/goreleaser.apk

# Install pre-commit
pipx install pre-commit
```

## Using the Devcontainer

The easiest setup path is using the provided VS Code devcontainer:

### Prerequisites

- [VS Code](https://code.visualstudio.com/)
- [Dev Containers extension](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers)
- Docker or Iknite running

### Open in Devcontainer

1. Clone the repository:

   ```bash
   git clone https://github.com/kaweezle/iknite.git
   cd iknite
   ```

2. Open in VS Code:

   ```bash
   code .
   ```

3. Press `F1` → **Dev Containers: Reopen in Container**

The devcontainer is based on Alpine Linux and pre-installs all dependencies
automatically.

### Devcontainer Features

The devcontainer (`.devcontainer/`) includes:

- Go development environment
- All required Alpine packages
- Pre-commit hooks
- VS Code extensions for Go development
- BuildKit for image building

## Manual Setup on Alpine Linux

If you're working directly on Alpine Linux (or the Iknite WSL distribution):

### 1. Clone the Repository

```bash
git clone https://github.com/kaweezle/iknite.git
cd iknite
```

### 2. Run the Development Setup Script

```bash
./hack/make-rootfs-devenv.sh
```

This script installs all required packages and configures the development
environment.

### 3. Install Pre-commit Hooks

```bash
pre-commit install
```

### 4. Configure the Signing Key (for APK builds)

APK package builds require a signing key. Contact the maintainers to get the
signing key, or use a development key:

```bash
go run hack/iknitedev/ install signing-key secrets.sops.yaml .
```

### 5. Configure SOPS (for encrypted secrets)

```bash
mkdir -p ~/.config/sops/age
# Copy your age key to ~/.config/sops/age/keys.txt
```

## Setting Up Docker Build Environment

Iknite requires a Docker or containerd build environment for building APK
packages and container images.

### Using containerd (recommended on Alpine)

```bash
# Start containerd
rc-service containerd start
rc-service buildkitd start

# Verify
nerdctl version
buildctl --version
```

### Using Docker (alternative)

```bash
# Install Docker (Debian/Ubuntu)
curl -fsSL https://get.docker.com | sh

# Verify
docker version
```

## Verifying the Setup

```bash
# Build the binary
goreleaser build --single-target --auto-snapshot --clean

# Run tests
go test ./...

# Check linting
golangci-lint run

# Run pre-commit hooks
pre-commit run --all-files
```

## Project Structure Overview

```
iknite/
├── cmd/iknite/       ← Main CLI entry point
├── pkg/              ← Go packages
│   ├── alpine/       ← Alpine Linux utilities
│   ├── apis/         ← Kubernetes-style API types
│   ├── cmd/          ← Cobra command implementations
│   ├── config/       ← Configuration management
│   ├── constants/    ← Project-wide constants
│   ├── cri/          ← Container Runtime Interface utilities
│   ├── k8s/          ← Kubernetes interaction layer
│   ├── provision/    ← Bootstrap kustomization
│   ├── server/       ← Status HTTPS server
│   └── testutils/    ← Test utilities
├── packaging/        ← Alpine packages, rootfs, scripts
│   ├── apk/iknite/   ← Main iknite APK config
│   └── rootfs/       ← Container/VM images
├── docs/             ← Documentation (MkDocs)
├── deploy/iac/       ← Terraform/Terragrunt infrastructure
└── hack/             ← Development utilities
```

See [STRUCTURE.md](https://github.com/kaweezle/iknite/blob/main/STRUCTURE.md)
for a detailed explanation of every directory.

## Next Steps

- [Development Workflow](development_workflow.md) – Build, test, lint
- [Testing](testing.md) – Running and writing tests
- [Code Style](code_style.md) – Style guidelines
