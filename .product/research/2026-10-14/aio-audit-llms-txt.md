# llms.txt, robots.txt for AI, and MCP discovery: research for AIO website audit

Research date: 2026-10-14. Sources listed at bottom.

---

## 1. llms.txt

### What it is

A plain-text Markdown file at a website's root (`/llms.txt`) that gives LLM
agents a curated map of a site's most important resources. Think of it as a
table of contents for AI: robots.txt says what crawlers *may* access;
sitemap.xml lists *every* URL; llms.txt lists *what matters* and why.

Created by Jeremy Howard (Answer.AI), published September 3, 2024.
Updated to v2 in August 2026 (v2 permits subpaths like `/docs/llms.txt`).

### Spec status

Convention/proposal at llmstxt.org, not an IETF RFC or W3C standard.
Described as "the first widely-adopted B2A (Business-to-Agent) standard."
No major search provider treats it as a ranking signal. Google has said on
the record that AI Overviews require no special AI file.

### File format

Valid CommonMark Markdown. Only the H1 is required.

```markdown
# Project Name

> One-paragraph summary of what this site/project does.

Optional body text: context, scope, conventions.

## Section Name

- [Page title](https://example.com/page.md): One-line description
- [Another page](https://example.com/other.md): Description

## Optional

- [Less important page](https://example.com/extra.md)
```

Rules:
- File named exactly `llms.txt` (not `llm.txt`)
- Served at root (`/llms.txt`) or a subpath (`/docs/llms.txt`, v2)
- Accessible over HTTPS
- Small enough to fit in an LLM context window (1-10 KB, ~250-2500 tokens)
- Pages listed SHOULD offer a clean `.md` version at the same URL + `.md`
- HTML pages SHOULD include `rel="alternate" type="text/markdown"` link
  and `rel="describedby"` pointing to the llms.txt file

### llms-full.txt

A companion file containing the actual text of listed pages, concatenated
and cleaned of navigation chrome.

| Aspect | llms.txt | llms-full.txt |
|--------|----------|---------------|
| Purpose | Navigation map | Complete content |
| Content | Links + one-line descriptions | Full page text |
| Size | 1-10 KB | 100 KB - several MB |
| Tokens | ~250-2,500 | ~25,000-1,000,000+ |
| Best for | Marketing sites, small sites | API docs, knowledge bases |

Ship only llms.txt for small/marketing sites. Ship both for docs-heavy
sites where accuracy in specifics matters.

### Adoption

Sites with llms.txt (confirmed):
- Anthropic: `platform.claude.com/docs/llms.txt` (763 pages indexed, also
  advertises its MCP server and llms-full.txt)
- Stripe: `stripe.dev/llms.txt` (200+ blog posts indexed)
- Cloudflare, Mintlify, Fern: adopted per multiple sources
- ~10% adoption across 300K domains studied (as of Q1 2026)

### How agents consume it

Primary real-world consumers are **developer tooling** (Cursor, Claude Code,
Windsurf, coding assistants). The flow:
1. Agent fetches `/llms.txt` (milliseconds, few tokens)
2. Parses the Markdown to find relevant section/link
3. Fetches the specific `.md` page it needs
4. Or fetches `llms-full.txt` for complete context in one shot

Crawlers (GPTBot, ClaudeBot, etc.) do NOT generally request llms.txt.
Its value is at inference time, not training time.

### Criticism

- No enforcement: purely advisory, like robots.txt
- No major LLM provider has confirmed reading it in production systems
- Google says it's unnecessary for AI search inclusion
- Risk of becoming an SEO-gaming vector (keyword stuffing in descriptions)
- Maintenance burden: must stay in sync with actual content

---

## 2. robots.txt for AI crawlers (2026 state)

### The training vs. search split

The critical 2025-2026 development: AI companies now run separate crawlers
for training vs. search/retrieval. You can block training while staying
eligible for AI search citations.

