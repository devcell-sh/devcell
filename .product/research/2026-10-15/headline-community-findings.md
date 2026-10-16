# Headline research: "Claude Code" vs "Agentic Coding" (community + demand signals)

**Date:** 2026-10-15
**Question:** Which hero headline fits devcell: "Claude Code, without the blast radius" (A) or "Agentic Coding, without the blast radius" (B)?
**Method:** market-research/community, scoped to vocabulary, demand proxies, competitor taglines, vendor-lock perception. Influencer mapping skipped (out of scope). Reddit JSON and `site:reddit.com` were blocked; Reddit evidence comes via secondary sources.

## Demand proxies

HN (Algolia API, stories created in the last 365 days, title-only match):

| Phrase | Story titles | Loose stories | Loose comments |
|---|---|---|---|
| "claude code" | 3,887 | 6,188 | 22,870 |
| "coding agents" | 1,585 | | |
| "agentic coding" | 257 | 391 | 2,436 |
| "claude code" + sandbox | 40 | 188 | 467 |
| "agentic coding" + sandbox | ~0 | 13 | 30 |
| "blast radius" + agent | | 38 | 215 |
| dangerously-skip-permissions | | 39 | 269 |

GitHub repository search (text match, 2026-10-15):

| Query | Repos |
|---|---|
| "claude code" in name/description | 254,790 |
| topic:claude-code | 92,396 |
| "claude code" sandbox (name/desc/readme) | 86,343 |
| "agentic coding" sandbox (name/desc/readme) | 13,330 |
| "agentic coding" in name/description | 8,743 |
| topic:agentic-coding | 2,459 |

Stack Overflow 2026 Developer Survey, coding agents among agent users (survey.stackoverflow.co/2026/ai/data): Claude Code 65.5%, GitHub Copilot 58.7%, OpenAI Codex 29.5%, Cursor 25.5%, Google Antigravity 16.0%, Gemini Code Assist 14.6%, OpenCode 13.6%. Claude Code trend: 41% (2025) -> 55% (Apr 2026 pulse) -> 65.5% (Oct 2026). Intent to keep using next year: Anthropic 49% vs OpenAI 32%.

Google Trends (secondary, zentor.ai): "Claude Code" went from 0 to a normalized 100 between Apr 2025 and Feb 2026, 6.25x GitHub Copilot's five-year peak. No published volume for "agentic coding"; the HN title ratio above (15:1) is the best proxy.

Wikipedia pageviews 2026-09-01..10-14: Vibe_coding 73,928; AI_agent 55,015; GitHub_Copilot 13,678; Claude_Code 4,120; Agentic_AI 3,688; OpenAI_Codex 3,107. Lay audiences say "vibe coding"; neither headline candidate is a lay term.

## How the audience talks

- "Blast radius" is native HN vocabulary in sandbox threads, not a marketing coinage. HN 46694641: "what we all want is to run agents in a locked-down sandbox where the blast radius of mistakes and/or prompt injection attacks is minimal/acceptable." Also 47185250, 47184049, 49925969, 49114250.
- Horror stories name the tool, never the category: r/ClaudeAI "Claude CLI deleted my entire home directory" (Dec 2025, amplified by Simon Willison on X), Aug 2026 rm -rf of a drive, GitHub issues #10077 and #12637. The persona's precipitating event is a Claude Code event.
- "Agentic coding" on HN is the skeptic's word: "Agentic Coding Is a Trap" (463 pts), "Ask HN: Do you have any evidence that agentic coding works?" (461 pts), "Beyond agentic coding" (269). Vendors and analysts own the term: Anthropic's "2026 Agentic Coding Trends Report", Google Cloud "What is agentic coding?", Sourcegraph, IBM.
- Senior-dev framing: "Professional Software Developers Don't Vibe, They Control" (arXiv 2512.14012). They retain control; the sandbox is the control.

## Vendor-lock perception

- Pattern in the wild: tools launch Claude-named, then add Codex and Gemini. Axon (HN 46954765): "supports codex and gemini cli as well as claude code now." StoreClaudeWork issue #1: "Support Codex, Cursor and Gemini CLI." Omnara Launch HN title: "Run Claude Code and Codex from anywhere" (147 pts).
- Pushback appears as feature requests, not as bounces. "vendor lock" + "claude code": 30 HN comments in a year. No thread found where a user rejected a sandbox tool because its headline named Claude Code.
- Risk direction: Claude Code share rose through 2026, so vendor-dependence risk is falling this year. It becomes real only if the share drops.

## Legal and brand constraint (new, decisive for wording)

Anthropic, Claude Code "Legal and compliance" (code.claude.com/docs/en/legal-and-compliance), section "Can customers offer Claude Code in their products?":

> You can accurately say, in plain text, that your product has Claude Code preinstalled or that it runs Claude Code. But you can't use the Claude Code or Anthropic names or logos as part of your own product, feature, or company name, in your own logo, or in a way that suggests Anthropic built, endorses, or is partnered with your product. Any other use of Anthropic's names or logos is governed by our Trademark Guidelines and requires our written permission.

Same page: running Claude Code "in hosted sandboxes or other agent infrastructure" requires the Commercial Terms, an unmodified binary, and each end user authenticating with their own credentials. devcell complies (unmodified binary, user's own login). The page also states a subscription sign-in to the unmodified binary is permitted "where a platform hosts Claude Code", which covers the "Claude Max/Pro works" claim.

Implications for the hero:
- Headline A as a standalone h1 under the DevCell name is plain-text nominative use, but it reads like a product line and could be argued to "suggest" endorsement. The literal permitted form is "runs Claude Code": e.g. "Run Claude Code without the blast radius."
- The "Works with" strip uses Anthropic's logo. Logo use is the explicitly restricted case. Either replace with a text wordmark or pair with a non-affiliation line.
- Precedent: Anthropic forced Clawd -> Moltbot -> OpenClaw renames in Jan 2026 over "Claude"-derived naming.

## Sources

- https://hn.algolia.com/api (queries above)
- https://api.github.com/search/repositories (queries above)
- https://survey.stackoverflow.co/2026/ai/data
- https://stackoverflow.blog/2026/10/06/the-results-of-the-2026-developer-survey-are-here/
- https://zentor.ai/blog/cursor-vs-copilot-vs-claude-code-trends
- https://news.ycombinator.com/item?id=46694641 , 47185250, 47184049, 49925969, 49114250, 46954765
- https://github.com/janbrogger/StoreClaudeWork/issues/1
- https://x.com/simonw/status/1998447540916936947
- https://explainx.ai/blog/claude-code-rm-rf-reddit-incident-sandbox-guide-august-2026
- https://arxiv.org/html/2512.14012v1
- https://code.claude.com/docs/en/legal-and-compliance
- https://en.wikipedia.org/wiki/OpenClaw
