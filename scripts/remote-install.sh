#!/usr/bin/env bash
# Non-interactive remote install: copy binary + config, then install service.
# Usage: ./remote-install.sh <host> <binary-path>
set -e
HOST="$1"
BINARY="$2"
if [ -z "$HOST" ] || [ -z "$BINARY" ]; then
  echo "Usage: $0 <host> <binary-path>" >&2
  exit 1
fi
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CONFIG="$REPO_ROOT/config.example.yaml"
ssh -o BatchMode=yes -o ConnectTimeout=10 "$HOST" "mkdir -p /tmp/git-builder-install"
scp -o BatchMode=yes -o ConnectTimeout=10 "$BINARY" "$HOST:/tmp/git-builder-install/git-builder"
scp -o BatchMode=yes -o ConnectTimeout=10 "$CONFIG" "$HOST:/tmp/git-builder-install/config.example.yaml"
ssh -o BatchMode=yes -o ConnectTimeout=10 "$HOST" "sudo mkdir -p /etc/git-builder /var/lib/git-builder/repos && (sudo systemctl stop git-builder 2>/dev/null || true; sleep 1) && sudo cp /tmp/git-builder-install/config.example.yaml /etc/git-builder/config.yaml && sudo /tmp/git-builder-install/git-builder --install && rm -rf /tmp/git-builder-install"
echo "Installed on $HOST"
