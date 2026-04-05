#!/usr/bin/env bash
# Add every change, commit if needed, deploy, push, and optionally tag/release.
# Usage: ./ship.sh "commit message" [tag] [--host HOST]
set -euo pipefail

cd "$(dirname "$0")"

HOST=""
MSG=""
TAG=""
POSITIONAL=()

usage() {
	cat <<'EOF'
Usage:
  ./ship.sh "commit message" [tag] [--host HOST]
  ./ship.sh [-m|--message "commit message"] [--tag TAG] [--host HOST]

Options:
  -m, --message   Commit message (required when the tree has changes)
  -t, --tag       Tag to create/push and publish with release.sh --gh
  --host          Deploy host for release.sh --host
  -h, --help      Show this help
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
		-t|--tag)
			TAG="${2:-}"
			shift 2
			;;
		-h|--help)
			usage
			exit 0
			;;
		--)
			shift
			while [[ $# -gt 0 ]]; do
				POSITIONAL+=("$1")
				shift
			done
			;;
		-*)
			echo "Unknown argument: $1" >&2
			usage >&2
			exit 1
			;;
		*)
			POSITIONAL+=("$1")
			shift
			;;
	esac
done

if [[ ${#POSITIONAL[@]} -gt 0 && -z "$MSG" ]]; then
	MSG="${POSITIONAL[0]}"
fi
if [[ ${#POSITIONAL[@]} -gt 1 && -z "$TAG" ]]; then
	TAG="${POSITIONAL[1]}"
fi
if [[ ${#POSITIONAL[@]} -gt 2 ]]; then
	echo "Too many positional arguments." >&2
	usage >&2
	exit 1
fi

echo "=== Adding all changes ==="
git add -A
STATUS="$(git status --porcelain)"

if [[ -n "$STATUS" ]]; then
	if [[ -z "$MSG" ]]; then
		echo "Working tree has staged or unstaged changes; pass -m/--message." >&2
		exit 1
	fi
	echo "$STATUS"
	echo ""
	echo "=== Committing ==="
	git commit -m "$MSG"
else
	echo "Working tree clean; skipping commit."
fi

if [[ -n "$HOST" ]]; then
	echo "=== Building and deploying to $HOST ==="
	./release.sh --host "$HOST"
fi

BRANCH="$(git branch --show-current)"
echo "=== Pushing origin $BRANCH ==="
git push origin "$BRANCH"

if [[ -n "$TAG" ]]; then
	echo "=== Tagging and releasing $TAG ==="
	git tag -f "$TAG"
	git push --force origin "$TAG"
	./release.sh --gh "$TAG"
	echo "Shipped and released $TAG"
else
	echo "Shipped."
fi
