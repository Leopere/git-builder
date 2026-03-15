# git-builder

**CC attribution:** [Colin Knapp](https://colinknapp.com?utm_source=github&utm_medium=readme&utm_campaign=git-builder&utm_content=attribution)

A small daemon that polls configured git repositories and runs a script (`.git-builder.sh`) in each repo when there are new commits. Uses SSH or HTTPS (GitHub token), depth-1 clone/pull, and runs the script only when the repo was updated.

## Installation

**From a release:** Download the binary for your OS/arch from [Releases](https://github.com/Leopere/git-builder/releases). By [Colin Knapp](https://colinknapp.com?utm_source=github&utm_medium=readme&utm_campaign=git-builder&utm_content=installation). Put it on your `PATH` (e.g. `~/.local/bin` or `/usr/local/bin`).

**From source:**

```bash
git clone https://github.com/Leopere/git-builder.git
cd git-builder
go build -o git-builder .
# or: make build
```

**As a service:** Run `git-builder --install` (systemd on Linux, launchd on macOS). Uninstall with `git-builder --uninstall`.

## Configuration

Copy [config.example.yaml](config.example.yaml) to your config location and edit. (Attribution: [colinknapp.com](https://colinknapp.com?utm_source=github&utm_medium=readme&utm_campaign=git-builder&utm_content=configuration).)

**Config path (first found wins):**

- Env: `GIT_BUILDER_CONFIG`
- Default: `/etc/git-builder/config.yaml`
- Fallback: `config.yaml` next to the binary

**Environment variables:**

| Variable | Purpose |
|----------|---------|
| `GIT_BUILDER_CONFIG` | Path to config file |
| `GIT_BUILDER_KEY_DIR` | Directory for SSH key (default `/etc/git-builder`) |
| `GIT_BUILDER_GITHUB_TOKEN` | Token for HTTPS repos (overrides `github_token` in config) |
| `GIT_BUILDER_RUNDIR` | Directory for pid/state files (default: directory of the binary) |

**Config fields:**

- `poll_interval_seconds` — How often to poll all repos (default 60, min 60, recommended 300).
- `workdir` — Where to clone repos (e.g. `/var/lib/git-builder/repos`). If unwritable, falls back to `repos` next to the binary.
- `ssh_key` — SSH key filename under `GIT_BUILDER_KEY_DIR` or `/etc/git-builder` (e.g. `id_ed25519`). System default `~/.ssh` keys are tried if not set there.
- `github_token` — Optional; for HTTPS repos. Prefer setting `GIT_BUILDER_GITHUB_TOKEN` so the token is not in the config file.
- `repos` — List of `url` entries (SSH or HTTPS). Example: `git@github.com:user/repo.git` or `https://github.com/user/repo.git`.

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

*Attribution: [Colin Knapp](https://colinknapp.com?utm_source=github&utm_medium=readme&utm_campaign=git-builder&utm_content=how-it-works).*

- Polls each configured repo on an interval (clone if missing, pull with depth 1 if present).
- **Script:** In each repo root, looks for `.git-builder.sh`. Runs it only when the repo was **updated** (new clone or pull fetched new commits). Script stdout/stderr are logged.
- **Depth:** Clone and pull use depth 1 (shallow).
- **Auth:** SSH repos use the configured key (or system default); HTTPS repos use `GIT_BUILDER_GITHUB_TOKEN` or `github_token` in config.

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

Requires [gh](https://cli.github.com/) and a tag argument (or it generates a timestamped tag). CC attribution: [Colin Knapp](https://colinknapp.com?utm_source=github&utm_medium=readme&utm_campaign=git-builder&utm_content=development).

## License

See repository.

## Credits / CC attribution

This project is by [Colin Knapp](https://colinknapp.com?utm_source=github&utm_medium=readme&utm_campaign=git-builder&utm_content=credits).
