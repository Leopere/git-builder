#!/usr/bin/env sh
# Copy into Leopere/taylor-co-smsbridge repo root as: .git-builder.sh  &&  chmod +x .git-builder.sh
#
# Swarm secrets / volumes: stack.production.yml uses external secrets and volumes. This script
# only pulls the production image and redeploys; it does not recreate secrets.
#
# Run on the Taylor Co Swarm manager / node that hosts this stack.
# Pattern matches app.a250.ca rfetcher (see copied-from-app-a250-rfetcher.sh):
# - Wait for image to match git HEAD (OCI revision label, else GIT_COMMIT in image env)
# - Set IMAGE to whatever your pipeline tags (full registry/repo:tag). Default matches stack.production.yml image name.
# - Optional: DOCKER_REGISTRY + REGISTRY_USER + REGISTRY_PASSWORD in script_env for docker login before pull
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

# Must match: docker stack ls  (e.g. smsbridge_smsbridge -> stack smsbridge)
STACK_NAME="${STACK_NAME:-smsbridge}"
# Single service name in stack.production.yml
SWARM_SVC="smsbridge"
# Tag in stack.production.yml is taylor-co-smsbridge:production; override with full image ref for docker pull
IMAGE="${IMAGE:-taylor-co-smsbridge:production}"
MAX_WAIT_SECS="${MAX_WAIT_SECS:-300}"
POLL_INTERVAL_SECS="${POLL_INTERVAL_SECS:-10}"
# No default host — set when your registry needs login (e.g. script_env in git-builder config)
DOCKER_REGISTRY="${DOCKER_REGISTRY:-}"

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

if [ -n "${REGISTRY_USER:-}" ] && [ -n "${REGISTRY_PASSWORD:-}" ] && [ -n "${DOCKER_REGISTRY}" ]; then
  log "logging into ${DOCKER_REGISTRY}"
  echo "$REGISTRY_PASSWORD" | docker login -u "$REGISTRY_USER" --password-stdin "$DOCKER_REGISTRY" >/dev/null
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
