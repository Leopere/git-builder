#!/usr/bin/env bash
# Add every change, commit, push, and optionally tag and release.
# Usage: ./ship.sh "commit message" [tag]
#   e.g. ./ship.sh "Add config hotload and user-space install" v0.1.0
# If tag is given, creates tag, pushes it, and runs release.sh to publish the release.
set -e

cd "$(dirname "$0")"
MSG="${1:-}"
TAG="${2:-}"

if [ -z "$MSG" ]; then
	echo "Usage: $0 \"commit message\" [tag]"
	echo "  tag: e.g. v0.1.0 — if set, tag is pushed and release.sh is run"
	exit 1
fi

echo "=== Adding all changes ==="
git add -A
STATUS=$(git status --porcelain)
if [ -z "$STATUS" ]; then
	echo "Nothing to commit (working tree clean)."
	exit 0
fi
echo "$STATUS"
echo ""

echo "=== Committing ==="
git commit -m "$MSG"
BRANCH=$(git branch --show-current)
echo "=== Pushing origin $BRANCH ==="
git push origin "$BRANCH"

if [ -n "$TAG" ]; then
	echo "=== Tagging and releasing $TAG ==="
	git tag -f "$TAG"
	git push --force origin "$TAG"
	./release.sh "$TAG"
	echo "Shipped and released $TAG"
else
	echo "Shipped (no tag; use: $0 \"msg\" v0.1.0 to release)"
fi
