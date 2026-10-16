# README Review Research — 2026-10-12

Cross-agent research on 7 README sections. Four parallel forks ran
web search + codebase verification. Findings below, conclusions at
the end.

---

## 1. MCP Tools: Top 10 Hardest to Install Locally

86% of MCP servers run locally. The #1 failure class is
PATH/environment mismatch: GUI-launched apps can't find `npx`,
`node`, `kubectl`. A container with everything pre-installed
eliminates this entire class.

| # | MCP Server | Core friction | DevCell ships it? |
|---|-----------|--------------|-------------------|
| 1 | **Playwright** | 200 MB browser binaries + ~15 system libs on Linux; GUI apps can't find `npx` | Yes |
| 2 | **Puppeteer** | 200 MB Chromium download; fails on proxies, CI, Alpine, rootless containers | Yes (Playwright replaces) |
| 3 | **PostgreSQL** | Running instance + connection string + network config; reference server deprecated with CVE | No |
| 4 | **Kubernetes** | kubectl + helm + kubeconfig + RBAC; multi-cluster incomplete | Partial (helm in go stack) |
| 5 | **Memory/Knowledge-Graph** | `better-sqlite3` needs native compilation (Python + C++ + node-gyp) | Partial (sqlite ships) |
| 6 | **GitHub** | PAT scope confusion; GUI apps can't see env vars | Env passthrough |
| 7 | **Filesystem** | Symlink CVEs, sandbox-in-sandbox confusion, security config | Sandboxed by design |
| 8 | **AWS/Terraform** | 4 prerequisites (uv, Python, Terraform, Checkov); deprecated server still in search results | Yes (OpenTofu) |
| 9 | **Docker** | Socket mounting security; no canonical server; fragmented implementations | No (no DinD) |
| 10 | **Git** | Package unpublished from npm; SSH/GPG config inaccessible to MCP process | Yes (git ships) |

**Takeaway for README:** The MCP bullet should lead with the pain
DevCell eliminates: "You don't install MCP servers; they come
ready. The backing tools ship too." DevCell already ships 4 of the
top 10 pain points (Playwright, FFmpeg, OpenTofu, KiCad).

---

## 2. Stack Popularity (ranked by demand signal)

| # | Category | Evidence | DevCell has it? | Setup pain |
|---|----------|----------|----------------|-----------|
| 1 | Data Science / ML | Transformers 163k, PyTorch 102k stars | Partial (Python stack) | High |
| 2 | Security / Pentesting | Awesome-Hacking 122k, PayloadsAllTheThings 82k, r/netsec 570k | Yes (bbb stack) | Very high |
| 3 | **Reverse Engineering** | rea 142k, Ghidra 85k, ImHex 55k, r/RE 176k members | Yes (android module + bbb) | Very high |
| 4 | Browser Automation | Playwright, Browser Use 50k stars, 45% search demand jump | Yes | High |
| 5 | Infrastructure / DevOps | OpenTofu 10M downloads, Docker shipped Terraform Ops Kit | Yes (go stack) | Moderate |
| 6 | Electronics / PCB | KiCad ecosystem, near-zero competition for pre-built envs | Yes (electronics stack) | Very high |
| 7 | Fullstack Web | Node/TS/React, commodity | Yes (fullstack) | Low |
| 8 | Rust Systems | Tauri 102k, 82% admiration rate | No | Moderate |
| 9 | Mobile Dev | Flutter 178k, React Native 126k | Partial | High |
| 10 | Game Dev | Godot 118k, Bevy 49k | No | Moderate |

**Key insight:** RE, security, embedded, electronics have the
worst "setup pain to available pre-built environments" ratio. This
is DevCell's biggest differentiation: these toolchains are the
hardest to set up AND the least served by existing sandboxes.

**Recommendation:** Show off security/RE, electronics, and infra
in the README. These are the categories where "wow, that's
pre-installed?" hits hardest. Fullstack web is table stakes.

---

## 3. Browser Login: Rename and Demote

### Current problems

- "Browser login" is ambiguous: sounds like the agent logs in
- `auth` in CLI means "authenticate with the tool itself" (gh auth,
  gcloud auth), not "forward sessions"
