# Search & Download Trends for devcell Feature Areas

**Date:** 2026-10-12  
**Sources:** Wikipedia Pageviews API, Google Autocomplete, PyPI Stats, NPM Registry, GitHub API

---

## 1. Wikipedia Monthly Pageviews (Topic Interest Proxy)

October is partial (12 days). Trend is Apr-Sep full months.

| Article | Apr | May | Jun | Jul | Aug | Sep | Trend |
|---|---|---|---|---|---|---|---|
| **Model_Context_Protocol** | 68,262 | 69,812 | 62,745 | 60,953 | 61,752 | 70,013 | **stable-high** (largest by 3x) |
| **Docker_(software)** | 35,821 | 36,237 | 39,407 | 53,620 | 34,807 | 43,042 | **stable** |
| **Chromium_(web_browser)** | 32,058 | 34,425 | 30,612 | 43,614 | 56,099 | 63,000 | **UP +96%** (Apr->Sep) |
| **WireGuard** | 17,998 | 16,793 | 15,348 | 14,926 | 16,189 | 20,485 | **stable** |
| **Remote_Desktop_Protocol** | 10,667 | 9,654 | 9,406 | 10,845 | 11,858 | 9,537 | **stable** |
| **Sandbox_(computer_security)** | 6,841 | 8,057 | 7,614 | 9,845 | 11,199 | 17,912 | **UP +162%** (Apr->Sep) |
| **1Password** | 7,927 | 8,304 | 8,113 | 7,785 | 7,496 | 13,801 | **UP +74%** (spike Sep) |
| **Nix_(package_manager)** | 6,059 | 6,344 | 6,449 | 5,833 | 7,381 | 8,267 | **UP +36%** |
| **Virtual_Network_Computing** | 2,576 | 2,708 | 2,642 | 2,659 | 2,370 | 2,520 | **stable-low** |

**Key signals:**
- "Sandbox (computer security)" pageviews nearly tripled Apr-Sep: the concept is getting mainstream attention
- MCP remains the largest topic by 3x, steady at ~65K/mo: market awareness is established
- Chromium interest nearly doubled: likely driven by agent browser automation (Playwright, stealth browsing)
- Nix is growing modestly: aligns with devenv 2.0 launch and reproducibility trend

---

## 2. Google Autocomplete Suggestions

Shows what people are actually typing. Ordered by Google's predicted relevance.

### "AI agent sandbox"
1. ai agent sandbox
2. ai agent sandbox **github** (looking for OSS solutions)
3. ai agent sandbox **windows** (OS-specific demand)
4. ai agent sandbox **docker** (Docker is the expected runtime)
5. ai agent sandboxing
6. ai agent sandbox **environment**
7. ai agent sandbox **escape** (security concern)
8. ai agent sandbox **open source** (OSS demand signal)

### "claude code sandbox"
1. claude code sandbox
2. claude code sandboxing
3. claude code sandbox **docker** (Docker is the expected runtime)
4. claude code sandbox **mode**
5. claude code sandbox **windows** (Windows demand)
6. claude code sandbox **settings**
7. claude code sandbox **macos** (macOS demand)
8. claude code sandbox **network** (network control demand)

### "codex sandbox"
1. codex sandbox mode
2. codex sandbox
3. codex sandbox **issues** (pain point)
4. codex sandbox **offline** (local/offline demand)
5. codex sandbox **docker**
6. codex sandbox **policy** (policy control demand)
7. codex sandbox **settings**
8. codex sandbox **windows**

### "AI coding agent isolation"
(no suggestions: term is too niche/academic)

### "dev container MCP"
1. dev container mcp
2. dev container playwright mcp
(only 2: emerging but not established)

### "local dev environment AI"
(no suggestions: term not yet established in search)

**Key signals from autocomplete:**
- **Docker** appears in 3/3 sandbox queries: it's the expected runtime
- **Windows** appears in 3/3: significant unmet demand for Windows sandboxing
- **macOS** appears for Claude Code: macOS VM sandboxing has search demand
- **open source** appears for general AI sandbox: OSS positioning has pull
- **network** appears for Claude Code sandbox: network control is searched for
- **offline** appears for Codex: local-first/offline has real search intent
- **escape** appears for AI agent sandbox: security is a top concern
- **policy** appears for Codex: policy-gated autonomy has search demand

---

## 3. PyPI Monthly Downloads (with mirrors)

### e2b (core SDK)

| Month | Downloads | MoM |
|---|---|---|
| 2026-04 | 2,921,781 | |
| 2026-05 | 4,740,777 | +62% |
| 2026-06 | 4,483,080 | -5% |
| 2026-07 | 5,822,497 | +30% |
| 2026-08 | 5,841,440 | ~0% |
| 2026-09 | 5,855,363 | ~0% |

Trend: **grew fast, now plateauing at ~5.8M/mo**

### e2b-code-interpreter

| Month | Downloads | MoM |
|---|---|---|
| 2026-04 | 1,302,140 | |
| 2026-05 | 2,170,892 | +67% |
| 2026-06 | 2,098,172 | -3% |
| 2026-07 | 2,649,808 | +26% |
| 2026-08 | 2,761,844 | +4% |
| 2026-09 | 2,563,481 | -7% |

Trend: **plateauing at ~2.6M/mo** (same pattern as core)

### daytona-sdk

| Month | Downloads | MoM |
|---|---|---|
| 2026-04 | 268,381 | |
| 2026-05 | 405,356 | +51% |
| 2026-06 | 728,256 | +80% |
| 2026-07 | 903,371 | +24% |
| 2026-08 | 753,879 | -17% |
| 2026-09 | 808,058 | +7% |

