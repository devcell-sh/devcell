# Changelog

All notable changes to this project are documented in this file.
Format follows [Keep a Changelog](https://keepachangelog.com/).

## [Unreleased]

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
