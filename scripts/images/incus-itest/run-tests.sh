#!/usr/bin/env bash
set -euo pipefail

# This script is intended to be executed inside the incus-itest container.
# It ensures Incus is running and initialized minimally, then runs tests.

WORKDIR=${WORKDIR:-/workspace}
PKGS=${PKGS:-./...}
INTEGRATION=${INTEGRATION:-1}

echo "[run-tests] Waiting for systemd to become ready..."
if command -v systemctl >/dev/null 2>&1; then
  # systemd might report 'degraded' but services are usable; use --wait for readiness
  timeout 60s bash -c 'until systemctl is-system-running --wait >/dev/null 2>&1; do sleep 1; done' || true
fi

echo "[run-tests] Ensuring Incus service is enabled and running..."
systemctl enable --now incus.service >/dev/null 2>&1 || true

echo "[run-tests] Initializing Incus minimally if needed..."
if ! incus info >/dev/null 2>&1; then
  incus admin init --minimal || {
    echo "[run-tests] Failed to initialize Incus" >&2
    exit 1
  }
fi

echo "[run-tests] Incus version: $(incus version || true)"

cd "$WORKDIR"
echo "[run-tests] Running tests in $WORKDIR"
if [[ "$INTEGRATION" == "1" ]]; then
  INCUS_TESTS=1 go test -v -tags=integration "$PKGS"
else
  go test -v "$PKGS"
fi

