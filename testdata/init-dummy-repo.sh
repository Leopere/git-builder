#!/usr/bin/env bash
# Initialize the dummy repo so it can be used with file:// for local testing.
set -e
cd "$(dirname "$0")/dummy-repo"
if [ -d .git ]; then
	echo "Already a git repo"
	exit 0
fi
git init
git add .
git commit -m "Dummy repo for git-builder testing"
echo "Done. Use in config: url: file://$(pwd)"
