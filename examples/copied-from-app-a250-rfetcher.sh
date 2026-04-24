#!/usr/bin/env sh
# Source: 2026-04-24 from app.a250.ca
#   /var/lib/git-builder/repos/rfetcher/.git-builder.sh
# Copy into your repo root as: .git-builder.sh  &&  chmod +x .git-builder.sh
#
# Deploy rfetcher from the checkout already prepared by git-builder.
# git-builder owns the git sync step; this script only verifies the image and deploys it.
set -eu

log() {
  echo "[.git-builder] $*"
}

warn() {
  echo "[.git-builder] WARNING: $*" >&2
}

die() {
  echo "[.git-builder] ERROR: $*" >&2
  exit 1
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

STACK_NAME="${STACK_NAME:-rfetcher}"
IMAGE="${IMAGE:-ghcr.io/leopere/rfetcher:production-rfetcher}"
MAX_WAIT_SECS="${MAX_WAIT_SECS:-300}"
POLL_INTERVAL_SECS="${POLL_INTERVAL_SECS:-10}"
NETWORK_NAME="${NETWORK_NAME:-reddit-messaging}"

need_cmd git
need_cmd docker
need_cmd date
need_cmd hostname

[ -f "./stack.production.yml" ] || die "missing ./stack.production.yml"

SWARM_STATE="$(docker info --format '{{.Swarm.LocalNodeState}}' 2>/dev/null || true)"
[ "$SWARM_STATE" = "active" ] || die "Docker Swarm is not active (LocalNodeState=${SWARM_STATE:-unknown})"

TARGET_COMMIT="$(git rev-parse HEAD 2>/dev/null || true)"
[ -n "$TARGET_COMMIT" ] || die "could not determine current git HEAD"

log "start $(date -u +"%Y-%m-%dT%H:%M:%SZ") host=$(hostname 2>/dev/null || echo unknown)"
log "using prepared checkout at ${TARGET_COMMIT}"
log "waiting for GHCR image revision to match git HEAD: ${TARGET_COMMIT}"

if [ -n "${GHCR_TOKEN:-}" ]; then
  log "logging into ghcr.io"
  echo "$GHCR_TOKEN" | docker login -u Leopere --password-stdin ghcr.io >/dev/null
fi

deadline=$(( $(date +%s) + MAX_WAIT_SECS ))
RFETCHER_BUILD_COMMIT=""

while :; do
  if docker pull "${IMAGE}" >/dev/null 2>&1; then
    RFETCHER_BUILD_COMMIT="$(docker image inspect "${IMAGE}" \
      --format '{{index .Config.Labels "org.opencontainers.image.revision"}}' 2>/dev/null || true)"
    if [ "$RFETCHER_BUILD_COMMIT" = "$TARGET_COMMIT" ]; then
      log "pulled image matches target commit ${TARGET_COMMIT}"
      break
    fi
    log "image revision is '${RFETCHER_BUILD_COMMIT:-<empty>}', waiting for ${TARGET_COMMIT}"
  else
    warn "docker pull failed for ${IMAGE}; retrying until deadline"
  fi

  now="$(date +%s)"
  if [ "$now" -ge "$deadline" ]; then
    break
  fi

  left=$((deadline - now))
  if [ "$left" -lt "$POLL_INTERVAL_SECS" ]; then
    sleep "$left"
  else
    sleep "$POLL_INTERVAL_SECS"
  fi
done

RFETCHER_BUILD_COMMIT="$(docker image inspect "${IMAGE}" \
  --format '{{index .Config.Labels "org.opencontainers.image.revision"}}' 2>/dev/null || true)"
export RFETCHER_BUILD_COMMIT
log "RFETCHER_BUILD_COMMIT=${RFETCHER_BUILD_COMMIT:-<empty label>}"

if [ "$RFETCHER_BUILD_COMMIT" != "$TARGET_COMMIT" ]; then
  die "after ${MAX_WAIT_SECS}s the image revision still does not match git HEAD; want ${TARGET_COMMIT}, got '${RFETCHER_BUILD_COMMIT:-<empty>}'"
fi

docker network inspect "${NETWORK_NAME}" >/dev/null 2>&1 || \
  docker network create --driver overlay --attachable "${NETWORK_NAME}" >/dev/null

log "deploying stack from ./stack.production.yml"
docker stack deploy --with-registry-auth -c ./stack.production.yml "${STACK_NAME}"

log "forcing ${STACK_NAME}_rfetcher rollout"
docker service update --force "${STACK_NAME}_rfetcher" >/dev/null

log "waiting for ${STACK_NAME}_rfetcher to converge"
docker service update --force "${STACK_NAME}_rfetcher" >/dev/null 2>&1 || true
docker service ps "${STACK_NAME}_rfetcher" --no-trunc >/dev/null 2>&1 || true

log "image revision on host: ${RFETCHER_BUILD_COMMIT}"
log "done"
