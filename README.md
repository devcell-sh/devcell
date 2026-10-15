# DevCell

Claude Code, without the blast radius.

[![CI](https://img.shields.io/github/actions/workflow/status/devcell-sh/devcell/build.dev.yml?branch=main)](https://github.com/devcell-sh/devcell/actions)
[![Release](https://img.shields.io/github/v/release/devcell-sh/devcell)](https://github.com/devcell-sh/devcell/releases)
[![License](https://img.shields.io/github/license/devcell-sh/devcell)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/devcell-sh/devcell)](https://goreportcard.com/report/github.com/devcell-sh/devcell)

<p align="center">
  <img src="web/public/demo.svg" alt="devcell claude boot sequence">
</p>

## Why DevCell?

AI coding agents need root-level access to be productive: they install packages, rewrite configs, delete files. If you let them do that on your actual machine, one bad tool call can wreck your environment. If you lock them down with permission prompts, you lose the speed that makes agents useful.

DevCell gives agents a full sandbox with zero permission prompts. Your toolchain, your Claude config, your MCP servers: all carried over. Your host filesystem, SSH keys, and credentials: unreachable.

> "Run this in a container, not your actual machine."
> -- Anthropic, Claude Code docs

## Quickstart

**Prerequisites:** Docker Desktop, OrbStack, Colima, Rancher Desktop, or Docker Engine.

```bash
brew install devcell-sh/tap/devcell
cd your-project
devcell claude
```

On first run, `devcell` creates `devcell.toml` and `.devcell/` in your project directory, then builds the image (~5 min). Also: `devcell codex`, `devcell opencode`, `devcell gemini`.

## What you get

After `devcell claude` (or `codex`, `opencode`, `gemini`):

### Container sandbox with full auto-approve

Every `devcell <agent>` runs inside an isolated cell. SSH keys, other repos, and host credentials are unreachable. The container is the guardrail, not the approval loop. Agents run with no permission prompts: the sandbox makes it safe, the speed makes it worth it.

### MCP servers with backing tools

You don't install MCP servers; they come ready. 86% of MCP servers need local binaries that break on PATH mismatches and missing system libs. Here, the backing tools ship too: the agent can run `tofu plan` or edit PCBs because OpenTofu and KiCad are installed, not just their MCP servers.

### Your setup carries over

CLAUDE.md, hooks, skills, and MCP config mount into the cell. Claude Max/Pro works out of the box: no API key, no proxy, no workarounds. No account or login required.

**Also:** three OS engines (Linux container, macOS VM, Windows VM<sup><a href="#experimental-features">e</a></sup>) | egress lockdown with `--solitary` | 1Password secrets on RAM-only tmpfs | session handoff for authenticated browsing

## Comparison

| | DevCell | Docker Sandboxes | E2B | Microsandbox | Raw `docker run` |
|---|---|---|---|---|---|
| Host isolation | container, VM | microVM | Firecracker | libkrun | manual |
| Account / login required | no | yes | API key | no | no |
| Claude Max/Pro (no API key) | yes | documented but broken | no | no | manual |
| CLAUDE.md + hooks preserved | yes | no | no | no | manual |
| MCP servers + backing tools | yes, auto-config | catalog (manual) | no | own MCP | manual |
| macOS + Windows VMs<sup><a href="#experimental-features">e</a></sup> | yes | no | no | no | no |
| Egress lockdown | yes | yes | yes | yes | manual |
| Local-first | yes | yes | cloud only | yes | yes |
| Credential brokering | not yet | yes (proxy) | yes (proxy) | yes | manual |
| Snapshot / restore | not yet | not yet | yes | yes | no |

Use Docker Sandboxes for microVM isolation or cloud-hosted sandboxes. Use E2B for ephemeral cloud sandboxes with an SDK. Use Microsandbox for libkrun microVMs. Use DevCell when the agent needs full auto-approve with your toolchain and Claude setup intact, and no account or cloud dependency.

## Stacks

Batteries included, everything swappable. Stacks and modules are defined in [devcell-sh/home](https://github.com/devcell-sh/home). Use them as-is, extend with modules, or fork and build your own. Base images and nix cache published to `ghcr.io/devcell-sh/devcell` (linux/amd64 + linux/arm64); `devcell build` layers your project config on top.

| Stack | Highlights |
|---|---|
| **base** (default) | zsh + starship, git, tmux, ripgrep, jq, sqlite, mise, nix |
| **ultimate** | Go, Node.js, Python, OpenTofu, Playwright, stealth Chromium, IceWM desktop, Draw.io, Inkscape |
| **bbb** (beyond bloated baseline) | ultimate + security/RE (Ghidra-ready), Wine, QEMU, Swift, KiCad, FFmpeg, FreeRDP |
| **electronics** | KiCad, ngspice, ESPHome, PlatformIO, wokwi-cli, GUI desktop |

Set `stack = "ultimate"` in `devcell.toml` or run `devcell build --stack ultimate`.

Add-on modules (set `modules = ["android"]` in `devcell.toml`):

| Module | Highlights |
|---|---|
| **android** | Full app RE toolkit: jadx, apktool, frida-tools, mitmproxy, scrcpy, Android SDK |
| **desktop** | GUI desktop: VNC, RDP, IceWM, PulseAudio |
| **scraping** | Playwright stealth scripts, anti-fingerprint Chromium config |
| **infra** | Cloud CLIs: AWS, GCP, Azure |

Full package list and more stacks in [devcell-sh/home](https://github.com/devcell-sh/home). Fork it, edit one Nix file, run `devcell build`.

## Engines and guests

The guest is the OS inside the cell (`--os`); the engine is how the cell runs (`--engine`). Pick the guest and the engine follows:

| `--os` | Engine | What runs | Host | Requires |
|---|---|---|---|---|
| `linux` (default) | `docker` | Linux container | any | Docker (see Quickstart) |
| `macos` | `tart` | macOS VM | Apple Silicon | [Tart](https://github.com/cirruslabs/tart) |
| `windows` or `winpe`<sup><a href="#experimental-features">e</a></sup> | `winkit` | Windows PE + WSL1 VM (vz on macOS, qemu on Linux) | macOS, Linux | QEMU (Linux hosts) |

```bash
devcell claude --os macos       # open Claude Code in a macOS VM
devcell build --os windows --experimental=engine-windows   # build the Windows VM image (experimental)
```

Set permanently in `devcell.toml`:

```toml
[cell]
os = "macos"
```

## MCP servers

Two layers, both work:

- **Your servers** - anything you add with `claude mcp add` or put in `.mcp.json` carries over into the cell. Your config is preserved.
- **Nix-baked servers** - auto-merged into the agent's config at container startup. The backing tools ship alongside: the agent can run `tofu plan`, analyze PCBs, or edit SVGs because OpenTofu, KiCad, and Inkscape are installed, not just their MCP servers. Pinned to commit hashes in the nix flake, so rebuilds get the same versions.

Headline servers: OpenTofu (IaC), KiCad (PCB analysis), Inkscape (SVG), Yahoo Finance, EdgarTools (SEC filings), FRED API, Google Maps, Linear, Notion. 11 servers total. Full list and auth requirements in [devcell-sh/home](https://github.com/devcell-sh/home).

## Security

- Project directory mounted at `/<cellName>` and at its original host path (e.g. `/Users/alex/dev/api-backend`). `$DEVCELL_PROJECT_DIR` points to `/<cellName>`. Host filesystem outside the project is unreachable
- SSH keys, `.env` files outside the project, and host credentials are not mounted unless explicitly configured via `[cell] volumes`
- Session user runs without root privileges
- 1Password secrets injected at runtime, never persisted
- GPG isolation per container (prevents SQLite lock contention)
- Gitleaks pre-commit hook and CI secret scanning

Found a vulnerability? See [SECURITY.md](SECURITY.md) for reporting instructions.

## Configuration

Project config at `devcell.toml` (created by `devcell init` or first run) with [JSON Schema](https://devcell.sh/schema/v0.0.0/devcell.json) for editor validation and completion. Optional global defaults at `~/.config/devcell/devcell.toml`. See `devcell --help` for the full reference.

## WireGuard tunnels

Add one `[[wireguard]]` block per tunnel to `devcell.toml` with a standard wg-quick config. The container gets `NET_ADMIN`, `/dev/net/tun`, and brings the tunnel up at start. Secrets never land in the config file on disk:

- `WG_PRIVATE_KEY` (required): the `[Interface]` private key. Any inline `PrivateKey` is stripped and loaded from this variable at runtime.
- `WG_PRESHARED_KEY` (optional): applied to every `[Peer]` in the tunnel. Any inline `PresharedKey` is stripped the same way. Peers with different PSKs are rejected at validation time, so use one tunnel per PSK.

Set both in the host environment where you run `devcell`. When a tunnel is enabled they are forwarded into the container by name (values never appear in the `docker run` command line), written to a `/run/secrets` tmpfs, and applied with `wg set` after the interface is up. No manual `PostUp` is needed.

## When you need more

- **Runtime versions** - `.tool-versions` or `mise.toml`, no rebuild
- **Extra packages** - `devcell.toml` then `devcell build`
- **Full control** - `.devcell/flake.nix` for nix packages
- **Org-wide stacks** - fork [devcell-sh/home](https://github.com/devcell-sh/home), point your flake to your fork

<details>
<summary><strong>Development</strong></summary>

### Building images

Local development uses `devcell build` — `task image:*` is for CI/release only.

```bash
devcell build                    # Rebuild the local cell image from this checkout
devcell build --update           # Bump nix flake inputs + rebuild
devcell build --thin             # Incremental, mounts nix store on a Docker volume
task image:pure:build         # CI/release: build pure base + ultimate
task image:impure:build       # [DEPRECATED] legacy Dockerfile path, kept one release
```

### Testing

Tests use [testcontainers-go](https://testcontainers.com/guides/getting-started-with-testcontainers-for-go/) and require Docker.

```bash
task test                     # Short mode - uses pre-built image
go test -v -timeout 600s ./test/...   # Long mode - rebuilds image first
```

| Variable | Purpose |
|---|---|
| `DEVCELL_TEST_IMAGE` | Use this image instead of rebuilding |
| `DEVCELL_TEST_BASE_IMAGE` | Override base image for tests |

### Nix modules

The image is built from composable Nix home-manager modules in the [devcell-sh/home](https://github.com/devcell-sh/home) repo, assembled into stacks. Validate after edits:

```bash
task nix:validate    # Syntax check + attribute resolution across all stacks
```

</details>

## Terminology

| Term | What it means |
|---|---|
| **workspace** | A persistent grouping and boundary. One shared `$HOME` (`~/.devcell/<workspaceName>/`), one network, one secrets scope. Examples: `CELL`, `work`, `personal`, `main` (default). May host many projects. |
| **project** | A host directory with code. Mounted into a container. |
| **cell** | The engine-agnostic runtime unit for one (workspace, project) pair. Named `<project>-<cellID>`. |
| **stack** | The image variant a container is built from (`base`, `dev`, `ultimate`, `bbb`). |
| **module** | A toggleable Nix capability composed into a stack (`modules = ["android"]` in `devcell.toml`). |

**One-line model:** *a workspace is the boundary; many projects live inside it; each project spawns one or more cells.*

## Experimental features

<!-- experiments:begin -->
Off by default. Turn one on with `--experimental=<name>`, `DEVCELL_EXPERIMENTAL=<name>` or `[cell] experimental = ["<name>"]`; several are comma-separated. Experimental features may change or go away without a deprecation cycle.

| Feature | What it does | Docs | Tracking |
|---|---|---|---|
| `ui-cage` | one-line bottom bar (workspace, cell name, clocks) drawn by a host tmux session around the agent | [docs/cage.md](docs/cage.md) | CELL-655 |
| `ui-boot-popup` | boot panel in a centred tmux popup, closed before the agent starts (needs ui-cage) | [docs/cage.md](docs/cage.md) | CELL-657 |
| `engine-windows` | Windows guest (--os windows or winpe): a Windows PE + WSL VM run by the winkit engine on vz or QEMU | [README.md#engines-and-guests](README.md#engines-and-guests) | - |
<!-- experiments:end -->

Run `devcell experimental` to see what is on. Details, lifecycle and how to add one: [docs/experimental.md](docs/experimental.md).

## Not yet

| Gap | Status | Where to look |
|---|---|---|
| Credential brokering proxy (secrets never enter the sandbox) | planned | `internal/engine/docker` |
| Snapshot/restore cells | planned | `internal/engine/docker` |
| `devcell pull` (skip build entirely for default stacks) | planned | image build pipeline |
| `windows-full` guest | internal only | `internal/engine` capability table |

## Contributing

Contributions welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for setup, rules, and PR expectations. [CHANGELOG.md](CHANGELOG.md) tracks releases.

## License

Apache 2.0
