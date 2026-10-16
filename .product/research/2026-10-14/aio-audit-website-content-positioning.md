# Content & Positioning Audit -- 2026-10-14

Checked: `web/src/` source files, accessibility snapshot, and `.product/` specs.

## P0 Status (broken commands/links from Oct 13 audit)

- [x] Fixed: #1 Hero.astro GitHub URLs -> `devcell-sh/devcell`
- [x] Fixed: #2 Shields.io badges -> `devcell-sh/devcell`
- [x] Fixed: #3 Terminal shows `devcell claude` (was `cell claude`)
- [x] Fixed: #4 "Opening cell myproject" -- still says "Opening cell" but `cell` is also the runtime entity name, not just the old CLI name, so acceptable
- [x] Fixed: #5 Quickstart.astro -> `brew install devcell-sh/tap/devcell`
- [x] Fixed: #6 Quickstart.astro -> `devcell claude`, `devcell codex`, `devcell opencode`
- [x] Fixed: #7 FeatureCards -- old `cell login` card is gone; no login card exists anymore
- [x] Fixed: #8 WhatsInside.astro -> `github.com/devcell-sh/home`
- [ ] Still broken: #9 StackTable.astro:42 -- `public.ecr.aws/w1l3v2k8/devcell` should be `ghcr.io/devcell-sh/devcell`. WhatsInside.astro (line 58) correctly says `ghcr.io`, but StackTable.astro is stale. **Note:** StackTable.astro is not imported in index.astro -- it appears to be dead code (WhatsInside.astro replaced it). Still should be fixed or deleted.
- [x] Fixed: #10 Base.astro nav/footer GitHub links -> `devcell-sh`
- [x] Fixed: #11 quickstart.md -- DimmKirr gone (grep confirms clean)
- [x] Fixed: #12-14 docs content -- no DimmKirr references remain
- [x] Fixed: #15a Gemini added to quickstart agent list

## P1-high Status

- [x] Fixed: #16 Hero heading -> "Claude Code, Without The Blast Radius." (Hero.astro:13)
- [x] Fixed: #16a Hero body copy -> matches target: "local-first sandbox for AI coding agents" (Hero.astro:14-18)
- [x] Fixed: #17 Page title -> "DevCell -- Claude Code, without the blast radius" (index.astro:823)
- [x] Fixed: #18 Meta description -> "Local-first sandbox for AI coding agents..." (Base.astro:11, index.astro:823)
- [x] Fixed: #26 Egress lockdown card exists (FeatureCards.astro:14-16)
- [x] Fixed: #27 "No account, no login" card exists (FeatureCards.astro:18-21)

## P1-mid Status

- [ ] Partial: #19 CompareSection.astro still compares vs Dev Containers + OpenClaw. However, it is NOT imported in index.astro. The live comparison table in index.astro uses Docker Sandboxes, Daytona, Microsandbox -- mostly correct but see below.
- [x] Fixed: #20 Stack columns updated to base, ultimate, bbb, electronics (index.astro:13)
- [x] Fixed: #23 "session handoff via devcell auth chrome" in stackFeatures (index.astro:18)
- [x] Fixed: #25 Terminal mock shows "Opus 4.6" (Hero.astro:57)
- [x] Fixed: #28 No "browser login" terminology in feature cards
- [x] Fixed: #29a FAQ #4 mentions `--solitary` for egress lockdown (index.astro:56)
- [x] Fixed: #29b FAQ #3 references Docker Sandboxes, not Dev Containers (index.astro:54)
- [x] Fixed: #29c FAQ -- no OpenClaw reference in FAQ items
- [x] Fixed: #29d FAQ #2 references `devcell-sh/home` modules, not "nixhome" (index.astro:52)

## P1-low Status

- [x] Fixed: #21 MCP count says "11 servers" not "12+" (index.astro:22, FeatureCards.astro:26)
- [x] Fixed: #22 IceWM in stack features (index.astro:19)

## Comparison Table: Daytona vs E2B

**Issue:** LANDING.md spec says competitors should be "Docker Sandboxes, E2B, Microsandbox" (Daytona archived Oct 2026). The live site uses **Daytona** instead of **E2B**.

- index.astro:36-46 comparison data uses `daytona` column
- index.astro:839 column header says "Daytona"
- index.astro:858 guidance paragraph mentions Daytona

**Action needed:** Replace Daytona with E2B in the comparison table per LANDING.md spec. Daytona was archived; E2B is the relevant competitor.

## CompareSection.astro: Dead Code

CompareSection.astro is imported in index.astro (line 7) but is NOT rendered -- it is imported but only the inline comparison table (lines 829-859) is used. The component still references Dev Containers and OpenClaw. Should either:
1. Delete the component (dead code), or
2. Refactor to use the new comparison data

## StackTable.astro: Dead Code with Stale Data

