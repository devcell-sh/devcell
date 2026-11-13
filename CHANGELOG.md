# Changelog

All notable changes to this project are documented in this file.
Format follows [Keep a Changelog](https://keepachangelog.com/).

## [Unreleased]

## [0.12.0] - 2026-11-13

### Removed

- **`[cell] thin`, `--thin`/`--no-thin`/`--thick` flags, and `DEVCELL_USER_IMAGE_THIN` env var are gone** — configs containing these keys are now rejected; delete them from your devcell.toml
- **`devcell cleanup` command removed** — use `cell clean {cache,build,nix,solitary}` instead

### Added

- `cell clean {cache,build,nix,solitary}` subcommands for granular cleanup instead of one destructive wipe
- `cell reap` stops stale oneshot containers with no active session, reclaiming resources from crashed or orphaned runs
- `--no-nix-volume` experimental flag on `cell build` bakes /nix into image layers for portable, volume-free deployment
- `peer-memory` MCP server for cross-agent memory sharing: Claude and Cursor agents can read each other's project memories
- New config sections `[storage]`, `[solitary]`, `[agents]` for storage caches, solitary isolation, and peer-memory wiring
- s6 entrypoint creates the session user via direct /etc/passwd edits with GID support and boot diagnostics
- Stealth-fingerprint MCP test suite and cross-architecture validation; `[stealth] arch` on an incompatible host now fails with a clear error
- `chunk_size` and `upload_jobs` in `[build]` make nix-cache push concurrency and chunk size configurable via TOML

### Changed

- All `thin_*.go` files renamed to `nix_*.go` and `Thin*` symbols to `Nix*` — no user-facing impact
- CI builds moved off paid ARM runners, cross-compiling arm64 via QEMU

### Fixed

- `cell build prune` no longer force-kills running containers by default — prevents accidental termination of active builds
- Interrupted nix builds now report as interrupted instead of failed — clearer error message on Ctrl-C
- Nixhome falls back to main when it has no matching release tag — prevents build failures on new releases

## [0.11.0] - 2026-11-04

### Added

- `devcell ps` shows running cells filtered to the current workspace; `devcell list` enumerates all cells including stopped ones; `devcell top` ranks cells by CPU usage
- Containers carry a `devcell.workspace` label so `ps` and `top` filter to the current workspace by default
- Host timezone auto-detection from `/etc/localtime` or `/etc/timezone`, exposed in the container system prompt
- `short_project_dir` config key creates bind-mount aliases via `DEVCELL_PROJECT_MOUNTS` with nesting detection
- WireGuard s6 service wiring integration test covering the full boot path

### Changed

- `devcell build prune` runs both Docker and nix cleanup in a single pass with concurrent closure resolution (bounded to 5 goroutines)
- Borderless tabwriter output replaces lipgloss bordered tables (docker ps / kubectl style)
- Nix store health thresholds raised (clean: 1 to 3 hashes, error: 1 to 5) and "drifted" renamed to "bloating"
- Default workspace name changed from `main` to `devcell`
- `docker inspect` and `docker rm` calls enforce timeouts

### Deprecated

- `--pure` flag on `devcell build prune` (nix pruning now always included)
- `devcell cleanup` command (use `devcell build prune`)

### Fixed

- [CELL-667] Browser stealth identity hardened: CrOS user-agent for arm64, new `stealth.enabled` and `stealth.user_agent` config fields
- [CELL-667] Stealth test suite consolidated from seven separate probe pages into one comprehensive Chromium session

## [0.10.0] - 2026-10-19

### Removed

- **CLI binary renamed from `cell` to `devcell`** — existing `cell` installs must be replaced; Homebrew casks, goreleaser builds, and CI all use the new name
- **Config file renamed from `.devcell.toml` to `devcell.toml`** (dotless) and sections flattened (`[ports] forward`, `[[volumes]]`, `[packages.nix] stable` → `[cell] ports`, `[cell] volumes`, `[cell] packages`) — old keys still work with a deprecation warning
- **Env vars renamed**: `DEVCELL_CELL_NAME` → `DEVCELL_WORKSPACE`, `APP_NAME` → `CELL_NAME`, `WORKSPACE` → `DEVCELL_PROJECT_DIR`, `IS_SANDBOX` → `DEVCELL_CONTAINER` — old names forwarded with deprecation warnings
- **Core identity terms renamed**: `session` → `workspace`, `bunk` → `cellID`, `cellName` derived from project + cellID — `DEVCELL_BUNK` and `DEVCELL_CELL_NAME` deprecated
- Retired `--local`, `--macos`, `--vagrant-*`, and `--qemu-*` flags in favor of `--os`, `--engine`, and `--winkit-*` equivalents — old flags still parse but warn

### Added

- Unified engine capability table pairing engines (docker, tart, winkit) with guests (linux, macos, winpe, windows-full); `--os` flag lets users pick a guest OS instead of learning engine names
- Docker engine works on Podman and rootless-container hosts — no daemon privilege or Docker Desktop required
- Nix store hydration from a pre-built GHCR cache image on first build — eliminates the cold-start penalty
- Config drift detection: `devcell start` restarts stale containers when devcell.toml changes instead of silently running an outdated configuration
- macOS VMs get a GUI toggle (`[gui] enabled`) and VNC endpoint — headless by default, graphical when needed
- Windows cells boot a PE+WSL1+Nix environment via go-winkit, replacing the full-install QEMU path
- OCI-backed VM disk cache with a `devcell disk-store` CLI — QEMU builds pull pre-built disks instead of re-downloading ISOs
- Squid-based egress proxy sidecar with domain/CIDR allowlists/blocklists; `--solitary` or `[network.proxy]` routes all traffic through a filtering proxy
- Engine-agnostic ContainerPolicy with open and solitary presets — solitary mode drops all capabilities, denies privileged/docker-socket/KVM/WireGuard/TUN
- Experimental-features registry with lifecycle tracking; features ship behind `--experimental=<name>`, validated on boot so typos fail loudly
- Host tmux bar and boot popup behind `ui-cage` and `ui-boot-popup` experiments; boot-failure view when services fail
- Streak-chart boot panel visualizing Prepare / Configure / Boot / Services progress in real time
- Cell-level `packages` shorthand: `[cell] packages = ["ripgrep", "jq"]` replaces the verbose `[packages.nix] stable` table
- Relaunched Astro marketing site around "Claude Code, without the blast radius" positioning
- CONTRIBUTING.md, SECURITY.md, and package-level Go docs with runnable examples

### Fixed

- s6-rc service-activation timeout raised from 30s to 5m with one retry — cold starts with slow oneshots no longer fail to boot
- Dangling nix profile symlink repaired when hydrated volume cache is reused across image rebuilds
- `--dry-run` no longer triggers a full image rebuild

### Changed

- CI workflows migrated to Blacksmith runners; PR-triggered Go unit tests and Dependabot added
- Go module renamed to `github.com/devcell-sh/devcell`
- nixhome reference updated to `devcell-sh/home`

## [v0.9.0] - 2026-08-28

- Extracted nixhome into standalone [devcell-sh/community-home](https://github.com/devcell-sh/home) repo
- WinPE build logic moved to external go-winkit/winpe package
- Docker builds always use the thin nix-on-volume path
- Retired internal tools/ directory (hmoptgen joins cmd/, renderps1 moves to go-winkit)

## [v0.8.2] - 2026-07-05

- Fixed flake vendorHash so `nix build .#cell` works out of the box
- `[[volumes]] mount` accepts a single path; thin images stop losing packages on shared nix-store volume rebuilds

## [v0.8.1] - 2026-07-05

- `[[volumes]] mount` single-path fix (hotfix for v0.8.0)

## [v0.8.0] - 2026-07-05

- `cell chrome`/`cell login` renamed to `cell auth chrome`
- `cell nix-store push` logs byte/throughput progress every 5s
- CI: arm64/amd64 builds run sequentially to avoid GHCR push contention
- Dropped `financial` module from ultimate stack (moved to dedicated config)

## [v0.7.0] - 2026-06-02

- First-run image pull (minutes to seconds for pre-built images)
- `cell serve` gains background mode + SSE
- Added `cell gemini` agent command
- Patchright Chromium extensions (CapSolver pinned)
- Docker disk reclaim, login cookie freshness checks

## [v0.6.0] - 2026-05-05

- `cell serve` gains OpenAI Responses API + SSE streaming
- Configurable system prompts across flags/env/toml
- Per-stack user-image tagging (ends per-session image sprawl)

## [v0.5.0] - 2026-04-21

- Vagrant engine, `cell models` shows cloud + local rankings
- `cell login` redesigned for bot-detection avoidance
- `--format` flag (text/json/yaml) on listings
- Ollama local provider support for codex command

## [v0.4.2] - 2026-04-06

- Hotfix release

## [v0.4.1] - 2026-04-06

- Hotfix release

## [v0.4.0] - 2026-04-06

- Android module with full RE toolkit
- MCP server auto-merge into agent configs

## [v0.3.0] - 2026-03-26

- Desktop module (VNC, RDP, Fluxbox, PulseAudio)
- 1Password secrets injection

## [v0.2.0] - 2026-03-04

- Multi-engine support (docker, tart)
- Stack system with composable modules

## [v0.1.0] - 2026-03-03

- Initial release: `cell claude`, `cell codex`, `cell shell`
- Isolated sandbox with project mount
- MCP servers bundled in image
