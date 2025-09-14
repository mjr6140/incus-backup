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

- [ ] Incus wrapper interfaces + fake
  - Narrow interfaces over `github.com/lxc/incus/client` for instances, volumes,
    images, and config; concrete impl + in-memory fake.
  - Tests: unit verifying wrapper contracts (no real Incus).

- [~] Config backup + restore preview
  - `backup config` writes `config/` JSONs + manifest/checksums. (projects implemented)
  - `restore config` previews changes; `--apply` gated and confirmed. (projects apply implemented)
  - Tests: unit (fakes) and integration (temp project export + preview).

- [ ] Instance backup (snapshot, portable)
  - `backup instances [NAME ...]` with snapshot-by-default; `--optimized` flag.
  - Tests: unit for options/manifest; integration with minimal Alpine instance.

- [ ] Instance restore (rename/replace, mapping)
  - `restore instance NAME [--version TS] [--target-name NEW]` and
    `--replace|--skip-existing`, `--pool-map`/`--network-map`/`--project-map`/`--profile-map`.
  - Tests: unit for conflict/mapping logic; integration: restore to new name and with replace.

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

- Requirements/spec: `REQUIREMENTS.md`
- Agent/development guidelines: `AGENTS.md`
