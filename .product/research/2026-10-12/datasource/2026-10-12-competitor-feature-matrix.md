# Competitor Feature Matrix: AI Agent Sandboxes (October 2026)

**Date:** 2026-10-12
**Sources:** Verified via WebFetch/WebSearch against each competitor's current docs and landing pages.

## GitHub Stars

| Project | Stars | Forks | License | Trend |
|---|---|---|---|---|
| **Lima** (lima-vm/lima) | 22.1k | 976 | Apache-2.0 | +68/wk, CNCF Incubating |
| **Coder** (coder/coder) | 16.9k | 1.6k | AGPL-3.0 | steady |
| **DevPod** (loft-sh/devpod) | 15.3k | 587 | MPL-2.0 | +36/wk, stalled (alpha-only since 2025) |
| **E2B** (e2b-dev/E2B) | 14.3k | 1.1k | Apache-2.0 | +86/wk, $21M Series A |
| **Microsandbox** | 8.7k | 473 | Apache-2.0 | +153/wk, fastest growing |
| **devenv** (cachix/devenv) | 7.7k | - | - | steady |
| **Docker Sandboxes** | N/A (proprietary) | N/A | Proprietary | launched Sep 2026 |
| **devcell** (devcell-sh/devcell) | ~4 | 0 | Apache-2.0 | pre-launch |

## Feature Matrix

Legend: Y = ships, P = partial, - = absent, ? = unclear

| Feature | devcell | Docker Sbx | E2B | Microsandbox | Coder | Lima | DevPod | devenv |
|---|---|---|---|---|---|---|---|---|
| **Isolation** | | | | | | | | |
| Container isolation | Y (Docker) | - | - | - | Y | - | Y | - |
| MicroVM isolation | - | Y | Y (Firecracker) | Y (libkrun) | - | Y (QEMU/vz/krunkit) | - | - |
| Separate kernel | - | Y | Y | Y | - | Y | - | - |
| **Agent Support** | | | | | | | | |
| Claude Code | Y | Y | Y | Y | Y | Y | - | - |
| Codex | Y | Y | Y | Y | Y | Y | - | - |
| OpenCode | Y | Y | Y | Y | Y | Y | - | - |
| Gemini CLI | Y | Y | - | - | Y | - | - | - |
| Copilot | - | Y | - | - | Y | - | - | - |
| Cursor | - | Y (SSH) | - | - | Y (SSH) | - | Y | - |
| Devin | - | Y | - | - | - | - | - | - |
| Agent-agnostic | Y | Y | Y (SDK) | Y (MCP) | Y | P (MCP) | - | - |
| **MCP Servers** | | | | | | | | |
| Bundled MCP servers | Y (12+) | Y (catalog) | - | P (own MCP) | - | Y (built-in) | - | P (built-in) |
| MCP gateway | - | Y | Y | - | - | - | - | - |
| **Browser/Auth** | | | | | | | | |
| Stealth browser login | Y (unique) | - | - | - | - | - | - | - |
| Anti-bot (Cloudflare/Kasada) | Y (unique) | - | - | - | - | - | - | - |
| Cookie/session sync | Y (unique) | - | - | - | - | - | - | - |
| **Multi-OS** | | | | | | | | |
| Linux | Y | Y | Y | Y | Y | Y | Y | Y |
| macOS VM | Y (Tart) | - | - | - | - | Y | - | - |
| Windows VM | Y (winkit) | - | - | - | - | P (WSL2) | - | - |
| **Deployment** | | | | | | | | |
| Local-first | Y | Y | - | Y | P (self-host) | Y | Y | Y |
| Cloud hosted | - | Y ($0.07-1.12/hr) | Y ($100+ credits) | P (config change) | Y | - | Y (providers) | P (cloud.devenv) |
| Free / open-source | Y (Apache-2.0) | P (CLI free, cloud paid) | P (hobby free, paid tiers) | Y (Apache-2.0) | P (AGPL, enterprise paid) | Y (Apache-2.0) | Y (MPL-2.0) | Y |
| **Auto-Approve** | | | | | | | | |
| Full auto-approve mode | Y (built-in) | Y (default in sbx) | Y (SDK: full control) | Y (MCP: full control) | Y (agent runs free) | Y (inside VM) | - | - |
| **Secrets** | | | | | | | | |
| 1Password integration | Y (native) | - | - | - | - | - | - | Y (native) |
| Credential brokering proxy | P (env vars on tmpfs) | Y (proxy injects headers) | Y (vault + proxy) | Y (destination-bound) | - | - | - | - |
| Secrets never enter sandbox | - | Y | Y | Y | - | - | - | - |
| **Network** | | | | | | | | |
| WireGuard tunnels | Y (built-in) | - | - | - | - | - | - | P (Tailscale) |
| Egress control / allowlist | Y (Solitary Mode) | Y (deny-by-default) | Y (CIDR/hostname) | Y (allowlist) | Y (Agent Firewall) | - | - | - |
| Proxy gateway | Y (squid sidecar) | Y (host-side proxy) | Y (egress proxy) | Y | - | - | - | - |
| **State** | | | | | | | | |
| Snapshot/fork/restore | - (planned) | - | Y (pause/resume/fork) | Y (snapshots) | - | Y (VM snapshots) | - | - |
| Persistent identity/HOME | Y (workspace HOME) | - | P (volumes) | P (snapshots) | Y (workspaces) | - | - | Y (services) |
| Pre-built images/registry | - (planned) | Y (kits) | Y (templates) | Y (base images) | Y (templates) | Y (templates) | Y (devcontainers) | Y (OCI build) |
| **Environment** | | | | | | | | |
| Nix-based | Y (home-manager) | - | - | - | - | - | - | Y (core) |
| Composable stacks/modules | Y (4 stacks, 4+ modules) | Y (kits) | Y (templates) | - | Y (templates) | - | Y (devcontainer features) | Y (modules) |
| Runtime version management | Y (mise) | - | - | - | - | - | - | Y (languages) |
| **GUI** | | | | | | | | |
| VNC desktop | Y | - | Y (computer-use) | - | - | - | - | - |
| RDP desktop | Y | - | - | - | - | - | - | - |
| **Connectivity** | | | | | | | | |
| Port forwarding | Y | P (cloud URLs) | Y (HTTPS) | - | Y | Y | Y | Y |
| SSH access | Y | Y | - | - | Y | Y | Y | - |
| IDE integration | P (any editor) | Y (VS Code, Cursor) | - | - | Y (VS Code, JetBrains) | - | Y (VS Code, JetBrains) | - |
| **LLM Management** | | | | | | | | |
| Model discovery/ranking | Y (devcell models) | - | - | - | - | - | - | - |
| Local LLM support (Ollama) | Y | - | - | - | - | Y (GPU accel) | - | - |
| Claude Max/Pro (no API key) | Y (unique) | - | - | - | - | - | - | - |
| **Operations** | | | | | | | | |
| GPU passthrough | - | Y | - | - | - | Y (krunkit) | - | - |
| Clone mode (read-only mount) | - | Y (--clone) | - | - | - | - | - | - |
| Disk usage / prune tools | Y | - | - | - | - | - | - | - |
| Config schema + validation | Y (JSON Schema) | - | - | - | - | - | - | - |