Trend: **grew fast, now plateauing at ~800K/mo** (repo archived Oct 2026)

### microsandbox

| Month | Downloads | MoM |
|---|---|---|
| 2026-04 | 7,977 | |
| 2026-05 | 16,695 | +109% |
| 2026-06 | 29,048 | +74% |
| 2026-07 | 33,636 | +16% |
| 2026-08 | 34,613 | +3% |
| 2026-09 | 38,100 | +10% |

Trend: **still growing, 5x in 6 months**, but absolute numbers are small (~38K/mo vs e2b's 5.8M)

---

## 4. NPM Monthly Downloads

### e2b (NPM)

| Month | Downloads | MoM |
|---|---|---|
| 2026-04 | 2,577,198 | |
| 2026-05 | 3,004,853 | +17% |
| 2026-06 | 4,164,660 | +39% |
| 2026-07 | 5,185,767 | +25% |
| 2026-08 | 6,307,003 | +22% |
| 2026-09 | 7,311,926 | +16% |

Trend: **steady growth, +184% in 6 months** (NPM growing faster than PyPI)

### @e2b/sdk (NPM, narrower package)

| Month | Downloads |
|---|---|
| 2026-04 | 15,942 |
| 2026-05 | 22,927 |
| 2026-06 | 37,158 |
| 2026-07 | 31,948 |
| 2026-08 | 44,446 |
| 2026-09 | 44,678 |

Trend: **growing, ~3x in 6 months**

### @anthropic-ai/claude-code (NPM, context: the agent devcell sandboxes)

| Month | Downloads |
|---|---|
| 2026-04 | 49,361,496 |
| 2026-05 | 34,562,178 |
| 2026-06 | 44,038,610 |
| 2026-07 | 49,100,543 |
| 2026-08 | 82,199,042 |
| 2026-09 | 55,930,237 |

Trend: **massive scale (~50M+/mo), growing** — the addressable market for devcell's primary agent

---

## 5. GitHub Competitor Metrics

| Repo | Stars | Forks | Open Issues | License | Last Push | Status |
|---|---|---|---|---|---|---|
| **daytonaio/daytona** | 71,562 | 5,646 | 458 | ? | 2026-07-24 | **Archived** |
| **lima-vm/lima** | 22,075 | 977 | 510 | Apache-2.0 | 2026-10-12 | Active |
| **coder/coder** | 16,976 | 1,620 | 1,173 | AGPL-3.0 | 2026-10-12 | Active |
| **loft-sh/devpod** | 15,257 | 586 | 47 | MPL-2.0 | 2025-11-14 | **Stalled** |
| **e2b-dev/E2B** | 14,294 | 1,102 | 96 | Apache-2.0 | 2026-10-09 | Active |
| **superradcompany/microsandbox** | 8,661 | 474 | 96 | Apache-2.0 | 2026-10-12 | Active |
| **cachix/devenv** | 7,734 | 577 | 386 | Apache-2.0 | 2026-10-12 | Active |
| **devcell-sh/devcell** | 4 | 0 | 4 | Apache-2.0 | 2026-10-11 | Active |

**Key signals:**
- Daytona (71K stars) archived: largest OSS competitor exited, leaving a gap
- DevPod (15K stars) stalled (last push Nov 2025): another gap
- E2B (14K stars) active with 5.8M PyPI + 7.3M NPM downloads/mo: market leader
- Microsandbox (8.6K stars) growing fast: the emerging local-first competitor
- devcell (4 stars): feature-rich but zero community awareness

---

## 6. Homebrew Analytics

devcell is distributed via a custom tap (`devcell-sh/tap/devcell`), not the core Homebrew formula repository. Homebrew analytics only covers the core and cask repositories. No install count data available.

---

## Summary: Demand Signals Aligned to devcell's Current Features

| devcell Feature | Search Signal | Download/Stars Signal | Wikipedia Signal | Strength |
|---|---|---|---|---|
| **Docker sandbox** | 3/3 autocomplete | e2b: 13M dl/mo (PyPI+NPM combined) | Docker: 43K views/mo | **Very Strong** |
| **Solitary Mode (network egress)** | "sandbox network", "sandbox escape" | — | Sandbox: +162% | **Strong** |
| **MCP servers baked in** | "dev container mcp" (2 suggestions) | — | MCP: 70K views/mo (largest) | **Strong** |
| **Multi-OS (Windows VM)** | 3/3 "sandbox windows" | — | — | **Strong** |
| **Multi-OS (macOS VM)** | "claude code sandbox macos" | — | — | **Moderate** |
| **Local-first / offline** | "codex sandbox offline" | DevPod stalled, Daytona archived | — | **Moderate** |
| **Open source** | "ai agent sandbox open source" | Daytona archived, gap forming | — | **Moderate** |
| **Stealth browser auth** | Chromium +96% pageviews | — | Chromium: 63K/mo and rising | **Moderate** |
| **1Password secrets** | — | — | 1Password: +74% spike | **Weak-Moderate** |
| **Nix reproducibility** | — | devenv: 7.7K stars | Nix: +36% | **Moderate** |
| **WireGuard tunnels** | — | — | WireGuard: stable 17K/mo | **Weak** |
| **VNC/RDP desktop** | — | — | VNC: 2.5K/mo (low); RDP: 10K/mo | **Weak** |
| **Claude Max/Pro support** | — | Claude Code: 55M NPM dl/mo | — | **Strong** (addressable market) |
