# README Revision Proposal (2026-10-12)

Merged from: README-LANDING.md (26-finding factual audit), FEATURE-RANKING.md
(6-signal demand scoring), FEATURE-DEMAND.md (59-finding consensus research),
competitor feature matrix, search trends.

---

## 1. Content changes

### Factual fixes (must ship)

| # | Line(s) | Current | Fix | Source |
|---|---------|---------|-----|--------|
| F1 | 5, 17 | "Claude Code, Codex, or OpenCode" | Add Gemini (cmd/gemini.go exists, shipped) | Landing #1 |
| F2 | 22 | "12+ MCP servers" | "25+ MCP servers" (published catalog count) | Landing #2 |
| F3 | 29, 46, 219 | `./MIGRATION.md` (3 refs) | File does not exist. Remove refs or create file | Landing #3 |
| F4 | 56 | `ultimate`: "General development, cloud tools, browser automation, desktop and Draw.io" | Name key tools: Go, Node, Python, Terraform/OpenTofu, AWS, Inkscape, Linear, Playwright, desktop GUI | Landing #5 |
| F5 | 64 | `desktop` module: "VNC, RDP, Fluxbox, PulseAudio" | "VNC, RDP, IceWM/Fluxbox, PulseAudio" (catalog says IceWM primary) | Landing #4, #7 |
| F6 | 66 | `infra` module: "Cloud CLI tools: AWS, GCP, Azure" | "IaC + Cloud: Terraform/OpenTofu, AWS CLI v2, Helm, Packer" (no GCP, no Azure in catalog) | Landing #9 |
| F7 | 107 | MCP-NixOS in server table | Not in published catalog. Remove or verify | Landing #10 |
| F8 | 135 | `https://devcell.sh/docs/cell` | 404. Replace with `devcell --help` | Landing link |
| F9 | 170 | nixhome repo link | Verify canonical: `devcell-sh/home` returns 200 but AGENTS.md says `community-home` (404). If `home` is correct, update AGENTS.md | Landing #12 |
| F10 | 203 | "composable Nix home-manager modules (`nixhome/modules/`)" | Path doesn't exist in this repo. Link to external repo | Landing #13 |
| F11 | 221 | "each project spawns one cell" | Wrong: projects can have multiple cells via cellID. Fix: "each project spawns one or more cells" | Landing #14 |

### Positioning rewrites (demand-driven)

| # | Line(s) | Current | Proposed | Demand signal |
|---|---------|---------|----------|---------------|
| P1 | 5 | "containerized sandbox" | "local-first sandbox" | Score 70. "open source" in autocomplete. Daytona archived, DevPod stalled. Positioning moat. |
| P2 | 5 | "Your SSH keys, other repos, and host credentials stay out of reach" | "Your Max/Pro subscription works directly, 25+ MCP servers come pre-installed, and your host stays untouched. Free and open-source." | Lead with primary bets (Max score 66, MCP score 76) not implementation details. |
| P3 | 21 | "Isolated sandbox: agents edit freely inside your project; your host system is untouched" | "Full auto-approve in isolation: agents run with no permission prompts. They write, delete, and install freely inside the cell. Your host is untouched" | Combined score 92+79. This IS the product. Merge sandbox + auto-approve. |
| P4 | 22 | "Yahoo Finance, Google Maps, Linear, KiCad, Inkscape" | "Linear, Notion, OpenTofu, KiCad, Inkscape" | Lead with dev-relevant servers. Yahoo Finance confuses the dev-sandbox pitch. |
| P5 | 24 | "locks down outbound traffic to allow-listed domains only, strips privileged mode, dangerous capabilities, docker socket, and published ports. A squid gateway sidecar proxies all egress; everything else is denied" | "denies all outbound traffic except allow-listed domains. A sidecar proxy handles egress; everything else is blocked" | Implementation detail (squid, caps, docker socket) belongs in docs, not README. |
| P6 | 25 | "Anti-fingerprint Playwright replays sessions that pass Cloudflare and Kasada" | "The agent replays authenticated sessions without seeing your password" | Drop vendor names (adversarial tone). Value, not mechanism. |
| P7 | 28 | "Docker or VM engine" | "Three OS engines: Linux container (Docker), macOS VM (Tart), Windows VM (winkit). Same commands, same config" | "Docker or VM" undersells. Multi-OS is supplementary bet #4 (score 74, unique). |
| P8 | 34-41 | Comparison: VS Code Dev Containers, Daytona | Comparison: Docker Sandboxes, Microsandbox | Daytona archived Oct 2026. Dev Containers not a sandbox competitor. Docker Sandboxes and Microsandbox are the 2026 landscape. |
| P9 | 34-41 | 5 comparison rows | 8 rows: +local-first, +egress control, +snapshot (honest gap), +pricing, +isolation model | Current table hides advantages and gaps. Honest comparison builds trust (MIT AI Agent Index: most products hide gaps). |
| P10 | 42 | "Use Daytona for team-managed remote environments" | Drop Daytona. Add Docker Sandboxes, Microsandbox guidance | Daytona is dead. |
| P11 | 92 | "Baked into the image and auto-merged into each agent's config at container startup" | Add: "Nix-pinned to commit hashes in flake.lock, so every rebuild gets the same versions" | Landing #18: MCP section misses the Nix-pinning differentiator (Bucket 1). |
| P12 | 96-107 | MCP table column: "Domain" | Column: "What the agent can do" | "Domain" is abstract. Reader's question: "what can my agent do with this?" |
| P13 | 109-122 | Browser section: 14 lines (CDP port, headless instance, Patchright identity) | 8 lines: value + one-line how | Implementation belongs in docs. |
| P14 | 223-229 | "Not yet": 3 gaps | 4 gaps: add credential brokering proxy | #1 gap in top-8 features. Docker/E2B/Microsandbox ship proxy where secrets never enter sandbox. |

