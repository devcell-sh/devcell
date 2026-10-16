# AI Discoverability Standards Research: robots.txt, llms.txt, MCP Routes -- 2026-10-14

Research for the `aio-audit` website sub-skill. Covers what a website
needs to be findable, citable, and usable by AI agents and LLM search.

## 1. Known AI crawler user-agents

Five functional categories. An AIO audit checks that the right ones
are allowed, not that all are blocked.

### Training crawlers (scrape for model improvement)

| Bot | Company | User-Agent Token | Notes |
|-----|---------|-----------------|-------|
| GPTBot | OpenAI | `GPTBot` | Most blocked. 1,255:1 crawl-to-refer ratio |
| ClaudeBot | Anthropic | `ClaudeBot` | Supports `Crawl-delay`. Replaced deprecated `Claude-Web`, `Anthropic-AI` |
| Google-Extended | Google | `Google-Extended` | Not a real crawler: a robots.txt-only opt-out token controlling Gemini training. Never appears in logs |
| Applebot-Extended | Apple | `Applebot-Extended` | Same pattern as Google-Extended: opt-out token only |
| Meta-ExternalAgent | Meta | `meta-externalagent` | Multi-product training |
| CCBot | Common Crawl | `CCBot` | Open corpus |
| Amazonbot | Amazon | `Amazonbot` | Nova LLM + Alexa |
| Bytespider | ByteDance | (spoofs UAs) | Undeclared. Ignores robots.txt. WAF-only |

### Search/retrieval crawlers (index for AI-powered search answers)

| Bot | Company | User-Agent Token | Notes |
|-----|---------|-----------------|-------|
| OAI-SearchBot | OpenAI | `OAI-SearchBot` | ChatGPT web search index |
| Claude-SearchBot | Anthropic | `Claude-SearchBot` | Claude web search index. Launched May 2026 |
| PerplexityBot | Perplexity | `PerplexityBot` | Answer generation |
| Google-CloudVertexBot | Google | `Google-CloudVertexBot` | Vertex AI agent retrieval |
| DuckAssistBot | DuckDuckGo | `DuckAssistBot` | AI-assisted answers |
| Bingbot | Microsoft | `bingbot` | Bing + Copilot |

### User-triggered fetchers (fetch during a live user conversation)

| Bot | Company | User-Agent Token | robots.txt |
|-----|---------|-----------------|------------|
| ChatGPT-User | OpenAI | `ChatGPT-User` | May not apply |
| Claude-User | Anthropic | `Claude-User` | Respects |
| Perplexity-User | Perplexity | `Perplexity-User` | Claims to respect |
| MistralAI-User | Mistral | `MistralAI-User` | Respects |
| Google-Agent | Google | Standard Chrome UA | **Ignores** robots.txt |

### Key insight for AIO

A site wanting to be **found** by AI agents should explicitly allow
search and user-triggered crawlers, not just rely on `User-agent: *`.
Under RFC 9309, a named group replaces the wildcard entirely: it does
not inherit from `*`.

## 2. robots.txt best practices for AI discoverability

### The "Agent-Allow" strategy (recommended for OSS projects wanting visibility)

```
# -- Search/retrieval: ALLOW (these return traffic) --
User-agent: OAI-SearchBot
Allow: /

User-agent: Claude-SearchBot
Allow: /

User-agent: PerplexityBot
Allow: /

User-agent: ChatGPT-User
Allow: /

User-agent: Claude-User
Allow: /

# -- Training: site owner's choice --
# Allow training = more chance of being in the model's weights
# Block training = protect content, keep search visibility
User-agent: GPTBot
Allow: /        # or Disallow: / to block training

User-agent: ClaudeBot
Allow: /        # or Disallow: /

User-agent: Google-Extended
Allow: /        # or Disallow: /

User-agent: CCBot
Disallow: /     # usually block (open corpus, no attribution)

# -- Standard search --
User-agent: *
Allow: /

Sitemap: https://example.com/sitemap-index.xml
```

For an open-source project that **wants** maximum AI visibility,
allowing all crawlers (including training) is the simplest choice.
The project's code is already public on GitHub anyway.

### Additional robots.txt signals

- **`Sitemap:` directive**: always include. All AI crawlers use it.
- **`Crawl-delay:`**: supported by ClaudeBot. Not standard but respected.
- **Agentmap line** (new, 2026): `Agentmap: /ai-catalog.json` --
  signals Agentic Resource Discovery to agents. Part of Google's
  Agentic Resource Discovery spec (Jun 2026).

## 3. llms.txt

### What it is

A Markdown file at `/llms.txt` giving AI agents a curated index of a
site's most important content. Proposed by Jeremy Howard (Answer.AI)
in Sep 2024. By 2026, widely adopted by dev-tool sites.

### Who consumes it

