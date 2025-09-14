Incus Backup — a backup utility for an Incus server (https://linuxcontainers.org/incus/).

It is intended to run on the Incus host and produce backups that can be easily
restored. It automates common backup and restore workflows while prioritizing
safety and auditability.

Incus docs on backups: https://linuxcontainers.org/incus/docs/main/backup/

This tool will use the Incus API directly and orchestrate exports/imports for
instances, volumes, images (optional), and declarative config (projects,
profiles, networks, storage pool config) needed to recreate environment state.

# Tech Stack

- Language: Go 1.22+
- Incus API: `github.com/lxc/incus/client` (no shelling to `incus`)
- CLI framework: `spf13/cobra`; configuration via `spf13/viper`
- Logging: Go `log/slog` with levels; human-readable by default
- Testing: `go test` with build tags (`unit`, `integration`), `testify`
- Lint/Format: `gofmt` + `golangci-lint`

# Project Structure

- `src/cli/` — Cobra commands and flag parsing
- `src/incus/` — thin wrappers around the Incus client; interfaces for testing
- `src/backup/` — backup planning, storage layout, restore logic
- `src/config/` — config loading (Viper), defaults, validation
- `src/logging/` — logging setup and hooks
- `tests/` — mirrors `src/` with unit and integration tests
- `assets/` — sample config, example policies

# Storage Backends

- `directory` (default): write exports to a filesystem tree.
- `restic` (future): pluggable backend via a `StorageBackend` interface.

The initial implementation targets `directory` only, with an interface to
enable `restic` later without changing the CLI.

# Backup Location

A backup target must be configured.

- Canonical: `--target` as a backend URI.
  - Directory backend: `--target dir:/mnt/nas/sysbackup/incus`
  - Future Restic: `--target restic:/path` or `--target restic:repo=https://...`
- `--backend` may be provided but is inferred from `--target` when present.

# CLI Syntax

Top-level commands: `backup`, `restore`, `list`, `verify` (future), `prune` (future).

Conventions:

- Prefer long flags with double hyphens: `--flag value` (also accept `--flag=value`).
- Short flags for common toggles only: `-y` (`--yes`), `-q` (`--quiet`).
- Hyphenated flag names: `--log-level`, `--dry-run`.
- Verb then resource: `backup instances`, `restore volume`, `list images`.
- Positional names after the resource; omit names to mean “all of that kind”.
- Repeatable flags for multi-values where needed.

Global flags:

- `--target` string: backend URI, e.g., `dir:/mnt/nas/sysbackup/incus`.
- `--backend` string: backend name; inferred from `--target` when present.
- `--project` string: Incus project scope (default `default`).
- `--config` string: optional config file path.
- `--log-level` string: `info` (default), `debug`, `warn`, `error`.
- `--dry-run`: show actions without making changes.
- `--yes, -y`: auto-confirm prompts (non-destructive checks still apply).
- `--force`: implies `--yes` and relaxes certain safety checks when necessary
  (e.g., stop/replace attached volumes); use sparingly.
- `--quiet, -q`: reduce non-essential output.
- `--parallel N`: concurrency for exports/imports.

Backup:

- All: `incus-backup backup all --target dir:/mnt/nas/sysbackup/incus`
- Instances: `incus-backup backup instances [NAME ...] --target dir:/path [--images none|referenced|all]`
- Volumes: `incus-backup backup volumes [POOL/NAME ...] --target dir:/path`
- Images: `incus-backup backup images [FINGERPRINT ...] --target dir:/path`
- Config (declarative state only): `incus-backup backup config --target dir:/path`

Backup options and defaults:

- Snapshots for consistency: by default, create a temporary snapshot for
  instances and volumes, export from the snapshot, then remove it.
  Use `--no-snapshot` to disable (advanced use only).
- Portability first: exports default to portable format. Use `--optimized`
  to enable storage-backend-optimized exports (same-backend restores only).

Restore:

- All (latest): `incus-backup restore all --target dir:/path [--latest|--version TS]`
- Instance: `incus-backup restore instance NAME --target dir:/path [--version TS] [--target-name NEW]`
- Volume: `incus-backup restore volume POOL/NAME --target dir:/path [--version TS] [--target-name NEW]`
- Images: `incus-backup restore images [FINGERPRINT ...] --target dir:/path [--version TS]`
- Config: `incus-backup restore config --target dir:/path [--version TS] [--apply]`
  - Default: preview only (prints changes). `--apply` required to change
    profiles, projects, networks, or storage pool settings.

Restore mapping flags (for differing environments):

- `--pool-map old=new` (repeatable)
- `--network-map old=new` (repeatable)
- `--project-map old=new` (repeatable)
- `--profile-map old=new` (repeatable)

Restore conflict handling:

- `--replace`: replace existing resources with the restored version.
- `--skip-existing`: skip restore of resources that already exist.

List:

- All: `incus-backup list all --target dir:/path [--output table|json|yaml]`
- Instances: `incus-backup list instances [NAME] --target dir:/path`
- Volumes: `incus-backup list volumes [POOL/NAME] --target dir:/path`
- Images: `incus-backup list images [FINGERPRINT] --target dir:/path`
- Config: `incus-backup list config --target dir:/path`

Verify & Prune (future):

- Verify: `incus-backup verify [all|instances|volumes|images|config] --target ...`
- Prune: `incus-backup prune --target ... [--keep N | --policy daily=7,weekly=4,monthly=12]`

# Requirements

When a backup is being restored, any destructive operation must require explicit
confirmation or an override by the user. If confirmed (or `--yes`/`-y`), the
operation proceeds and the tool performs any prerequisite actions needed (e.g.,
stopping instances to replace an attached volume), with clear logging.

For example:
`incus-backup restore volume POOL/dockge-data --version 20250914T121314`

If the `dockge-data` volume already exists, the application prompts for
confirmation. If confirmed (or with `--yes`), it replaces the contents
atomically (best effort) and restarts any previously running instances after
replacement. `--force` implies `--yes` and also permits actions like
auto-stopping attached instances to ensure replacement proceeds.

Additional safety controls:

- `--dry-run` shows planned actions and impact without changing the host.
- Pre-flight checks validate free space, target availability, and conflicts.
- Clear, colorized prompts list affected instances/volumes before proceeding.

Scope & limitations:

- Cluster/server DB restore is out of scope for v1. This tool focuses on
  instance, volume, image, and declarative config backups/restores on a running
  server/cluster via the API.
- Creating or modifying managed networks and storage pools can disrupt the
  host. Applying such changes requires `restore config --apply` and explicit
  confirmation (or `--yes`). `--force` expands allowable automated actions but
  should be used sparingly.

# Backup Storage Layout (directory backend)

Under the directory specified by `--target dir:/...`, create a stable,
auditable layout with metadata and checksums.

```
<dir>/
  metadata.json                  # repo-level info (schema version, created)
  instances/<project>/<name>/
    <timestamp>/
      export.tar.xz              # Incus export
      manifest.json              # type, project, name, created, source, checksums
      checksums.txt              # sha256 sums for files in this snapshot
  volumes/<project>/<pool>/<name>/
    <timestamp>/
      volume.tar.xz
      manifest.json
      checksums.txt
  images/<fingerprint>/
    <timestamp>/
      image.tar.xz
      manifest.json
      checksums.txt
  config/
    <timestamp>/
      projects.json              # export of projects
      profiles.json              # export of profiles
      networks.json              # export of networks
      storage_pools.json         # export of storage pool configs
      manifest.json              # captures scope and hashes of the above
      checksums.txt
```

- `<timestamp>` format: `YYYYMMDDThhmmssZ` (UTC) to avoid collisions.
- `manifest.json` includes Incus server version, project, resource identifiers,
  export options (snapshot/optimized), and references to source objects for
  traceability.

# Configuration & Logging

- Config sources: flags > env > config file (Viper). Example: `INCUS_BACKUP_DIR`.
- Logging: `info` by default; `--log-level debug` adds Incus API request traces.
- Progress: concise per-resource progress indicators; optional `--quiet` mode.

# Performance & Concurrency

- Parallelize exports/imports with a configurable worker pool (`--parallel N`).
- Bound memory usage; stream to disk where possible.
- Respect Incus limits and backoff on transient errors.

# Open Questions

- Retention policy and `prune` semantics (time-based vs. count-based).
- Which images to include by default (none vs. referenced-only)?
- Encryption or at-rest protection for `directory` backend (GPG, fscrypt?).
- Exact restore conflict semantics (rename vs. replace vs. fail-by-default).

# Testing & Quality

- Unit tests for all logic with ≥80% coverage.
- Integration tests (tagged `integration`) that use the Incus API for real
  operations, isolated via Incus projects. These tests must:
  - Create a unique, temporary project; clean up on success/failure.
  - Use small images (e.g., `images:alpine`) and minimal instances/volumes.
  - Write under `tests/fixtures/.tmp/` and remove after runs.
  - Be opt-in only: require build tag and an env like `INCUS_TESTS=1`.
- Lint/format via `golangci-lint` and `gofmt`.

# Commands & Tooling

- `make setup` — install toolchain and local dependencies
- `make lint` — run linters/formatters
- `make test` — run unit tests (no integration)
- `make test-integration` — run integration tests with safeguards
- `make build` — produce a static binary
- `make run` — run the CLI locally (e.g., `make run ARGS="--help"`)
