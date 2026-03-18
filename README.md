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

**Config fields:**

- `poll_interval_seconds` — How often to poll all repos (default 60, min 60, recommended 300).
- `workdir` — Where to clone repos (e.g. `/var/lib/git-builder/repos`). If unwritable, falls back to `repos` next to the binary.
- `ssh_key` — SSH key filename under `/etc/git-builder` (e.g. `id_ed25519`). System default `~/.ssh` keys are tried if not set there.
- `github_token` — Optional; for HTTPS repos. Set in the YAML config file.
- `script_env` — Optional map of env vars passed to the build script (e.g. `GHCR_TOKEN` for `docker login ghcr.io`). Keys are variable names.
- `ghcr_token` — Optional shorthand; sets `GHCR_TOKEN` in script env if not already in `script_env`.
- `repos` — List of `url` entries (SSH or HTTPS). Example: `git@github.com:user/repo.git` or `https://github.com/user/repo.git`.
- `local_override_dir` — Optional. Directory for host-local scripts named `OWNER-REPO.sh` (e.g. `Leopere-git-builder.sh`). If set and a matching file exists, it is run instead of the repo's `.git-builder.sh`, still with the repo clone as working directory.

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

Syncs that repo and runs its build script once, then exits. The repo must be listed in config. Use after a deploy to run the build on demand, e.g.:

```bash
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
- **Script:** In each repo root, looks for `.git-builder.sh` (or a local override `OWNER-REPO.sh` in `local_override_dir` if set). Runs it only when the repo was **updated** (new clone or pull fetched new commits). Script stdout/stderr are logged.
- **Depth:** Clone and pull use depth 1 (shallow).
- **Auth:** SSH repos use the configured key (or system default); HTTPS repos use `github_token` in config.

## CI/CD

Builds and releases are handled by GitHub Actions:

- **Push to `main`:** Tests run, all OS/arch binaries are built (cross-compiled), and the **latest** release is updated (tag `latest`).
- **Push a version tag (e.g. `v0.1.3`):** Tests run, binaries are built, and a **versioned** release is created for that tag.

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

**Local multi-arch release (all targets, then create release with gh):**

```bash
./release.sh v0.1.3
```

Requires [gh](https://cli.github.com/) and a tag argument (or it generates a timestamped tag).

**For agents:** See [docs/AGENT.md](docs/AGENT.md) for install, use, and repo conventions. MCP config lives in [.cursor/mcp.json](.cursor/mcp.json); see AGENT.md for prerequisites and token setup.

## License

See [LICENSE](LICENSE). Use and redistribution require attribution to the original author.

## Credits

Original author: [Colin Knapp](https://colinknapp.com?utm_source=github&utm_medium=readme&utm_campaign=git-builder&utm_content=credits).
