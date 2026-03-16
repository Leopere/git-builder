# Guide for agents: install and use git-builder

This doc helps automated or augmented agents (CLI agents, IDEs, MCP clients) install, build, test, and use git-builder correctly.

## What git-builder is

A Go CLI daemon that polls configured git repositories (clone or pull with depth 1), then runs `.git-builder.sh` in each repo when it has new commits. Supports SSH and HTTPS (GitHub token). Can be installed as a service via `--install` (systemd on Linux, launchd on macOS).

## Installation (for users)

- **From a release:** Download the binary for the target OS/arch from [Releases](https://github.com/Leopere/git-builder/releases) and put it on `PATH` (e.g. `~/.local/bin` or `/usr/local/bin`).
- **From source:** `git clone` this repo, then `go build -o git-builder .` or `make build`. Then run `./git-builder --install` to install and start the service (optional).

Config: copy `config.example.yaml` to the config path (env `GIT_BUILDER_CONFIG` or default `/etc/git-builder/config.yaml`) and set `workdir`, `repos`, and optionally `ssh_key` or `github_token` / `GIT_BUILDER_GITHUB_TOKEN`.

## Commands (for agents working in this repo)

Use finite, non-interactive commands only (no interactive prompts).

- **Build:** `go build -o git-builder .` or `make build`
- **Test:** `go test ./... -count=1 -timeout=60s`
- **Release (local):** `./release.sh <tag>` (e.g. `./release.sh v0.1.4`) — requires `gh`; builds 10 OS/arch binaries and runs `gh release create`
- **Ship:** Commit changes, then `git push origin main`. For a versioned release: `git tag v0.1.4 && git push origin v0.1.4`

## Repo conventions

- No interactive CLI; use finite commands or nohup. No long-running search in agent invocations.
- Avoid changing Docker files; do not modify production Dockerfiles unless necessary.
- Keep files under 400 lines; split into subpackages when needed.
- Prefer git versioning over backups or duplicate copies.
- Original author: Colin Knapp — keep attribution in README and LICENSE.

## Code layout

- `main.go` — flags, daemon loop, parallel poll (`max_concurrent`), SIGUSR1/SIGUSR2 job state
- `config/` — YAML config: `MaxConcurrent`, workdir, SSH key, token, repos
- `gitops/` — sync (clone/pull)
- `run/` — run `.git-builder.sh` in repo root (or `local_override_dir`/`OWNER-REPO.sh` if set)
- `svc/` — pid/state files, install/uninstall, ListJobs/KillJobs
- Config example: `config.example.yaml`. Default config path: `GIT_BUILDER_CONFIG` or `/etc/git-builder/config.yaml`

## CI

GitHub Actions (`.github/workflows/build-release.yml`): push to `main` → test, build, update `latest` release; push tag `v*` → test, build, versioned release. Use `gh run list`, `gh run view <id> --log-failed` to debug failures.

## MCP

Project root `mcp.json` registers the GitHub MCP server so agents can work with releases, issues, and PRs. Set `GITHUB_PERSONAL_ACCESS_TOKEN` in the environment when using GitHub MCP tools.
