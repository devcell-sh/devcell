# MCP Route, llms.txt, robots.txt, and Agent Discovery Research -- 2026-10-14

Research for the `aio-audit` website sub-skill: what signals should a
website expose for AI agent discoverability, and what should the audit
check?

## 1. llms.txt

**Spec:** [llmstxt.org](https://llmstxt.org) (Jeremy Howard, Answer.AI, Sep 2024).
**Path:** `/llms.txt` (root or any subpath covering URLs beneath it).
**Format:** Markdown. Intentionally lean.

### Required structure

```markdown
# Project Name

> One-paragraph summary (key info for understanding the rest)

Optional prose details.

## Section Name

- [Link title](https://url): Optional description

## Optional

- [Secondary link](https://url)
```

Rules:
- H1 is mandatory and must be first.
- H2 sections contain link lists: `- [title](url): description`.
- "Optional" section (by convention) holds secondary links an agent
  can skip when context is tight.
- Keep under ~3,000 tokens so the whole file fits any model's context.
- Serve as `text/plain; charset=utf-8`.

### Companion: llms-full.txt

Optional. Concatenates the full body content of linked pages into one
file so an LLM can consume the site in a single fetch. No separate
spec; just "dump everything linked from llms.txt into one file."

### HTML hints

Pages can add:
- `<link rel="alternate" type="text/markdown" href="/page.md">` for
  markdown versions of pages.
- `<link rel="describedby" href="/llms.txt">` pointing to the
  applicable llms.txt.

### Adoption (Oct 2026)

Widely adopted by Mintlify (auto-generates for all hosted docs),
Anthropic (recommends for Claude docs ingestion), Cursor, GitBook.
OpenAI, Google, Perplexity have not formally committed. Google Search
said "not needed for AI Overviews" (May 15) but Chrome Lighthouse
added an "Agentic Browsing" category that audits for it (May 20).

**Sources:**
- [llmstxt.org](https://llmstxt.org)
- [Bluehost: What is llms.txt? (2026)](https://www.bluehost.com/blog/what-is-llms-txt/)
- [Search Engine Land: Meet llms.txt](https://searchengineland.com/llms-txt-proposed-standard-453676)
- [Mavik Labs: llms.txt practical guide 2026](https://www.maviklabs.com/blog/llms-txt-guide-2026)

---

## 2. robots.txt for AI crawlers

**Spec:** RFC 9309 (robots.txt) + de facto AI bot user-agent tokens.
**Path:** `/robots.txt`.

### Three types of AI bots (2026)

| Type | Purpose | Examples | Block? |
|------|---------|----------|--------|
| Training crawler | Collects for future model training | GPTBot, ClaudeBot, Meta-ExternalAgent, CCBot, Bytespider, Google-Extended, Amazonbot | Often yes |
| Search crawler | Indexes for retrieval/AI search | OAI-SearchBot, Claude-SearchBot, PerplexityBot | Usually allow |
| User fetcher | Real-time fetch on user request | ChatGPT-User, Claude-User, Perplexity-User | Usually allow |

### Anthropic's bots (independent tokens)

| Token | Purpose |
|-------|---------|
| ClaudeBot | Model training |
| Claude-SearchBot | Search/citations |
| Claude-User | User-initiated fetch |

Blocking `ClaudeBot` does NOT block `Claude-SearchBot` or `Claude-User`.

### Dominant 2026 strategy

Block training bots, allow search/user bots:

```
User-agent: GPTBot
Disallow: /

User-agent: ClaudeBot
Disallow: /

User-agent: Meta-ExternalAgent
Disallow: /

User-agent: CCBot
Disallow: /

User-agent: Google-Extended
Disallow: /

User-agent: OAI-SearchBot
Allow: /

User-agent: Claude-SearchBot
Allow: /

User-agent: PerplexityBot
Allow: /
```

### Content Signals (draft extension)

An IETF draft adds purpose-based directives to robots.txt:
```
Content-Signal: ai-train=no, search=yes, ai-input=yes
```
Not yet honored by major providers.

### Blocking stats (Jul 2026)

From 4,223 robots.txt files: GPTBot blocked by 633 domains, CCBot by
567, ClaudeBot by 563, Google-Extended by 522.

**Sources:**
- [Anagram: AI crawler user-agent list 2026](https://www.anagram.ai/blog/ai-crawler-user-agent-list-2026-14-bots-and-robotstxt-tokens-to-know)
- [DataImpulse: Robots.txt & AI Crawlers 2026](https://dataimpulse.com/blog/robots-txt-ai-crawlers/)
- [OpenHermit: Agent-Allow Strategy 2026](https://www.openhermit.com/blog/robots-txt-ai-agents-2026-strategy)

---

## 3. ai.txt

**Spec:** Proposed by Spawning.ai (2025). Purpose-based crawling
control, distinct from robots.txt's bot-based control.
**Path:** `/ai.txt`.

### Key directives

| Directive | Meaning |
|-----------|---------|
| No-Training | Prohibits using data to train LLMs |
| No-Inference | Prohibits real-time answer use |
| Allow-RAG | Allows retrieval if back-references are provided |

### Adoption

Honored by Spawning.ai partners (Hugging Face, Stability AI). NOT
honored by OpenAI, Anthropic, Google, or Meta. Targets EU TDM
Article 4 exception for legal standing. Currently a niche standard:
include for completeness but don't weight it heavily in an audit.

**Source:**
- [Cookie Script: Beyond Robots.txt](https://cookie-script.com/guides/beyond-robots-txt-implementing-ai-txt-and-llms-txt-for-purpose-based-scraping-control)

---

## 4. MCP Server Card (/.well-known/mcp)

**Spec:** SEP-1649 (Anthropic, Oct 2025), evolving to SEP-2127.
MCP 2026-07-28 revision is current.
**Path:** `/.well-known/mcp/server-card.json` (SEP-1649) or
`/.well-known/mcp` (SEP-1960 manifest endpoint).

### When it applies

A website should expose an MCP server card ONLY if it actually runs
an MCP server. For a marketing/docs site like devcell.sh, this is
irrelevant unless the site exposes tools (search, doc lookup, etc.)
via MCP.

### SEP-1649 server card schema

```json
{
  "$schema": "https://modelcontextprotocol.io/schemas/server-card/v1.0",
  "version": "1.0",
  "protocolVersion": "2025-06-18",
  "serverInfo": {
    "name": "my-server",
    "version": "1.0.0",
    "description": "What this server does",
    "homepage": "https://example.com"
  },
  "transport": {
    "type": "streamable-http",
    "url": "https://example.com/mcp"
  },
  "capabilities": {
    "tools": true,
    "resources": true,
    "prompts": false
  }
}
```

### IETF draft: mcp:// URI scheme

draft-serra-mcp-discovery-uri-04 (expired Sep 2026) defines:
- `/.well-known/mcp-server` well-known URI
- DNS TXT record for DNS-native discovery
- Manifest integrity via JWS with DNS-published keys

### MCP transport (2026-07-28)

Streamable HTTP replaced SSE. Single POST endpoint, stateless
(no session ID, no initialize handshake). Every request is
self-contained. HTTP+SSE is formally deprecated.

**Sources:**
- [MCP Spec: Streamable HTTP](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http)
- [IETF draft-serra-mcp-discovery-uri-04](https://datatracker.ietf.org/doc/draft-serra-mcp-discovery-uri/04/)
- [GitHub MCP Discussion #1147](https://github.com/modelcontextprotocol/modelcontextprotocol/discussions/1147)
- [Ekamoira: MCP Server Discovery Guide](https://www.ekamoira.com/blog/mcp-server-discovery-implement-well-known-mcp-json-2026-guide)
- [Ginger Labs: MCP Server Cards](https://gingerlabs.ai/blog/mcp-server-discovery-well-known-server-cards)

---

## 5. Other agent discovery files

### A2A Agent Card

**Path:** `/.well-known/agent-card.json`
**Spec:** Agent-to-Agent Protocol (Google).
**When it applies:** Only if the website IS an agent or hosts agents.
Not relevant for a docs/marketing site.

### Agent Skills Index

**Path:** `/.well-known/agent-skills/index.json`
**Spec:** Cloudflare RFC v0.2.0.
**When it applies:** Sites that publish reusable skill procedures for
agents (how-to guides, recipes). Could be relevant for a dev-tools
site with rich documentation.

### ARD (Agential Resource Discovery)

**Path:** `/.well-known/ai-catalog.json`
**Spec:** ARD v0.9.
**Purpose:** Unified manifest listing MCP servers, A2A agents, skills,
and APIs.

### auth.md

**Path:** `/auth.md`
**Purpose:** Prose documentation of authentication methods for agent
consumption. Proposal stage.

### AGENTS.md

**Path:** `/AGENTS.md` (in repository root).
**Spec:** Joint proposal by OpenAI (Codex), Amp, Google (Jules),
Cursor, Factory (Aug 2025).
**Purpose:** Give coding agents project context. In-repo, not on the
website.

---

## 6. Cloudflare "Is It Agent Ready?" scorecard

[isitagentready.com](https://isitagentready.com) runs 22 checks across
five axes:

| Axis | Checks |
|------|--------|
| Discoverability | robots.txt, sitemap, Link headers, DNS-AID |
| Content | Markdown content negotiation |
| Bot Access | AI bot rules in robots.txt, Content Signals, Web Bot Auth |
| Protocol | MCP Server Card, Agent Skills, WebMCP, API Catalog, OAuth discovery, OAuth Protected Resource, auth.md, ARD manifest |
| Commerce | x402, MPP, UCP, ACP |

All checks are optional (scored, not required).

---

## 7. Relevance to AIO website audit

### What a website audit should check (ranked by adoption/impact)

| Priority | Signal | Path | Check | Applies to all sites? |
|----------|--------|------|-------|-----------------------|
| 1 | robots.txt | `/robots.txt` | Exists, valid syntax, has AI bot directives (at minimum ClaudeBot, GPTBot, CCBot) | Yes |
| 2 | llms.txt | `/llms.txt` | Exists, valid markdown, H1 first, link lists, under 3K tokens | Yes |
| 3 | Sitemap | `/sitemap.xml` or `/sitemap-index.xml` | Exists, referenced in robots.txt | Yes |
| 4 | OG + meta | HTML `<head>` | og:title, og:description, og:image (1200x630+), canonical, meta description | Yes |
| 5 | llms-full.txt | `/llms-full.txt` | Optional: full content concatenation | Nice to have |
| 6 | ai.txt | `/ai.txt` | Optional: purpose-based directives | Low priority (low adoption) |
| 7 | MCP server card | `/.well-known/mcp/server-card.json` | Only if site runs an MCP server | Conditional |
| 8 | Agent Skills | `/.well-known/agent-skills/index.json` | Only if site publishes agent skills | Conditional |
| 9 | A2A Agent Card | `/.well-known/agent-card.json` | Only if site IS an agent | Conditional |

### For devcell.sh specifically

devcell.sh is a marketing/docs site for an open-source CLI tool. It
should have:

1. **robots.txt** -- already exists. Should add AI bot directives
   (allow search bots, block or allow training bots per preference).
2. **llms.txt** -- does NOT exist. Should create one: H1 "DevCell",
   summary blockquote, links to quickstart, docs, comparison,
   CONTRIBUTING, CHANGELOG, GitHub.
3. **llms-full.txt** -- optional. Could concatenate all docs pages.
4. **sitemap** -- already exists (@astrojs/sitemap).
5. **OG tags** -- exist but OG image is 64x64 (broken, needs
   1200x630).
6. **MCP server card** -- NOT applicable unless devcell.sh exposes an
   MCP endpoint (it doesn't).
7. **ai.txt** -- optional, low priority.
