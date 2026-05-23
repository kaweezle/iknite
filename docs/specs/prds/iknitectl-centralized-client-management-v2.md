<!-- cSpell: words iknitectl gitops incus hyperv vhdx -->

# Iknitectl Centralized Client Management V2

## Intent

Iknitectl is control-center CLI for iknite clusters and workspaces. It replaces
legacy provisioning scripts and centralizes runtime credentials, image
management, backend provisioning, and workspace operations.

## Command Model

Top-level commands:

- `env` (`e`): initialize and manage local iknitectl environment.
- `image` (`i`, `img`): inspect and pull provisioning artifacts.
- `cluster` (`c`, `cl`): create/start/stop/delete cluster instances.
- `workspace` (`w`, `ws`): workspace rendering/secrets/application operations.
- `auth` (`a`): credential lifecycle and key material operations.
- `backend` (`b`, `bck`): backend definitions and validation.

Migration policy: hard switch. Legacy top-level commands are removed.

## State Ownership

Local config root:

- Linux: `$XDG_CONFIG_HOME/iknite` fallback `~/.config/iknite`
- macOS: `$HOME/Library/Application Support/iknite`
- Windows: `%APPDATA%/iknite`

Directory contract:

- `auth/`: key material and CA files.
- `shared/`: shared values/secrets for backend integrations.
- `images/`: artifact cache and metadata.
- `clusters/`: per-cluster state and bindings.

## Behavioral Contracts

- `env init` is idempotent by default.
- Existing generated files are preserved unless `--force` is set.
- `--non-interactive` forbids prompts and fails fast on required input.
- `--print-paths` prints resolved canonical paths.
- Errors are contextual and typed by stage: validation, IO, network,
  provisioning, auth.

## Image Contracts

Artifact types:

- `rootfs`
- `vm-vhdx`
- `incus-metadata`

Image flow:

1. Resolve reference (tag or digest).
2. Resolve platform manifest when applicable.
3. Validate media type against artifact type.
4. Download blob(s).
5. Store metadata and paths in local state.

## Provisioning Contracts

Cluster provisioning is backend-driven and uses dependency injection through
host abstractions. Orchestration invokes backend provisioners with prepared
local artifacts.

Initial backend parity targets:

- WSL (legacy `Get-Iknite.ps1` behavior)
- Hyper-V (legacy `Get-IkniteVM.ps1` behavior)
- Incus (legacy `get-iknite.sh` behavior)

## Testability Requirements

- No package-level mutable global state for runtime behavior.
- Business logic in services with injected dependencies.
- Cobra handlers remain thin adapters.
- Unit tests use existing host mocks and delegate host patterns.
