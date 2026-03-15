#!/usr/bin/env bash
set -e

REPO_URL="${GIT_BUILDER_REPO:-git@github.com:Leopere/git-builder.git}"
BRANCH="${GIT_BUILDER_BRANCH:-main}"

cd "$(dirname "$0")"

go build -o git-builder .
go test ./... -count=1 -timeout=30s 2>/dev/null || true

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
	git init
	git remote add origin "$REPO_URL"
elif ! git remote get-url origin >/dev/null 2>&1; then
	git remote add origin "$REPO_URL"
fi

if [ -z "$(git status --porcelain)" ]; then
	echo "No changes to commit"
	exit 0
fi

git add .
git commit -m "git-builder: build $(date -u +%Y%m%d%H%M%S)"
git push -u origin "$BRANCH" 2>/dev/null || git push -u origin HEAD:"$BRANCH"
