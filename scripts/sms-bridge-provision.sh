#!/usr/bin/env bash
# One-time (or repeat-safe) setup: GitHub deploy key for taylor-co-smsbridge, then push
# private key + config to the server over SSH. Requires: gh auth, ssh to Host sms-bridge.
#
# Usage: ./scripts/sms-bridge-provision.sh
# Env:   SMS_BRIDGE_HOST=sms-bridge  (default; must match ~/.ssh/config)
#        DRY_RUN=1                 (only create local key + gh; skip ssh)
set -euo pipefail

cd "$(dirname "$0")/.."
ROOT="$PWD"
REPO="Leopere/taylor-co-smsbridge"
REPO_SSH="git@github.com:Leopere/taylor-co-smsbridge.git"
KEYDIR="$ROOT/deploy-keys/sms-bridge"
KEY_NAME="id_ed25519_taylor_smsbridge"
PUB_PATH="$KEYDIR/${KEY_NAME}.pub"
PRIV_PATH="$KEYDIR/$KEY_NAME"
CONFIG_SRC="$ROOT/config/sms-bridge.yaml"
HOST="${SMS_BRIDGE_HOST:-sms-bridge}"

if [[ ! -f "$CONFIG_SRC" ]]; then
  echo "Missing $CONFIG_SRC" >&2
  exit 1
fi

mkdir -p "$KEYDIR"
if [[ ! -f "$PRIV_PATH" ]]; then
  echo "Generating deploy key (repo-only) at $PRIV_PATH"
  ssh-keygen -t ed25519 -N "" -f "$PRIV_PATH" -C "git-builder ${REPO_SSH}"
fi

MARKER="$KEYDIR/.github-deploy-key-registered"
if [[ -f "$MARKER" ]]; then
  echo "GitHub: deploy key was registered before ($MARKER); skip gh repo deploy-key add."
else
  echo "Registering read-only deploy key on ${REPO} (if new)..."
  set +e
  gh_err="$(
    gh repo deploy-key add -R "$REPO" -t "sms-bridge git-builder" "$PUB_PATH" 2>&1
  )"
  gh_ec=$?
  set -e
  if [[ $gh_ec -ne 0 ]]; then
    if echo "$gh_err" | grep -qiE 'already|422|key already exists'; then
      echo "GitHub: deploy key already present (OK)."
    else
      echo "$gh_err" >&2
      exit "$gh_ec"
    fi
  else
    echo "GitHub: deploy key added."
  fi
  date -Iseconds >"$MARKER"
fi

if [[ "${DRY_RUN:-0}" == "1" ]]; then
  echo "DRY_RUN=1: skip SSH. Key: $PRIV_PATH  Config: $CONFIG_SRC"
  exit 0
fi

echo "Pushing key + config to ${HOST}..."

tmp_k="/tmp/.gb-k-$$"
tmp_c="/tmp/.gb-c-$$"
scp -o BatchMode=yes -o ConnectTimeout=20 "$PRIV_PATH" "$HOST:$tmp_k"
scp -o BatchMode=yes -o ConnectTimeout=20 "$CONFIG_SRC" "$HOST:$tmp_c"

ssh -o BatchMode=yes -o ConnectTimeout=20 "$HOST" "set -e
sudo mkdir -p /etc/git-builder /var/lib/git-builder/repos
sudo install -m 600 -T ${tmp_k} /etc/git-builder/${KEY_NAME}
rm -f ${tmp_k}
sudo install -m 644 -T ${tmp_c} /etc/git-builder/config.yaml
rm -f ${tmp_c}
if systemctl is-enabled git-builder >/dev/null 2>&1; then
  sudo systemctl restart git-builder
  sudo systemctl --no-pager -l status git-builder
else
  echo 'git-builder service not installed yet; run your binary install (e.g. ./release.sh --host ${HOST}) first.'
fi
"

echo "Done."
