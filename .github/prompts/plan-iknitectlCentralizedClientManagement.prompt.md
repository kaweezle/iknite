## Plan: Iknitectl Centralized Management Refactor

Rewrite PRD into a clearer v2 contract, then execute an implementation sequence
that first reshapes current iknitectl command architecture, then adds
deterministic config/auth bootstrap (`env init`), then replaces shell/PowerShell
provisioning flows with DI-driven image + backend services. Approach keeps
testability first: all new logic behind injected interfaces, reusing existing
host abstractions and mock patterns.

**Steps**

1. Phase 1: Produce PRD v2 (spec cleanup and logic completion).
2. Step 1.1: Create
   `docs/specs/prds/iknitectl-centralized-client-management-v2.md` as source of
   truth (keep original PRD unchanged).
3. Step 1.2: Normalize terminology and lifecycle: `env`, `workspace`, `image`,
   `cluster`, `auth`, `backend`; define responsibilities and boundaries for
   each.
4. Step 1.3: Add missing behavior contracts: idempotency, overwrite policy,
   dry-run behavior, non-interactive CI behavior, error classes, logging levels,
   platform support matrix, and state ownership.
5. Step 1.4: Specify command migration as hard switch (no legacy aliases) per
   decision; document exact renamed commands and removal list.
6. Phase 2: Adapt existing iknitectl command tree first (blocking for all later
   features).
7. Step 2.1: Replace root registration in
   `/home/alpine/iknite/pkg/cmd/iknitectl/root.go` from
   `{install,kustomize,application,secrets}` to
   `{env,image,cluster,workspace,auth,backend}`.
8. Step 2.2: Move current logic under `workspace` namespace: current
   `application`, `kustomize`, and current top-level `secrets` command become
   `workspace application`, `workspace kustomize`, `workspace secrets`.
9. Step 2.3: Remove/retire current `install` top-level semantics or relocate
   needed pieces under `auth`/`env` as defined in PRD v2.
10. Step 2.4: Introduce `RootDependencies` container (in root options package)
    to inject `host.Host`/`host.FileExecutor`, logger factory, env provider,
    platform detector; avoid package-global mutable state.
11. Step 2.5: Update command tests to assert new tree and removed legacy
    commands.
12. Phase 3: Implement configuration directory + credentials bootstrap
    (`env init`) (_depends on Phase 2_).
13. Step 3.1: Add path resolver service: Linux `$XDG_CONFIG_HOME/iknite`
    fallback `~/.config/iknite`; Windows `%APPDATA%/iknite`; macOS
    `$HOME/Library/Application Support/iknite`.
14. Step 3.2: Define directory contract under config root:
15. Step 3.2.a: `auth/` for key material and CAs.
16. Step 3.2.b: `shared/` for backend-shared values/secrets.
17. Step 3.2.c: `images/` for downloaded artifacts/cache + manifest metadata.
18. Step 3.2.d: `clusters/` for per-cluster metadata/state references.
19. Step 3.3: Implement `iknitectl env init` idempotently: create tree,
    create/ensure default SSH/SOPS keypair, generate default CA cert/key (or CA
    bootstrap metadata), initialize shared templates.
20. Step 3.4: Reuse existing `pkg/secrets` key generation/init behavior where
    possible (`InitSecrets`, `ensureSSHKeyPair`) by wrapping into env/auth
    services rather than duplicating logic.
21. Step 3.5: Add explicit `--force`, `--config-dir`, `--non-interactive`,
    `--print-paths` behavior to `env init`.
22. Phase 4: Image download + provisioning replacement (_parallelizable by
    backend after shared image core_).
23. Step 4.1: Implement shared OCI image service with `oras-go` (auth, manifest
    resolution, multi-arch selection, media type validation, blob download,
    optional referrers retrieval for Incus metadata).
24. Step 4.2: Model artifact types explicitly: `rootfs`, `vm-vhdx`,
    `incus-metadata`; map acceptable media types and digest validation.
25. Step 4.3: Implement `image pull` and `image inspect` commands to replace raw
    script download logic.
26. Step 4.4: Backend provisioning orchestration:
    `cluster create --backend <...>` calls backend-specific provisioner
    interfaces that consume prepared local artifacts.
27. Step 4.5: Reproduce current script behavior by backend:
28. Step 4.5.a: WSL flow parity with `Get-Iknite.ps1` (download rootfs layer,
    `wsl --import`, install dir/name handling).
29. Step 4.5.b: Hyper-V flow parity with `Get-IkniteVM.ps1` (download VHDX
    layer, optional cloud-init ISO generation, VM creation profile, optional
    wait-for-SSH).
30. Step 4.5.c: Incus flow parity with `get-iknite.sh` (rootfs + Incus metadata
    referrer, image import, profile setup, launch/init).
