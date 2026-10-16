# Headline verdict: "Claude Code, without the blast radius" vs "Agentic Coding, without the blast radius"

**Date:** 2026-10-15. Evidence: [headline-community-findings.md](./headline-community-findings.md), [headline-competitor-messaging.md](./headline-competitor-messaging.md). Advisory lenses from opensource-project-advisor.

## Pros and cons

### A. "Claude Code, without the blast radius"

Pros
- Matches the persona's search and tab-close test: `claude code sandbox` is the query; "Claude Code named in hero" is PERSONAS.md check #2. HN: 40 story titles pair "claude code" with sandbox in a year; "agentic coding" + sandbox is ~0.
- Names the thing that caused the pain. Every horror story (home-dir wipe, drive rm -rf, dangerously-skip-permissions) is a Claude Code story. The Anthropic quote in the hero ("Run this in a container") only makes sense next to the product name.
- Rides the largest head term in the category: "claude code" 15x "agentic coding" on HN titles, 29x on GitHub descriptions, Trends peak 6x Copilot.
- Precedent among local wrappers: the Claude-named ones (Trail of Bits 951, textcortex 323) outdraw the generic one (210).
- Share is rising, not falling: 41% -> 55% -> 65.5% in 2026 among agent users.

Cons
- Trademark exposure. Anthropic permits plain-text "runs Claude Code" statements but forbids use "in a way that suggests Anthropic built, endorses, or is partnered with your product." A bare h1 "Claude Code, without the blast radius" under the DevCell name is at the edge. Anthropic forced three renames on OpenClaw in Jan 2026.
- Logo exposure: the "Works with" strip shows Anthropic's logo; logos are the explicitly restricted case.
- Vendor dependence: if Claude Code's share falls below ~50%, the headline ages. Copilot (58.7%) and Codex (29.5%) users read the body copy, not the h1.
- Title-case casing on the site ("Without The Blast Radius") drifts from the spec's sentence case.

### B. "Agentic Coding, without the blast radius"

Pros
- Vendor-neutral; survives a Codex or Copilot swing; no trademark surface.
- Sounds like a category claim, which is what the Oct 10 naming research liked.
- Lets the product name carry "dev" and the tagline carry "agent".

Cons
- "Agentic coding" is not the audience's word; it is the vendor and analyst word (Anthropic's own trends report, Google Cloud, IBM). On HN it headlines skeptic threads ("Agentic Coding Is a Trap", 463 pts). Senior engineers mock it.
- Near-zero search intent for the sandbox job: "agentic coding" + sandbox is 13 loose HN stories and 30 comments in a year.
- Category already claimed by funded players with the same words (Daytona "Run AI Code", E2B "AI Agent Cloud", Docker "Run agents safely"). A 0-star project cannot own a category term; it can own a concrete one.
- Fails the persona's 90-second test #2 unless the body copy rescues it, which costs the one line the h1 is for.
- Loses the pairing with the Anthropic quote.

## Perspective rotation

- **Persona and search (Alex, PERSONAS.md):** A, decisively. The query, the fear, and the quote all say "Claude Code."
- **Discoverability (GITHUB-DISCOVERABILITY.md "front-load keywords"):** A. "claude code" is the head term; "agentic coding" is a tail term owned by vendors.
- **Hykes (Docker):** A, with a caveat. Docker won by naming the concrete thing ("containers"), not the abstraction. But Hykes also keeps his trademark strictly controlled, and so does Anthropic: use their name only in the form they permit.
- **Evan You (Vite):** A now, B later. Vite rode one ecosystem's traffic, then went framework-agnostic once it had pull. Same path: Claude Code headline at 0-800 stars; widen when the comparison table and launch comments show Codex and Copilot users arriving.
- **Hashimoto (Terraform):** B. Vendor neutrality compounds over years and a one-vendor headline caps the ceiling. Weighted lower because the data shows the vendor's share rising, and the ceiling problem is a 2027 problem for a project whose win condition is 300-800 stars in year one.
- **DHH (name it bluntly):** A. "Agentic coding" is the phrase a product-company page uses; "Claude Code, without the blast radius" is the phrase a senior engineer repeats to a colleague. The HN launch line ("Claude Code with the lights on") is already vendor-specific.
- **Mitchell (licensing and trademark hygiene):** A only in the permitted form. The legal page draws a line at "says it runs Claude Code" vs "suggests endorsement." Choose wording that is unambiguously the former and add a non-affiliation line.

Agreement: six of seven lenses pick A for this stage. Disagreement: Hashimoto on ceiling, Mitchell on wording. Both are answered by the recommendation below rather than by switching to B.

## Recommendation

1. Keep Claude Code in the headline. B is weaker on every audience-facing axis and only wins on risk axes that A can mitigate.
2. Move the h1 to Anthropic's permitted form so it reads as a statement of what devcell runs, not as a product line: "Run Claude Code without the blast radius." Keep the next sentence naming Codex, OpenCode, and Gemini.
3. Add a one-line footer: "Not affiliated with Anthropic, OpenAI, or Google. Claude Code, Codex, and Gemini are their owners' trademarks."
4. Decide on the "Works with" logo strip: swap to text wordmarks, or keep logos with the footer line. Logos are the explicitly restricted case; this is the user's call.
5. Sentence case everywhere ("without the blast radius"), matching README, page title, and LANDING.md.
6. Revisit triggers: Claude Code under 50% in the next Stack Overflow pulse; HN launch comments where Codex or Copilot users say the page is not for them; any contact from Anthropic about the name.

No copy was changed as part of this research; see LANDING.md changes log 2026-10-15.
