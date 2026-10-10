# DevCell

Your AI agent can `rm -rf /` and you're fine.

DevCell is a containerized sandbox for AI coding agents. Run Claude Code, Codex, or OpenCode with full auto-approve. Your SSH keys, other repos, and host credentials stay out of reach.

## Quickstart

**Prerequisites:** [Docker Desktop](https://www.docker.com/products/docker-desktop/) or Docker Engine.

```bash
brew install devcell-sh/tap/devcell
cd your-project
cell claude
```

On first run, `cell` creates `.devcell.toml` and `.devcell/` in your project directory, then builds the image (~5 min). Works with `cell codex` and `cell opencode` too.

## What you get

- **Isolated sandbox** - agents edit freely inside your project; your host system is untouched
- **12+ MCP servers** - Yahoo Finance, Google Maps, Linear, KiCad, Inkscape, and more. Backing tools ship in the image alongside their servers
- **Claude Max/Pro support** - runs Claude Code directly, no API key or proxy needed
- **Stealth Chromium + zero-password login** - `cell login <url>` opens a clean browser on your host, you log in, press Enter; cookies and localStorage sync to the container. The agent never sees your password. Anti-fingerprint Playwright replays sessions that pass Cloudflare and Kasada
- **Remote desktop** - VNC and RDP into the container to watch or interact with GUI apps
- **1Password secrets** - list document names in `.devcell.toml`; fields are injected as env vars into the container at runtime, written to a RAM-only tmpfs, gone when the container stops
- **Docker or VM engine** - default: a Linux Docker container. `--os macos` runs a macOS VM on Tart, `--os windows` a Windows VM via winkit; same commands
- **4 primary stacks**: `base` (minimal), `dev` (seed: stealth browser + IaC MCPs), `ultimate` (general development), `bbb` (ultimate plus specialist tools). Legacy: go/node/python/fullstack/electronics. See [MIGRATION.md](./MIGRATION.md).
- **Model ranking** - `cell models` shows cloud models (Anthropic, OpenAI, Google via OpenRouter) and local ollama models ranked by SWE-Bench score and speed, side by side

## Comparison

| | DevCell | VS Code Dev Containers | E2B | Daytona | Raw `docker run` |
|---|---|---|---|---|---|
| Host filesystem isolated | yes | no (mounts workspace) | yes | partial | manual |
| Claude Max/Pro (no API key) | yes | no | no | no | manual |
| VM-backed macOS/Windows | yes (tart, winkit) | no | no | no | no |
| MCP servers bundled | yes | manual | no | no | manual |
| IDE integration | CLI + any editor | VS Code only | SDK | IDE plugins | manual |

Use Dev Containers when you want IDE-integrated rebuild-on-save. Use E2B when you need ephemeral cloud sandboxes with an SDK. Use Daytona for team-managed remote environments. Use raw Docker when you already have a hardened seccomp/AppArmor profile. Use DevCell when the agent needs full auto-approve in a cell that can't touch your host.

## Stacks

Published to `ghcr.io/devcell-sh/devcell`. Multi-arch: linux/amd64, linux/arm64. The `dev` seed and `ultimate` general development stack support extra capability modules; `bbb` retains the specialist tools previously bundled with ultimate (see [MIGRATION.md](./MIGRATION.md)). Legacy stacks (go, node, python, fullstack, electronics) still build.

| Stack | What's inside |
|---|---|
| **base** | zsh + starship, git, tmux, ripgrep, jq, sqlite, gnupg, hurl, go-task, gitleaks, mise, nix |
| **go** | base + Go, Terraform, OpenTofu, Packer, Helm |
| **node** | base + Node.js 22, npm, stealth Chromium |
| **python** | base + Python 3.13, uv, stealth Chromium |
| **fullstack** | go + node + python |
| **electronics** | base + GUI desktop + KiCad, ngspice, ESPHome, PlatformIO, wokwi-cli |
| **ultimate** | General development, cloud tools, browser automation, desktop and Draw.io |
| **bbb** | ultimate + KiCad, Wine, QEMU, Swift, publishing, security/RE, Inkscape/GIMP, FFmpeg/yt-dlp, Kitty, wxWidgets, FreeRDP, native UI development and personal-service MCPs |

Add-on modules (set `modules = ["android"]` in `.devcell.toml`):

| Module | What's inside |
|---|---|
| **android** | ADB + fastboot and the full app RE toolkit — decompilers (jadx, apktool, cfr, dex2jar, enjarify, procyon, androguard), APK acquisition/signing (apkeep, bundletool, apksigner), static triage (apkleaks, apkid, quark-engine), dynamic analysis (mitmproxy, mitmproxy2swagger, frida-tools, jnitrace, scrcpy), OTA/boot-image tools — all platforms; Android SDK + build-tools + emulator (x86_64 only) |
| **desktop** | GUI desktop: VNC, RDP, Fluxbox, PulseAudio |
| **scraping** | Playwright stealth scripts, anti-fingerprint Chromium config |
| **infra** | Cloud CLI tools: AWS, GCP, Azure |

## Engines and guests

The guest is the OS inside the cell (`--os`); the engine is how the cell runs (`--engine`). Pick the guest and the engine follows:

| `--os` | Engine | What runs | Host |
|---|---|---|---|
| `linux` (default) | `docker` | Linux container | any |
| `macos` | `tart` | macOS VM | Apple Silicon |
| `windows` or `winpe` | `winkit` | Windows PE + WSL1 VM (vz on macOS, qemu on Linux) | macOS, Linux |

```bash
cell claude --os macos       # open Claude Code in a macOS VM
cell build --os windows      # build the Windows VM image
```

Set permanently in `.devcell.toml`:

```toml
[cell]
os = "macos"
```

## MCP servers

Baked into the image and auto-merged into each agent's config at container startup. User-defined servers are preserved. Where applicable, the backing tools ship too: KiCad, Inkscape, and OpenTofu are installed alongside their MCP servers, so the agent can run `tofu plan`, analyze PCBs, or edit SVGs. New servers ship with image updates.

| Server | Domain | Auth |
|---|---|---|
| OpenTofu | IaC provider/module docs | None |
| Yahoo Finance | Stock data, financials, options | None |
| EdgarTools | SEC filings: 10-K, 10-Q, 8-K, XBRL | None |
| FRED API | 800K+ US economic time series | Free key |
| Google Maps | Geocoding, routing, places, elevation, weather | API key |
| TripIt | Trip itinerary management | Credentials |
| Inoreader | RSS feeds, articles, search, tagging | OAuth 2.0 |
| KiCad | PCB analysis, netlist extraction, DRC, BOM | None |
| Inkscape | SVG vector graphics and DOM operations | None |
| Linear | Project and issue management | OAuth 2.1 |
| Notion | Database and page management | OAuth 2.1 |
| MCP-NixOS | Nix package search and docs | None |

## Browser login & anti-bot protection

`cell login` lets the agent use authenticated sessions without ever seeing passwords:

```bash
cell login https://example.com   # opens a real browser on your host
                                  # you log in normally, press Enter
                                  # cookies + localStorage sync to the container
cell login --force https://...   # wipe saved session and start fresh
```

**How it avoids bot detection:** the login browser opens with no CDP debugging port — no `--remote-debugging-port`, no special flags. Cloudflare, Kasada, and similar systems cannot detect it as automated. After you close the browser, a separate headless CDP instance reads the cookies from the same profile and writes `storage-state.json` for Playwright. The agent replays the session; your password is never exposed.

The fingerprint (`User-Agent`, platform, browser brands) is read from your real installed Chrome binary and saved alongside the session so Patchright uses an identical identity.

## Security

- Project directory mounted at `/workspace`, exposed inside the container as `$DEVCELL_PROJECT_DIR` (`$WORKSPACE` still set for compatibility). Host filesystem is unreachable
- SSH keys, `.env` files outside the project, and host credentials are not mounted
- Session user runs without root privileges
- 1Password secrets injected at runtime, never persisted
- GPG isolation per container (prevents SQLite lock contention)
- Gitleaks pre-commit hook and CI secret scanning

## Configuration

Project config at `.devcell.toml` (created by `cell init` or first run). Optional global defaults at `~/.config/devcell/devcell.toml`. See `cell --help` and the [CLI docs](https://devcell.sh/docs/cell) for the full reference.

A JSON Schema for the config is published at `https://devcell.sh/schema/<version>/devcell.json`. Today `<version>` is always `v0.0.0`, tracking `main`; per-release directories can be added later without changing URLs. Add this line to the top of the file for editor validation and completion (taplo, Even Better TOML for VS Code, Zed, Neovim):

```toml
#:schema https://devcell.sh/schema/v0.0.0/devcell.json
```

The schema carries a top-level `version` field so you can tell which one you are looking at. The module, stack, and package catalog of the matching home flake is published next to it, so editors offer `modules` and `stack` completions and you can check what a stack already ships before adding a package:

- `https://devcell.sh/schema/<version>/devcell-sh/home/modules.json`: module name, description, MCP servers, size
- `https://devcell.sh/schema/<version>/devcell-sh/home/stacks.json`: modules enabled and packages installed per stack
- `https://devcell.sh/schema/<version>/devcell-sh/home/packages.json`: package version and which stacks ship it

Only `v0.0.0` is published for now. It is regenerated by the `schema-generate` pre-commit hook (and on every `task cell:build`) whenever the config struct or the catalog changes; a drift test in `internal/cfg` backs that up in CI. Refresh the catalog with `task schema:catalog` (needs nix) after the home flake changes.

## WireGuard tunnels

Add one `[[wireguard]]` block per tunnel to `.devcell.toml` with a standard wg-quick config. The container gets `NET_ADMIN`, `/dev/net/tun`, and brings the tunnel up at start. Secrets never land in the config file on disk:

- `WG_PRIVATE_KEY` (required): the `[Interface]` private key. Any inline `PrivateKey` is stripped and loaded from this variable at runtime.
- `WG_PRESHARED_KEY` (optional): applied to every `[Peer]` in the tunnel. Any inline `PresharedKey` is stripped the same way. Peers with different PSKs are rejected at validation time, so use one tunnel per PSK.

Set both in the host environment where you run `cell`. When a tunnel is enabled they are forwarded into the container by name (values never appear in the `docker run` command line), written to a `/run/secrets` tmpfs, and applied with `wg set` after the interface is up. No manual `PostUp` is needed.

## Customization

Start simple, go deeper when you need to.

**Runtime versions** - drop a `.tool-versions` or `mise.toml` in your project. Runtimes install automatically at startup. No rebuild needed.

**Add packages** - add npm or Python packages in `devcell.toml`, then `cell build`.

**Extend a stack** - edit `.devcell/flake.nix` to add nix packages. Run `cell build` to apply.

**Fork nixhome** - fork the [nixhome](https://github.com/devcell-sh/home) repo, point your flake to your fork. Upstream updates still merge cleanly.

<details>
<summary><strong>Development</strong></summary>

### Building images

Local development uses `cell build` — `task image:*` is for CI/release only.

```bash
cell build                    # Rebuild the local cell image from this checkout
cell build --update           # Bump nix flake inputs + rebuild
cell build --thin             # Incremental, mounts nix store on a Docker volume
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

The image is built from composable Nix home-manager modules (`nixhome/modules/`), assembled into stacks (`nixhome/profiles/`). Validate after edits:

```bash
task nix:validate    # Syntax check + attribute resolution across all stacks
```

</details>

## Terminology

| Term | What it means |
|---|---|
| **cell** | A named, persistent identity. A boundary. One shared `$HOME` (`~/.devcell/<cellName>/`), one network, one secrets scope. Examples: `DIMM`, `work`, `personal`, `main` (default). May host many projects. May be running or stopped. |
| **project** | A host directory with code. Mounted into a container. |
| **container** | The running docker instance for one (cell, project) pair. Ephemeral. The cell is the boundary; the container is the runtime. |
| **stack** | The image variant a container is built from (`base`, `dev`, `ultimate`, `bbb`). |
| **module** | A toggleable Nix capability composed into a stack (see [MIGRATION.md](./MIGRATION.md)). |

**One-line model:** *a cell is the boundary; many projects live inside it; each project at a time spawns one container.*

## Not yet

| Gap | Status | Where to look |
|---|---|---|
| `windows-full` guest | internal only | `internal/engine` capability table |
| Snapshot/restore cells | planned | `internal/engine/docker` |
| Pre-built registry images (`cell pull`) | planned | image build pipeline |

## License

Apache 2.0
