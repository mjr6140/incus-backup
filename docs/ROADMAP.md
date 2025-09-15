# Implementation Roadmap

This file tracks the incremental, test-first plan for building incus-backup.
Each step is small, has clear deliverables, and includes unit and/or integration
tests. Update statuses as we progress.

Legend: [ ] Todo · [~] In progress · [x] Done

## Milestones

- [x] Containerized integration test harness
  - Docker/Podman-based, privileged container with systemd + Incus + Go.
  - Script: `scripts/test_integration_container.sh`
  - Caches: `.cache/gomod`, `.cache/gocache` persisted in workspace.

- [x] Incus probe integration test
  - Build tag: `integration`; gated by `INCUS_TESTS=1`.
  - Connect via UNIX socket, log version, create/delete temporary project.
  - File: `tests/integration/probe_incus_test.go`

- [x] CLI skeleton + version
  - Cobra root command, `--version`, logging init scaffold.
  - Tests: unit tests for command wiring; help text snapshot.

- [x] Target URI parser + validation
  - Parse canonical backend URIs (e.g., `dir:/path`).
  - Backend registry interface; helpful errors.
  - Tests: table-driven unit; reject invalid URIs.

- [x] Directory backend layout + list
  - Manifest schemas; read-only `list` across types.
  - Output formats: table/json/yaml.
  - Tests: unit with temp dirs; golden manifests; list formatting.

- [x] Safety utilities
  - `--dry-run`, `--yes`/`-y`, `--force`; interactive prompt helper.
  - Tests: unit for plan decisions; prompt behavior mocked.

- [x] Incus wrapper interfaces + fake
  - Narrow interfaces over `github.com/lxc/incus/client` for projects, profiles,
    networks, storage pools, and instances; real + in-memory fake.
  - Tests: unit verifying wrappers using fakes; probe integration tests.

- [x] Config backup + restore preview
  - `backup config` writes `config/` JSONs + manifest/checksums. (projects, profiles, networks, storage pools)
  - `restore config` previews and applies (projects, networks, storage pools); deletions for networks/pools require `--force`.
  - Tests: unit for planners/apply; integration for backup/preview and apply.

- [x] Instance backup (portable, snapshot-by-default)
  - `backup instances [NAME ...]` implemented using Incus backup API; `--optimized` flag.
  - Snapshot-by-default behavior implemented: create a temporary snapshot, export from it, then delete it. `--no-snapshot` escape hatch available.
  - Tests: unit for manifest; integration with Alpine instance.

Snapshot-by-default behavior
- Goal: Make exported data consistent when instances or volumes are running.
- How it works (instances):
  - Create a temporary snapshot (e.g., `tmp-incus-backup-<ts>`).
  - Export from that snapshot rather than the live instance.
  - Delete the temporary snapshot after a successful export (and on failure when possible).
- How it works (volumes):
  - Create a storage volume snapshot, export from the snapshot, then delete it.
- Flags and UX:
  - Enabled by default for `backup instances` and `backup volumes`.
  - `--no-snapshot` disables the behavior (may be faster but risks inconsistency).
  - `--optimized` still supported; document portability trade-offs vs. speed.

- [x] Instance restore (rename/replace)
  - `restore instance NAME [--version TS] [--target-name NEW]` with `--replace|--skip-existing`.
  - Table-style preview printed in dry-run and before confirmation.
  - Progress during import plus server-side status polling (Running → Success).
  - Tests: integration for standard restore, --replace, --target-name, --skip-existing.

- [x] Bulk restore (instances, volumes)
  - `restore instances [NAME ...]` restores all if unspecified, with single preview/confirmation.
  - `restore volumes [POOL/NAME ...]` restores all if unspecified, with single preview/confirmation.
  - Applies `--replace|--skip-existing` policies consistently across items.
  - Per-item headers and progress on import.

- [x] Volume backup/restore (project-scoped)
  - `backup volumes` and `restore volume` with snapshot-by-default.
  - Paths: `volumes/<project>/<pool>/<name>/...`.
  - Tests: unit for roundtrip; integration (roundtrip, target-name, replace, skip-existing).

