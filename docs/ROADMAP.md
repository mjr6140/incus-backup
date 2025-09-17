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

Implementation checklist (Restic backend)
1) Baseline decisions & validation
   - Require `restic >= 0.18.0`; call `restic version`, warn on mismatch, and prompt the user before continuing when older.
   - Lock in streaming via `restic backup --stdin`; no staging of temporary files.
   - Decide tag schema (`type=instance|volume|config`, project, pool, name, timestamp, schema version) and canonical snapshot naming.
   - Define how restic credentials are supplied (password env vars or `--password-file`) and how errors are surfaced.
   - Choose progress strategy (default to streaming restic stdout/stderr; optionally parse `--json` later).

2) Backend scaffolding
   - Create a `restic` implementation of the storage backend interface with capability flags (`supportsStream=true`, `supportsVerify=true`).
   - Parse `restic:` URIs (local repo paths and key/value options), normalize repo paths, and auto-initialize repositories when missing.
   - Detect restic binary location, enforce version check, and convert failures into actionable CLI errors.
   - Centralize restic process execution helper (context cancellation, env injection, stdout/stderr capture, log streaming).

3) Integration tooling groundwork
   - Update `scripts/test_integration_container.sh` to install restic ≥ 0.18.0, configure password env vars/files, and expose repo cache directories.
   - Add CI hooks or local instructions so restic-based integration tests are runnable from the start of implementation.
   - Lay down shared test helpers/fixtures for creating restic repos, seeding/incus test data, and cleaning up between runs.

4) Instance backup/restore pipeline
   - During backup: snapshot instance (unless `--no-snapshot`), export portable tar, stream into `restic backup --stdin --stdin-filename instances/<project>/<name>/<ts>.tar`, apply metadata tags, and store manifest/checksum alongside (second tiny snapshot or metadata store).
   - During restore: resolve desired snapshot via tags/filters, stream back via `restic dump` (or `restore --target -`) directly into Incus import, honoring `--replace`, `--skip-existing`, `--target-name`, and snapshot lifecycle.
   - Ensure errors propagate with context (restic failures, Incus import failures) and clean up temporary state.

5) Volume backup/restore pipeline
   - Mirror instance logic for custom volumes: snapshot/export portable tar, stream into restic with pool/name tags, and capture manifests.
   - On restore, stream snapshot back into Incus volume import respecting `--replace`, `--skip-existing`, and target name flags.
   - Support optional `--optimized` by toggling Incus export parameters while documenting portability caveats.

6) Config backup/restore
   - Collect config artifacts (`projects.json`, `profiles.json`, `networks.json`, `storage_pools.json`, manifest, checksums) and stream them into restic with deterministic filenames (e.g., `config/<timestamp>/<file>`).
   - Update restore preview/apply to read config data from restic (latest or requested version), run diff/apply with existing safety flags (`--apply`, `--force`), and maintain checksum verification.

7) Listing & selection UX
   - Implement `restic` backend list/readers by calling `restic snapshots --json`, grouping snapshots by resource, and selecting the latest unless `--version` supplied.
   - Expose table/json output formats consistent with directory backend, including filters for names, fingerprints, and types.
   - Ensure `backup list images/config/...` works with restic snapshot metadata.

8) Verify & prune workflows
   - Map `incus-backup verify` to `restic check` (full or `--read-data-subset`), then iterate manifests in restic to compute per-file status identical to directory backend.
   - Translate prune policies into `restic forget --prune`; produce preview tables of affected snapshots before confirmation (respect `--dry-run` and `--force`).
   - Integrate confirmation gating so restic operations cannot proceed without explicit consent when warnings (e.g., version mismatch) are present.

9) Concurrency, locking, and observability
   - Serialize restic invocations to avoid repo lock contention while allowing Incus operations to run concurrently.
   - Surface restic stdout/stderr live in CLI output; consider optional JSON parsing for richer progress feedback.
   - Handle repo unlock/retry logic if restic reports a stale lock.

10) Testing & step-level validation
    - Add integration coverage incrementally as features land: instance/volume/config round-trips, list filtering, verify mismatch detection, prune behaviour, version-warning prompt.
    - Introduce unit tests for restic backend helpers (URI parsing, command construction, tag mapping) alongside the relevant features.
    - Ensure regression tests exist for restic process orchestration (log streaming, locking) before proceeding to release hardening.

11) Documentation & release readiness
    - Update README and docs to describe restic backend usage, credential expectations, retention guidance, and troubleshooting (e.g., repo corruption, password mistakes).
    - Capture operational notes: recommended restic version, warning prompt behaviour, how to inspect restic logs, and migration steps from directory backend.
    - Announce feature status in ROADMAP milestones once implementation stabilizes.
