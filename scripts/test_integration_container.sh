#!/usr/bin/env bash
set -euo pipefail

# Run integration tests inside a privileged systemd container with Incus.
# Supports Docker or Podman. Requires a Linux host capable of running privileged containers.

IMAGE_TAG=${IMAGE_TAG:-incus-itest:latest}
CONTAINER_NAME=${CONTAINER_NAME:-incus-itest-run}
RUNTIME=${RUNTIME:-}
REBUILD=${REBUILD:-0}
KEEP_CONTAINER=${KEEP_CONTAINER:-0}
PKGS=${PKGS:-./...}

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
IMG_DIR="$ROOT_DIR/scripts/images/incus-itest"

pick_runtime() {
  if [[ -n "${RUNTIME}" ]]; then
    echo "$RUNTIME"; return
  fi
  if command -v podman >/dev/null 2>&1; then
    echo podman; return
  fi
  if command -v docker >/dev/null 2>&1; then
    echo docker; return
  fi
  echo "No container runtime found (docker or podman)." >&2
  exit 1
}

runtime=$(pick_runtime)
echo "[itest] Using runtime: $runtime"

build_image() {
  echo "[itest] Building image $IMAGE_TAG from $IMG_DIR"
  "$runtime" build -t "$IMAGE_TAG" "$IMG_DIR"
}

image_exists() {
  if [[ "$runtime" == "docker" ]]; then
    docker image inspect "$IMAGE_TAG" >/dev/null 2>&1
  else
    podman image exists "$IMAGE_TAG"
  fi
}

if [[ "$REBUILD" == "1" || ! $(image_exists && echo yes || echo no) == yes ]]; then
  build_image
else
  echo "[itest] Image $IMAGE_TAG already exists; REBUILD=$REBUILD"
fi

cleanup() {
  local code=$?
  if [[ "$KEEP_CONTAINER" != "1" ]]; then
    echo "[itest] Cleaning up container $CONTAINER_NAME"
    "$runtime" rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true
  else
    echo "[itest] KEEP_CONTAINER=1; leaving $CONTAINER_NAME running"
  fi
  exit $code
}
trap cleanup EXIT INT TERM

# Common systemd-in-container flags
RUN_FLAGS=(
  --privileged
  --cgroupns=host
  --tmpfs /run:exec,mode=755
  --tmpfs /run/lock
  -v /sys/fs/cgroup:/sys/fs/cgroup:rw
  -v "$ROOT_DIR":/workspace:Z
  -w /workspace
  --name "$CONTAINER_NAME"
  -d
)

echo "[itest] Starting container $CONTAINER_NAME from $IMAGE_TAG"
"$runtime" run "${RUN_FLAGS[@]}" "$IMAGE_TAG"

echo "[itest] Running integration tests"
"$runtime" exec -e INCUS_TESTS=1 -e WORKDIR=/workspace "$CONTAINER_NAME" bash -lc \
  "/workspace/scripts/images/incus-itest/run-tests.sh"
