<!-- cSpell: words mycommand mysubcmd -->

# iknitedev

Development tools for the iknite project.

## Overview

`iknitedev` is a collection of development utilities for the iknite project. It
provides commands for managing secrets, building artifacts, and other
development tasks that are not part of the main iknite binary.

## Architecture

The project is organized into two layers:

- **`pkg/<domain>/`** — Business logic packages containing the actual
  implementation, options types, and exported functions (e.g. `pkg/secrets/`).
- **`pkg/cmd/<command>/`** — Cobra command wrappers, one file per subcommand,
  that delegate to the corresponding business logic package (e.g.
  `pkg/cmd/secrets/`).
- **`cmd/`** — Root and top-level command registration, wiring `pkg/cmd`
  packages into the CLI.

This separation keeps business logic independently testable without Cobra and
allows the cobra layer to focus on flag parsing and I/O wiring.

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
go run hack/iknitedev/main.go install signing-key secrets.sops.yaml .

# Install to specific directory
go run hack/iknitedev/main.go install signing-key --key apk_signing_key secrets.sops.yaml /path/to/dest
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

Follow the two-layer pattern used by the `secrets` command:

**For a new top-level command (e.g. `iknitedev mycommand`):**

1. Create `pkg/mycommand/mycommand.go` with the business logic, options struct,
   and exported functions.
2. Create `pkg/cmd/mycommand/mycommand.go` with `CreateMycommandCmd` that wires
   options and calls the business logic functions.
3. Register the command in `cmd/root.go`:

```go
import "github.com/kaweezle/iknite/hack/iknitedev/pkg/cmd/mycommand"
// ...
rootCmd.AddCommand(mycommand.CreateMycommandCmd(opts.Fs))
```

4. Add tests:

- `pkg/mycommand/mycommand_test.go` — unit tests calling exported functions
  directly.
- `pkg/cmd/mycommand/mycommand_test.go` — tests for Cobra wiring (flags,
  argument parsing, stdin handling).

5. Update this README with the new command documentation.

**For a new subcommand under an existing command (e.g.
`iknitedev secrets mysubcmd`):**

1. Add the business logic function to the relevant `pkg/<domain>/` package.
2. Create `pkg/cmd/<command>/mysubcmd.go` with a `createMysubcmdCmd` function
   that calls the business logic.
3. Register it in `pkg/cmd/<command>/secrets.go` (or the parent command file)
   with `secretsCmd.AddCommand(createMysubcmdCmd(opts))`.
4. Add tests to `pkg/<domain>/<domain>_test.go` and/or
   `pkg/cmd/<command>/<command>_test.go` as appropriate.

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