- **IDE agents**: Cursor, Windsurf, Claude Code, GitHub Copilot, Cline,
  Aider all look for `/llms.txt` and `/llms-full.txt`
- **Anthropic**: explicitly recommends it in "Writing for Agents" guidance
- **OpenAI**: maintains llms.txt for Agents SDK
- **Google Lighthouse 13.3** (May 2026): added "Agentic Browsing" audit
  category that checks for `/llms.txt`. Marks sites without it.

### Format

```markdown
# Project Name

> One-paragraph summary of what this project does and who it's for.

Additional context paragraphs go here.

## Docs

- [Quickstart](/docs/quickstart): Get started in 2 minutes
- [Configuration](/docs/config): All TOML keys and env vars
- [CLI Reference](/docs/cli): Every command and flag

## Examples

- [Basic usage](/examples/basic): Minimal setup
- [CI/CD](/examples/ci): GitHub Actions integration

## Optional

- [Architecture](/docs/architecture): Internal design decisions
- [Contributing](/contributing): How to contribute
```

Rules:
- H1 heading is the only required element
- Blockquote summary follows the H1
- H2 sections contain link lists: `- [title](url): description`
- "Optional" section convention: content agents can skip in short contexts
- Keep it 10-50 curated links, not 500

### llms-full.txt

Convention (not formally in the spec): a concatenated version of all
content linked from `llms.txt`. Useful for small sites where the
entire docs fit in a single context window. Some doc platforms
(Mintlify, Fern) auto-generate it.

### Relationship to robots.txt

Different purposes. robots.txt controls **access**. llms.txt curates
**content**. Both should be present. A site with a good llms.txt but
`Disallow: /` for all AI crawlers is contradictory.

## 4. MCP route / .well-known/mcp.json

Two competing MCP discovery proposals (may merge):

### SEP-1649: Server Cards

Path: `/.well-known/mcp/server-card.json`

