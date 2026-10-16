# Landing

The canonical spec for what the README and `devcell.sh` homepage say.
Both surfaces render the same message. Verified against codebase,
published catalog, and market research as of 2026-10-12.

Links: [FEATURES.md](./FEATURES.md), [PERSONAS.md](./PERSONAS.md),
[COMPETITION.md](./COMPETITION.md), [MARKET.md](./MARKET.md),
[VOCABULARY.md](./VOCABULARY.md).

---

## Voice rules

- State facts. Let the reader decide.
- No "only," "unique," "batteries-included," "battle-tested,"
  "production-ready." The bullet list is the proof.
- No emoji.
- No competitor bashing. The comparison table states facts.
- No inflated numbers. MCP count is per-stack, not cherry-picked.
- No feature claims without shipped code. Planned features go in
  "Not yet."
- Technical peer, not marketing. Reader runs `nix flake show` for fun.

---

## Target copy

### Hero

```
# DevCell

Claude Code, without the blast radius.

DevCell is a local-first sandbox for AI coding agents. Run Claude Code,
Codex, OpenCode, or Gemini inside an isolated cell with network egress
locked down by default. Agents write, delete, and install freely. Your
host is untouched.

No account. No API key with Max/Pro. brew install and go.

> "Run this in a container, not your actual machine."
> — Anthropic, Claude Code docs
```

### Quickstart

```
## Quickstart

**Prerequisites:** Docker Desktop or Docker Engine.

brew install devcell-sh/tap/devcell
cd your-project
devcell claude

On first run, devcell creates devcell.toml and .devcell/ in your
project directory, then builds the image (~5 min). Also: devcell codex,
devcell opencode, devcell gemini.
```

### What you get (8 bullets)

```
## What you get

- **Egress locked down** — Solitary Mode denies all outbound traffic
  except allow-listed domains. The agent cannot phone home, push to
  the wrong remote, or exfiltrate tokens
- **Full auto-approve in isolation** — agents run with no permission
  prompts. The sandbox makes it safe; the speed makes it worth it.
  SSH keys, other repos, and host credentials are unreachable unless
  explicitly configured
- **No account, no login** — brew install devcell-sh/tap/devcell,
  cd your-project, devcell claude. No Docker Hub login, no cloud
  dashboard
- **Claude Max/Pro works out of the box** — no API key, no proxy, no
  workarounds
- **Your Claude setup carries over** — CLAUDE.md, hooks, skills, and
  MCP config mount into the cell
- **MCP servers with backing tools** — servers auto-merge into agent
  config at startup. The backing tools ship too: the agent can run
  tofu plan or edit PCBs because OpenTofu and KiCad are installed,
  not just their MCP servers
- **Three OS engines** — Linux container (Docker), macOS VM (Tart),
  Windows VM (winkit). Same commands, same config
- **1Password secrets** — fields injected as env vars at runtime on a
  RAM-only tmpfs, gone when the cell stops
```

Priority order: leads 1-4 are the positioning (egress, isolation,
no-account, Max/Pro). Supports 5-8 are real features but not the
identity. See [FEATURES.md](./FEATURES.md) for the full ranked list.

Research note: "browser login" dropped from bullets to reduce noise.
It is a demo/content feature (0 community threads ask for it), not a
landing-page bullet. Cover it in a dedicated section below.

### Comparison

```
## Comparison

|                              | DevCell          | Docker Sandboxes     | Daytona        | Microsandbox | Raw docker run |
|------------------------------|------------------|----------------------|----------------|--------------|----------------|
| Host isolation               | container, VM    | microVM              | container      | libkrun      | manual         |
| Account / login required     | no               | yes                  | yes            | no           | no             |
| Claude Max/Pro (no API key)  | yes              | documented but broken| no             | no           | manual         |
| CLAUDE.md + hooks preserved  | yes              | no                   | no             | no           | manual         |
| MCP servers + backing tools  | yes, auto-config | catalog (manual)     | no             | own MCP      | manual         |
| macOS + Windows VMs          | yes              | no                   | no             | no           | no             |
| Egress lockdown              | yes              | yes                  | yes            | yes          | manual         |
| Local-first                  | yes              | yes                  | cloud only     | yes          | yes            |
| Credential brokering         | not yet          | yes (proxy)          | yes (API)      | yes          | manual         |
| Snapshot / restore           | not yet          | not yet              | yes            | yes          | no             |

Use Docker Sandboxes for microVM isolation or cloud-hosted sandboxes.
Use Daytona for ephemeral cloud sandboxes with an SDK. Use Microsandbox
for libkrun microVMs. Use DevCell when the agent needs full
auto-approve with your toolchain and Claude setup intact, and no
account or cloud dependency.
```

Research note: "Account / login required" added because Docker
Sandboxes' mandatory login is a 2-surface strong pain point (GitHub
sbx-releases#321: 31 negative reactions on closure; HN thread: 694
points, multiple commenters object). "Credential brokering" added
because proxy-based credential injection is a 2-surface strong demand
signal and DevCell's honest gap vs Docker Sandboxes, E2B, Microsandbox.

Lima note: Lima v2.1 added macOS guests, v2.2 added Windows guests
(CNCF blog). Lima is not in this table (different category: general-
purpose VM manager, not agent-specific) but the "macOS + Windows VMs"
row should be qualified as "no agent-focused competitor covers all
three" if Lima is discussed in prose.

### Remaining sections

Updated per README review research (2026-10-12T19:00:00Z):

- **Stacks** : reframe around community-home as content layer.
  Lead with "batteries included, everything swappable." Show 3-4
  hero stacks (security/RE, electronics, infra) inline with brief
  descriptions. Link to community-home for full catalog. Don't
  enumerate every stack; show what's impressive, link to the rest.
