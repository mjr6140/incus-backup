#!/usr/bin/env bash
set -euo pipefail

# Run unit tests with caches confined to the workspace.

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
export GOMODCACHE=${GOMODCACHE:-"$ROOT_DIR/.cache/gomod"}
export GOCACHE=${GOCACHE:-"$ROOT_DIR/.cache/gocache"}
mkdir -p "$GOMODCACHE" "$GOCACHE"

cd "$ROOT_DIR"
echo "[test] GOMODCACHE=$GOMODCACHE"
echo "[test] GOCACHE=$GOCACHE"

go test ./...

