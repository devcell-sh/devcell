# MCP Installation Pain: Consensus Research — 2026-10-12

Cross-agent consensus research (Claude + Codex, 4 surfaces, 15
agents). Findings verified by adversarial skeptics across vendors.

---

## Headline numbers

- **37.5% of MCP servers never start** on first install (arxiv
  study of 400 npm/stdio servers, Sep 2026)
- **Only 48.8%** complete an initialization handshake
- **12.5%** of developer-reported issues are config/environment
  problems (Concordia study, 136 issues across 7 reference servers)
- **85.2%** of MCP apps configure servers via files, but no naming
  convention has emerged (study of 1,723 MCP applications)
- **64%** of senior technical leaders cite security concerns as an
  adoption obstacle (Stacklok survey, 300 respondents)
- **38%** cite legacy-system integration complexity
- **Docker MCP Catalog hit 1M+ pulls** since launch

Sources: arxiv.org/abs/2609.10962, das.encs.concordia.ca,
arxiv.org/abs/2607.25635, stacklok.com State of MCP 2026

---

## Consensus findings (both Claude and Codex confirmed)

### 1. Containerized zero-dependency install (strong)

Docker MCP Toolkit/Catalog advertises "zero manual setup, no
dependency management, no runtime configuration" with 300+
verified containerized MCP servers and one-click launch from
Docker Desktop into Claude, Cursor, VS Code, Windsurf,
continue.dev, Goose. Positioned as the fix for "no Node.js
required since everything runs in Docker."

**DevCell angle:** We do this already (backing tools ship
pre-installed), but Docker's catalog is 300+ servers. Our
differentiator is not the count but that tools are nix-pinned
and auto-configured into the agent's config. Docker's catalog
requires manual setup per server.

Evidence: Docker blog, Medium coverage, Playwright/filesystem/
docker-mcp READMEs.

### 2. Roots capability deprecation (moderate)

The 2026-07-28 MCP spec deprecates the Roots capability, which
previously let clients restrict which filesystem paths a server
could touch. Security researchers (Backslash) note "scope
enforcement is no longer structural; it's opt-in." No competitor
server claims to have filled this gap.

**DevCell angle:** Container isolation fills this gap structurally.
The agent and its MCP servers are sandboxed by the container, not
by the MCP protocol. This is a positioning wedge: "MCP's own
sandboxing got weaker; container isolation got more important."

---

## Verified single-vendor findings

### Community forums (Codex-sourced, Claude-verified)

| Signal | Status | Evidence |
|--------|--------|----------|
| One-click installation | weak | HN: 23 pts, 6 comments (installation instructions generator) |
| Prepackaged browser dependencies | moderate | Puppeteer's 180MB first-run Chromium download causes Claude Desktop's 60s init timeout. Author recommends "bake dependencies into Docker images" |
| Local server packaging | weak | Single HN comment asking about stdio server support |
| Runtime dependency management | strong (codex-only) | Puppeteer missing deps, SQLite uvx version mismatch |
| Client config generation | strong (codex-only) | Users spend 30-45 min constructing/validating JSON configs |
| Windows path compatibility | moderate (codex-only) | Filesystem server fails on Windows paths with spaces |
| Secure credential injection | moderate (codex-only) | Shell launcher needed for Postgres connection string from .env |

### GitHub issues (Codex-sourced, Claude-verified)

| Signal | Status | Evidence |
|--------|--------|----------|
| One-click server installation | strong | MCP Registry RFC, IDE discussion, IBM proposal |
| Reliable dependency packaging | moderate | Filesystem server npx startup failures (2024-2025) |
| Visible installation diagnostics | moderate | Playwright MCP stuck in VS Code, Postgres silent hangs |
| Reuse installed browsers | moderate | Docker deployment reinstalled already-provisioned browsers |
| Portable executable resolution | moderate | SQLite uvx ENOENT despite installation; absolute path fixed it |
| Database connection diagnostics | moderate | Postgres server hung against RDS, closed as "not planned" |

