#!/usr/bin/env bash
# Local deploy wrapper (intentionally not designed to be committed).
# Builds a fresh linux/amd64 binary and deploys to a node via systemd.
#
# Usage:
#   ./release.sh --host app.a250.ca
#
# Optional:
#   ./release.sh --gh v0.1.0     # run the original GitHub release flow (requires gh)
set -euo pipefail

cd "$(dirname "$0")"

HOST="app.a250.ca"
DO_GH_RELEASE=0
TAG=""

usage() {
  cat <<'EOF'
Usage:
  ./release.sh --host <host>
  ./release.sh --gh <tag>

Options:
  --host <host>         Target host to deploy to (default: app.a250.ca)
  --gh <tag>            Create a GitHub release using gh (runs the original multi-arch release build)
  -h, --help            Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "${1:-}" in
    --host)
      HOST="${2:-}"
      shift 2
      ;;
    --gh)
      DO_GH_RELEASE=1
      TAG="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ "$DO_GH_RELEASE" -eq 1 ]]; then
  if [[ -z "$TAG" ]]; then
    echo "--gh requires a tag (e.g. --gh v0.1.0)" >&2
    exit 1
  fi
  TARGETS="linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 freebsd/amd64 freebsd/arm64 openbsd/amd64 openbsd/arm64 netbsd/amd64 netbsd/arm64"
  rm -rf dist
  mkdir -p dist
  LIST="dist/.built"

  for t in $TARGETS; do
    GOOS="${t%%/*}"
    GOARCH="${t##*/}"
    OUT="dist/git-builder-${GOOS}-${GOARCH}"
    echo "Building $GOOS/$GOARCH -> $OUT"
    GOOS="$GOOS" GOARCH="$GOARCH" go build -o "$OUT" . && echo "$OUT" >> "$LIST" || true
  done

  if [[ ! -f "$LIST" || ! -s "$LIST" ]]; then
    echo "No artifacts built" >&2
    exit 1
  fi

  echo "Creating GitHub release $TAG"
  gh release create "$TAG" $(cat "$LIST") --generate-notes
  rm -f "$LIST"
  echo "Done: $TAG"
  rm -rf dist
  exit 0
fi

echo "Building fresh linux/amd64 binary..."
TMP_LOCAL="/tmp/git-builder-new.$$"
GOOS=linux GOARCH=amd64 go build -o "$TMP_LOCAL" .

TMP_REMOTE="/tmp/git-builder-new.$$"
# Keepalives reduce mid-transfer "Connection reset by peer" on flaky paths.
SSH_OPTS=(-o BatchMode=yes -o ConnectTimeout=30 -o ServerAliveInterval=30 -o ServerAliveCountMax=6)
echo "Uploading to $HOST..."
scp "${SSH_OPTS[@]}" "$TMP_LOCAL" "$HOST:$TMP_REMOTE"

echo "Restarting git-builder on $HOST..."
ssh "${SSH_OPTS[@]}" "$HOST" \
  "sudo systemctl stop git-builder 2>/dev/null || true && \
   sudo cp '$TMP_REMOTE' /usr/local/bin/git-builder && sudo chmod 0755 /usr/local/bin/git-builder && \
   sudo systemctl start git-builder && \
   sudo systemctl --no-pager -l status git-builder"

ssh "${SSH_OPTS[@]}" "$HOST" "rm -f '$TMP_REMOTE' 2>/dev/null || true"
rm -f "$TMP_LOCAL" 2>/dev/null || true

echo "Deploy complete."
