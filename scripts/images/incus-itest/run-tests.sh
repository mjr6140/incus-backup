#!/usr/bin/env bash
set -euo pipefail

# This script is intended to be executed inside the incus-itest container.
# It ensures Incus is running and initialized minimally, then runs tests.

WORKDIR=${WORKDIR:-/workspace}
PKGS=${PKGS:-./...}
INTEGRATION=${INTEGRATION:-1}

echo "[run-tests] Starting Incus service..."
systemctl start incus.service >/dev/null 2>&1 || true

echo "[run-tests] Verifying Incus is responsive (initializing if needed)..."
if ! incus info >/dev/null 2>&1; then
  incus admin init --minimal >/dev/null 2>&1 || true
fi
# Retry a few times for the daemon to come up
for i in {1..20}; do
  if incus info >/dev/null 2>&1; then
    break
  fi
  sleep 0.5
done
if ! incus info >/dev/null 2>&1; then
  echo "[run-tests] Incus did not become ready" >&2
  exit 1
fi

echo "[run-tests] Incus version: $(incus version || true)"
echo "[run-tests] restic version: $(restic version || true)"

export RESTIC_PASSWORD=${RESTIC_PASSWORD:-incus-itest}
export RESTIC_PROGRESS=${RESTIC_PROGRESS:-1}
export RESTIC_CACHE_DIR=${RESTIC_CACHE_DIR:-/workspace/.cache/restic}
mkdir -p "$RESTIC_CACHE_DIR"

cd "$WORKDIR"

echo "[run-tests] Running tests in $WORKDIR"
if [[ "$INTEGRATION" == "1" ]]; then
  INCUS_TESTS=1 go test -v -tags=integration "$PKGS"
else
  go test -v "$PKGS"
fi