- **MCP servers** : lead with installation pain elimination ("you
  don't install MCP servers; they come ready"). Show 3-4 headline
  capabilities, then "11 servers included. Full list in
  community-home." Don't list the full table in README.
- **Session handoff** (formerly "Browser login") : demote from
  standalone section to one bullet. Lead with "your password never
  enters the sandbox." Drop anti-bot language entirely.
- **Security** : fix mount path to `/<cellName>` + original host
  path (not `/workspace`). Add "unless explicitly configured" to
  SSH keys line.
- **Configuration** : collapse to 2 lines. "Project config at
  `devcell.toml` with JSON Schema for editor validation. See
  `devcell --help`." Move schema URLs, catalog detail to docs.
- **WireGuard** : collapse into `<details>`
- **Customization** : reframe as "When you need more" in 5 lines.
  Show the 4-level escape hatch (mise, packages, flake, fork).
- **Development** : keep in `<details>`, fix community-home ref
- **Terminology** : matches VOCABULARY.md
- **Not yet** : remote cell execution (`--host ssh://`, CELL-654),
  credential brokering proxy, snapshot/restore, `windows-full`
  guest, pre-built registry images

### What this landing does NOT do

Conscious choices, not oversights:

- No "free and open-source" in hero: LICENSE badge says it
- No MCP count in hero: numbers in a tagline feel salesy
- No Max/Pro in hero: it's a feature, not the identity
- No "only" or "unique" claims
- No competitor bashing
- No inflated numbers

### What we are not doing (from POSITIONING.md)

| Not doing | Why |
|---|---|
| Broaden the headline to "multi-agent local environment" | Bigger TAM, smaller credit. Narrowness signals taste. |
| Comparison pages vs Docker Sandboxes, claudebox, nono | SEO-shaped, low credit yield. Deferred (CELL-269, 270). |
| Long-tail SEO content series | Volume strategy. Replaced by one postmortem. |
| Paid ads, LinkedIn, conferences, webinars, influencers | Wrong distribution for the persona. |
| Chase `claude code container` head-term SERP | Lost ground. Anthropic + Docker own it. |
| Sales-led demos, enterprise procurement motion | Wrong persona. |
| Hide Nix because Nix is "a tax" | In credit-mode, the opinionated choice IS the signal. |
| Star count above 1K in year one | Credit != stars. Target 300-800. |
| Re-launch the same HN post if it bounces | Write a different one with a sharper wedge. |

### Litmus test (from POSITIONING.md)

Before publishing anything:

> "Does this sharpen one of the three buckets: MCPs pinned, persistent
> multi-agent, environment-with-the-lights-on: in a way Anthropic's
> /sandbox cannot match?"

- If yes: ship.
- If it's true of claudebox or Docker Sandboxes too: sharpen until
  it isn't.
- If it broadens the wedge to feel bigger: narrow it back.
- If it could be in any dev-tool launch from 2024: rewrite.
- If the headline implies a GIF and no GIF exists: the GIF ships first.

### Three-bucket one-liners (from POSITIONING.md)

| Bucket | One-liner |
|--------|-----------|
| 1. MCPs pinned | "Anthropic sandboxes the bash. We ship the MCPs, pinned." |
| 2. Persistent multi-agent | "Anthropic's sandbox is per-bash. Ours is per-engineer." |
| 3. Environment with lights on | "Anthropic gives you a wall. We give you a room: with the lights on." |

### Execution sequence (from POSITIONING.md)

| Step | What | Timeline | Linear |
|------|------|----------|--------|
| 1 | Polish the artifact (MCP catalog table, flake.lock, identity migration, comparison page) | 4-6 weeks | CELL-265, 266, 267, 268, 273 |
| 2 | Produce the demo asset (15-30s headed Chromium loop) | 1 week | CELL-274 |
| 3 | Write one postmortem (recommended: "How I got Playwright headed in a container") | 2-4 weeks | CELL-271 |
| 4 | Launch on HN once, well | 1 day + 14 days follow-up | CELL-272 |

Sequence matters. Each step gates the next. No demo = postpone launch.
No postmortem = no first-comment hook.

----

# Changes needed

## 2026-10-12T16:30:00Z Initial assessment

Delta from current `README.md` to target copy above. Grouped by
section.

## Hero

| # | Current | Target | Why |
|---|---------|--------|-----|
| 1 | No pitch line | Add "Claude Code, without the blast radius" | Locked-in pitch line from POSITIONING.md |
| 2 | "containerized sandbox" | "local-first sandbox" | Market category alignment. "Containerized" implies Docker-only; DevCell also runs VMs |
| 3 | "Claude Code, Codex, or OpenCode" | "Claude Code, Codex, OpenCode, or Gemini" | Gemini ships (`cmd/gemini.go`). Missing from hero and quickstart |
| 4 | SSH keys line in hero paragraph | Move to bullet 1 ("Full auto-approve in isolation") | Hero should name the problem, not list what's safe |
| 5 | No Anthropic quote | Add: "Run this in a container, not your actual machine." — Anthropic | Real quote, real attribution. Validates the category |

## What you get

| # | Current | Target | Why |
|---|---------|--------|-----|
| 6 | No "config preservation" bullet | Add: "Your Claude setup carries over" as bullet 2 | Primary differentiator vs Docker Sandboxes. Currently invisible |
| 7 | "12+ MCP servers" then list | "MCP servers with backing tools" + mechanism description | MCP value is the mechanism, not the count. 12 is per-stack, not total. See FEATURES.md #4 |
| 8 | Solitary Mode: 2 long lines | One line: "--solitary denies all outbound except allow-listed" | Implementation details (squid, caps, docker socket) go in docs, not README |
| 9 | Browser login: "devcell login" | "devcell auth chrome" | CLI was renamed. Current README has wrong command |
| 10 | Browser login: 14 lines with CDP/Patchright detail | 8 lines, no vendor names | Drop CDP port, Patchright identity, Cloudflare/Kasada names from hero bullets |
| 11 | Bullet order: isolated, MCP, Max, Solitary, browser, RDP, 1Password, engines | Reorder: egress, isolation, no-account, Max, config, MCP, engines, 1Password | Leads with market-validated signals (egress: 4-surface strong, no-account: 2-surface strong), supports follow. Browser login dropped from bullets (0 demand signal). |

## Comparison table

| # | Current | Target | Why |
|---|---------|--------|-----|
| 12 | Competitors: VS Code Dev Containers, E2B, Daytona | Replace with: Docker Sandboxes, Daytona, Microsandbox | Docker Sandboxes launched Sep 2026. Microsandbox fastest-growing. Dev Containers are a different category. Daytona kept per deliberate commit 094f18f (2026-10-14). |
| 13 | "Claude Max/Pro: yes / no / no / no" | Docker Sandboxes: "documented but broken" | Honest. docker/for-mac#7842 |
| 14 | No "CLAUDE.md + hooks preserved" row | Add row | Primary differentiator, currently invisible in comparison |
| 15 | No "Egress lockdown" row | Add row | #2 market demand feature |
| 16 | No "Snapshot / restore" row | Add with "not yet" for DevCell | Honest gap |
| 17 | Guidance paragraph references Daytona, Dev Containers | Update to reference Docker Sandboxes, Microsandbox | Match new competitor columns |

## Stacks table

| # | Current | Target | Why |
|---|---------|--------|-----|
| 18 | `ultimate` description: "General development, cloud tools, browser automation, desktop and Draw.io" | Name actual tools (Go, Node, Python, IaC, Inkscape, Playwright, etc.) | Vague description. Other rows name tools. |
| 19 | `desktop` module: "VNC, RDP, Fluxbox, PulseAudio" | "VNC, RDP, IceWM/Fluxbox, PulseAudio" | Published catalog says IceWM is primary |
| 20 | `infra` module: "AWS, GCP, Azure" | Verify against catalog. Drop if not shipped. | May not be in current catalog |

## MCP servers table

| # | Current | Target | Why |
|---|---------|--------|-----|
| 21 | MCP-NixOS listed | Remove if not in published catalog | README-LANDING audit flagged this |
| 22 | No pinning note | Add: "Pinned to commit hashes in the nix flake, so rebuilds get the same versions" | Differentiator from manual MCP installs |

## Browser login section

| # | Current | Target | Why |
|---|---------|--------|-----|
| 23 | 14 lines with CDP port, Patchright identity, fingerprint detail | 8 lines: what it does, not how | Persona doesn't need implementation detail in README |

## Links and references

| # | Current | Target | Why |
|---|---------|--------|-----|
| 24 | 3 references to `./MIGRATION.md` (lines 29, 46, 219) | Remove references or create the file | File does not exist in repo |
| 25 | `devcell.sh/docs/cell` link (line 135) | Replace with `devcell --help` | Returns 404 |
| 26 | nixhome link: `devcell-sh/home` | Verify: is canonical repo `home` or `community-home`? | AGENTS.md says "community-home" but `devcell-sh/home` returns 200 |
| 27 | `nixhome/modules/` path in Development section | Update to reference external repo | Nix modules moved to standalone repo |

## Terminology section

| # | Current | Target | Why |
|---|---------|--------|-----|
| 28 | "each project spawns one cell" | "each project spawns one or more cells" | A project can have multiple cell IDs |

## Not yet section

| # | Current | Target | Why |
|---|---------|--------|-----|
| 29 | Missing: credential brokering proxy | Add: "Credential brokering proxy (secrets never enter the sandbox)" | Honest gap vs Docker Sandboxes, E2B, Microsandbox. 2-surface strong demand (consensus research 2026-10-12). |
| 30a | Missing: snapshot/restore | Add: "Snapshot and restore: fast checkpoint/resume of cell state" | 2-surface strong demand (consensus research 2026-10-12). E2B and Microsandbox ship it. |

## Parallel actions (not README changes)

| # | Action | Effort |
|---|--------|--------|
| 31 | Set GitHub repo description + topics (`claude-code`, `agent`, `sandbox`, `container`, `docker`, `cli`, `nix`, `mcp`) | 5 min |
| 32 | Create CHANGELOG.md | 30 min |
| 33 | Create CONTRIBUTING.md | 30 min |
| 34 | Add package doc comment to `main.go` | 5 min |

## Obstacles to launch-ready landing (from marketing/OBSTACLES.md, Jun 2026)

Status-checked against codebase as of 2026-10-12:

### S-tier (fix before launch)

| # | Obstacle | Status | Notes |
|---|----------|--------|-------|
| S1 | Naming identity (4 names competing) | DONE | Commit c0a52b5: CLI renamed to `devcell`. Org is `devcell-sh`. Tap is `devcell-sh/tap/devcell`. |
| S2 | First-run is unconsented 20GB download | RESOLVED | Default stack is `dev` (~3GB) not `ultimate` (~20GB). Acceptable: `dev` is the right default per research. Not a launch blocker. |
| S3 | Trust signals absent from README | DONE | Version badge + last-commit badge added to website hero (2026-10-13). CHANGELOG.md and CONTRIBUTING.md exist. |

### A-tier (fix this quarter)

| # | Obstacle | Status | Notes |
|---|----------|--------|-------|
| A4 | Solo maintainer / bus factor | PARTIAL | CHANGELOG.md and CONTRIBUTING.md exist. CONTRIBUTING linked in footer. Still need: security policy, visible co-maintainer. |
| A5 | Docker Desktop license trap | DONE | README says "Docker Desktop, OrbStack, Colima, Rancher Desktop, or Docker Engine." Website quickstart says "Requires `docker`" (generic). |
| A6 | Config-survived confirmation banner | OPEN | On first `devcell claude`, print: "MCP servers loaded: N, Skills: N, Hooks: N, CLAUDE.md: found." Half day of work. |

### B-tier (fix when A-tier done)

| # | Obstacle | Status |
|---|----------|--------|
| B7 | Error messages mention Nix/Docker internals | OPEN |
| B8 | Privacy / no-telemetry statement | OPEN (one line in README) |
| B9 | Clean uninstall (`devcell uninstall`) | OPEN |
| B10 | Agent compatibility table | OPEN |

### Conflicts with current .product/ decisions

- OBSTACLES S2 says "default to `base`" but current default is `dev`.
  Research supports `dev` (ships MCP servers + Playwright, which are
  differentiators). Resolve: keep `dev` default but add size warning
  on first build.
- OBSTACLES S3 says add badges. LANDING voice rules say "no emoji."
  Resolve: badges are metadata, not emoji. Add version + last-commit
  badges.

## 2026-10-12T17:30:00Z Market research alignment

Consensus feature-demand research (Claude + Codex, 4 surfaces, 65
findings after skeptic verification). Full report:
`.scratch/FEATURE-DEMAND.md`.

### Alignment with FEATURES.md top 5

| Rank | Feature | FEATURES.md Score | Research Result | Verdict |
|------|---------|-------------------|-----------------|---------|
| 1 | Container sandbox | 92 | 1 surface, strong | Validated but table-stakes: all competitors ship it. Not a differentiator. |
| 2 | Network egress | 88 | **4 surfaces, strong** | **Under-ranked.** Strongest signal in entire report. Should lead positioning. |
| 3 | Auto-approve framing | 79 | Not a standalone signal | **Misaligned.** Market says "sandbox" and "egress," not "auto-approve." Keep the framing but don't lead with it. |
| 4 | MCP with backing tools | 76 | Zero findings | **Show-don't-tell.** Market doesn't know to ask for it. Demo feature, not headline. |
| 5 | Multi-OS | 74 | Absent (no survey data) | **Weakened.** Lima v2.1/2.2 closes the gap. No measured demand. |

### New signals the product docs underweighted

| Signal | Surfaces | Status | Action |
|--------|----------|--------|--------|
| Credential brokering (proxy) | 2 | strong | Ship-next. Elevated to Not Yet section. |
| Snapshot and restore | 2 | strong | Strategic. Elevated to Not Yet section. |
| Account-free local use | 2 | strong | Added to hero and bullets. Differentiator vs Docker Sandboxes. |
| MicroVM isolation | 1 | strong | Honest gap. Acknowledged in comparison. Container is sufficient for persona but weaker in head-to-head. |
| Config preservation | 1 | weak | Keep as bullet but don't oversell. Market implements it (bind-mounts) but doesn't demand it loudly. |

### Changes applied to this doc

1. Hero: added egress and no-account language
2. Bullets: reordered to lead with egress, isolation, no-account, Max/Pro
3. Bullets: added "No account, no login" (new bullet)
4. Bullets: reframed auto-approve as consequence, not identity
5. Bullets: dropped browser login from bullets (0 demand signal)
6. Comparison: added "Account / login required" row
7. Comparison: added "Credential brokering" row with honest "not yet"
8. Comparison: updated guidance paragraph to mention no-account
9. Not Yet: added snapshot/restore with demand evidence
10. Not Yet: added demand evidence citation to credential brokering
11. Lima note added to comparison section (multi-OS softening)

## 2026-10-12T19:00:00Z README review research

Cross-agent research on 7 README sections. Full report:
`research/README-REVIEW-2026-10-12.md`.

### New findings

| Finding | Source | Action |
|---------|--------|--------|
| MCP install pain is a stronger wedge than MCP count | 86% need local install, #1 fail is PATH mismatch | Lead MCP bullet with pain elimination |
| Security/RE stacks are the "wow" differentiator | Ghidra 85k, r/RE 176k, highest setup pain ratio | Hero these in stacks section |
| "Browser login" naming is wrong | IETF draft uses "session handoff", `auth` means tool auth | Rename section, demote to bullet |
| mackit: zero codebase references | grep confirmed | Don't mention until shipped |
| Configuration is not a selling point | Target persona assumes CLI tools have config | Collapse to 2 lines |
| Community-home is an under-marketed differentiator | No competitor offers forkable stack definitions | "Batteries included, everything swappable" |

### Changes to target copy

| Section | Change |
|---------|--------|
| What you get, bullet 2 | SSH keys: add "unless explicitly configured" |
| What you get, bullet 6 | MCP: lead with install-pain elimination |
| Comparison | DevCell isolation: "container, VM" |
| Stacks | Reframe: community-home content layer, hero 3-4 stacks |
| MCP servers | Headline capabilities + link to catalog |
| Browser login | Rename "Session Handoff", demote to bullet |
| Security | Fix mount path to `/<cellName>` + host path |
| Configuration | Collapse to 2 lines |
| Customization | Compress to "When you need more" (5 lines) |
| Engines | Don't mention mackit. Keep Tart/winkit. |

### Future items to track

| Item | Type | Priority |
|------|------|----------|
| CLI rename: `devcell auth chrome` to `devcell browser sync` | Breaking change | Medium |
| Stack: Rust | community-home addition | Medium |
| Stack: Java/JVM | community-home addition | Low |
| Stack: Game dev (Godot) | community-home addition | Low |
| PostgreSQL MCP server | community-home addition | Medium |
| mackit engine | devcell core | In progress (not announced) |

## 2026-10-13T15:00:00Z Website landing fixes

Cross-agent review of `web/` against current README positioning and
`.product/FEATURES.md`. The website is a full version behind the README:
built before the org rename (DimmKirr → devcell-sh), CLI rename
(cell → devcell), nixhome repo split (devcell-sh/home), and the
.product positioning work.

### Visual/UX review (Playwright screenshot, 2026-10-13)

Screenshots: `.scratch/screenshots/2026-10-13/`.
Perspective rotation: Sijbrandij (radical transparency), DHH (OSS as
competitive moat), Ronacher (pragmatic solo maintainer), Torvalds
(show me the code).

#### V1. Section order: NEEDS WORK

Current: Hero → What agent gains (3 cards) → What it can't touch
(3 cards + boundary table) → Quickstart → Stacks → FAQ → CTA → Footer.

Quickstart is **4th**. Alex's 90-second evaluation: what is it → how
do I install it → what happens. `brew install` is not visible above
the fold. Torvalds: "If I have to scroll past philosophy to find the
install command, I've already closed the tab."

"What the agent gains" and "What it can't touch" are the same concept
(sandbox with tools) split into two sections for rhetorical effect.
Merge them.

"READY TO TRY IT?" CTA at the bottom just links back to Quickstart.
Redundant: remove or replace with GitHub link.

**Target order:** Hero → Quickstart → Feature cards (merged: gains +
boundary) → Comparison → Stacks (collapsed) → FAQ → Footer.

#### V2. Hero accent: NEEDS WORK

Neon-green (#C8FF00) on "ITS OWN" emphasizes possession. The decided
positioning emphasizes consequence ("without the blast radius"). When
the heading changes to the target copy, the accent should land on
"BLAST RADIUS" (the pain) or "WITHOUT" (the relief). The accent color
itself is fine: high-contrast, brutalist, memorable. Just on the
wrong word right now.

#### V3. Tone/vibe: NEEDS WORK

Reads more like a product company page than an OSS project homepage.

**Too sales-y (fix):**
- Section headers use marketing phrasing: "The payoff," "The boundary,"
  "What the agent gains inside." Compare Bun, Hono, Drizzle: functional
  section names ("Features," "Getting Started," "Performance").
- "Built-in sandboxes subtract tools to make agents safe. DevCell adds
  tools and stays safe." — reads as ad copy. Alex would prefer: "A
  container with your tools already installed."
- "One cell per client" card mentions Acme and BigCorp. This is
  consulting-company messaging, not open-source community messaging.
  Signals "this is a product, not a project."

**Good technical tone (keep):**
- Terminal mock is authentic (real CLI output, real paths).
- Project/Cell/Host boundary explanation is clear and factual.
- "Docker by default. When your threat model demands a real kernel
  boundary, `--engine tart` runs the same cell in a VM." — this is
  the right voice. Write more copy like this.

**Action:** rewrite section labels to factual names. Replace
gain/loss sales framing with feature descriptions. Remove
Acme/BigCorp consulting language.

#### V4. Missing trust signals: PROBLEM (biggest gap)

What's visible: star count (4), "RELEASES" link.

What's missing:
- No version number visible
- No CI status badge
- No "last commit" or activity indicator
- No contributor count or contributor avatars
- No CHANGELOG link on the page
- No CONTRIBUTING.md link anywhere (nav, footer, or body)
- No community channel (Discussions, Discord, Matrix)
- Star count of 4 prominently displayed with a Shields.io badge —
  at this stage, showing a low count works against you. Consider
  hiding the badge until >50 stars, or show "last commit" instead.

Sijbrandij: "Radical transparency includes showing your build passes,
your release cadence, and who else is working on this."

Alex's #1 evaluation question per PERSONAS.md is "Is it real?" (recent
commits, version number, stars trending). The current page fails this.

**Action:** Replace stars badge with version + last-commit badges.
Add CONTRIBUTING link to nav or footer. Add Issues/Discussions link
to nav. Show contributor avatars if >1 contributor.

#### V5. Honesty: GOOD

No inflated claims. "12 servers today" is factual. The `cell login`
command is wrong but that's a bug (#7), not dishonesty. One borderline:
"12+" when README says "11." Voice rules say "no inflated numbers."

#### V6. Visual design: GOOD

Brutalist aesthetic works for the persona. Terminal mock helps (shows
real CLI, not a marketing screenshot). Typography hierarchy is clear.
Card grid layout (3-column) is well-executed. Green accent is
distinctive and memorable. Dark terminal against white page creates
good visual rhythm.

#### V7. Open source red flags: PROBLEM

- **Bus factor:** no visible contributors beyond the implicit single
  maintainer. No "Created by" or team section.
- **No CONTRIBUTING.md** linked anywhere on the page.
- **No comparison table on the live page:** CompareSection.astro exists
  but is not imported in index.astro. Visitors can't see how this
  compares to alternatives.
- **"Docs" nav link** points to `/docs/quickstart` which may be broken
  post-rename.
- **Acme/BigCorp card** signals "this is a product, not a project."
  DHH: "An open source project that looks like it's built for one
  person's consulting clients isn't inviting contributions."

**Action:** Wire CompareSection back into index.astro with the updated
README comparison data. Verify /docs/ links work. Replace
consulting-persona card with contributor-welcoming language.

### P0: Broken commands and links (will fail for users)

| # | File | Line | Issue | Fix |
|---|------|------|-------|-----|
| 1 | Hero.astro | 12, 22 | `github.com/DimmKirr/devcell` | → `github.com/devcell-sh/devcell` |
| 2 | Hero.astro | 12 | Shields.io badge uses `DimmKirr/devcell` | → `devcell-sh/devcell` |
| 3 | Hero.astro | 33 | Terminal shows `cell claude` | → `devcell claude` |
| 4 | Hero.astro | 34 | "Opening Cell myproject" | → match actual CLI output |
| 5 | Quickstart.astro | 14, 34 | `brew install DimmKirr/tap/devcell` | → `devcell-sh/tap/devcell` |
| 6 | Quickstart.astro | 21, 38 | `cell claude`, `cell codex`, `cell opencode` | → `devcell *` |
| 7 | FeatureCards.astro | 24 | `cell login <site>` | → `devcell auth chrome <site>` |
| 8 | WhatsInside.astro | 23 | `DimmKirr/devcell/tree/main/nixhome` | → `github.com/devcell-sh/home` |
| 9 | WhatsInside.astro | 58 | `public.ecr.aws/w1l3v2k8/devcell` | → `ghcr.io/devcell-sh/devcell` (verify ECR is fully retired first) |
| 10 | Base.astro | 60, 70, 72 | Nav/footer GitHub links use `DimmKirr` | → `devcell-sh` |
| 11 | quickstart.md | 11 | `brew install DimmKirr/tap/devcell` | → `devcell-sh/tap/devcell` |
| 12 | howto-ci.md | 46 | Install URL with `DimmKirr` | → `devcell-sh` |
| 13 | howto-team-setup.md | 45 | Same broken install URL | → `devcell-sh` |
| 14 | stacks-and-modules.md | 168 | Fork URL `DimmKirr/nixhome` | → `devcell-sh/home` |
| 15 | mcp-tripit.md | 17 | GitHub link `DimmKirr/tripit-mcp` | → `devcell-sh/tripit-mcp` |
| 15a | Quickstart.astro:21 | Missing Gemini from agent list | → add `devcell gemini`. Gemini ships (`cmd/gemini.go`), docs exist (`cell_gemini.md`). Promoted from P1 #24: a user who tries `devcell gemini` based on the README will not find it on the website. |

### P1: Positioning misalignment with README / .product

Priority tiers within P1. **P1-high** items affect the hero, the
#2 market-demand feature (egress), or the primary differentiator
(no-account). **P1-mid** are visible mismatches. **P1-low** are
cosmetic or minor factual.

#### P1-high

| # | File | Issue | What README / .product says |
|---|------|-------|----|
| 16 | Hero.astro:13 | Heading: "Your Agent Gets Its Own Computer" | "Claude Code, without the blast radius" |
| 16a | Hero.astro:14-19 | Hero body copy talks about "auto-approve everything" and "a screen it can see" | README: "local-first sandbox for AI coding agents. Run Claude Code, Codex, OpenCode, or Gemini inside an isolated cell." Body copy is completely different from target, not just the heading. |
| 17 | index.astro:673 | Page title says "your agent's own computer" | → "devcell -- Claude Code, without the blast radius" to match positioning. Original audit said "agent workspace" but actual text is "your agent's own computer". |
| 18 | Base.astro:11 | Default meta desc: "Run AI coding agents in an isolated container" | → "Local-first sandbox for AI coding agents. Run Claude Code, Codex, OpenCode, or Gemini inside an isolated cell." |
| 26 | FeatureCards | No egress lockdown card | #2 market-demand feature (4-surface strong). Completely absent from website: zero mentions of `--solitary`, `solitary`, or `egress` anywhere in web/src/. |
| 27 | FeatureCards | No "no account" card | Key differentiator vs Docker Sandboxes (2-surface strong). |

#### P1-mid

| # | File | Issue | What README / .product says |
|---|------|-------|----|
| 19 | CompareSection.astro | Compares vs Dev Containers + OpenClaw | README: Docker Sandboxes, E2B, Microsandbox. **Note:** CompareSection.astro is not imported in index.astro: it may be dead code. Verify before spending time. If dead, either remove or wire it back in with the new comparison data. |
| 20 | index.astro:10-21 | 6 stacks in `stackColumns` array (base, go, node, python, fullstack, ultimate) | README: 4 (base, ultimate, bbb, electronics). Fix the `stackColumns` and `stackFeatures` arrays in index.astro, not just prose. |
| 23 | index.astro:18 | "cookie sync via cell login" in `stackFeatures` | "session handoff via devcell auth chrome" |
| 25 | Hero.astro:46 | "Sonnet 4.6" model name in terminal mock | Update to current model |
| 28 | FeatureCards:24 | "browser login" terminology in card text | Renamed to "Session handoff" |
| 29a | index.astro:43 | FAQ #5: "Network access is unrestricted" | Contradicts README's `--solitary` egress lockdown, the #2 positioning feature. Fix: "Network access is unrestricted by default. `devcell claude --solitary` locks down egress to allow-listed domains only." |
| 29b | index.astro:39 | FAQ #3: references Dev Containers | README comparison dropped Dev Containers. Update to reference Docker Sandboxes. |
| 29c | index.astro:41 | FAQ #4: references OpenClaw | README comparison dropped OpenClaw. Either update or remove. |
| 29d | index.astro:37 | FAQ #2: says "nixhome modules" | nixhome moved to `devcell-sh/home`. Update link/name. |

#### P1-low

| # | File | Issue | What README / .product says |
|---|------|-------|----|
| 21 | index.astro:20 | "12+ MCP servers" in `stackFeatures` | README: "11 servers total". Update to match published catalog count. |
| 22 | index.astro:19 | "VNC + RDP + Fluxbox" in `stackFeatures` | README: IceWM primary. → "VNC + RDP + IceWM" |
| 24 | (moved to P0 #15a) | | |

### P2: Deprecated config and terminology in docs (50+ hits, 19+ files)

| Category | Count | Worst offenders | Notes |
|----------|-------|-----------------|-------|
| Old CLI name `cell` | 19+ files | 12 `cell_*.md` frontmatter titles, plus `howto-multi-agent.md`, `howto-git-identity.md`, `howto-macos-vm.md`, `howto-financial-research.md`, `howto-ports.md`, `howto-mcp-development.md`, `mcp-playwright.md` | **Caution:** `cell_*.md` files may be auto-generated by `cmd/gendoc.go`. Check `task docs:generate` or equivalent before manual editing: edits will be overwritten on next generation. If generated, fix the Go source or add a title-rewrite post-processor. |
| `[[volumes]]` (deprecated) | 18 hits | `configuration.md:166-174`, `howto-mounts.md` (entire guide), `howto-monorepo.md` | howto-monorepo.md is high-traffic for the target persona (Alex). |
| `use_ollama` (deprecated) | 8 hits | `howto-ollama.md:45-71` (entire guide), `configuration.md:44,54,218`, `cell_claude.md:20`, `cell_codex.md:17` | `cell_claude.md` and `cell_codex.md` reference `use_ollama` because the Go CLI flag still emits it. If the flag is deprecated in Go, the generated docs will fix themselves. If not, the docs can't be fixed independently. |
| Old org `DimmKirr` | 4 files | `quickstart.md`, `stacks-and-modules.md` | |
| Retired term `bunk` | 1 hit | `howto-tmux.md:15` | |

### P3: SEO gaps

| # | Issue | Fix |
|---|-------|-----|
| 31 | No `<link rel="canonical">` tag | Add to Base.astro. Decide canonical URL structure first: bare domain (`devcell.sh`) vs `www`, no trailing slash. Apply consistently. |
| 32 | No `robots.txt` | Add to `public/` |
| 33 | No `@astrojs/sitemap` integration | Add to `astro.config.mjs` |
| 34 | OG image + OG tags | Present, working (no action) |
| 35 | GitHub repo description not optimized | Set description to "Local-first sandbox for AI coding agents. Claude Code, Codex, OpenCode, Gemini inside isolated cells with egress lockdown." (under 350 chars, front-loads keywords per GitHub search ranking). |
| 36 | GitHub topics not set | Add: `claude-code`, `ai-agent`, `sandbox`, `container`, `docker`, `cli`, `nix`, `mcp`, `developer-tools`, `devtools`, `llm`, `coding-agent` (12 topics, 4-bucket strategy). |

### Execution plan

| Pass | What | Effort | Verify |
|------|------|--------|--------|
| 0 | Pre-flight: determine if `cell_*.md` docs are auto-generated (`cmd/gendoc.go`). If yes, fix Go source, not markdown. Check if `CompareSection.astro` is dead code. Verify ECR → GHCR migration is complete. Verify `/docs/quickstart` route works. | 15min | |
| 1 | P0: mechanical find-replace (DimmKirr → devcell-sh, cell → devcell, old URLs, add Gemini) | 1h | `astro build` after: template syntax can break on find-replace in `.astro` files |
| 2 | V1+P1-high: restructure section order (Hero → Quickstart → merged features → Comparison → Stacks → FAQ). Rewrite hero heading + body copy to match target. Update meta descriptions. Add egress lockdown and no-account cards. Remove "READY TO TRY IT?" redundant CTA. | 2-3h | Playwright screenshot, visual diff against `.scratch/screenshots/2026-10-13/` baseline |
| 3 | V3: rewrite section labels to factual names ("Features" not "The payoff"). Remove Acme/BigCorp consulting language from "One cell per client" card. Replace sales framing with feature descriptions. Keep good-tone copy (terminal mock, boundary table, `--engine tart` line). | 1h | Read-through for voice rule compliance |
| 4 | V4: trust signals. Replace stars badge with version + last-commit badges. Add CONTRIBUTING link to nav or footer. Add Issues/Discussions to footer. Hide star count badge until >50 stars. | 30min | Visual check |
| 5 | V7: wire CompareSection.astro back into index.astro with updated README comparison data (Docker Sandboxes, E2B, Microsandbox). Remove stale Dev Containers + OpenClaw comparison. | 1h | Visual check |
| 6 | V2+P1-mid: update accent word when hero heading changes. Fix FAQ content (4 items: egress, Dev Containers, OpenClaw, nixhome). Fix stack table data arrays. Session handoff rename. Update terminal mock model name. | 1h | |
| 7 | P1-low: MCP count, IceWM | 15min | |
| 8 | P2: deprecated config in docs (volumes, use_ollama, cell titles, bunk). Skip auto-generated files if gendoc owns them. | 3-4h | Grep for old terms after |
| 9 | P3: canonical tag (decide URL structure first), robots.txt, sitemap, GitHub repo metadata (description + 12 topics). | 30min | Lighthouse SEO audit |
| 10 | Post-flight: full `astro build`, Playwright full-page screenshot, grep for `DimmKirr`, `cell ` (with trailing space), `[[volumes]]`, `use_ollama`, `bunk` across `web/src/`. Fix any survivors. | 15min | Compare before/after screenshots |

### Priority summary

| Tier | Issues | Theme |
|------|--------|-------|
| PROBLEM | V4 (trust signals), V7 (OSS red flags) | The page doesn't answer "is it real?" and doesn't invite contributions. Biggest gap. |
| NEEDS WORK | V1 (section order), V2 (accent word), V3 (tone), P1-high (hero, egress, no-account) | The page is misaligned with the decided positioning and reads too sales-y for the persona. |
| GOOD | V5 (honesty), V6 (visual design) | Brutalist aesthetic works. No inflated claims. Keep the terminal mock, the card grid, the green accent. |

## 2026-10-13T20:00:00Z Fixes applied

### Go source: cell -> devcell in examples (13 files)

Fixed all bare `cell <subcommand>` in cobra Example/Long fields across
`cmd/claude.go`, `cmd/codex.go`, `cmd/gemini.go`, `cmd/opencode.go`,
`cmd/models.go`, `cmd/chrome.go`, `cmd/shell.go`, `cmd/serve.go`,
`cmd/nix_store.go`, `cmd/disk_store.go`, `cmd/vnc.go`, `cmd/rdp.go`,
`cmd/auth.go`. Docs regenerated via `task docs:generate`. All generated
`devcell_*.md` files now show `devcell <subcommand>` in examples.

### Docs sidebar: cell -> devcell

Fixed `web/src/pages/docs/[...slug].astro`: sidebar filter updated
from `cell_*` to `devcell_*`, root doc from `cell` to `devcell`,
display prefix strip from `cell_` to `devcell_`. CLI reference
section was broken (zero commands visible); now works.

### Trust signals: version badge

Added shields.io version badge to Hero.astro eyebrow bar (alongside
existing last-commit badge). Both use `C8FF00` accent color.

### SEO: sitemap integration

Added `@astrojs/sitemap` to `astro.config.mjs`. `robots.txt` already
references `sitemap-index.xml`; now generates it.

### Obstacle status updates

| Obstacle | Old | New |
|----------|-----|-----|
| S2 (unconsented build) | OPEN | RESOLVED (dev default is ~3GB, acceptable) |
| S3 (trust signals) | OPEN | DONE (version + last-commit badges, CHANGELOG, CONTRIBUTING) |
| A4 (bus factor) | OPEN | PARTIAL (CHANGELOG + CONTRIBUTING exist) |
| A5 (Docker Desktop license) | OPEN | DONE (README lists alternatives) |

### Remaining open items

| Item | Priority | Notes |
|------|----------|-------|
| P2: deprecated terms in howto docs | Low | `[[volumes]]`, `use_ollama` in non-generated content docs. Hand-curated files, not gendoc-owned. |
| A6: config-survived banner | Medium | CLI feature request, not a doc issue |
| Demo asset | Medium | `web/public/demo.svg` exists as animated terminal SVG. Whether it satisfies the HN launch gate depends on scope. |
| Social preview upload | Low | Image created (`web/public/social-preview.png`), needs manual upload in GitHub Settings > Social preview |

## 2026-10-14T18:00:00Z AIO and cleanup session

### AIO improvements applied

| Change | File(s) |
|--------|---------|
| llms.txt created | `web/public/llms.txt` |
| robots.txt: full AI crawler directives | `web/public/robots.txt` |
| OG image: 1200x630 dark social card | `web/public/og-image.png` |
| Social preview: 1280x640 | `web/public/social-preview.png` (needs GitHub upload) |
| GitHub description set | via `gh repo edit` |
| GitHub topics: 16 set | via `gh repo edit --add-topic` |
| FAQ: `<code>` tags render correctly | `web/src/components/FAQ.astro` (`set:html`) |

### Code quality improvements

| Change | File(s) |
|--------|---------|
| Timeout errors: actionable diagnostics | `internal/engine/docker/run.go`, `internal/serve/pty_exec.go` |
| Docker socket config toggle | `internal/cfg/cfg.go` (`docker_socket`), `internal/engine/docker/runner.go` |
| Package doc comments: 10 packages | `internal/{cfg,serve,scaffold,ollama,op,config,cloudmodels,backup,engine/tart,ptyctl}` |
| BootDirWatcher dead code deleted | `boot_watcher.go`, `boot_consumer.go`, sentinel compat in `s6.go` |
| Dead web components deleted | `CompareSection.astro`, `StackTable.astro` |
| LANDING.md: comparison updated to Daytona | Matches deliberate commit 094f18f |

### P3 #35-36 resolved

GitHub repo description and topics set via Taskfile task `aio:update-repo`.
Description: "Local-first sandbox for AI coding agents. Run Claude Code,
Codex, or Gemini in an isolated container or VM with zero permission
prompts. No account required."
Topics: ai-agents, agentic-ai, agents, ai, claude-code, cli, containers,
devcontainer, developer-tools, docker, go, macos, mcp, nix, sandbox,
security (16 total).

## 2026-10-14: Remote cell research integration

### What changed

- Added "remote cell execution (`--host ssh://`, CELL-654)" to the
  "Not yet" list in remaining sections
- Remote cells are not yet shipped, so they do not appear in the
  hero, quickstart, bullets, or comparison table
- When CELL-654 ships, add a bullet: "Remote cells: `--host
  ssh://user@host` runs the cell on a remote machine with your
  project mounted via reverse SFTP. No sync, no conflicts."
- When shipped, add a comparison row: "Remote cell execution" with
  devcell "yes (reverse SFTP)" and competitors "no" (sandbox
  competitors don't do remote dev)
- Consider adding Coder to comparison table at that point (it's the
  closest remote-dev comp)

### Positioning note

Remote cells extend the sandbox story ("your sandbox, on a remote
machine") rather than replacing it. The hero ("Claude Code, without
the blast radius") remains correct. When remote ships, the hero
could expand to "without the blast radius, anywhere" but only if
the feature works well enough to lead with.

### GitHub topics update

When remote cells ship, add topics: `remote-development`, `ssh` to
the GitHub repo.

## 2026-10-15: Experimental markers fixed; headline research (Claude Code vs Agentic Coding)

### Site fixes applied (web/)

- FAQ "How is DevCell different from Docker Sandboxes?" claimed "macOS and Windows VMs" with no experimental marker. Fixed: the answer now carries the `engine-windows` marker. Marker markup moved to `web/src/lib/exp.ts` so `set:html` call sites and the `Exp` component share one implementation; unregistered names still fail the build.
- Quickstart step 1 listed "Windows (not verified yet)" as a host platform with a hand-rolled superscript. Removed: no shipped code for Windows hosts (voice rule "no feature claims without shipped code"). Now reads "Platforms: macOS, Linux."
- `ui-cage` and `ui-boot-popup` stay off the landing (policy: experimental features are not sold). Verified `go test ./internal/experiments/` passes; 4 markers render on index.

### Headline research

Research in [research/2026-10-15/headline-verdict.md](./research/2026-10-15/headline-verdict.md) with evidence files alongside. Verdict: keep "Claude Code" in the hero; "Agentic Coding" loses on persona vocabulary, search intent, and competitor whitespace, and only wins on vendor-neutrality and trademark risk.

Gap found: Anthropic's Claude Code legal page permits plain-text "runs Claude Code" statements but forbids names or logos used "in a way that suggests Anthropic built, endorses, or is partnered with your product." The current h1 is at the edge; the "Works with" strip uses Anthropic's logo.

Changes to make (not applied; needs the user's decision):
- Hero h1 -> "Run Claude Code without the blast radius." (Hero.astro, README.md line 3, index.astro title). Keeps the pitch, lands inside the permitted form.
- Footer: "Not affiliated with Anthropic, OpenAI, or Google. Claude Code, Codex, and Gemini are their owners' trademarks."
- "Works with" strip: swap Anthropic logo for a text wordmark, or keep logos with the footer line.
- Casing: site h1 uses title case ("Without The Blast Radius"); spec, README, and page title use sentence case. Pick sentence case.
- Line 242 of this file references POSITIONING.md, which does not exist. Point it at MARKET.md "Pitch lines".

