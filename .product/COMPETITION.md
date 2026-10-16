# Competition

Top 4 direct competitors + 8 market-adjacent tools = 12 total.
Data collected 2026-10-12 unless noted otherwise. See
[MARKET.md](./MARKET.md) for market conditions and demand data.

---

## Direct competitors (1:1 overlap)

### 1. Docker Sandboxes

- **What it is:** Docker's own agent sandbox, launched Sep 2026. MicroVM
  isolation via Docker Desktop.
- **Stars:** new (Docker brand, not star-driven)
- **Strengths:** Docker brand trust, microVM isolation (stronger than
  containers), egress control, MCP catalog with manual setup
- **Weaknesses:** no CLAUDE.md/hooks/skills preservation, Max/Pro OAuth
  documented but broken (docker/for-mac#7842), Linux only, no macOS/Windows VMs
- **Where they beat us:** brand recognition, microVM isolation model
- **Where we beat them:** config preservation (their biggest gap), MCP
  auto-configuration, Max/Pro works out of the box, three OS engines

### 2. Microsandbox

- **What it is:** Lightweight microVM sandbox using libkrun. Apache-2.0,
  local-first. Fastest-growing competitor.
- **Stars:** 8.7K (+153/wk as of 2026-10-12)
- **Strengths:** libkrun microVMs (stronger isolation than containers),
  snapshot/restore, own MCP server, fast growth trajectory
- **Weaknesses:** no config preservation, no Max/Pro support, no
  macOS/Windows VMs, no MCP auto-configuration
- **Where they beat us:** isolation model (microVM), snapshot/restore
  (we don't have this yet), growth velocity
- **Where we beat them:** config preservation, MCP backing tools,
  Max/Pro auth, three OS engines
- **Agent coverage (2026-10-13):** expanding fast. OpenCode integration
  is live with docs (docs.microsandbox.dev/how-to/agents/opencode):
  `msb run` mounts project as writable, isolates agent tools inside
  microVM. Claude Code support likely next. Community forks are
  extending the runtime: msb-omarchy adds display/GPU to microVMs on
  Apple Silicon (github.com/ya-luotao/msb-omarchy). Ecosystem momentum
  is real.

### 3. Daytona

- **What it is:** Cloud AI sandbox SDK. Pivoted from open-source dev
  environment manager to agent-native sandbox infrastructure in 2025.
  Went closed-source Jun 2026. $31M total funding ($24M Series A,
  Feb 2026, FirstMark Capital).
- **Stars:** 71K (frozen, repo archived Oct 2026)
- **Strengths:** sub-90ms sandbox creation, stateful snapshots, massive
  parallelization, SDK (Python/TS/JS) + REST API + CLI. Official
  sandbox partner for OpenAI Agents SDK v2 and Anthropic Managed
  Agents. Enterprise customers (LangChain, SambaNova, Stanford CS).
  HIPAA/SOC2/GDPR compliant.
- **Weaknesses:** cloud-only, closed-source, account + API key required,
  no local-first mode, no config preservation, no Max/Pro BYO support,
  container isolation (shared kernel, weaker than microVM competitors)
- **Where they beat us:** funding, brand recognition (71K stars),
  enterprise partnerships, snapshot/restore, SDK ecosystem
- **Where we beat them:** local-first (their revenue model cannot adopt
  this), config preservation, open source (Apache-2.0), no account
  required, BYO subscription, multi-OS VMs

### 4. E2B

- **What it is:** Cloud-hosted code-execution sandboxes consumed via
  SDK. Firecracker microVMs. $21M Series A.
- **Stars:** 14.3K
- **Strengths:** cloud SDK (no local setup), Firecracker isolation
  (stronger than Daytona's containers), snapshot/restore, well-funded,
  mature API. 100K+ teams, 94 of Fortune 100.
- **Weaknesses:** cloud-only (the persona does not want managed infra or
  opaque billing), no local-first mode, no config preservation
- **Where they beat us:** snapshot/restore, cloud SDK for AI app
  developers, funding, enterprise adoption
- **Where we beat them:** local-first (their billing model cannot adopt
  this), config preservation, no vendor lock-in, BYO subscription

---

## Market-adjacent (not 1:1 but compete for attention)

### 5. Anthropic /sandbox (first-party threat)

- **What it is:** Anthropic's built-in `--sandbox` flag for Claude Code.
  Sandboxes bash commands, not MCP servers.
- **Probability of full sandbox shipping:** Medium (next 12mo)
- **Our response:** retreat to wider-than-agent positioning (MCP +
  environment + auth pass-through). Their sandbox is the agent process;
  DevCell is the environment.
- **Key gap:** no MCP sandboxing, no `$HOME`, no secrets scope, no
  browser, no GUI. `docker` is incompatible with /sandbox and must be
  excluded.

### 6. Lima

- **What it is:** Linux VMs on macOS (and Linux). Apache-2.0.
- **Stars:** ~20K
- **Overlap:** local-first, supports Linux + macOS. Does not support
  Windows. Not agent-specific: general-purpose VM manager.
- **Why it's adjacent, not direct:** no agent integration, no MCP, no
  config preservation, no auto-approve framing. Different category.

### 7. Dev Containers / VS Code Remote

- **What it is:** Microsoft's containerized dev environment spec. IDE-
  native.
- **Overlap:** containerized development. Widely adopted.
- **Why it's adjacent, not direct:** IDE-centric (Alex runs agents in a
  terminal), requires `devcontainer.json` per project, team-shaped
  setup overhead. Different workflow.

### 8. Coder

- **What it is:** Self-hosted cloud development environments. Open
  source (enterprise model). Raised $90M Series C (2026) targeting
  enterprise AI agent workflows. Uses Mutagen for bidirectional
  file sync between local and remote workspaces.
- **Stars:** ~9K
- **Revenue:** $128M+ ARR, 300% YoY bookings growth. Customers
  include KKR, QRT. 79% of surveyed users lack clarity about CDE
  features (Coder 2025 State of Dev Environments report).
- **Overlap:** containerized dev environments with agent support.
  Most relevant competitor for remote cell feature: ships
  bidirectional file sync (Mutagen-based), automatic port
  forwarding, and editor-independent workspace access.
- **Why it's adjacent, not direct:** team-shaped, self-hosted infra,
  enterprise procurement motion. Wrong persona for sandbox story.
  But the most relevant comp for remote cell positioning: they
  solved the file sync problem with Mutagen, DevCell will solve it
  with reverse SFTP.

### 9. DevPod

- **What it is:** Open-source dev environment manager by Loft Labs.
  Supported SSH, Kubernetes, and cloud providers with multi-IDE
  access. Had Mutagen file sync support request (#962) but never
  shipped it natively.
- **Stars:** ~10K (stalled: last push Nov 2025, last release Mar 2025)
- **Status:** effectively abandoned. Not actively competing.
- **Remote dev relevance:** DevPod's abandonment leaves a user base
  with unmet needs. GitHub issues (#889 SFTP sync, #962 Mutagen
  support, #277 remote DOCKER_HOST, #996 initializeCommand file
  staging) document exactly what those users wanted. DevPod's SSH
  provider had recurring agent install failures (#1666, #1210,
  #1000). These stranded users are a potential adoption channel
  for devcell's remote cells.

### 10. OpenWork

- **What it is:** Open-source desktop app, "free alternative to Claude
  Cowork and Codex." Workspace for AI agents on local files.
- **Stars:** 24.1K
- **Overlap:** multi-agent workspace, MCP interface, macOS/Windows/Linux.
  50+ LLM providers, works with existing agents (Claude Code, Cursor,
  Codex) via MCP.
- **Why it's adjacent, not direct:** not a sandbox. It's an orchestrator/
  workspace. No isolation model, no egress control, no container/VM
  boundary. Different category (governance, not security).

### 11. agent-vm

- **What it is:** Rust-based tool running AI agents (Claude Code, Codex,
  OpenCode) inside libkrun microVMs. Boots in ~2s.
- **Stars:** 6 (very early)
- **Overlap:** same problem space as DevCell. Credential isolation via
  TLS-intercepting proxy (host OAuth tokens never enter the VM).
  GitHub repo access control with allow-list.
- **Why it's adjacent, not direct:** too early to be a competitor.
  Worth watching for ideas, especially the credential brokering
  approach (TLS proxy that intercepts and injects tokens).

### 12. Mutagen

- **What it is:** Standalone bidirectional file sync + network
  forwarding for remote development. Apache-2.0. Created by Jacob
  Howard (active on HN, id 33225703). Used by Coder, ddev, and
  others as the sync layer.
- **Stars:** ~4K
- **Overlap:** direct competitor to devcell's reverse SFTP mechanism.
  Mutagen is the de facto standard for copy-based file sync in
  remote dev environments. Two-way-safe mode with three-way-merge
  conflict algorithm.
- **Why it's adjacent, not direct:** sync tool, not a sandbox or
  dev environment. But it is what devcell's reverse SFTP competes
  against technically. Known pain points: performance degrades with
  large/many files (#100, #81, #87), full snapshot every cycle is
  the bottleneck, conflict resolution requires manual rsync
  workarounds.
- **DevCell's angle:** reverse SFTP avoids the copy-based model
  entirely: no sync state, no conflicts, no snapshot overhead.
  Tradeoff: requires live SSH connection (no offline editing on
  remote files).

### 13. Okteto

- **What it is:** Kubernetes-native development platform. Uses
  Syncthing for in-cluster bidirectional file sync. Supports a
  hybrid mode where services run locally while simulating cluster
  membership via SSH tunnels.
- **Stars:** ~3K
- **Overlap:** file sync + remote execution. Ships `mode: hybrid`
  (local execution with remote k8s network) and bidirectional
  Syncthing sync.
- **Why it's adjacent, not direct:** Kubernetes-centric, team/
  platform-shaped. Wrong persona. But the hybrid local/remote
  execution model is conceptually similar to what devcell's remote
  cells achieve.

### 14. Tilt

- **What it is:** Kubernetes development tool with declarative
  `live_update` that syncs code into running containers without
  rebuilding. Now part of Docker.
- **Overlap:** live code sync into containers, conceptually similar
  to devcell's file mounting. Declarative sync rules in Tiltfile.
- **Why it's adjacent, not direct:** Kubernetes-only, no SSH remote
  dev, no sandbox framing. But `live_update` validates the demand
  for "edit locally, run remotely" without full image rebuilds.

### 15. Raw `docker run`

- **What it is:** the DIY alternative. Engineer writes their own
  Dockerfile, mounts, and flags.
- **Overlap:** same isolation mechanism as DevCell's Docker engine.
- **Why it's adjacent, not direct:** DevCell's value is everything
  around the container: config preservation, MCP auto-configuration,
  auth pass-through, multi-agent persistence. "I'll write my own
  Dockerfile" is a valid anti-persona choice (see [PERSONAS.md](./PERSONAS.md)).

---

## Comparison matrix

| | DevCell | Docker Sandboxes | Microsandbox | Daytona | E2B | Raw docker |
|---|---|---|---|---|---|---|
| Host isolation | container, VM | microVM | libkrun | container | Firecracker | manual |
| Local-first | yes | yes | yes | cloud only | cloud only | yes |
| Claude Max/Pro (no API key) | yes | broken | no | no | no | manual |
| CLAUDE.md + hooks preserved | yes | no | no | no | no | manual |
| MCP servers + backing tools | auto-configured | catalog (manual) | own MCP | no | no | manual |
| macOS + Windows VMs | yes | no | no | no | no | no |
| Egress lockdown | yes | yes | yes | yes | yes | manual |
| Snapshot / restore | not yet | not yet | yes | yes | yes | no |
| Open source | Apache-2.0 | partial | Apache-2.0 | closed | Apache-2.0 | n/a |

---

## Remote dev comparison (added 2026-10-14)

How devcell's remote cell approach compares to established remote
dev file sync mechanisms. See [research/2026-10-14/remote-cell-competitors.md](research/2026-10-14/remote-cell-competitors.md).

| | DevCell (planned) | Coder | VS Code Remote | DevPod | Okteto |
|---|---|---|---|---|---|
| File sync mechanism | reverse SFTP (mount) | Mutagen (copy) | none (remote FS) | none (git clone) | Syncthing (copy) |
| Offline editing (remote files) | no (needs SSH) | yes (local copy) | no | n/a | yes (local copy) |
| Sync conflicts | impossible (single source) | three-way merge | n/a | n/a | Syncthing resolver |
| Large repo performance | SSH + SFTP latency | degrades (#100, #87) | native | n/a | Syncthing overhead |
| Editor requirement | none (CLI) | none | VS Code | multi-IDE | none |
| Setup complexity | `--host ssh://` | server deploy | extension | provider config | k8s cluster |
| Agent support | native (sandbox) | workspace agents | extension agents | none (abandoned) | none |

**Key insight from research:** bidirectional local-remote file sync
is the #1 demand signal across all 4 surfaces (strong). The reverse
SFTP approach is unclaimed whitespace: every competitor uses copy-based
sync. The tradeoff is clear: mount-based avoids sync state/conflicts
but requires a live SSH connection.

---

## Moat (in order of durability)

1. **Local-first, BYO-subscription, no cloud billing.** Strategy, not a
   feature. Daytona, E2B, and Modal cannot adopt this without breaking
   their revenue model.
2. **Config preservation as a first-class promise.** MCP servers,
   `.claude/`, hooks, skills: all carried over. Easy in principle,
   tedious to ship correctly.
3. **Search-result ownership for high-intent queries.** Non-technical
   moat that compounds: every README mention, blog post, HN comment
   makes the next one easier.

If we lose all three, we lose. Minimum defensible claim is #1.

---

## Threat triggers (checked monthly)

| Event | Probability (12mo) | Response |
|---|---|---|
| Anthropic ships `claude code --sandbox` natively | Medium | Retreat to wider-than-agent positioning |
| Docker Sandboxes fixes Max/Pro OAuth | Medium | Lose "out of the box" edge. Compete on config preservation + MCP |
| Microsandbox adds config preservation | Low | Compete on MCP + multi-OS |
| Microsandbox adds Claude Code integration | High | Lose "they don't support Claude" edge. Compete on config preservation + MCP |
| E2B launches local CLI | Low | No change: they remain SDK-shaped |
| Daytona launches local mode | Low | No change: they remain cloud/SDK-shaped |
| A new well-funded entrant | Medium | Decide within 30 days: compete on speed or narrow niche |
| Anthropic deprecates Max/Pro CLI auth | High-impact, Low | Existential. Pivot to API-key story + 1Password |
| Coder ships SSH-based agent sandboxes | Medium | Coder's $90M + Mutagen gives them resources. Differentiate on reverse SFTP (no sync state) and local-first story |
| VS Code Remote adds native agent sandbox | Medium | v1.121 added remote agent sessions experimentally. If they add isolation, compete on editor-independence |
| OpenAI/Ona ships local-remote dev for Codex | Low | Post-acquisition, Ona could pivot to local-first. Monitor for SDK announcements |

----

# Changes needed

## 2026-10-14: Remote dev competitors integrated

### What changed

- Coder (#8): updated with $90M Series C, Mutagen-based file sync,
  AI agent workflow pivot. Most relevant comp for remote cells.
- DevPod (#9): expanded with remote dev context. Abandoned user base
  is a potential adoption channel. GitHub issues document unmet needs.
- Added 3 new adjacent entries:
  - Mutagen (#12): de facto sync standard, the technical alternative
    to reverse SFTP
  - Okteto (#13): k8s-native with Syncthing sync and hybrid mode
  - Tilt (#14): declarative live_update validates "edit locally, run
    remotely" demand
- Raw docker renumbered to #15
- Added remote dev comparison table with mechanism-level detail
- Added 3 new threat triggers for Coder, VS Code Remote, OpenAI/Ona

### Data sources

Consensus research (Claude + Codex, 4 surfaces, 68 findings):
`research/2026-10-14/remote-cell-competitors.md`.

## 2026-10-12T16:30:00Z Initial assessment

### Data to refresh monthly

- Docker Sandboxes: check docker/for-mac#7842 (Max/Pro OAuth). If
  fixed, update comparison matrix and LANDING.md change #13
- Microsandbox: star count and weekly growth rate. Currently 8.7K,
  +153/wk. If they add config preservation, escalate to direct threat
- E2B: check for local CLI announcements
- Anthropic /sandbox: monitor Claude Code release notes for expanded
  sandbox scope (MCP sandboxing, persistent $HOME)

### Competitor entries to update

- Daytona (#3): now a direct competitor (cloud SDK, $31M funded).
  Monitor for local mode or config preservation features
- DevPod (#9): confirm still stalled. Last check: Nov 2025 push,
  Mar 2025 release
- Modal: not in the top 12 but mentioned in marketing/COMPETITORS.md.
  Monitor if they enter the agent sandbox space
- OpenWork (#10): monitor for sandbox/isolation features
- agent-vm (#11): monitor credential brokering approach (TLS proxy)

### Missing from this doc

- No comparison page exists on devcell.sh yet. POSITIONING.md defers
  this (CELL-269, CELL-270, Low priority) but it would strengthen
  SEO for "claude code sandbox" queries
- Star counts are point-in-time snapshots. Need automated tracking
  or quarterly manual refresh

## 2026-10-12T19:00:00Z README review research

### Stack gaps vs competitors

Research (2026-10-12) identified stack categories where demand is
high but DevCell has no dedicated stack:

| Gap | Demand evidence | Competitor coverage | Priority |
|-----|----------------|--------------------|---------| 
| Rust | Tauri 102k stars, 82% admiration rate | None ship it | Medium |
| Java / JVM enterprise | Spring Boot mainstream | None ship it | Low |
| Game dev (Godot, Bevy) | Godot 118k, Bevy 49k | None ship it | Low |
| Data Science / ML | Transformers 163k, PyTorch 102k | E2B covers via cloud | Medium |

These are community-home additions, not DevCell core changes.

### Content layer as differentiator

Community-home (github.com/devcell-sh/home) is a differentiator that
no competitor has: forkable, composable, Nix-based stack definitions.
Docker Sandboxes has a manual MCP catalog. E2B has SDK templates.
Neither offers a forkable home-manager config.

README should frame this as: "batteries included, everything
swappable. Fork community-home, edit one Nix file, run `devcell
build`."

### MCP installation pain as competitive wedge

86% of MCP servers need local installation. The #1 failure is
PATH/env mismatch (GUI apps can't find binaries). DevCell
eliminates this entire class by shipping tools pre-installed.
This is a stronger wedge than "we have more MCP servers" (Docker
has 270+ in their catalog). The wedge is: ours work out of the box.

## 2026-10-13: Competitive landscape update

### Daytona promoted to direct competitor (#3)

Daytona is not dead. It pivoted from open-source dev environment
manager to cloud AI sandbox SDK ($31M funded, $24M Series A Feb 2026).
Open-source repo archived, production codebase closed-source since
Jun 2026. Active product at daytona.io with sub-90ms sandbox creation,
official OpenAI Agents SDK and Anthropic Managed Agents partner.
Enterprise customers include LangChain, SambaNova, Stanford CS.
71K GitHub stars (frozen) give it significant brand recognition.
Replaced E2B on the landing page comparison table (both are cloud-only
SDK sandboxes, but Daytona is the bigger name Alex will encounter).
E2B retained as direct competitor #4 in this doc.

### Microsandbox ecosystem expanding

OpenCode integration is live with docs at
docs.microsandbox.dev/how-to/agents/opencode. Community forks are
extending the runtime: msb-omarchy (github.com/ya-luotao/msb-omarchy)
adds display/GPU support on Apple Silicon. Claude Code integration
is likely next. Added as High-probability threat trigger.

### New adjacent entries

- OpenWork (#10, 24.1K stars): open-source desktop agent workspace.
  Not a sandbox (no isolation model), but competes for attention in
  the "run AI agents safely" search space.
- agent-vm (#11, 6 stars): Rust/libkrun microVM tool with
  interesting credential brokering via TLS-intercepting proxy. Too
  early to compete but worth watching for the proxy approach.

### Docker Sandboxes isolation model clarified

Docker Sandboxes uses a custom-built VMM (not Firecracker, not
libkrun) providing hardware-level isolation (VT-x/AMD-V) with
dedicated kernel per sandbox. Launched Cloud Sandboxes Sep 2026
extending the same microVM model to Docker-managed cloud infra.
"microVM" in the comparison matrix is accurate.
