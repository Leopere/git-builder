# git-builder

**Original author:** [Colin Knapp](https://colinknapp.com?utm_source=github&utm_medium=readme&utm_campaign=git-builder&utm_content=attribution)

A small daemon that polls configured git repositories and runs a script (`.git-builder.sh`) in each repo when there are new commits. Uses SSH or HTTPS (GitHub token), depth-1 clone/pull, and runs the script only when the repo was updated.

## Installation

**From a release:** Download the binary for your OS/arch from [Releases](https://github.com/Leopere/git-builder/releases). Put it on your `PATH` (e.g. `~/.local/bin` or `/usr/local/bin`).

**From source:**

```bash
git clone https://github.com/Leopere/git-builder.git
cd git-builder
go build -o git-builder .
# or: make build
```

**As a service:** Run `git-builder --install` (systemd on Linux, launchd on macOS). Uninstall with `git-builder --uninstall`.

## Configuration

Copy [config.example.yaml](config.example.yaml) to your config location and edit.

**Config path:** `/etc/git-builder/config.yaml` (no environment overrides).

Configuration is read exclusively from the YAML file; no environment variables are used.

**Config fields (YAML only):**

- `poll_interval_seconds` — Poll interval in seconds (default 60, min 60).
- `workdir` — Clone directory (default `/var/lib/git-builder/repos`).
- `ssh_key` — Key filename under `/etc/git-builder` (default `id_ed25519`).
- `github_token` — Optional; for HTTPS clone/pull.
- `ghcr_token` — Optional; sets `GHCR_TOKEN` in script env; used for `docker login ghcr.io` before scripts.
- `GHCR_TOKEN` — Optional alternate YAML key for same as `ghcr_token`.
- `ghcr_user` — Optional; username for `docker login ghcr.io` (default Leopere).
- `script_env` — Optional map of env vars passed to the script.
- `max_concurrent` — Max repos building at once (default: NumCPU).
- `local_override_dir` — Optional; directory for `OWNER-REPO.sh` override scripts.
- `run_log_path` — Optional; append-only **run audit log** (JSON Lines: timestamps, repo URL, full commit, script path/kind, `start` / `success` / `failure`). Empty disables the file; use host **logrotate** if you enable it (see below).
- `repos` — List of `url` (SSH or HTTPS).

**Local override scripts:** When `local_override_dir` is set, git-builder looks for a file named `OWNER-REPO.sh` in that directory (e.g. `Leopere-git-builder.sh` for `git@github.com:Leopere/git-builder.git`). If present, that script is run instead of the repo's `.git-builder.sh`, so the host can define build steps that are triggered by repo updates but not stored in the repo.

**GitHub token (HTTPS):** Create a fine-grained token with **Contents: Read** only:  
[https://github.com/settings/personal-access-tokens/new?name=git-builder&description=Clone+and+pull+repos+over+HTTPS&contents=read](https://github.com/settings/personal-access-tokens/new?name=git-builder&description=Clone+and+pull+repos+over+HTTPS&contents=read)

## Usage

**Run as daemon (foreground):**

```bash
git-builder
```

Logs go to stdout (e.g. journald when run as a service).

**One-off poll (no daemon):**

```bash
git-builder --run-once
```

Syncs all repos and runs `.git-builder.sh` only in repos that were just cloned or had new commits pulled.

**Manual trigger (one repo):**

```bash
git-builder --trigger <url>
```

Syncs that repo and runs its build script once, then exits. The repo must be listed in config. Use after a deploy to run the **same** deploy script on demand that the daemon would run for a new commit (see [docs/AGENT.md](docs/AGENT.md)):

```bash
ssh app.a250.ca 'sudo git-builder --trigger https://github.com/Leopere/feedmon.git'
ssh app.a250.ca 'sudo git-builder --trigger https://github.com/Leopere/rfetcher.git'
```

**Install / uninstall service:**

```bash
git-builder --install    # systemd (Linux) or launchd (macOS)
git-builder --uninstall
```

**Job control (when daemon is running):**

```bash
git-builder --listjobs   # print current job or "idle"
git-builder --killjobs   # cancel current script run
```

## How it works

- Polls each configured repo on an interval (clone if missing, pull with depth 1 if present).
- **Clean worktree before pull:** For existing clones, git-builder runs `git reset --hard` to `HEAD` and `git clean -fd` before each pull so automation hosts never get stuck on local edits or untracked files.
- **Broken or corrupt clones:** If opening the repo, resetting, fetching, or advancing to `origin/main` fails for any reason, git-builder **deletes that clone directory** and does a fresh shallow clone. Local tree problems must never block sync.
- **Deploy state:** Under `workdir/.git-builder-state/` (one small JSON file per repo), git-builder records the last **successfully deployed** commit SHA. If a sync updates the checkout to a commit that is **already** recorded there, the script is skipped and the clone is left on disk for the next poll.
- **After a deploy job:** When the script runs (new commit or not yet recorded as deployed), it finishes, state is updated on success, then the **entire repo clone directory is removed** from `workdir`. The host keeps **journal logs** and **`.git-builder-state`** only; the next sync re-clones or pulls as needed. A failed script does not update state, so the next poll retries.
- **`--trigger`:** Always runs the script (ignores deploy state), then removes the clone and updates state on success.
- **When the script runs (daemon / `--run-once`):** Only after a sync that **changed** the checkout (new clone or fetch brought new commits) **and** the resulting full commit SHA is **not** already in deploy state. The script that runs is always the one resolved **after** that sync: local override `OWNER-REPO.sh` if present, else `.git-builder.sh` in the repo root (latest tree after hard reset).
- **Script:** In each repo root, looks for `.git-builder.sh` (or a local override `OWNER-REPO.sh` in `local_override_dir` if set). Script stdout/stderr are logged with microsecond timestamps on the standard logger.
- **Run audit log:** If `run_log_path` is set, each script execution appends two JSON objects (one at start, one at end with `duration_ms` and `success` or `failure`). Inspect with `jq`: e.g. `tail -1 /var/lib/git-builder/run-events.log | jq .`
- **Logs:** Successful sync lines end with a **short commit hash** (7 hex chars) for the checkout used for that run. Compare to `git ls-remote <repo-url>` (or the SHA on GitHub) to confirm the server matches the revision you expect. You will also see `script start … path=… kind=override|in-repo` before a script runs.
- **Depth:** Clone and pull use depth 1 (shallow).
- **Auth:** SSH repos use the configured key (or system default); HTTPS repos use `github_token` in config.

## CI/CD

Builds and releases are handled by GitHub Actions:

- **Push to `main`:** Tests run, all OS/arch binaries are built (cross-compiled), and the **latest** release is updated (tag `latest`).
- **Push a version tag (e.g. `v0.1.3`):** Tests run, binaries are built, and a **versioned** release is created for that tag.
- **Manual (`workflow_dispatch`):** Same build and **latest** release steps as a push to `main` — run from the Actions tab without a new commit (branch selector: use `main`).

To publish a new version:

```bash
git tag v0.1.3
git push origin v0.1.3
```

Workflow file: [.github/workflows/build-release.yml](.github/workflows/build-release.yml).

## Development

**Build:**

```bash
go build -o git-builder .
# or
make build
```

**Test:**

```bash
go test ./...
```

**Publish:** convenience wrapper; runs **`./ship.sh`** with your `-m` / `--host` (defaults in [`publish.sh`](publish.sh)). **`ship.sh`** still owns git and calling **`release.sh`**.

```bash
./publish.sh -m "your message" [--host app.a250.ca]
```

- If you have local changes, `-m` is required. If the tree is clean, you can omit `-m`.
- **Direct:** `./ship.sh "message" [--host …]` is the same pipeline without publish’s flag parsing.
- **Deploy only** (no git): `./release.sh --host app.a250.ca`

**Local multi-arch GitHub release (requires [gh](https://cli.github.com/)):**

```bash
./release.sh --gh v0.1.3
```

**For agents:** See [docs/AGENT.md](docs/AGENT.md) for install, use, and repo conventions. MCP config lives in [.cursor/mcp.json](.cursor/mcp.json); see AGENT.md for prerequisites and token setup.

## License

See [LICENSE](LICENSE). Use and redistribution require attribution to the original author.

## Credits

Original author: [Colin Knapp](https://colinknapp.com?utm_source=github&utm_medium=readme&utm_campaign=git-builder&utm_content=credits).