## New Features Added by Competitors in 2026

| Competitor | Feature | Date | Significance |
|---|---|---|---|
| **Docker Sandboxes** | Cloud Sandboxes launch | Sep 2026 | Pay-per-second cloud compute, `sbx move` local<->cloud |
| **Docker Sandboxes** | GPU passthrough | 2026 | GPU workloads in microVM sandboxes |
| **Docker Sandboxes** | Network policies (org-level) | 2026 | Enterprise governance for agent egress |
| **Docker Sandboxes** | Kits (OCI-packaged environments) | 2026 | Composable, shareable agent environments |
| **E2B** | Sandbox forking (CLI) | Sep 2026 | Fork up to 100 copies from saved state |
| **E2B** | APAC region | 2026 | US, EU, APAC availability |
| **E2B** | Secrets vault + egress proxy | 2026 | Credential brokering: value never enters sandbox |
| **Microsandbox** | Destination-bound credentials | 2026 | Secrets injected host-side into intercepted requests |
| **Microsandbox** | Egress allowlist | 2026 | Configurable egress reduction |
| **Coder** | Agent Firewall | 2026 | Domain + HTTP method allowlisting for agents |
| **Coder** | Coder Agents (free self-hosted) | 2026 | AI agent orchestration at no per-seat cost |
| **Lima** | v2.0: MCP server built-in | Nov 2025 | VM filesystem exposed as MCP tools |
| **Lima** | v2.1: macOS guest + --sync | Mar 2026 | macOS VMs on same machine, safe agent code editing |
| **Lima** | GPU acceleration (krunkit) | 2026 | Apple Silicon GPU passthrough to Linux VMs |
| **devenv** | MCP server | 2026 | Claude Code integration |
| **devenv** | SecretSpec (1Password/Keychain) | 2026 | Declarative secrets from multiple providers |

## devcell Unique Strengths (features no single competitor replicates)

1. **Stealth browser login + anti-bot bypass**: No competitor offers authenticated browser session sync with anti-fingerprint protection. Docker Sandboxes, E2B, and microsandbox have no browser auth at all.
2. **Three OS engines in one CLI**: Linux (Docker) + macOS (Tart) + Windows (winkit). Lima covers Linux+macOS but not Windows. No other competitor does all three.
3. **12+ MCP servers with backing tools baked in**: Docker has a catalog but requires manual setup per kit. Lima has a built-in MCP server but not domain-specific ones. E2B has none.
4. **Claude Max/Pro without API key**: No competitor supports Claude Max direct usage. All others require API keys or model-provider billing.
5. **LLM model discovery and ranking**: `devcell models` ranks cloud and local models by SWE-Bench score. No competitor has this.
6. **WireGuard tunnels built into config**: devenv has Tailscale Funnel; no other competitor has VPN config in the sandbox definition.
7. **1Password native integration**: Only devenv also has 1Password support (via SecretSpec). Docker/E2B/microsandbox use their own proxy-based brokering.
8. **Nix-based composable stacks**: Only devenv shares the Nix foundation. No agent-sandbox competitor uses Nix.
9. **Persistent workspace identity with shared HOME**: Coder has persistent workspaces but requires server infrastructure.

## devcell Gaps vs. Competitors

| Gap | Who ships it | Impact |
|---|---|---|
| MicroVM isolation | Docker Sbx, E2B, Microsandbox, Lima | High: security positioning |
| Snapshot/fork/restore | E2B, Microsandbox, Lima | High: parallel exploration |
| Pre-built registry images | All competitors | High: onboarding friction |
| Credential brokering (secrets never enter sandbox) | Docker Sbx, E2B, Microsandbox | Medium: security posture |
| Cloud-hosted execution | Docker Sbx, E2B, Coder | Medium: unattended runs |
| Clone mode (read-only host mount) | Docker Sbx | Medium: safety default |
| GPU passthrough | Docker Sbx, Lima | Low-Medium: niche use case |
| IDE integration (VS Code/Cursor SSH) | Docker Sbx, Coder, DevPod | Low: devcell works with any editor |
