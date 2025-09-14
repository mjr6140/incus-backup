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

- [~] Instance backup (portable, snapshot-by-default)
  - `backup instances [NAME ...]` implemented using Incus backup API; `--optimized` flag.
  - Snapshot-by-default behavior (planned): when an instance is running, take a
    temporary Incus snapshot, export from that snapshot, then delete the temp
    snapshot. This improves consistency of the exported filesystem without
    forcing downtime. Provide a `--no-snapshot` escape hatch for advanced use.
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

- [ ] Volume backup/restore (project-scoped)
  - `backup volumes` and `restore volume` with snapshot-by-default.
  - Paths: `volumes/<project>/<pool>/<name>/...`.
  - Tests: unit for mapping/paths; integration round-trip for a small volume.

- [ ] Verify/prune
  - `verify` checks checksums/manifest integrity; `prune` keep-N or policy.
  - Tests: unit for policy and verification; light integration for verify.

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
