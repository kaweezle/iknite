<!-- cSpell: words iknitectl hyperv incus vhdx -->

# Implementation Plan: Iknitectl Centralized Client Management

## Phase 1: Command Migration

1. Replace root command tree with:
   - `env`, `image`, `cluster`, `workspace`, `auth`, `backend`.
2. Move existing command logic under `workspace`:
   - `application`, `kustomize`, `secrets`.
3. Expose credential operations under `auth`.
4. Keep command handlers thin; route runtime through dependency container.

## Phase 2: Environment Bootstrap

1. Implement `env init` with flags:
   - `--config-dir`, `--force`, `--non-interactive`, `--print-paths`.
2. Resolve config root path per OS.
3. Create directory tree:
   - `auth/`, `shared/`, `images/`, `clusters/`.
4. Reuse secrets initialization logic for default SSH/SOPS keypair and encrypted
   secrets scaffold.
5. Create/ensure default local CA material in `auth/`.

## Phase 3: Image Services

1. Implement image service with typed artifacts:
   - `rootfs`, `vm-vhdx`, `incus-metadata`.
2. Implement `image inspect`.
3. Implement `image pull` with media-type validation and artifact output path.
4. Keep registry access behind injectable repository factory for tests.

## Phase 4: Cluster Provisioning Orchestration

1. Add backend provisioner interface.
2. Implement `cluster create --backend <name>` orchestration using injected
   executor/host dependencies.
3. Add adapters for WSL, Hyper-V, and Incus behavior parity incrementally.

## Phase 5: Hardening

1. Add table-driven tests for path resolution and idempotency.
2. Add tests for command tree migration and nested workspace commands.
3. Add tests for image reference parsing, inspect, and pull.
4. Run lint/test gates and pre-commit hooks.
