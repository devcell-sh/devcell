# Features

All features ranked by market demand (6-signal composite scoring).
Top 5 are the focus of positioning and README. Features 6-16 ship
but are not led with in marketing.

Source: GitHub issue density 25%, community engagement 20%,
feature-demand consensus 20%, search demand 15%, competitor adoption
10%, market growth 10%. Collected 2026-10-12.

Full data: [research/2026-10-12/FEATURE-RANKING.md](research/2026-10-12/FEATURE-RANKING.md),
[research/2026-10-12/datasource/](research/2026-10-12/datasource/).

Persona: [PERSONAS.md](./PERSONAS.md) (Alex, 8-20yr senior engineer).

---

## Top 5: positioning focus

### 1. Container sandbox isolation (score: 92, Anchor)

The #1 demand signal across all surfaces. 8+ HN threads, Wikipedia
"AI agent" views +162% YoY, 20+ GitHub projects, 7/7 competitors
ship it. This is why the category exists.

- Every `devcell <agent>` runs inside a container
- SSH keys, other repos, host credentials unreachable
- The container IS the guardrail, not the approval loop
- Engagement: 5/5 GitHub, 5/5 community, 4 strong FEATURE-DEMAND
  surfaces, 5/5 search, 7/7 competitors

Table stakes (7/7 competitors ship it) but also the headline: this
is the category-defining need that gets Alex in the door. Auto-approve
(#3) is the aha moment; egress (#2) and multi-OS (#5) are comparison-
table differentiators. Lead externally with the sandbox outcome
("without the blast radius"), not the mechanism.

Research: "Lead with these in positioning."

**CLI:** `devcell claude`, `devcell codex`, `devcell opencode`,
`devcell gemini`.

### 2. Network egress control (score: 88, Anchor)

Second-highest demand. NVIDIA calls it "mandatory non-negotiable."
OWASP guidelines, CSA reports, 5,624 claude-code issues reference
network control. 5/7 competitors ship it.

- Solitary Mode: deny-by-default egress via squid proxy
- Only allow-listed domains pass
- Default presets: claude, github, npm, pypi, go, cargo, nix
- Engagement: 5/5 GitHub, 5/5 community, 4 strong surfaces, 4/5
  search, 5/7 competitors

Research: "Lead with these in positioning."

**CLI:** `devcell claude --solitary`.

### 3. Auto-approve in sandbox (score: 79, Strong)

The product identity. HN survey: most common answer "YOLO!" 2,395
claude-code issues reference auto-approve or permission frustration.
4/7 competitors ship auto-approve.

- `devcell claude` starts with full auto-approve by default
- No `--dangerously-skip-permissions` needed
- The sandbox makes it safe; the auto-approve makes it fast
- Engagement: 3/5 GitHub, 4/5 community, 2 moderate surfaces, 3/5
  search, 4/7 competitors

**Differentiator:** everyone has sandboxes (#1), everyone has
auto-approve. DevCell frames the combination as the product. No
competitor leads with this framing. This is the pitch: "Claude Code,
without the blast radius."

### 4. MCP servers with backing tools (score: 76, Strong)

Strong demand: 70K wiki views/mo for MCP, 14,584 claude-code issues.
0 HN threads ask for "baked-in MCPs." Docker has a catalog (270+
servers, manual setup). Others have none.

- Servers auto-merge into agent config at startup
- Backing tools install alongside: agent can run `tofu plan` or edit
  PCBs because OpenTofu and KiCad are installed, not just MCP binaries
- Pinned to commit hashes in `flake.lock`: byte-reproducible
- Community home ships 12+ examples; adding your own follows the
  same pattern
- Engagement: 5/5 GitHub, 1/5 community, 1 weak surface, 4/5 search,
  2/7 competitors

**Differentiator:** mechanism, not count. Docker has catalog (manual).
DevCell installs tools alongside servers.

**MCP installation pain (consensus research 2026-10-12, Claude +
Codex, 4 surfaces):** 37.5% of MCP servers never start on first
install (arxiv study of 400 npm/stdio servers). 86% need local
installation. #1 failure class is PATH/environment mismatch: GUI
apps can't find `npx`, `node`, `kubectl`. Docker responded with
300+ containerized servers (1M+ pulls). Our wedge: not count, but
auto-config + backing tools + nix-pinned versions. See
`research/MCP-INSTALL-PAIN-2026-10-12.md` for full data.

MCP spec Roots deprecation (2026-07-28) weakened the protocol's own
sandboxing: scope enforcement is now opt-in. Container isolation
fills this gap structurally.

Top 10 hardest-to-install MCP servers (ranked by community friction):

| # | Server | Core friction | DevCell ships it? |
|---|--------|---------------|-------------------|
| 1 | Playwright | 200 MB browser bins + ~15 system libs | Yes |
| 2 | Puppeteer | 200 MB Chromium, fails on proxies/CI/Alpine | Yes (Playwright) |
| 3 | PostgreSQL | Running instance + connection string | No |
| 4 | Kubernetes | kubectl + helm + kubeconfig + RBAC | Partial |
| 5 | Memory/Knowledge-Graph | better-sqlite3 native compilation | Partial (sqlite) |
| 6 | GitHub | PAT scope confusion, GUI can't see env vars | Env passthrough |
| 7 | Filesystem | Symlink CVEs, sandbox-in-sandbox | Sandboxed by design |
| 8 | AWS/Terraform | 4 prerequisites, deprecated server in results | Yes (OpenTofu) |
| 9 | Docker | Socket mounting security, fragmented | No (no DinD) |
| 10 | Git | Package unpublished from npm, SSH inaccessible | Yes |

**README action:** Lead MCP bullet with the pain DevCell eliminates,
not the count. "You don't install MCP servers; they come ready."

**CLI:** `devcell modules list`.

### 5. Multi-OS support (score: 74, Strong)

52K claude-code issues mention macOS or Windows. "macOS" appears in
claude-code autocomplete. 3/3 Google autocomplete signals.

- Linux container (Docker)
- macOS VM (Tart)
- Windows VM (winkit)
- Same commands, same config across all three
- Engagement: 5/5 GitHub, 3/5 community, 2 strong surfaces, 4/5
  search, 1/7 competitors

**Differentiator:** no single competitor covers all three. Lima covers
Linux + macOS but not Windows. Docker Sandboxes, E2B, Microsandbox:
Linux only.

**CLI:** `devcell --os linux|macos|windows <agent>`.

---

## Features 6-9: Strong tier (60-84)

Ship and are real value. Don't lead with them in marketing but include
in README "What you get" and comparison table.

### 6. Remote cell execution (score: 75, Strong, PLANNED)

Bidirectional local-remote file sync is the #1 demand signal across
all 4 research surfaces (strong). `devcell start --host ssh://user@host`
runs the cell on a remote machine while the local project directory
is mounted via a reverse SFTP tunnel. No copy-based sync, no sync
state, no conflicts.

- **Demand:** 4 surfaces, strong (same strength as egress control
  at #2). DevPod #889, Coder #16575, VS Code Remote #196/#2709.
  Docker 2025: 64% of developers use non-local environments.
- **Differentiator:** reverse SFTP mount is unclaimed whitespace.
  Every competitor (Coder/Mutagen, Okteto/Syncthing, DevPod/git)
  uses copy-based sync. Mount-based avoids sync state and conflicts
  at the cost of requiring a live SSH connection.
- **Competitive context:** Coder ($90M Series C), VS Code Remote,
  JetBrains Gateway, and Okteto all ship SSH remote dev. DevPod
  (~10K stars) is abandoned, leaving stranded users.
- **Risk:** scope expansion into a well-funded market. Mitigated by:
  (1) reverse SFTP is a genuinely different mechanism, not a worse
  copy of Mutagen; (2) devcell's sandbox story extends naturally
  ("your sandbox, on a remote machine"); (3) DevPod abandonment
  creates a timing window.
- **Status:** CELL-654. Implementation phases: host flag, SFTP
  reverse tunnel, port forwarding/UX, remote build.
- **Strategy:** design architecture pre-launch, ship post-launch
  (100-500 stars). See [strategy](research/2026-10-14/oss-remote-cell-strategy.md).
- Engagement: 4/5 competitor-features, 3/5 community, 4 surfaces
  strong, 2/5 search, 6/7 competitors ship SSH remote

Research: [research/2026-10-14/remote-cell-competitors.md](research/2026-10-14/remote-cell-competitors.md).
Strategy: [research/2026-10-14/oss-remote-cell-strategy.md](research/2026-10-14/oss-remote-cell-strategy.md).

**CLI:** `devcell start --host ssh://user@host`, then
`devcell claude`.

### 7. Secrets / credential management (score: 72)

1Password fields injected as env vars at runtime on RAM-only tmpfs,
gone when the cell stops. 1Password Wikipedia +74%. 2,646 claude-code
issues. 9 public credential incidents in 2025-2026.

- 4/7 competitors ship some form of credential management
- devenv also has native 1Password; Docker/E2B use proxy-based
- DevCell's gap: secrets enter the sandbox as env vars (competitors
  like Docker Sandboxes, E2B, Microsandbox use proxy-based injection
  where secrets never enter the sandbox)

**CLI:** configured via `devcell.toml` `[secrets]`.

### 8. Local-first / offline (score: 70)

Core architecture. Competitor gap widening: E2B is cloud-only, Daytona
archived, DevPod stalled. 2 moderate FEATURE-DEMAND surfaces.

- No cloud billing, no managed infrastructure
- BYO subscription (Max/Pro)
- Works offline once images are cached
- Engagement: 4/5 GitHub, 2/5 community, 2 moderate surfaces, 3/5
  search, 4/7 competitors

### 9. Claude Max/Pro without API key (score: 66)

55M npm dl/mo addressable market. 8+ pricing/setup articles. 0/7
competitors currently ship working Max/Pro support.

- Docker Sandboxes documents it but broken (docker/for-mac#7842)
- No API key, no proxy, no workarounds
- Uses `devcell auth chrome` for session replay
- Third-party proxies (CLIProxyAPI) blocked Apr 2026

**CLI:** `devcell auth chrome claude` to set up; then `devcell claude`.

---

## Features 10-13: Medium tier (45-59)

Genuine value with moderate demand. Often unique to DevCell but the
community doesn't know to ask for them. "Show, don't tell" features:
demos and content drive awareness, not feature requests.

### 10. Multi-agent support (score: 63)

4 agents in one CLI: Claude Code, Codex, OpenCode, Gemini. `resume`
for Codex and OpenCode sessions. Most sandboxes are agent-specific or
SDK-based. Rivet's Sandbox Agent SDK addresses "poorly standardized"
agents.

**CLI:** `devcell claude`, `devcell codex`, `devcell opencode`,
`devcell gemini`.

### 11. Nix reproducibility (score: 61)

Core architecture: home-manager + mise. 7 Nix-pinned stacks
(base/dev/ultimate + go/node/python/fullstack). devenv (7.7K stars)
shares Nix but is not agent-focused.

- HN engagement 4/5 ("Nix is a better Docker image builder")
- Wikipedia +36% for Nix
- Nix + containers are complementary (HN consensus)

### 12. Session handoff (score: 55)

Formerly "stealth browser auth" / "browser login." Renamed per
research (2026-10-12): "browser login" is ambiguous (sounds like
the agent logs in), `auth` in CLI means "authenticate with the
tool itself" everywhere else (gh auth, gcloud auth).

`devcell auth chrome <app>` logs in via host Chromium, exports
cookies for in-cell Playwright. 0/7 competitors. 0 community
threads ask for it (they don't know it's possible).

**Naming research:** "Session Handoff" matches IETF draft
(draft-moros-oauth-browser-session-handoff-00, Apr 2026) and the
AgentBay/Hermes Agent pattern. Alternative: "Browser Session
Forwarding" (self-explanatory but verbose).

**README action:** Demote from standalone section to one bullet.
Lead with "your password never enters the sandbox." Drop anti-bot
language ("identical browser fingerprint", "cannot detect").

**CLI rename candidate:** `devcell browser sync` or keep
`devcell auth chrome` (breaking change risk).

**CLI:** `devcell auth chrome <app> [-- urls]`.

### 13. LLM model discovery (score: 48)

Detect local/cloud LLMs, show capability ratings. BenchLM, LLM Stats,
WhatLLM exist but not inside sandboxes. 218+ models tracked. 3/5
community engagement.

**CLI:** `devcell models [--source S]`.

---

## Features 14-17: Long-tail (< 45)

Real features, low demand signal. Useful for specific personas. Don't
lead with these.

### 14. VNC/RDP desktop (score: 44)

IceWM/Fluxbox desktop inside the container via VNC or RDP. Relevant
for computer-use agents and GUI tools (KiCad, Inkscape, Draw.io).
E2B has VNC for computer-use; only DevCell has RDP.

This is the "lights on" demo feature: the agent drives a headed
Chromium visible to the human. See POSITIONING.md for demo asset spec.

**CLI:** `devcell vnc` / `devcell rdp`.

### 15. Port forwarding (score: 42)

Forward ports from sandbox to host. Growing demand: "safe preview
ports." CVE-2026-82533 (DeepSeek) showed port security risks. 5/7
competitors have it.

**CLI:** configured via `devcell.toml` `[ports] forward`.

### 16. Persistent cell identity (score: 38)

One `$HOME`, one network, one secrets scope per workspace. Container
restarts in seconds; cell state survives across days. Coder has
persistent workspaces but needs a server.

**CLI:** `devcell --cell-name NAME`.

### 17. WireGuard tunnels (score: 35)

WireGuard configs baked into the cell. Wikipedia stable 17K/mo. 959
claude-code issues. 0/7 competitors have VPN in sandbox config. Niche
for agent use case.

**CLI:** configured via `devcell.toml` `[[wireguard]]`.

---

## Differentiators summary

What DevCell answers that competitors don't, regardless of demand score:

| Feature | Gap | Evidence |
|---------|-----|----------|
| Sandbox + auto-approve as product identity | No competitor leads with this framing | HN: "YOLO!" top answer |
| Config preservation (CLAUDE.md, hooks, skills) | Docker Sandboxes drops all of it. 0 demand signal. | Gap discovered only when switching |
| MCP servers with backing tools | Docker: catalog (manual). Others: none. | 70K wiki views/mo for MCP |
| Claude Max/Pro out of the box | Docker Sbx: documented but broken. 0/7 others. | 55M npm dl/mo market |
| Three OS engines | No single competitor covers all three | 52K issues mention macOS/Windows |
| Remote cell via reverse SFTP | Every competitor uses copy-based sync (Mutagen, Syncthing). Reverse SFTP mount is unclaimed. | #1 demand signal, 4 surfaces strong. PLANNED. |
| Session handoff (browser session forwarding) | 0/7 competitors. 0 threads ask for it. | Niche but unique. Demote to bullet, not section. |
| 1Password secrets injection | devenv also has it. Docker/E2B use proxy. | 1Password Wikipedia +74% |

----

# Changes needed

## 2026-10-14: Remote cell execution added as feature #6

### What changed

Added remote cell execution (CELL-654) as feature #6 (score: 75,
Strong tier, PLANNED). Renumbered all subsequent features (#6-16
became #7-17). Based on consensus research (Claude + Codex, 4
surfaces, 68 findings):

- Bidirectional file sync is #1 demand signal across all surfaces
- Reverse SFTP mount is unclaimed whitespace (no competitor uses it)
- CDE market pivoting to AI agent execution (OpenAI/Ona, Coder $90M)
- DevPod abandoned, stranding ~10K users with unmet remote dev needs

### Score justification

75 (Strong) rationale: 4-surface strong demand signal (matches
egress at #2), but placed below top 5 because: (1) not yet shipped,
(2) scope expansion into well-funded competitor territory, (3)
persona's primary need is still local sandbox safety. Placed above
secrets (#7, score 72) because demand signal is stronger (4 surfaces
vs 2).

### Feature re-sort assessment

Top 5 unchanged. The top 5 features (sandbox, egress, auto-approve,
MCP, multi-OS) are the sandbox identity. Remote cells are an
expansion of that identity ("your sandbox, on a remote machine")
but do not replace any of the core positioning. If remote cells
ship and gain traction, consider promoting to top 5 in the next
quarterly re-ranking.

## 2026-10-12T19:00:00Z README review research

### Stack positioning (research 2026-10-12)

Security/RE and electronics have the worst "setup pain to available
pre-built environments" ratio. This is DevCell's biggest show-off
opportunity. Ranked by demand signal:

| # | Category | Evidence | DevCell? | Setup pain |
|---|----------|----------|----------|-----------|
| 1 | Data Science / ML | Transformers 163k, PyTorch 102k | Partial | High |
| 2 | Security / Pentesting | Awesome-Hacking 122k, r/netsec 570k | Yes (bbb) | Very high |
| 3 | Reverse Engineering | Ghidra 85k, ImHex 55k, r/RE 176k | Yes (bbb) | Very high |
| 4 | Browser Automation | Browser Use 50k, +45% search demand | Yes | High |
| 5 | Infrastructure / DevOps | OpenTofu 10M downloads | Yes (go) | Moderate |
| 6 | Electronics / PCB | KiCad, near-zero competition | Yes | Very high |

**Action:** README stacks section should hero security/RE,
electronics, and infra. Fullstack web is table stakes.

### Content layer split (community-home)

Stacks, modules, and MCP servers live in community-home
(github.com/devcell-sh/home), not in the devcell repo. README
should present this as: "batteries included, everything swappable."
Show 3-4 hero stacks inline, link to full catalog.

### Session handoff (rename)

"Browser login" renamed to "Session Handoff" per IETF draft
(draft-moros-oauth, Apr 2026). Demote from section to bullet.
CLI rename candidate: `devcell browser sync` (breaking change,
defer to next minor).

### Actionable items

| Item | Priority | Status |
|------|----------|--------|
| MCP bullet: lead with install-pain elimination | Now | README change |
| Stacks: hero security/RE, electronics, infra | Now | README change |
| Session handoff: rename section to bullet | Now | README change |
| Configuration: collapse to 2 lines | Now | README change |
| Customization: compress to 5 lines | Now | README change |
| CLI rename: `devcell auth chrome` to `devcell browser sync` | Future | Breaking change, needs deprecation |
| Stack gap: Rust | Future | No Rust stack in community-home |
| Stack gap: Java/JVM enterprise | Future | No JVM stack |
| Stack gap: Game dev (Godot, Bevy) | Future | No game dev stack |
| PostgreSQL MCP server | Future | Top-3 install pain, not shipped |
| Config-survived banner at startup | Future | OBSTACLES A6 |
| Nix cache auto-hydration in `devcell build` | Next | Plumbing exists (`devcell nix-store pull`), needs integration into build flow |
| `devcell pull` (opt-in instant cold start) | Future | After auto-hydration ships |

## 2026-10-12T16:30:00Z Initial assessment

### Gaps to close

| Gap | Competitors who ship it | Priority | Notes |
|-----|------------------------|----------|-------|
| Credential brokering proxy (secrets never enter sandbox) | Docker Sandboxes, E2B, Microsandbox | High | DevCell currently uses 1Password tmpfs env vars. Proxy-based injection is the industry direction. |
| Snapshot / restore | E2B, Microsandbox | Medium | Planned. See `internal/engine/docker`. |
| Pre-built registry images (`devcell pull`) | E2B | Medium | Planned. Would cut first-run time from ~5 min to ~30 sec. |
| MicroVM isolation (separate kernel) | Docker Sandboxes, E2B, Microsandbox, Lima | Low | Architectural decision. Container isolation is sufficient for persona but weaker in comparison tables. |

### Data to refresh

- Re-run feature demand scoring quarterly (next: 2027-01)
- Verify MCP server counts against published catalog after each
  community-home release
- Track Docker Sandboxes feature additions monthly
- Monitor Microsandbox growth trajectory (currently +153/wk)

### Positioning gaps

- Feature #11 (stealth browser auth) and #13 (VNC/RDP desktop) are
  the "lights on" demo features but have no demo asset yet. Demo
  gates the HN launch. See LANDING.md and launches/2026-Q4/hackernews.md
- Feature #3 (auto-approve + sandbox) is the product identity but
  the current README does not lead with this framing