- [x] Restore all (orchestrated)
  - `restore all` runs: config preview/apply → volumes (bulk) → instances (bulk).
  - Single preview across sections; single confirmation; policy flags applied.

- [x] Verify/prune
  - `verify` checks checksums/manifest integrity; `prune` keep-N per resource.
  - Supports `--dry-run` and table/json outputs for verify.

- [ ] Concurrency + polish
  - `--parallel N` worker pool; retries/backoff; progress polish; stable outputs.
  - Tests: unit for worker behavior; deterministic ordering in outputs.

## Integration Testing Notes

- Run locally without installing Incus on the host:
  - `scripts/test_integration_container.sh`
  - Gated by `INCUS_TESTS=1` and `-tags=integration` inside the container.
  - Use `KEEP_CONTAINER=1` to leave the container running for faster iteration.

## References

- Requirements/spec: `README.md`
- Agent/development guidelines: `AGENTS.md`

## Future: Restic Backend (design + plan)

Goals
- Leverage restic’s content-defined chunking, deduplication, and encryption.
- Avoid pre-compressing exports so restic can dedupe effectively.
- Preserve existing UX (targets via `--target`) while swapping backends.

Key decisions
- Stream uncompressed backups into restic:
  - Instances: request backup with `CompressionAlgorithm: "none"` and stream
    the tar to restic via `--stdin` (no temp files when possible).
  - Volumes: same approach using `StoragePoolVolumeBackupsPost` with
    `CompressionAlgorithm: "none"` (API supports it); stream to restic.
  - Config: back up JSON manifests directly (no compression) and store as
    separate restic snapshot(s) tagged appropriately.
- Tagging and metadata:
  - Use restic `--tag` to encode type=instance|volume|config, project, pool,
    name, and timestamp. Optionally include version schema tag.
  - Store manifest.json alongside the data stream by creating a second tiny
    snapshot (or include manifest content as an additional `--stdin` stream with
    a distinct `--stdin-filename`).
- Listing and selection:
  - Use `restic snapshots --json --tag ...` to list latest per (type, id).
  - Mirror directory backend’s “latest per resource unless --version provided”.
- Restore:
  - Use `restic restore --include` or `restic dump` (or `restic cat` for stdin)
    to stream the stored tar back into the importer (instance or volume).
  - No temp files where possible; import from the restic stream.
- Retention and verification:
  - `incus-backup verify` proxies to `restic check` and validates manifests.
  - `incus-backup prune` maps to `restic forget --prune` with policy flags.
- Configuration:
  - Target URI: `--target restic:/path` (local repo) or `restic:repo=...` for
    remote backends. Rely on RESTIC_PASSWORD/RESTIC_PASSWORD_FILE in env or
    `--password-file` in config.
  - Detect restic binary and version; provide clear error guidance if missing.

Open questions
- Whether to prefer a single synthetic tree per snapshot (restic backup of a
  virtual directory) or use `--stdin` streams per resource. Current plan favors
  streaming per resource with tags to avoid staging on disk.
- How to best surface restic progress in our progress UI (parse `--json` or
  pass through restic output verbatim).
- Import portability with `--optimized`: likely keep optimized=false by default
  for portability; make optimized opt-in due to driver constraints.

Implementation plan
1) Backend scaffolding
   - Add `restic` backend implementing StorageBackend interface with feature
     flags for `supportsStream=true`.
   - Detect restic, parse repo URI, initialize repo if needed.
2) Instances (export/import via stdin/stdout)
   - Set CompressionAlgorithm="none"; stream export → restic `backup --stdin`
     with tags. Stream restore back into `CreateInstanceFromBackup`.
3) Volumes (export/import via stdin/stdout)
   - Same as instances using volume backup APIs; ensure uncompressed streams.
4) Config
   - Store `projects.json`, `profiles.json`, `networks.json`, `storage_pools.json`
     as small files in restic with tags; keep manifest and checksums.
5) Listing + preview
   - Implement list by querying restic snapshots with tag filters; compute
     latest per resource.
6) Verify + prune
   - Wire `verify` to `restic check`; implement `prune` via `restic forget` and
     policies mapped from CLI.
7) Integration tests
   - Install restic in the container harness; run local repo tests (no network).
   - Roundtrip instance/volume and config with restic backend.
