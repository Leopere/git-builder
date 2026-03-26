# Guide for agents: install and use git-builder

This doc helps automated or augmented agents (CLI agents, IDEs, MCP clients) install, build, test, and use git-builder correctly.

## What git-builder is

A Go CLI daemon that polls configured git repositories (clone or pull with depth 1), then runs `.git-builder.sh` in each repo when it has new commits. Supports SSH and HTTPS (GitHub token). Can be installed as a service via `--install` (systemd on Linux, launchd on macOS).

## Installation (for users)

- **From a release:** Download the binary for the target OS/arch from [Releases](https://github.com/Leopere/git-builder/releases) and put it on `PATH` (e.g. `~/.local/bin` or `/usr/local/bin`).
- **From source:** `git clone` this repo, then `go build -o git-builder .` or `make build`. Then run `./git-builder --install` to install and start the service (optional).

Config: copy `config.example.yaml` to `/etc/git-builder/config.yaml`. Set `workdir`, `repos`; optionally `github_token` (HTTPS), `ghcr_token`/`ghcr_user` (GHCR pull), `script_env`, `local_override_dir`, `run_log_path` (optional JSON Lines audit file for script runs). All config is YAML only.

## Commands (for agents working in this repo)

Use finite, non-interactive commands only (no interactive prompts).

- **Build:** `go build -o git-builder .` or `make build`
- **Test:** `go test ./... -count=1 -timeout=60s`
- **Release (local, GitHub):** `./release.sh --gh v0.1.4` — requires `gh`; builds multi-arch binaries and runs `gh release create`
- **Publish:** `./publish.sh -m "msg" [--host app.a250.ca]` — invokes **`./ship.sh`** (ship calls **`./release.sh`**). Same pipeline: `./ship.sh "msg" [...]`. Deploy only: `./release.sh --host <host>`
- **Deploy binary only:** `./release.sh --host <host>` (no git)
- **Manual trigger (same logic as a fresh deploy):** `git-builder --trigger <url>` — sync and run the build script for one configured repo once, then exit. Unlike the daemon, **`--trigger` always runs the script if present** (ignores deploy state). Use for feeds like **feedmon** / **rfetcher** when you need a deploy without waiting for the next poll, e.g. `ssh app.a250.ca 'sudo git-builder --trigger https://github.com/Leopere/feedmon.git'` or the same with `rfetcher.git`.

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
- `run/` — resolve script (`ResolveScript`), run `.git-builder.sh` or override via `RunResolved` / `RunIfPresent`
- `runlog/` — optional mutex-safe append-only JSON Lines run audit log (`run_log_path` in config)
- `scriptaudit.go` — wires audit logging and `script start` lines for daemon, `--run-once`, and `--trigger`
- `svc/` — pid/state files, install/uninstall, ListJobs/KillJobs
- Config example: `config.example.yaml`. Default config path: `/etc/git-builder/config.yaml` (no env overrides)

## CI

GitHub Actions (`.github/workflows/build-release.yml`): push to `main` → test, build, update `latest` release; push tag `v*` → test, build, versioned release; **`workflow_dispatch`** runs the same build/test/release steps for `main` without a push (Actions → Run workflow). Use `gh run list`, `gh run view <id> --log-failed` to debug failures.

## MCP

MCP (Model Context Protocol) config for this project lives in **`.cursor/mcp.json`**. Cursor and Cloud Agents load that file automatically. It registers the **GitHub MCP server** so agents can work with releases, issues, and PRs.

**What you need to use MCP in this repo:**

- **Node/npx** — the GitHub server is run via `npx -y @modelcontextprotocol/server-github`.

For full “how to operate this project” (build, test, release, and MCP), this file (AGENT.md) is the single place to look.