31. Step 4.6: Move command execution to injected `host.Executor` wrappers and
    typed provisioner interfaces; no global command helpers.
32. Phase 5: Testability, DI, and verification hardening (_applies across
    phases_).
33. Step 5.1: Keep business logic in services with small interfaces; Cobra
    `RunE` remains thin adaptation layer.
34. Step 5.2: Reuse existing mocks from `/home/alpine/iknite/mocks/pkg/host` and
    delegate host in `/home/alpine/iknite/pkg/testutil/delegate_host_mock.go`.
35. Step 5.3: Add table-driven tests for command parsing, path resolution,
    idempotency, env precedence, media-type validation, backend flow branching,
    and error propagation.
36. Step 5.4: Add integration-style tests with temp dirs and fake executors for
    WSL/Hyper-V/Incus command invocation correctness (without requiring those
    runtimes in CI).
37. Step 5.5: Final quality gates: `golangci-lint run --fix`,
    `go test -v -race -covermode=atomic -coverprofile=coverage.out ./...`,
    `pre-commit run`.

**Relevant files**

- `/home/alpine/iknite/docs/specs/prds/iknitectl-centralized-client-management.md`
  — current PRD to supersede with v2 clarifications.
- `/home/alpine/iknite/docs/specs/prds/iknitectl-centralized-client-management-v2.md`
  — new improved specification document (to add).
- `/home/alpine/iknite/docs/specs/prds/iknitectl-centralized-client-management-plan.md`
  — requested implementation-plan artifact (to add).
- `/home/alpine/iknite/cmd/iknitectl/iknitectl.go` — main entrypoint remains
  thin.
- `/home/alpine/iknite/pkg/cmd/iknitectl/root.go` — command tree migration +
  dependency container wiring.
- `/home/alpine/iknite/pkg/cmd/iknitectl/application.go` — move under workspace
  subcommand.
- `/home/alpine/iknite/pkg/cmd/iknitectl/kustomize.go` — move under workspace
  subcommand.
- `/home/alpine/iknite/pkg/cmd/secrets/secrets.go` and
  `/home/alpine/iknite/pkg/cmd/secrets/init.go` — mount under workspace and
  potentially reuse for shared/auth concerns.
- `/home/alpine/iknite/pkg/secrets/secrets.go` and
  `/home/alpine/iknite/pkg/secrets/keys.go` — existing key/secrets init logic to
  reuse for env/auth.
- `/home/alpine/iknite/pkg/host/host.go` — top-level host contract.
- `/home/alpine/iknite/pkg/host/filesystem.go` — filesystem abstraction
  leveraged by new config/image services.
- `/home/alpine/iknite/pkg/host/executor.go` — command invocation abstraction
  for backend provisioners.
- `/home/alpine/iknite/pkg/host/network_host.go` and
  `/home/alpine/iknite/pkg/host/system.go` — optional backend runtime
  operations.
- `/home/alpine/iknite/get-iknite.sh` — Incus reference flow.
- `/home/alpine/iknite/Get-Iknite.ps1` — WSL reference flow.
- `/home/alpine/iknite/Get-IkniteVM.ps1` — Hyper-V reference flow.

**Verification**

1. Command structure verification: help output and command lookup tests confirm
   only new top-level commands exist and old ones are absent.
2. `env init` idempotency tests: run twice, verify no destructive overwrite
   without `--force`; with `--force`, verify deterministic regeneration policy.
3. Cross-platform config path tests: Linux/Windows/macOS path resolution with
   environment variable precedence.
4. Image service tests: manifest/index selection, digest pinning, media-type
   checks, and referrer resolution.
5. Backend provisioning tests: assert expected executor calls and arguments for
   WSL/Hyper-V/Incus flows.
6. Regression tests for existing workspace operations after namespace move.
7. Lint/test hooks: run project lint/tests and `pre-commit` gates.

**Decisions**

- Use new PRD file (v2), keep current PRD unchanged.
- Perform hard switch migration (no temporary aliases for legacy top-level
  commands).
- Plan artifact path:
  `docs/specs/prds/iknitectl-centralized-client-management-plan.md`.
- Architecture rule: strict dependency injection, avoid global mutable state,
  maximize host abstraction reuse for testability.
- Initial implementation scope includes command tree adaptation, env init, and
  image/provisioning parity for WSL/Hyper-V/Incus; excludes sync/deploy
  orchestration and centralized remote shared-store integration.

**Further Considerations**

1. Storage schema choice for cluster/image metadata: start with YAML files in
   config tree (simpler and testable) before optional SQLite migration.
2. Credential model: single default CA/keypair plus named profiles; profile
   support can be deferred if it delays env init.
3. OCI auth strategy: anonymous pull first with optional credential helper
   support in a follow-up phase.
