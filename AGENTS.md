# Repository Guidelines

## Project Structure & Module Organization
- `src/` — application code organized by feature (e.g., `src/backup/`, `src/cli/`).
- `tests/` — mirrors `src/` with the same layout for unit/integration tests.
- `scripts/` — developer utilities (e.g., `scripts/dev.sh`, `scripts/release.sh`).
- `docs/` — user and architecture docs; add diagrams to `docs/diagrams/`.
- `assets/` — static files such as sample configs or templates.

## Build, Test, and Development Commands
This repo uses scripts (no Makefile). Prefer these entry points:

- `scripts/setup.sh` — install toolchain and local dependencies (e.g., golangci-lint).
- `scripts/lint.sh` — run formatters/linters (e.g., `gofmt -s`, `golangci-lint run`).
- `scripts/test.sh` — run unit tests with coverage (no integration).
- `scripts/test_integration.sh` — run integration tests (see gating below).
- `scripts/build.sh` — produce a local build artifact.
- `scripts/run.sh` — run the CLI locally. Example: `scripts/run.sh -- --help`.

If a Makefile is later added, it should delegate to these scripts.

### Integration Test Gating
- Disabled by default. Enable with: `INCUS_TESTS=1 go test -tags=integration ./...`
- Tests must create and destroy a unique Incus project to avoid host impact.
- Use smallest artifacts (e.g., `images:alpine`) and temp dirs under `tests/fixtures/.tmp/`.

### Integration in a Container (no host install)
- Use `scripts/test_integration_container.sh` to run integration tests inside a
  privileged, systemd-enabled container that has Incus and Go preinstalled.
- Requires Docker or Podman on the host; no Incus install needed on the host.

Example:
- `scripts/test_integration_container.sh` — builds the image, boots the container, initializes Incus, runs `INCUS_TESTS=1 go test -tags=integration ./...`, then cleans up.

Environment variables:
- `RUNTIME=docker|podman` — choose container runtime (auto-detected by default).
- `REBUILD=1` — force rebuild the test image.
- `KEEP_CONTAINER=1` — leave the container running for debugging.
- `IMAGE_TAG=incus-itest:latest` — override image tag.
- `CONTAINER_NAME=incus-itest-run` — override container name.
- Caching: module and build caches persist under the workspace to avoid re-downloads
  on each run (`.cache/gomod`, `.cache/gocache`). Override with `GOMODCACHE` and
  `GOCACHE` if needed.

### Go Stack Notes
- Go 1.22+, Cobra for CLI, Viper for config, slog for logging.
- Use the Incus Go client (`github.com/lxc/incus/client`)—do not shell out to `incus`.
- Run `gofmt -s` and `golangci-lint` before committing.

## Coding Style & Naming Conventions
- Indentation: 4 spaces; max line length: 100.
- Files/dirs: lower_snake_case; constants: UPPER_SNAKE_CASE.
- Prefer pure functions, small modules, and clear errors (`Result`/exceptions with context).
- Run formatters before committing (e.g., `gofmt`, `black`, `prettier`).

Additional Go-specific conventions:
- Packages organized by feature under `src/` (`cli`, `backup`, `incus`, `config`, `logging`).
- Table-driven tests for units; use fakes for Incus wrappers.
- No global state for CLI; pass dependencies via small interfaces.

## Testing Guidelines
- Place tests in `tests/` mirroring `src/` (e.g., `tests/backup/test_snapshot.*`).
- Name tests descriptively; use table-driven/parametrized tests where possible.
- Aim for ≥80% line coverage; keep tests fast and deterministic.
- Provide sample data under `tests/fixtures/` and avoid network calls by default.

Integration tests specifics:
- Guard with build tag `integration` and env `INCUS_TESTS=1`.
- Use temporary Incus projects; ensure cleanup on failure.
- Never modify managed networks or storage pools without explicit gating.

CLI invariants (see REQUIREMENTS.md):
- Verb–resource syntax (e.g., `backup instances`, `restore volume`).
- Canonical backend URI via `--target` (e.g., `dir:/path`). No `--dir` shorthand.
- Snapshot-by-default exports; portable format by default; `--optimized` optional.

## Commit & Pull Request Guidelines
- Use Conventional Commits: `feat:`, `fix:`, `docs:`, `test:`, `refactor:`, `chore:`.
- Commit subject ≤72 chars; include a short, imperative description.
- PRs must include: what/why, how tested, screenshots/logs if applicable, and linked issues.
- Keep PRs focused; prefer incremental, reviewable changes.
- Keep `docs/ROADMAP.md` up to date in each PR by marking completed and in-progress items to reflect the current state.

## Security & Configuration Tips
- Do not commit secrets; use `.env` (gitignored) and provide `.env.example`.
- For backup/restore operations, default to dry-run where possible and require explicit confirmation for destructive actions.
- Validate paths and quotas; log enough to audit without leaking sensitive data.
