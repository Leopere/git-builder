#!/usr/bin/env sh
# Copy into Leopere/taylor-co-smsbridge repo root as: .git-builder.sh  &&  chmod +x .git-builder.sh
#
# Swarm secrets / volumes: stack.production.yml uses external secrets and volumes. This script
# only pulls the production image and redeploys; it does not recreate secrets (see Woodpecker).
#
# Pattern matches app.a250.ca rfetcher (see copied-from-app-a250-rfetcher.sh), but:
# - Image: git.nixc.us/colin/smsbridge:production (Woodpecker .woodpecker.yml)
# - Waits for image to match git HEAD using OCI revision label, else GIT_COMMIT in image env
# - Swarm: login to git.nixc.us if REGISTRY_USER + REGISTRY_PASSWORD are set (e.g. via config script_env)
# - Stack / service: align STACK_NAME and SERVICE with `docker stack ls` / your stack (Woodpecker uses CI repo name)
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

# Default stack name must match: docker stack deploy -c stack.production.yml <name> (see Woodpecker deploy-production)
STACK_NAME="${STACK_NAME:-taylor-co-smsbridge}"
# Single service in stack.production.yml
SWARM_SVC="smsbridge"
IMAGE="${IMAGE:-git.nixc.us/colin/smsbridge:production}"
MAX_WAIT_SECS="${MAX_WAIT_SECS:-300}"
POLL_INTERVAL_SECS="${POLL_INTERVAL_SECS:-10}"
REGISTRY_HOST="${REGISTRY_HOST:-git.nixc.us}"

image_revision() {
  _img="$1"
  _v=$(docker image inspect "$_img" --format '{{index .Config.Labels "org.opencontainers.image.revision"}}' 2>/dev/null || true)
  if [ -n "$_v" ] && [ "$_v" != "<no value>" ]; then
    echo "$_v"
    return
  fi
  docker image inspect "$_img" --format '{{range .Config.Env}}{{println .}}{{end}}' 2>/dev/null | sed -n 's/^GIT_COMMIT=//p' | head -1
}

need_cmd git
need_cmd docker
need_cmd date
need_cmd hostname
need_cmd sed

[ -f "./stack.production.yml" ] || die "missing ./stack.production.yml"

SWARM_STATE="$(docker info --format '{{.Swarm.LocalNodeState}}' 2>/dev/null || true)"
[ "$SWARM_STATE" = "active" ] || die "Docker Swarm is not active (LocalNodeState=${SWARM_STATE:-unknown})"

TARGET_COMMIT="$(git rev-parse HEAD 2>/dev/null || true)"
[ -n "$TARGET_COMMIT" ] || die "could not determine current git HEAD"

log "start $(date -u +"%Y-%m-%dT%H:%M:%SZ") host=$(hostname 2>/dev/null || echo unknown)"
log "using prepared checkout at ${TARGET_COMMIT}"
log "waiting for ${IMAGE} to match git HEAD: ${TARGET_COMMIT}"

if [ -n "${REGISTRY_USER:-}" ] && [ -n "${REGISTRY_PASSWORD:-}" ]; then
  log "logging into ${REGISTRY_HOST}"
  echo "$REGISTRY_PASSWORD" | docker login -u "$REGISTRY_USER" --password-stdin "$REGISTRY_HOST" >/dev/null
fi

deadline=$(( $(date +%s) + MAX_WAIT_SECS ))
GOT=""

while :; do
  if docker pull "${IMAGE}" >/dev/null 2>&1; then
    GOT="$(image_revision "${IMAGE}")"
    if [ "$GOT" = "$TARGET_COMMIT" ]; then
      log "pulled image matches target commit ${TARGET_COMMIT}"
      break
    fi
    log "image revision is '${GOT:-<empty>}', waiting for ${TARGET_COMMIT}"
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

GOT="$(image_revision "${IMAGE}")"
log "IMAGE_REVISION=${GOT:-<empty>}"

[ "$GOT" = "$TARGET_COMMIT" ] || \
  die "after ${MAX_WAIT_SECS}s image still not at git HEAD; want ${TARGET_COMMIT}, got '${GOT:-<empty>}'"

log "deploying stack from ./stack.production.yml"
docker stack deploy --with-registry-auth -c ./stack.production.yml "${STACK_NAME}"

FULL_SVC="${STACK_NAME}_${SWARM_SVC}"
log "forcing ${FULL_SVC} rollout"
docker service update --force "$FULL_SVC" >/dev/null 2>&1 || true
docker service ps "$FULL_SVC" --no-trunc >/dev/null 2>&1 || true

log "done; revision ${GOT}"
