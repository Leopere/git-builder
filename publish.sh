#!/usr/bin/env bash
# Parses flags then invokes ./ship.sh only. ship.sh runs git and ./release.sh (unchanged).
# Defaults below override ship's host when you pass --host.
#
# Usage:
#   ./publish.sh -m "commit message" [--host HOST]
#
# With a clean working tree, -m is optional (passes empty commit arg to ship; ship skips commit).
# With local changes, -m/--message is required.
#
# For GitHub multi-arch releases, use: ./release.sh --gh v0.1.0
#
set -euo pipefail

cd "$(dirname "$0")"

# --- defaults (edit here) ---
DEFAULT_DEPLOY_HOST="app.a250.ca"

HOST="$DEFAULT_DEPLOY_HOST"
MSG=""

usage() {
  cat <<'EOF'
Usage:
  publish.sh [-m|--message "msg"] [--host HOST]

  Invokes ./ship.sh with your message and host only.

  -m, --message   Commit message (required when the tree is dirty)
  --host          SSH host (default: DEFAULT_DEPLOY_HOST at top of this script)

  ./release.sh --gh <tag>   Multi-arch GitHub release (separate from publish)

EOF
}

while [[ $# -gt 0 ]]; do
  case "${1:-}" in
    --host)
      HOST="${2:-}"
      shift 2
      ;;
    -m|--message)
      MSG="${2:-}"
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

if [[ -n "$(git status --porcelain)" && -z "${MSG:-}" ]]; then
  echo "Working tree has uncommitted changes; pass -m/--message." >&2
  exit 1
fi

echo "=== publish: ./ship.sh (git + release.sh) ==="
./ship.sh "${MSG:-}" --host "$HOST"

echo "=== publish: done ==="