| Crawler | Operator | Purpose | Honors robots.txt |
|---------|----------|---------|-------------------|
| GPTBot | OpenAI | Training | Yes |
| OAI-SearchBot | OpenAI | ChatGPT search | Yes |
| ClaudeBot | Anthropic | Training | Yes |
| Claude-SearchBot | Anthropic | Search/citations | Yes |
| Claude-User | Anthropic | User-initiated fetch | Yes |
| PerplexityBot | Perplexity | Search/answers | Yes |
| Meta-ExternalAgent | Meta | Llama training | Yes |
| Google-Extended | Google | Gemini/Vertex training | Yes |
| Bytespider | ByteDance | Training | Inconsistent |

### Scale

- 22% of all non-search bot traffic in Q1 2026 came from AI crawlers
- Up to 72% of AI crawlers violate robots.txt rules
- 21% of top 1000 websites have GPTBot rules
- 2.5M+ websites block AI training via Cloudflare

### Recommended strategy

```
# robots.txt: block training, allow search
User-agent: GPTBot
Disallow: /

User-agent: Google-Extended
Disallow: /

User-agent: ClaudeBot
Disallow: /

User-agent: Meta-ExternalAgent
Disallow: /

User-agent: OAI-SearchBot
Allow: /

User-agent: Claude-SearchBot
Allow: /

User-agent: PerplexityBot
Allow: /
```

Key insight: all three files (robots.txt, llms.txt, ai.txt) depend on
crawler goodwill. The only layer with teeth is WAF/CDN enforcement.

### ai.txt

A root-level file expressing training opt-out, tied to Spawning.ai's
Do-Not-Train (DNT) registry. Honored by Stability AI and Hugging Face.
Less relevant for a dev-tool landing page; primarily for content/media sites.

---

## 3. MCP route: /.well-known/ai-catalog.json and Server Cards

### What happened

The MCP ecosystem went through several proposals for web discovery:

1. **SEP-1649** (Oct 2025, draft): proposed `/.well-known/mcp.json`.
   Now superseded and dead.
2. **SEP-2127** (Jan 2026, now Final as of Oct 6 2026): MCP Server Cards.
   The winning proposal.
3. **IETF draft-serra-mcp-discovery-uri-04**: proposed `mcp:` URI scheme +
   `/.well-known/mcp-server`. Expired Sep 2026.

### SEP-2127: the merged standard

**Status:** Final (merged October 6, 2026).
**Authors:** David Soria Parra (Anthropic), Sam Morrow (GitHub),
Tadas Antanavicius. Server Card Working Group.

Two-level discovery:

1. **AI Catalog** at `/.well-known/ai-catalog.json`:
   Domain-level discovery. A JSON document listing all MCP servers available
   on this domain. Protocol-agnostic. Can link to or embed Server Cards.

2. **Server Card** at `<streamable-http-url>/server-card`:
   Per-server metadata. Identity (name, version, description), transport
   endpoints, supported protocol versions. No primitives (tools/resources)
   by design: those are dynamic and discovered at runtime via `tools/list`.

### What a Server Card contains

From the schema (TypeScript source of truth in ext-server-card repo):
- `name`, `version`, `description` (required)
- `title`, `icons`, `repository`, `websiteUrl` (optional)
- Transport endpoints (URLs, headers, variable templates, protocol versions)
- Namespaced `_meta` for non-standard data
- Does NOT include: tools, resources, prompts, capabilities, credentials

### What it does NOT do

- No tool advertisement (intentionally excluded: tools are per-user/session)
- No capability negotiation (that happens at runtime)
- No authentication details (cards are public and read-only)

### Implementation status

"Final" SEP, but the wire format repo (`experimental-ext-server-card`) README
still says "Experimental" and SDK PRs (Python, Go) are unmerged as of
Oct 8, 2026. Treat the decision as made and the details as still moving.

### Real-world example