- Anti-bot language ("identical browser fingerprint", "cannot
  detect") is bot-evasion marketing. Anthropic's Claude Code
  Desktop explicitly does the opposite.

### Naming alternatives

| Name | Pros | Cons |
|------|------|------|
| **Session Handoff** | Matches IETF draft (draft-moros-oauth, Apr 2026), AgentBay/Hermes pattern | New term, less descriptive |
| **Browser Session Forwarding** | Self-explanatory | Verbose |
| **Cookie Bridge** | Technically accurate | Too low-level |

### Recommendation

- Rename section to **"Session Handoff"**
- Demote from standalone section to one bullet (too niche for
  90-second scan)
- Lead with: "your password never enters the sandbox"
- Drop anti-bot language entirely
- Suggested bullet: *"Session handoff: `devcell auth chrome` lets
  you log in on your host; cookies sync to the cell so the agent
  can browse authenticated pages. Your password never enters the
  sandbox."*

---

## 4. Engines: mackit and Engine Names

### mackit

- Zero references in codebase (grep confirmed)
- Not shipped, not announced
- **Do not mention.** Listing unshipped engines erodes credibility
  with senior engineers (target persona)
- When mackit ships, swap one line in the table

### Engine labels for README

- macOS: keep **"Tart"** (the tool users install). Apple
  Virtualization is the underlying framework, below the abstraction.
- Windows: **"winkit"** uses vz on macOS, qemu on Linux
  (`internal/engine/capability.go:16`). Can say "Windows PE VM"
  without exposing engine names.
- The user wanted to add "QEMU" and "Apple Virtualization" to the
  engine descriptions. Marketing recommendation: engine names
  belong in architecture docs, not the 90-second scan. The table
  should show what the user cares about: guest OS, host
  requirement, what runs.

---

## 5. Community-Home vs DevCell Repo Split

### The split

- **devcell repo** = CLI engine, container orchestration
- **community-home** (github.com/devcell-sh/home) = stacks,
  modules, MCP server configs, packages

### Recommendation

Reader doesn't care about Git topology. Present as:

> "Batteries included, everything swappable."

- Show 3-4 hero stacks inline (the impressive ones)
- One sentence: "Ships with curated stacks and MCP configs from
  [community-home](https://github.com/devcell-sh/home). Fork it to
  customize."
- Move full catalog to docs or the community-home README
- MCP: show 3-4 headline capabilities, then "11 servers included.
  Full list in community-home."

---

## 6. Configuration Section

**Verdict: not a selling point** for the 90-second scan. Alex
(target persona) cares that config exists (evaluation step 3:
"preserve my config"), not about JSON Schema URLs and catalog
endpoints.

**Recommendation:** Collapse to 2 lines: "Project config at
`devcell.toml` with JSON Schema for editor validation. See
`devcell --help` for the full reference." Move schema URLs,
catalog detail, and WireGuard into docs or `<details>`.

---

## 7. Customization Section

**Verdict: keep but compress.** Maps to Alex's evaluation step 6
("Can I get out?") as a trust signal, not a selling point.

**Recommendation:** Reframe as "When you need more" in 5 lines:

```
## When you need more
- Runtime versions: mise / .tool-versions (no rebuild)
- Extra packages: devcell.toml → devcell build
- Full control: .devcell/flake.nix
- Org-wide stacks: fork community-home
```

---

## Direct Fixes (no research needed)

| Section | Current | Proposed |
|---------|---------|----------|
| SSH keys bullet | "unreachable" | "unreachable unless explicitly configured" |
| Comparison: DevCell isolation | "container" | "container, VM" |
| Security: mount path | "mounted at /workspace" | "mounted at `/<cellName>` and at its original host path" |

---

## Conclusions

### Changes to apply to README.md

1. **SSH keys bullet** — add "unless explicitly configured"
2. **MCP bullet** — lead with installation pain DevCell eliminates
3. **Comparison table** — DevCell isolation: "container, VM"
4. **Stacks** — reframe: community-home is the content layer,
   show 3-4 hero stacks (security/RE, electronics, infra), link
   to full catalog
5. **Engines** — don't mention mackit, keep Tart/winkit labels
6. **MCP servers** — frame as "ships with community-home", show
   headline capabilities, link to catalog
7. **Browser login** — rename to "Session Handoff", demote to
   bullet, drop anti-bot language
8. **Security: mount path** — fix to `/<cellName>` + host path
9. **Configuration** — collapse to 2 lines
10. **Customization** — compress to 5 lines as "When you need more"

### New insights for product positioning

- **MCP installation pain is a stronger differentiator than we
  thought.** 86% of servers need local install, #1 failure is
  PATH/env mismatch. DevCell eliminates the entire class.
- **Security/RE and electronics are the "wow" stacks.** Highest
  setup pain, least served by competitors, most impressive when
  pre-installed.
- **Browser session forwarding is niche but unique.** No major
  competitor offers it. Worth keeping as a bullet, not a section.
- **Configuration and customization are trust signals, not selling
  points.** Compress both below the fold.