---

## 2. Reordering changes

| Item | Old position | New position | Score | Why |
|------|-------------|-------------|-------|-----|
| Auto-approve + isolation | Hero L5 (word only) + bullet 1 (separate concepts) | Bullet 1 (merged, explicit lead) | 92+79 | The combined story IS the product. Currently split. |
| Multi-OS engines | Bullet 8 (position 8/10) | Bullet 4 (position 4/8) | 74 | Unique. "windows" 3/3 autocomplete. Currently behind VNC (score 44). |
| Local-first / OSS | Not in README at all | Hero line + comparison row | 70 | Only remaining OSS local-first agent sandbox. Zero current mention. |
| Solitary Mode | Bullet 4 | Bullet 5 | 88 | Still above-fold but below primary bets. 5/7 competitors ship it: table stakes. |
| Browser login | Bullet 5 + 14-line section | Bullet 6 + 8-line section | 55 | Unique but moderate demand. Over-indexed in README space. |
| VNC/RDP | Bullet 6 (above 1Password and multi-OS) | Removed from bullets | 44 | Long-tail. Currently above features with 30+ higher scores. |
| Model ranking | Bullet 10 | Collapsed `<details>` | 48 | Not a bet. |
| WireGuard | Own section L151 (8 lines, same weight as Security) | Collapsed `<details>` | 35 | Lowest score. Gets more space than Claude Max (score 66). |
| Legacy stacks | Equal weight with primary stacks in table | Collapsed `<details>` | N/A | Maintained, not promoted. Dilute the pitch. |
| Credential brokering gap | Not in "Not yet" | Added to "Not yet" | 72 | Top-8 demand feature devcell lacks. |
| MCP table order | OpenTofu, Yahoo Finance, EdgarTools, FRED, Google Maps... | OpenTofu, Linear, Notion, KiCad, Inkscape... then financial/personal | 76 | Dev-relevant first. Non-dev servers confuse the pitch. |
| MCP-NixOS | In table | Remove (not in catalog) | - | Factual: not in published catalog. |

---

## 3. Tone changes