Anthropic's own docs llms.txt includes:
```
## MCP server
Connect an MCP client to https://platform.claude.com/docs/mcp
(Streamable HTTP transport) to search and read these docs. The server
card is at https://platform.claude.com/docs/mcp/server-card.
```

---

## 4. Relevance to AIO website audit

### Signals for the website sub-skill

These three standards create new audit signals for a website:

| Signal | Surface | Status criteria |
|--------|---------|-----------------|
| `/llms.txt` exists | Rank 1 (search/discovery) | ok: exists, has H1, blockquote, links to key pages with `.md` versions |
| `/llms-full.txt` exists | Rank 1 | ok for docs-heavy sites; optional for marketing sites |
| `robots.txt` AI crawler rules | Rank 1 (crawl control) | ok: explicit rules for GPTBot, ClaudeBot, Google-Extended; training/search split |
| `/.well-known/ai-catalog.json` | Rank 2 (API/agent discovery) | ok: exists if site exposes any MCP server; n/a otherwise |
| MCP server card | Rank 2 | ok: if site has an MCP endpoint, card at `<url>/server-card` |
| `.md` versions of key pages | Rank 2 (content for agents) | ok: at least quickstart and main docs have `.md` alternates |
| `rel="describedby"` link header | Rank 4 (installed copy) | ok: HTML pages link to llms.txt |

### For devcell.sh specifically

- **llms.txt**: should exist. Small marketing site + docs. List: homepage,
  quickstart, configuration reference, comparison, key guides. Point to
  `.md` versions of docs pages (Astro content collection already has them).
- **llms-full.txt**: optional but valuable. Docs are the primary content.
  Concatenate all docs into one file for agents.
- **robots.txt**: already exists. Should add explicit AI crawler rules
  (the training/search split strategy above).
- **MCP route**: only relevant if devcell.sh exposes an MCP server for its
  docs (it doesn't currently). Not applicable yet. If the docs get an MCP
  endpoint (like Anthropic's), then `/.well-known/ai-catalog.json` and a
  server card would apply.

---

## Sources

- [llmstxt.org - Official specification](https://llmstxt.org/)
- [Bluehost: What is llms.txt? (2026 Guide)](https://www.bluehost.com/blog/what-is-llms-txt/)
- [LLM Pulse: llms.txt Complete 2026 Guide](https://llmpulse.ai/blog/llms-txt-guide/)
- [Limy.ai: LLMs.txt in 2026](https://limy.ai/blog/llms-txt-in-2026-the-full-guide)
- [DEV Community: llms.txt vs llms-full.txt (2026)](https://dev.to/lab451/llmstxt-vs-llms-fulltxt-whats-the-difference-2026-3lhl)
- [llms-txt.io: Do You Need Both?](https://llms-txt.io/blog/llms-txt-and-llms-full-txt)
- [Coronium: robots.txt vs llms.txt vs ai.txt in 2026](https://www.coronium.io/blog/robots-txt-llms-txt-ai-txt-2026)
- [Paul Calvano: AI Bots and Robots.txt (2025)](https://paulcalvano.com/2025-08-21-ai-bots-and-robots-txt/)
- [DataImpulse: Robots.txt & AI Crawlers in 2026](https://dataimpulse.com/blog/robots-txt-ai-crawlers/)
- [OpenHermit: Robots.txt for AI Agents 2026](https://www.openhermit.com/blog/robots-txt-ai-agents-2026-strategy)
- [MCP SEP-2127: Server Cards (Final)](https://modelcontextprotocol.io/seps/2127-mcp-server-cards)
- [MCP SEP-1649: Original .well-known proposal](https://github.com/modelcontextprotocol/modelcontextprotocol/issues/1649)
- [Colin Knapp: MCP Discovery via Well-Known URI](https://colinknapp.com/specs/mcp-discovery.html)
- [MCP Blog: 2026-07-28 Specification](https://blog.mcpservers.org/posts/mcp-spec-2026-07-28)
- [Anthropic llms.txt](https://platform.claude.com/docs/llms.txt)
