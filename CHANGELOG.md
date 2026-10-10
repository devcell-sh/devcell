# Changelog

All notable changes to this project are documented in this file.
Format follows [Keep a Changelog](https://keepachangelog.com/).

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