| Where | Current tone | Proposed tone | Why |
|-------|-------------|---------------|-----|
| Hero follow-up L5 | Fear: "SSH keys stay out of reach" | Value: "Max works, MCPs pre-installed, host untouched" | Sell what you GET, not what you AVOID. |
| Solitary Mode L24 | Implementation: "strips privileged mode, dangerous capabilities, docker socket. A squid gateway sidecar proxies all egress" | Outcome: "denies all outbound except allow-listed domains" | Developers scanning care about what, not how. |
| Browser login L25 | Technical proof: "pass Cloudflare and Kasada" | User benefit: "replays authenticated sessions without seeing your password" | Value over mechanism. Naming vendors reads adversarial. |
| Browser section L120-122 | Deep technical: "no --remote-debugging-port, headless CDP instance reads cookies, Patchright uses identical identity" | Brief: "no automation flags. Playwright replays with matching fingerprint" | Detail belongs in docs. |
| Multi-OS bullet L28 | Understated: "Docker or VM engine" | Confident: "Three OS engines. Same commands, same config." | This is a moat. Don't undersell it. |
| Stacks intro L46 | Dense: "dev seed and ultimate general development stack support extra capability modules; bbb retains the specialist tools previously bundled" | Short: table speaks for itself. Remove product-history context. | Current intro requires prior knowledge. |
| bbb stack L57 | Kitchen-sink: "Wine, QEMU, Swift, publishing, security/RE, Inkscape/GIMP, FFmpeg/yt-dlp, Kitty, wxWidgets, FreeRDP, native UI development and personal-service MCPs" | Framed: "ultimate + specialist tools: KiCad, Wine, QEMU, Swift, security/RE, Inkscape, FFmpeg" | A user seeing this list questions focus. Frame as deliberate, not random. |

---

## 4. Priority changes

### Promoted (up)

| Feature | From | To | Score | Rationale |
|---------|------|----|-------|-----------|
| Auto-approve in isolation | Implicit across hero + bullet 1 | Explicit bullet 1 lead | 92+79 | Product identity. Make it the first thing after quickstart. |
| Multi-OS engines | Bullet 8 (position 8/10) | Bullet 4 (position 4/8) | 74 | Unique. Behind VNC (44) currently. "windows" 3/3 autocomplete. |
| Local-first / open-source | Absent from README | Hero + comparison row | 70 | Only remaining OSS local-first agent sandbox. Unmentioned currently. |
| Credential brokering (gap) | Absent from "Not yet" | "Not yet" row | 72 | Top-8 gap. Be honest. |
| MCP count | "12+" | "25+" | 76 | Factual fix, but also underselling a primary bet by 50%. |

### Demoted (down)

| Feature | From | To | Score | Rationale |
|---------|------|----|-------|-----------|
| VNC/RDP desktop | Bullet 6 (above 1Password, multi-OS) | Removed from bullets | 44 | Over-promoted by 30+ score points vs neighbors. |
| Model ranking | Bullet 10 | Collapsed `<details>` | 48 | Not a bet. Useful CLI command, not a selling point. |
| WireGuard tunnels | Own section, 8 lines | Collapsed `<details>` | 35 | Lowest score. Gets dedicated section while Claude Max gets one bullet. |
| Legacy stacks | Equal table weight | Collapsed `<details>` | N/A | Maintained, not promoted. |
| Browser section | 14 lines + bullet 5 | 8 lines + bullet 6 | 55 | Unique but moderate demand. Over-indexed in README space. |
| Yahoo Finance / financial MCPs | Listed first in MCP examples | Listed after dev-relevant MCPs | N/A | Confuses the dev-sandbox pitch when listed first. |
| MCP-NixOS | In MCP table | Remove | N/A | Not in published catalog. |
| GCP, Azure claims | In infra module description | Remove | N/A | Not in published catalog. False claim. |

### Unchanged

| Feature | Position | Rationale |
|---------|----------|-----------|
| Claude Max/Pro | Bullet 3 | Already well-placed. Primary bet #3. |
| 1Password secrets | Bullet 7 | Appropriately placed in second tier. |
| Security section | Dedicated section | Accurate, concise. |
| Configuration section | Dedicated section | JSON Schema is a differentiator. |
| Customization section | Dedicated section | Good progressive disclosure. |
| Terminology table | Dedicated section | Correct after CELL-641 rename (fix "spawns one cell" wording). |
| Quickstart | Lines 7-17 | Already good. Just add Gemini. |

---

## Full feature judgment table