```json
{
  "$schema": "https://modelcontextprotocol.io/schemas/server-card/v1.0",
  "version": "1.0",
  "protocolVersion": "2025-06-18",
  "serverInfo": {
    "name": "My MCP Server",
    "version": "1.2.0",
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

### SEP-1960: Manifest Endpoint

Path: `/.well-known/mcp`

```json
{
  "mcp_version": "2025-11-25",
  "endpoints": [
    {
      "url": "https://example.com/mcp",
      "transport": "streamable-http",
      "capabilities": ["tools", "resources"],
      "auth": {
        "type": "oauth2",
        "authorization_server": "https://example.com/.well-known/oauth-authorization-server"
      }
    }
  ]
}
```

### When to use

Only relevant if the website itself **serves** an MCP endpoint (e.g.,
a docs site with a query tool, or a product with an API). Not needed
for a static marketing/docs site unless it exposes tools.

## 5. Other discovery standards

### agents.json (Wildcard AI)

Path: `/.well-known/agents.json`. Extends OpenAPI to describe multi-step
workflows agents can execute. Relevant for SaaS/API products, not
static sites.

### agent.json (Google A2A)

Path: `/.well-known/agent.json`. The A2A protocol's "agent card":
discovery document for agent-to-agent communication. Only relevant if
the site IS an agent service.

### Agentic Resource Discovery / ai-catalog.json (Google, Jun 2026)

A unified catalog spec. Four discovery mechanisms:
1. `/.well-known/ai-catalog.json`
2. `Agentmap:` line in robots.txt
3. `<link rel="ai-catalog">` in HTML `<head>`
4. DNS Service Binding record

Backed by Google, Microsoft, GitHub, Hugging Face, Nvidia, Salesforce.
Early but significant. Relevant for sites with APIs or tools to expose.

### TDM Reservation Protocol (W3C, EU DSM Directive)

Three mechanisms to declare text/data mining rights:
1. HTTP header: `TDM-Reservation: 1`
2. HTML meta tag: `<meta name="tdm-reservation" content="1">`
3. JSON file: `/.well-known/tdmrep.json`

Legally significant in the EU under DSM Directive Art. 4(3). The AI Act
(Art. 53(1)(c)) requires GPAI providers to identify and comply with
these reservations. 23 providers (including OpenAI, Google, Anthropic)
signed the GPAI Code of Practice committing to honor them.

For an open-source project: usually NOT relevant (you want training).
For a commercial site protecting proprietary content: essential.

### ai.txt (various proposals)

Multiple competing proposals (Guardian, Spawning, menro/GitHub). None
standardized. Intent: purpose-based permissions (allow RAG, deny
training, require attribution). The W3C TDMRep and the IETF Content-
Signal drafts serve the same purpose with more institutional backing.
Skip ai.txt in favor of TDMRep if purpose-based control is needed.

## 6. For AIO audit: what a website needs (checklist)

### Tier 1: Essential (every site)

| Signal | File/Location | Purpose |
|--------|--------------|---------|
| robots.txt | `/robots.txt` | Allow search crawlers. Include `Sitemap:` |
| Sitemap | `/sitemap.xml` or `/sitemap-index.xml` | Content discovery for all crawlers |
| Meta tags | `<head>` | `description`, `og:*`, `twitter:*`, canonical |

### Tier 2: High value (2026 standard for dev/OSS sites)

| Signal | File/Location | Purpose |
|--------|--------------|---------|
| llms.txt | `/llms.txt` | Curated content index for AI agents |
| llms-full.txt | `/llms-full.txt` | Full concatenated docs (optional) |
| Structured data | JSON-LD in `<head>` | `SoftwareApplication`, `WebSite` schema |
| Per-page .md | Content negotiation or `.md` suffix | Clean markdown for agent consumption |

### Tier 3: Forward-looking (adopt when relevant)

| Signal | File/Location | Purpose |
|--------|--------------|---------|
| .well-known/mcp.json | `/.well-known/mcp/server-card.json` | MCP server discovery (only if you serve MCP) |
| ai-catalog.json | `/.well-known/ai-catalog.json` | Agentic Resource Discovery (if you have APIs/tools) |
| agents.json | `/.well-known/agents.json` | API workflow discovery (if you have APIs) |
| TDMRep | `/.well-known/tdmrep.json` | TDM rights (EU, if you need to restrict training) |

### What NOT to do (anti-patterns)

- Block all AI crawlers when you want visibility
- Add `llms.txt` but `Disallow: /` for AI search crawlers
- Use `User-agent: *` and assume it covers named AI bots (RFC 9309:
  a named group replaces the wildcard)
- Add keyword-stuffed hidden text "for AI agents"
- Skip `Sitemap:` directive (all AI crawlers use it for discovery)

## Sources

- [AI Crawlers Explained (Anagram, 2026)](https://www.anagram.ai/blog/ai-crawlers-explained-gptbot-claudebot-perplexitybot-and-how-to-let-them-in-2026)
- [AI User-Agent Landscape 2026 (NoHacks)](https://nohacks.co/blog/ai-user-agents-landscape-2026)
- [Robots.txt AI Crawlers Full Guide (DataImpulse)](https://dataimpulse.com/blog/robots-txt-ai-crawlers/)
- [Agent-Allow Strategy (OpenHermit)](https://www.openhermit.com/blog/robots-txt-ai-agents-2026-strategy)
- [Publishers Now Block Training Bots (TechnologyChecker)](https://technologychecker.io/blog/robots-txt-ai-crawlers-blocking-report)
- [llms.txt Specification (llmstxt.org)](https://llmstxt.org)
- [llms.txt 2026 Full Guide (Limy)](https://limy.ai/blog/llms-txt-in-2026-the-full-guide)
- [llms.txt Google Guidance (GetPassionfruit)](https://www.getpassionfruit.com/blog/should-i-create-an-llms.txt-file-google-s-2026-guidance-explained)
- [What is llms.txt (Mintlify)](https://www.mintlify.com/blog/what-is-llms-txt)
- [MCP Server Discovery Guide (Ekamoira)](https://www.ekamoira.com/blog/mcp-server-discovery-implement-well-known-mcp-json-2026-guide)
- [Agent Readiness Spec (specification.website)](https://specification.website/spec/agent-readiness/)
- [agents.json (Wildcard AI)](https://github.com/wild-card-ai/agents-json)
- [Agent Discovery Protocols (AgentsWelcome)](https://agentswelcome.dev/protocols)
- [Agentic Resource Discovery (Solo.io)](https://www.solo.io/blog/agent-discovery-naming-and-resolution---the-missing-pieces-to-a2a)
- [TDM Reservation (Shifter)](https://shifter.io/blog/robots-txt-ai-opt-outs-reservation-signals)
- [TDMRep and AI.txt (ScrapingBee)](https://www.scrapingbee.com/blog/tdmrep-ai-txt-scraping-controls/)
- [AI Standards 2026 (DEV/Astro)](https://dev.to/astro-official/new-ai-web-standards-and-scraping-trends-in-2026-rethinking-robotstxt-3730)
- [Anthropic ClaudeBot Framework (SearchEngineLand)](https://searchengineland.com/anthropic-claude-bots-470171)
- [Google-Extended Guide (PushLeads)](https://pushleads.com/google-extended-crawler/)
- [Robots.txt & AI Opt-Outs (Shifter)](https://shifter.io/blog/robots-txt-ai-opt-outs-reservation-signals)
- [Agent Readiness Discoverability (AgentsWelcome)](https://agentswelcome.dev/agent-readiness/discoverability)
- [Website Agent-Ready Guide (Suganthan)](https://suganthan.com/blog/how-to-make-website-agent-ready/)
- [agent.json Project](https://agent-json.com/)
