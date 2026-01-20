# Development Scripts

This directory contains development and build helper scripts for the iknite project.

## Directory Structure

```
hack/
├── README.md                    # This file
├── build-helper.sh              # Build automation helper
├── build-vm-image.sh            # VM image builder
├── configure-vm-image.sh        # VM image configuration
├── create-vm-nocloud-seed.sh    # Cloud-init seed creator
├── install-signing-key.sh       # APK signing key installer
├── make-rootfs-devenv.sh        # Development environment setup
├── test_start.sh                # Test cluster startup
├── tools.go                     # Go tool dependencies
├── update-codegen.sh            # Kubernetes code generator
├── verify-codegen.sh            # Verify generated code
├── boilerplate.go.txt           # Go file boilerplate
├── custom-boilerplate.go.txt    # Custom boilerplate template
├── devcontainer/                # VS Code devcontainer files
└── test/                        # Test fixtures and resources
```

## Build Scripts

### build-helper.sh

General build automation helper script.

**Usage:**
```bash
./hack/build-helper.sh [options]
```

### build-vm-image.sh

Builds VM images for testing iknite in virtual machines.

**Usage:**
```bash
./hack/build-vm-image.sh
```

Creates a VM image with Alpine Linux and iknite pre-installed.

### configure-vm-image.sh

Configures built VM images with specific settings.

**Usage:**
```bash
./hack/configure-vm-image.sh [image-path]
```

Applies configuration to a VM image after it has been built.

## Development Scripts

### make-rootfs-devenv.sh

Creates a development environment root filesystem for testing.

**Usage:**
```bash
./hack/make-rootfs-devenv.sh
```

This script:
- Builds a development-oriented rootfs
- Includes debugging tools
- Configures a development-friendly environment

### install-signing-key.sh

Installs the APK signing key for local package testing.

**Usage:**
```bash
./hack/install-signing-key.sh
```

Required when testing locally-built APK packages.

### create-vm-nocloud-seed.sh

Creates a cloud-init seed ISO for no-cloud datasource.

**Usage:**
```bash
./hack/create-vm-nocloud-seed.sh
```

Generates a cloud-init configuration for VM initialization.

## Testing Scripts

### test_start.sh

Tests the cluster startup process.

**Usage:**
```bash
./hack/test_start.sh
```

This script:
- Starts iknite in a test environment
- Validates cluster initialization
- Checks that all components are running

**Requirements:**
- Alpine Linux or WSL environment
- iknite binary in PATH or build directory

### test/

Directory containing test fixtures and Kubernetes manifests for testing.

**Contents:**
- `nginx.yaml` - Sample nginx deployment for testing

## Code Generation

### update-codegen.sh

Generates Kubernetes client code using code-generator.

**Usage:**
```bash
./hack/update-codegen.sh
```

This script:
- Generates clientset, listers, and informers
- Updates generated code in `pkg/apis/`
- Should be run when API definitions change

**When to run:**
- After modifying types in `pkg/apis/`
- When adding new API resources
- As part of the development workflow

### verify-codegen.sh

Verifies that generated code is up-to-date.

**Usage:**
```bash
./hack/verify-codegen.sh
```

This script:
- Runs code generation in a temporary directory
- Compares with existing generated code
- Fails if generated code is outdated

**Use in CI:**
This script is typically run in CI to ensure developers have run `update-codegen.sh`.

### Boilerplate Files

- `boilerplate.go.txt` - Standard Go file header
- `custom-boilerplate.go.txt` - Custom boilerplate template

These files are used by code generation scripts to add consistent headers to generated files.

## Tool Dependencies

### tools.go

Declares Go tool dependencies that are not directly imported by the codebase.

**Purpose:**
- Ensures code generation tools are version-controlled
- Downloaded with `go mod download`
- Available via `go run`

**Tools included:**
- Kubernetes code-generator
- Other development tools

## Development Container

### devcontainer/

VS Code development container configuration.

**Purpose:**
- Provides a consistent development environment
- Includes all required tools and dependencies
- Enables development inside a container

**Usage:**
Open the project in VS Code with the Remote-Containers extension installed.

## Usage Guidelines

### Running Scripts

Most scripts should be run from the project root:

```bash
# Correct
./hack/build-helper.sh

# May not work
cd hack && ./build-helper.sh
```

### Script Dependencies

Some scripts have dependencies:

- **Docker**: VM building scripts
- **Alpine Linux/WSL**: Testing scripts
- **Go tools**: Code generation scripts

Install missing dependencies before running scripts.

### Error Handling

Scripts follow these conventions:

- Exit with non-zero code on error
- Print error messages to stderr
- Use set -e for fail-fast behavior

### Adding New Scripts

When adding new scripts:

1. Follow existing naming conventions
2. Add usage documentation in script header
3. Update this README
4. Make scripts executable: `chmod +x script.sh`
5. Use shellcheck for linting

## Common Tasks

### Generate Kubernetes Code

```bash
./hack/update-codegen.sh
./hack/verify-codegen.sh
```

### Build and Test Locally

```bash
# Build binary
go build -o dist/iknite ./cmd/iknite

# Test startup
./hack/test_start.sh
```

### Build VM Image

```bash
./hack/build-vm-image.sh
./hack/configure-vm-image.sh output.img
```

### Setup Development Environment

```bash
./hack/make-rootfs-devenv.sh
./hack/install-signing-key.sh
```

## Best Practices

1. **Run from project root**: Always execute scripts from the project root directory
2. **Check requirements**: Ensure dependencies are installed before running scripts
3. **Read script headers**: Each script should document its usage and requirements
4. **Test locally**: Test scripts locally before committing changes
5. **Keep idempotent**: Scripts should be safe to run multiple times

## Troubleshooting

### Permission Denied

```bash
chmod +x hack/script.sh
```

### Missing Dependencies

Check script header for required tools and install them:

```bash
# Example for code generation
go install k8s.io/code-generator/cmd/...@latest
```

### Script Fails in CI

Ensure the script:
- Doesn't rely on local environment
- Uses relative paths
- Has all dependencies declared
- Exits with proper error codes

## Related Documentation

- [BUILD.md](../BUILD.md) - Build and release process
- [CONTRIBUTING.md](../CONTRIBUTING.md) - Contribution guidelines
- [STRUCTURE.md](../STRUCTURE.md) - Project directory structure
