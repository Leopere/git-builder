#!/usr/bin/env bash
# Build git-builder for multiple OS/arch and create a GitHub release with gh.
# Usage: ./release.sh [tag]   e.g. ./release.sh v1.0.0
set -e

cd "$(dirname "$0")"
TAG="${1:-}"
if [ -z "$TAG" ]; then
	TAG="v$(date -u +%Y%m%d.%H%M%S)"
fi

# BSD/nix and common targets
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

if [ ! -f "$LIST" ] || [ ! -s "$LIST" ]; then
	echo "No artifacts built"
	exit 1
fi

echo "Creating release $TAG"
gh release create "$TAG" $(cat "$LIST") --generate-notes
rm -f "$LIST"
echo "Done: $TAG"
rm -rf dist