StackTable.astro is not imported anywhere. It still references `public.ecr.aws/w1l3v2k8/devcell` (line 42). WhatsInside.astro replaced it and correctly references `ghcr.io/devcell-sh/devcell`. Should delete or update StackTable.astro.

## V1: Section Order

**Target:** Hero -> Quickstart -> Features -> Comparison -> Stacks -> FAQ
**Actual:** Hero -> Quickstart -> FeatureCards -> LogoStrips -> Compare -> WhatsInside -> FAQ

- [x] Hero is first
- [x] Quickstart is second
- [x] Features (FeatureCards) is third
- [x] LogoStrips inserted between Features and Compare (acceptable addition)
- [x] Compare is in position
- [x] Stacks (WhatsInside) is in position
- [x] FAQ is last before footer
- [x] No redundant "READY TO TRY IT?" CTA at bottom

Section order matches the target spec.

## V2: Hero Accent

- [x] Fixed: "Blast Radius." has the `hl` (highlight) class (Hero.astro:13). The accent is on the right words per LANDING.md V2 spec.

## V3: Tone

- [x] Fixed: Section labels are factual: "Features", "Getting started", "Comparison", "What's inside", "Common questions"
- [x] Fixed: No "The payoff" / "The boundary" / "What the agent gains" sales-y labels
- [x] Fixed: No Acme/BigCorp consulting language
- [x] Fixed: Terminal mock is authentic
- [x] Fixed: Boundary table (Project/Cell/Host) is clear and factual

## V4: Trust Signals

- [x] Version badge present (Hero.astro:12, shields.io)
- [x] Last-commit badge present (Hero.astro:12, shields.io)
- [x] CONTRIBUTING link in footer (Base.astro:77)
- [x] Issues link in footer (Base.astro:73)
- [x] Releases link in footer (Base.astro:75) and eyebrow
- [x] Star count badge removed (no shields.io stars badge)
- [ ] Missing: No Discussions/community channel link (no Discord, Matrix, or GitHub Discussions)

## V5: Honesty

- [x] No inflated claims
- [x] "not yet" honest gaps in comparison table (credential brokering, snapshot/restore)
- [x] MCP count is "11" (accurate per catalog)

## V6: Visual Design

- [x] Brutalist aesthetic maintained
- [x] Terminal mock present and authentic
- [x] Card grid layout well-executed
- [x] Green accent distinctive

## SEO (P3)

- [x] Fixed: #31 `<link rel="canonical">` present (Base.astro:25)
- [x] Fixed: #32 `robots.txt` exists with sitemap reference
- [x] Fixed: #33 `@astrojs/sitemap` integration in astro.config.mjs
- [x] Fixed: #34 OG image + OG tags present
- [ ] Open: #35-36 GitHub repo description + topics (manual GitHub settings, not a code change)

## Vocabulary Violations

- [x] No retired terms ("session", "bunk") used incorrectly
- [x] "workspace", "cell", "stack", "module" used per VOCABULARY.md
- [x] Brand names correct: "DevCell" in prose, "devcell" in CLI/code
- [x] "devcell-sh" org name consistent
- [ ] Minor: Hero.astro:44 says "Opening cell myproject" -- "cell" here is the runtime entity (correct per VOCABULARY.md), but could be confused with the retired CLI name. Acceptable.

## New Findings (not in Oct 13 audit)

1. **FAQ HTML injection risk:** FAQ item #4 (index.astro:56) uses `<code>` HTML tags inside a string that is rendered via `{item.a}` in FAQ.astro. Astro escapes this by default, so the `<code>` tags will render as literal text, not HTML. The `set:html` directive is NOT used. This means the `--solitary` command in the FAQ answer shows as `<code>devcell claude --solitary</code>` literally. Need to use `set:html` in FAQ.astro or remove HTML from the string.

2. **CompareSection.astro imported but unused:** The component is imported on index.astro:7 but never rendered in the template. It's dead weight in the bundle.

3. **Missing Gemini in hero terminal mock:** The terminal mock shows Claude Code only. This is fine per positioning (Claude is primary), but consider whether showing agent versatility would strengthen the multi-agent message.

4. **Console errors on docs page:** The docs quickstart page had 2 console errors during navigation (noted in Playwright output). Should investigate.

## Summary: what's left to fix

### Must fix (affects accuracy or user experience)
1. **Comparison table: replace Daytona with E2B** -- Daytona is archived, E2B is the correct competitor per LANDING.md
2. **FAQ HTML rendering** -- FAQ item #4 has `<code>` tags that render as literal text instead of formatted code

### Should fix (cleanup)
3. **Delete or update CompareSection.astro** -- dead code with stale Dev Containers/OpenClaw data, still imported in index.astro
4. **Delete or update StackTable.astro** -- dead code with stale ECR URL
5. **Remove unused CompareSection import** from index.astro:7

### Nice to have
6. **Add community channel link** (Discussions or Discord) to footer or nav
7. **GitHub repo description + topics** (manual action, not code)
8. **Investigate docs page console errors**
