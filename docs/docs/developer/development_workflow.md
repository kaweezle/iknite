!!! wip "Work in progress"

    This documentation is in draft form and may change frequently.

# Development Workflow

This page explains the development workflow for Iknite, including building,
testing, and deploying changes.

## Quick Reference

| Task | Command |
|------|---------|
| Build binary | `goreleaser build --single-target --auto-snapshot --clean` |
| Build APK | `goreleaser build --snapshot --skip=publish --clean` |
| Run tests | `go test ./...` |
| Run tests with coverage | `go test -v -race -covermode=atomic -coverprofile=coverage.out ./...` |
| Lint | `golangci-lint run --fix` |
| Format | `golangci-lint fmt` |
| Pre-commit checks | `pre-commit run --all-files` |
| Full image build | `./packaging/scripts/build-helper.sh --with-cache` |
| Build rootfs | `./packaging/scripts/build-helper.sh --only-build --with-cache` |

## Building

### Build the Binary

```bash
# Build for your local architecture (fast, for development)
goreleaser build --single-target --auto-snapshot --clean

# The binary is in ./dist/
./dist/iknite_linux_amd64_v1/iknite --version
```

### Build the APK Package

```bash
# Build all architectures as a snapshot (no version tags)
goreleaser build --snapshot --skip=publish --clean

# This produces APK packages in ./dist/
ls ./dist/*.apk
```

### Build the Full Image Pipeline

```bash
# Run the full pipeline (APK + rootfs + VM images)
./packaging/scripts/build-helper.sh --with-cache

# Run specific steps only
./packaging/scripts/build-helper.sh --only-goreleaser
./packaging/scripts/build-helper.sh --only-build --with-cache
./packaging/scripts/build-helper.sh --only-images
./packaging/scripts/build-helper.sh --only-export

# Skip specific steps
./packaging/scripts/build-helper.sh --skip-vm-image --with-cache
```

Available pipeline steps:
- `goreleaser` – Build APK packages
- `build` – Build rootfs base image
- `images` – Build iknite-images APK
- `add-images` – Add images to rootfs
- `export` – Export rootfs tarball
- `rootfs-image` – Build final rootfs image
- `make-apk-repo` – Build local APK repository
- `vm-image` – Build QCOW2/VHDX images

## Testing Changes Locally

### Run the Binary

```bash
# Build and run
goreleaser build --single-target --auto-snapshot --clean
sudo ./dist/iknite_linux_amd64_v1/iknite start -v debug -t 120
```

### Test Kustomize Changes

```bash
# Print the resolved kustomization
./dist/iknite_linux_amd64_v1/iknite kustomize -d packaging/apk/iknite/iknite.d print

# Or with kubectl kustomize
kubectl kustomize packaging/apk/iknite/iknite.d
```

## Code Quality

### Linting

Iknite uses [golangci-lint](https://golangci-lint.run/) with the configuration
in `.golangci.yml`:

```bash
# Run all linters with auto-fix
golangci-lint run --fix

# Run without fixing (for CI)
golangci-lint run
```

### Formatting

```bash
# Format all Go files
golangci-lint fmt

# Or use gofmt directly
gofmt -l -w .
```

### Pre-commit Hooks

The project uses [pre-commit](https://pre-commit.com/) for automated checks:

```bash
# Install hooks (first time)
pre-commit install

# Run all hooks manually
pre-commit run --all-files

# Run specific hooks
pre-commit run golangci-lint --all-files
pre-commit run gofmt --all-files
pre-commit run cspell --all-files
```

Configured hooks (`.pre-commit-config.yaml`):
- `gofmt` – Go code formatting
- `golangci-lint` – Go linting
- `shellcheck` – Shell script linting
- `cspell` – Spell checking

## VS Code Tasks

The project includes VS Code tasks in `.vscode/tasks.json`:

| Task | Shortcut | Description |
|------|---------|-------------|
| `goreleaser-build` | `Ctrl+Shift+B` | Build single target binary |
| `goreleaser` | — | Build all APK packages |
| `golangci-lint` | — | Lint with auto-fix |
| `golangci-fmt` | — | Format code |
| `go mod tidy` | — | Tidy Go modules |
| `pre-commit` | — | Run all pre-commit checks |
| `test with coverage` | — | Run tests with coverage report |

## Making Changes

### Adding a New Command

Follow the pattern from `pkg/cmd/status.go`:

1. Create `pkg/cmd/your_command.go`
2. Define `NewYourCmd(*v1alpha1.IkniteClusterSpec)` returning `*cobra.Command`
3. Implement `performYour(ikniteConfig)` with main logic
4. Register in `pkg/cmd/root.go`'s `NewRootCmd()`
5. Add tests in `pkg/cmd/your_command_test.go`

### Modifying Kubernetes Manifests

Edit manifests in `packaging/apk/iknite/iknite.d/base/`:

```bash
# Edit the manifest
vim packaging/apk/iknite/iknite.d/base/kube-flannel.yaml

# Verify the kustomization resolves
kubectl kustomize packaging/apk/iknite/iknite.d
```

### Modifying API Types

After modifying types in `pkg/apis/iknite/v1alpha1/`:

```bash
# Regenerate deepcopy and client code
./hack/update-codegen.sh

# Verify the generated code is correct
./hack/verify-codegen.sh
```

## Deployment for Testing

### Test Against Live OpenStack VM

The Terraform configuration in `deploy/iac/iknite/iknite-vm/` can be used to
deploy test VMs on OpenStack:

```bash
cd deploy/iac/iknite/iknite-vm
terragrunt apply
```

### Test in Local Container

```bash
# Build and test in a container
nerdctl run --privileged --rm -it \
  --name iknite-test \
  $(nerdctl build -q .) \
  /sbin/iknite start -t 120
```

## Release Process

See [RELEASE.md](https://github.com/kaweezle/iknite/blob/main/RELEASE.md) for
the release process.

Releases are triggered by pushing a tag:

```bash
git tag v1.2.3
git push origin v1.2.3
```

This triggers the GitHub Actions release workflow which:
1. Builds APK packages with goreleaser
2. Builds rootfs base image
3. Adds pre-pulled images
4. Exports rootfs tarball
5. Builds QCOW2 and VHDX VM images
6. Publishes to GitHub Releases

## Continuous Integration

The CI pipeline runs on every PR and push:

- `.github/workflows/go.yaml` – Go build, test, and pre-commit hooks
- `.github/workflows/docs.yaml` – Documentation build
- `.github/workflows/release.yml` – Full release pipeline (tag-triggered)
