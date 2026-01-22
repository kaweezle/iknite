# iknitedev

Development tools for the iknite project.

## Overview

`iknitedev` is a collection of development utilities for the iknite project. It
provides commands for managing secrets, building artifacts, and other
development tasks that are not part of the main iknite binary.

## Installation

The tool can be run directly using Go:

```bash
go run hack/iknitedev/ [command]
```

Or you can build it:

```bash
cd hack/iknitedev
go build -o iknitedev .
```

## Commands

### install signing-key

Extract and install an APK signing key from a SOPS encrypted secrets file.

The command decrypts the secrets file using SOPS, extracts the specified signing
key, and writes it to the specified destination directory with appropriate
permissions (0400).

**Usage:**

```bash
iknitedev install signing-key [secrets-file] [destination-directory] [flags]
```

**Example:**

```bash
# Install to current directory
go run hack/iknitedev/main.go install signing-key deploy/iac/iknite/secrets.sops.yaml .

# Install to specific directory
go run hack/iknitedev/main.go install signing-key --key apk_signing_key deploy/iac/iknite/secrets.sops.yaml /path/to/dest
```

**Flags:**

- `--key string`: Name of the key to extract from secrets file (default
  "apk_signing_key")

**Prerequisites:**

- SOPS must be configured with appropriate decryption keys (Age or GPG)
- The secrets file must be in YAML format
- The key structure in the secrets file must be:
  ```yaml
  apk_signing_key:
    name: "key-name"
    private_key: "-----BEGIN ... -----\n..."
  ```

## Development

The tool uses Go workspaces to keep dependencies separate from the main iknite
project. The workspace is configured in the root `go.work` file.

### Architecture

The tool is designed with testability in mind:

- **No static variables**: All commands are created via factory functions
  (`CreateXXXCmd`)
- **Dependency injection**: Filesystem operations use the `afero` library,
  allowing tests to use in-memory filesystems
- **Options structs**: Command options are passed via structs, making it easy to
  test without parsing flags
- **Separation of concerns**: Command creation is separate from business logic

### Testing

Run tests with:

```bash
cd hack/iknitedev
go test -v ./...
```

The tests demonstrate:

- Creating commands with custom options
- Testing filesystem operations with in-memory filesystems
- Validating command structure and flags
- Error handling for missing files

### Adding New Commands

1. Create a new file in `cmd/` directory (e.g., `cmd/my_command.go`)
2. Implement the command using Cobra patterns
3. Register the command in the appropriate parent command's `init()` function
4. Update this README with the new command documentation

## Dependencies

- [Cobra](https://github.com/spf13/cobra): CLI framework
- [Viper](https://github.com/spf13/viper): Configuration management
- [SOPS](https://github.com/getsops/sops): Secrets encryption/decryption

## Replacing the Shell Script

This tool replaces the previous `hack/install-signing-key.sh` shell script with
a more robust Go implementation that:

- Uses native SOPS library instead of shelling out
- Provides better error handling and validation
- Supports extensible command structure for future additions
- Works consistently across different platforms
- Allows explicit destination directory specification
- Is fully testable with dependency injection and mock filesystems