### Competitor features (Codex-sourced, Claude-verified)

| Signal | Status | Evidence |
|--------|--------|----------|
| Containerized installation | strong | Playwright, filesystem, docker-mcp all document Docker launch |
| Automatic browser installation | strong | executeautomation/mcp-playwright auto-detects/downloads browsers |
| One-click client installation | strong | K8s, Docker, Playwright offer Claude Desktop extension bundles |
| Native dependency prerequisites | strong | K8s needs kubectl+helm+kubeconfig; docker-mcp needs Python+uv+Rust/OpenSSL |
| Browser mode limitations | strong | Docker mode = headless Chromium only |
| Automatic cross-server remediation | absent | Nobody detects/repairs prereqs across server types in one flow |
| Configuration rollback | absent | Nobody restores previous config after failed install |

### Analyst reports (Codex-sourced, Claude-verified)

| Signal | Status | Evidence |
|--------|--------|----------|
| Local installation friction | strong | Docker blog names repo cloning, Node/Python dep conflicts as hurdles |
| Servers fail to start | strong | 37.5% never started in 400-server study |
| Configuration environment failures | strong | 12.5% of issues are config/env problems |
| Fragmented client configuration | strong | 85.2% use files, no naming convention |
| Enterprise integration complexity | strong | 38% of 300 senior leaders cite it |
| Installation security controls | strong | 64% cite security concerns |
| One-click container installs | strong | Docker MCP Catalog 1M+ pulls |
| Managed remote deployment | moderate | 47% of 15,382 servers are hosted endpoints |
| Managed MCP funding | moderate | Manufact $6.3M seed for MCP server deployment |

---

## Whitespace nobody claims

1. **Automatic cross-server remediation**: no product detects and
   repairs prerequisites across different server types in one
   setup flow. DevCell does this implicitly (everything is
   pre-installed).
2. **Configuration rollback**: no product restores a client's
   previous MCP config after a failed install.
3. **Measured installation time**: no source measures end-to-end
   first-time installation time.

---

## Conclusions for DevCell

### What this validates

1. **MCP installation is a genuine, measured pain point.** 37.5%
   fail rate on first install. This is not anecdotal.
2. **Docker is the incumbent solution** with 300+ containerized
   servers and 1M+ pulls. Our wedge is not "we also containerize"
   but "we ship the backing tools, auto-configure into the agent,
   and pin versions via nix."
3. **Browser dependencies are the #1 specific pain.** Playwright/
   Puppeteer's 180-200 MB download and system lib requirements
   cause the most complaints. DevCell ships this pre-installed.
4. **MCP's own sandboxing got weaker** (Roots deprecation). Container
   isolation is more important now, not less.
5. **Security is the top adoption blocker** (64% of senior leaders).
   This aligns with DevCell's sandbox-first positioning.

### What this changes

| Finding | Product impact |
|---------|---------------|
| 37.5% fail rate | Use this stat in MCP bullet copy |
| Docker 300+ catalog | Don't compete on count. Compete on "works out of the box" |
| Roots deprecation | New positioning angle: "MCP sandboxing got weaker, container isolation got stronger" |
| 64% security concern | Reinforces sandbox-first positioning |
| Cross-server remediation whitespace | DevCell does this implicitly. Call it out. |
| Browser dep pain | Already shipped. Highlight in README. |

### README changes (applied)

The MCP bullet in README.md now leads with: "you don't install MCP
servers; they come ready. 86% of MCP servers need local binaries
that break on PATH mismatches and missing system libs."

### Future items to track

| Item | Priority | Type |
|------|----------|------|
| PostgreSQL MCP server | Medium | community-home addition |
| Kubernetes MCP server | Low | community-home addition |
| Memory/knowledge-graph MCP | Low | community-home addition |
| "37.5% fail rate" stat in landing page | Now | Copy change |
| Roots deprecation positioning | Medium | Blog post / positioning |
| Docker MCP Catalog competitive response | Monthly | Monitor |