| Feature | Old: README? | Old: position | New: README? | New: position | Marketing/demand | Viability for devcell |
|---------|:-:|---|:-:|---|---|---|
| **Container sandbox isolation** | Yes | Hero L3 + bullet 1 | Yes | Hero + bullet 1 (merged with auto-approve) | Score 92. Anchor. 8+ HN threads, Wiki +162%. Table stakes: 7/7 ship it. | Shipped. Core. Not a differentiator but required. |
| **Auto-approve / full autonomy** | Yes | Hero L5 (word only) | Yes | Bullet 1 (explicit lead) | Score 79. HN: "YOLO!". 2,395 cc issues. Unique framing: sandbox+YOLO. | Shipped. Product identity. **Primary bet #1.** |
| **MCP servers baked in** | Yes | Bullet 2 + section L90 | Yes | Bullet 2 + section (reordered, fix count to 25+) | Score 76. MCP 70K wiki/mo. 14,584 cc issues. Docker: catalog (manual). Others: none. | Shipped. 25+ with backing tools. **Primary bet #2.** |
| **Claude Max/Pro (no API key)** | Yes | Bullet 3 | Yes | Bullet 3 (unchanged) | Score 66. 55M npm dl/mo market. 0/7 competitors. Financial hook. | Shipped. Completely unique. **Primary bet #3.** |
| **Multi-OS (Docker+Tart+winkit)** | Yes | Bullet 8 (buried) | Yes | Bullet 4 (promoted) | Score 74. 52K cc issues. "windows" 3/3 autocomplete. 0/7 do all three. | Shipped. Engineering moat. **Supplementary #4.** |
| **Solitary Mode (egress)** | Yes | Bullet 4 (overlong) | Yes | Bullet 5 (shortened) | Score 88. 4 surfaces strong. But 5/7 ship it: table stakes. | Shipped. Keep, shorten, don't lead. |
| **Local-first / open-source** | **No** | **Not mentioned** | **Yes** | **Hero + comparison row** | Score 70. Autocomplete: "open source", "offline". Gap widening. | Shipped (it's the architecture). **Supplementary #5.** |
| **1Password secrets** | Yes | Bullet 7 | Yes | Bullet 7 (shortened) | Score 72. Wiki +74%. devenv also has it. | Shipped. Partial (agent reads env vars). Keep. |
| **Stealth browser login** | Yes | Bullet 5 + section (14 lines) | Yes | Bullet 6 + section (8 lines) | Score 55. Chromium Wiki +96%. 0/7 competitors. 0 threads ask for it. | Shipped. Unique. Demos > README. Shorten. |
| **Nix reproducibility** | Yes | Implicit in stacks | Yes | Merged into stacks bullet + MCP pinning note | Score 61. 4/5 HN engagement. No agent sandbox uses Nix. | Shipped. Mention as outcome, not technology. |
| **Multi-agent (4 agents)** | Partial | Hero (3/4, Gemini missing) | Yes | Hero (all 4) + quickstart | Score 63. No competitor covers 4 agents in one CLI. | Shipped. **Factual fix: add Gemini.** |
| **VNC/RDP desktop** | Yes | Bullet 6 | Collapsed | Desktop module mention only | Score 44. Long-tail. Over-promoted currently. | Shipped. Niche. Demote. |
| **Model ranking** | Yes | Bullet 10 | Collapsed | `<details>` "More commands" | Score 48. Medium tier. Not a selling point. | Shipped. Move to CLI docs. |
| **Port forwarding** | No | Docs only | No | Docs only | Score 42. 5/7 ship it. Commodity. | Shipped. No README mention needed. |
| **Persistent cell identity** | Yes | Terminology table | Yes | Terminology table (unchanged) | Score 38. Hard to pitch in a bullet. | Shipped. Keep in terminology. |
| **WireGuard tunnels** | Yes | Own section (8 lines) | Collapsed | `<details>` | Score 35. Lowest. Gets more space than Max. | Shipped. Infra niche. Collapse. |
| **Stacks (primary)** | Yes | Section (8 stacks equal) | Yes | 4 visible, 4 legacy collapsed | N/A. | Shipped. Legacy dilutes pitch. |
| **Stacks (legacy)** | Yes | Same table as primary | Collapsed | `<details>` | N/A. | Shipped. Maintained, not promoted. |
| **Modules** | Yes | Sub-table L59 | Yes | Keep (fix infra: drop GCP/Azure) | N/A. | Shipped. Fix false claims. |
| **Config schema** | Yes | Section L133 | Yes | Keep (unique) | No competitor has this. | Shipped. Underrated. |
| **Customization** | Yes | Section L160 | Yes | Unchanged | N/A. | Shipped. Good structure. |
| **Host credential isolation** | Yes | Hero L5 + Security | Yes | Security only (remove from hero) | Implicit in sandbox. Not standalone. | Shipped. Move to Security. |
| **Gitleaks / scanning** | Yes | Security L131 | Yes | Unchanged | N/A. | Shipped. Correctly placed. |
| **Comparison table** | Yes | Section L32 | Yes | Updated: new competitors, new rows | N/A. | **2 dead competitors.** Update required. |
| **Snapshot/fork/restore** | Yes | "Not yet" L228 | Yes | "Not yet" (keep) | 2 strong + 2 moderate. 3 competitors ship it. | Planned. Top gap. |
| **Pre-built images** | Yes | "Not yet" L229 | Yes | "Not yet" (keep) | 3/3 consensus. All competitors have this. | Planned. #1 onboarding friction. |
| **Credential brokering proxy** | **No** | **Not mentioned** | **Yes** | **Added to "Not yet"** | Score 72. 3 competitors ship proxy-based. | **Gap. Add to "Not yet."** |
| **GPU passthrough** | No | Not mentioned | No | Not mentioned | 1 weak. Linux only. | Not planned. Skip. |
| **Cloud execution** | No | Not mentioned | No | Not mentioned | 1 moderate. Conflicts with local-first. | Not planned. Different product. |
| **MicroVM isolation** | No | Not mentioned | No | Comparison row (honest) | 2 strong. 4/7 ship it. | Not planned. Show honestly. |
| **IDE integration** | Yes | Comparison row L40 | Yes | Comparison row | Low demand. "CLI + any editor" is correct. | Shipped. Not a gap. |

---

## Broken links (fix in same PR)

| Link | Lines | Status | Fix |
|------|-------|--------|-----|
| `./MIGRATION.md` | 29, 46, 219 | File does not exist | Remove refs or create file |
| `devcell.sh/docs/cell` | 135 | 404 | Replace with `devcell --help` |
| `nixhome/modules/` | 203 | Path doesn't exist in repo | Link to `devcell-sh/home` |
| `devcell-sh/home` vs `community-home` | 170 | `home` returns 200, `community-home` returns 404 | Verify canonical; update AGENTS.md if `home` is correct |
| MCP-NixOS | 107 | Not in published catalog | Remove from table or verify |

---

## Section-level map (old vs new)

| # | Section | Lines | Change | Detail |
|---|---------|-------|--------|--------|
| 1 | Hero | 1-5 | Rewrite | +local-first, +Gemini, +Max/MCP in hero, +open-source, -SSH keys detail |
| 2 | Quickstart | 7-17 | Minor | +`devcell gemini` |
| 3 | What you get | 19-30 | Restructure | 10 bullets to 8. Reorder by demand. Remove VNC, model ranking. |
| 4 | Comparison | 32-42 | Rewrite | New competitors. 5 rows to 8. Honest gaps. |
| 5 | Stacks | 44-67 | Reorganize | 4 primary visible. 4 legacy collapsed. Fix ultimate desc, infra claims. |
| 6 | Engines | 68-88 | Unchanged | - |
| 7 | MCP servers | 90-107 | Reorder + fix | Dev-relevant first. Fix count. Remove MCP-NixOS. Add pinning note. |
| 8 | Browser login | 109-122 | Shorten | 14 lines to 8. Drop CDP/Patchright detail. |
| 9 | Security | 124-131 | Unchanged | - |
| 10 | Configuration | 133-149 | Link fix | Fix 404 docs link. |
| 11 | WireGuard | 151-158 | Collapse | Wrap in `<details>`. Same content. |
| - | More commands | new | New | Collapsed `<details>`: model ranking, build df, build prune, modules list. |
| 12 | Customization | 160-170 | Link fix | Fix nixhome link. |
| 13 | Development | 172-209 | Path fix | Fix nixhome/modules/ reference. |
| 14 | Terminology | 211-221 | Wording fix | "spawns one cell" to "spawns one or more cells". |
| 15 | Not yet | 223-229 | Add row | +credential brokering proxy. |
| 16 | License | 231-233 | Unchanged | - |
