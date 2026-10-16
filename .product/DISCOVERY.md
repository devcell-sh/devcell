# Discovery

How humans and agents find DevCell. Owns GitHub metadata, SEO,
AIO (AI optimization), and registry presence (pkg.go.dev, Homebrew).

LANDING.md owns what visitors see when they arrive. This file owns
how they get there.

Links: [LANDING.md](./LANDING.md), [FEATURES.md](./FEATURES.md),
[COMPETITION.md](./COMPETITION.md).

## GitHub metadata

### Description

```
Local-first sandbox for AI coding agents. Run Claude Code, Codex,
or Gemini in an isolated container or VM with zero permission prompts.
No account required.
```

Reuses the site's `<meta name="description">` (Base.astro:10).
Names the mechanism (sandbox, container, VM) and the action
(run, zero permission prompts) in words a user would search.

### Topics (target: 10-12)

```
ai-agents, claude-code, sandbox, cli, docker, devcontainer,
nix, mcp, go, developer-tools, containers, macos
```

Cover three axes: domain (ai-agents, claude-code, mcp), language
(go), and category (sandbox, cli, docker, devcontainer, containers,
developer-tools, nix, macos).

### Social preview image

1280x640, dark background, "DevCell" in large text, one-line tagline.
Match the website's brutalist aesthetic.

## SEO

### Canonical URL

`devcell.sh` (bare domain, no `www`, no trailing slash). Set via
`<link rel="canonical">` in Base.astro.

### robots.txt

Present at `web/public/robots.txt`. Per-crawler AI directives:
search/retrieval crawlers allowed, training crawlers allowed (OSS),
CCBot disallowed. RFC 9309 compliant (named user-agent groups).

### Sitemap

`@astrojs/sitemap` integrated (2026-10-13). Generates
`sitemap-index.xml` referenced by robots.txt.

### OG tags

Present and working (Base.astro). No action needed.

## AIO (AI optimization)

### llms.txt

Present at `web/public/llms.txt`. CommonMark format, under 3K tokens.
H1 "DevCell", blockquote summary, sections: Getting started (3 links),
CLI reference (10 links), Guides (8 links), MCP servers (6 links),
Optional (3 links).

### Registry presence (pkg.go.dev)

Module path: `github.com/DimmKirr/devcell` (stale, 301 redirects
to `devcell-sh/devcell`). Migration to `github.com/devcell-sh/devcell`
is a large coordinated effort (207 files).

Root package has no .go files, so `go doc .` shows nothing.
cmd/main.go:1-3 has a synopsis but it's not reachable from the
module root on pkg.go.dev.

### Package doc comments

All internal packages have `// Package` doc comments. Added to
cfg, serve, scaffold, ollama, op, config, cloudmodels, backup,
engine/tart, ptyctl (2026-10-14).

### Go Example functions

1 exists (internal/engine/example_test.go). Zero in cmd/,
internal/cfg, internal/cell, internal/scaffold. pkg.go.dev shows
Example functions prominently; agents use them to evaluate a library.

### Public symbol documentation

74/209 exported symbols (35%) have doc comments. Key gaps:
`Execute()`, `Scaffold()`, `WithSize()`, `StartWithOptions()`.

---

## 2026-10-14T12:00:00Z: AIO scan findings

Source: `.scratch/research/2026-10-14T12-00-00-aio-scan.md`
(Claude Workflow, 10 agents, verified + Playwright website review).

### Applied this session

1. README: badges, "Why DevCell?" section, feature hierarchy,
   Contributing section, SECURITY.md pointer
2. CONTRIBUTING.md: fixed clone URL (DimmKirr to devcell-sh),
   added response time expectations
3. SECURITY.md: created (reporting, timeline, scope)
4. Error URLs: 4 sites updated from DimmKirr to devcell-sh
   (capability.go, engine.go, responses.go, example_test.go)
5. Website: fixed stale `cell --version` in quickstart.md

### Gaps remaining

| Signal | Status | What to do |
|---|---|---|
| GitHub description | DONE | Set via `gh repo edit` (2026-10-14) |
| GitHub topics | DONE | 16 topics set (2026-10-14) |
| Social preview | created | Image at `web/public/social-preview.png`, needs manual upload in GitHub Settings |
| go.mod module path | weak | Plan migration from DimmKirr/devcell to devcell-sh/devcell (207 files) |
| llms.txt | DONE | Created `web/public/llms.txt` (2026-10-14) |
| robots.txt AI directives | DONE | Full per-crawler AI directive set (2026-10-14) |
| OG image | DONE | 1200x630 dark image at `web/public/og-image.png` (2026-10-14) |
| Package doc comments | DONE | All 10 packages documented (2026-10-14) |
| Timeout errors | DONE | Container name + `docker logs` hint in run.go, binary + duration in pty_exec.go (2026-10-14) |
| Docker socket toggle | DONE | `docker_socket` TOML key + `DEVCELL_DOCKER_SOCKET` env var (2026-10-14) |
| BootDirWatcher cleanup | DONE | Deleted dead code: boot_watcher.go, boot_consumer.go, 17 sentinel bridges (2026-10-14) |
| FAQ code rendering | DONE | `set:html` in FAQ.astro for `<code>` tags (2026-10-14) |
| Dead components | DONE | Deleted CompareSection.astro, StackTable.astro (2026-10-14) |
| Go Example functions | weak | Add ExampleXxx to internal/cfg, internal/scaffold |
| Public symbol docs | weak | Document Execute(), Scaffold(), WithSize(), StartWithOptions() |
| Third-party logos | weak | Add NOTICE file for 38 brand marks in web/public/logos/ |
