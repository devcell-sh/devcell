# Launch playbook

Companion to [PERSONA.md](./PERSONA.md), [POSITIONING.md](./POSITIONING.md), [ADOPTION.md](./ADOPTION.md), [OBSTACLES.md](./OBSTACLES.md), [IDENTITY.md](./IDENTITY.md), [COMPETITORS.md](./COMPETITORS.md).

This doc is the **HN launch recipe**. Strategy — including *why* the launch matters and what success looks like — lives in [POSITIONING.md](./POSITIONING.md), specifically Bucket 3.

Per POSITIONING.md, the launch is the **credit-minting event**, not a marketing campaign.

---

## What you're extracting from this launch (top 3)

A Show HN is not a marketing event. It is a structured feedback extraction. The persona cohort converges on three things that matter, everything else is vanity.

### 1. Problem validation: does the pain resonate?

**Hykes, Evan You**

The HN comment section tells you whether people recognize the problem before you describe the solution. The signal you want: "I've been wrapping docker run in shell scripts for exactly this" or "my agent deleted my .ssh last week." The anti-signal: "why would I need this?" or (worse) silence.

Hykes' Docker lesson: massive adoption doesn't mean the problem you think you're solving is the one users experience. Docker solved "works on my machine" but users thought it solved deployment. When those diverged, monetization collapsed. Read the comments for what problem people think DevCell solves, not whether they like it.

Evan You's Vue launch was successful because the "before/after" resonated instantly: people saw the pain of Angular's complexity without needing to be told. If your HN thread requires long explanations of why auto-approve on bare metal is dangerous, the pitch isn't landing.

**What to look for in comments:**
- "I've been doing X manually" = demand confirmed
- "Why not just docker run?" = differentiation not landing
- "Does this work with [tool you didn't consider]?" = adjacent use cases to explore
- "This is just a wrapper around Docker" = value prop unclear

### 2. First 10 bug-filing users (not star-clickers)

**Torvalds, DHH**

Stars are a lagging indicator of attention, not usage. The launch's real output is 10 people who install it, hit something unexpected, and file an issue. Those 10 people are:
- Your first external QA team (they find bugs your test suite misses because they have environments you don't)
- Your positioning oracle (the bugs they file reveal what they expected vs. what they got)
- Your future first contributors (Mozilla data: contributors who get a response within 48 hours have a much higher return rate)

DHH's extraction principle: the best open source comes from production use, and the first external users tell you where your assumptions about "production" diverge from theirs. Your devcell.toml defaults, your image size, your first-run time: these all encode assumptions that 10 real users will stress-test faster than 1,000 stars.

Torvalds' process discipline: "show me the code" applies to users too. A star costs nothing. An issue costs 5 minutes of someone's time. An issue with reproduction steps costs 15 minutes. That's the currency that matters.

**What to measure:**
- Issues filed in the first 7 days (target: 5-15)
- % of issues with reproduction steps (target: >50%)
- Time to first external PR (target: within 14 days)
- Homebrew installs vs. stars ratio (installs = real intent; target: >10% of stars)

### 3. Positioning test: do people get what this is in 5 seconds?

**Ronacher, Hykes**

Your one-line pitch either clicks or it doesn't, and HN is the harshest test environment for this. The comment section reveals whether your category claim is landing.

Ronacher's Fair Source launch succeeded because the name itself explained the positioning: "not open source, not proprietary, fair source." The category was instantly graspable even if people disagreed with it. DevCell's "Claude Code, without the blast radius" is strong because it names the fear (blast radius) and the solution (containment) in one phrase.

Hykes' Docker failure mode: Docker's brand became generic ("docker" = "container") while competitors captured the value. HN comments will tell you whether "DevCell" is becoming a category name ("I devcell'd my project") or being dismissed as "just a docker wrapper." The former is gold. The latter means you're not communicating the layer above Docker.

**What to look for in comments:**
- People using your terminology back to you ("cell," "workspace," "solitary mode") = positioning landed
- People comparing you to the right competitors (Docker Sandboxes, E2B) = category understood
- People comparing you to the wrong things (devcontainers, Codespaces, Gitpod) = you're being misread as cloud dev environments
- People asking about pricing/enterprise = they see a product, not a toy

---

Three operational consequences carry through everything below:

1. **Title leads with the Nix-MCP flex.** Working draft: *"Show HN: DevCell — a local Claude Code sandbox with a Nix-pinned MCP catalog."* Not the safer "local container for Claude Code, Codex, and Gemini CLI" framing.
2. **Gated on the postmortem.** The HN first comment links to one published technical postmortem (POSITIONING.md Bucket 2). No postmortem → no launch.
3. **Success target is 300-800 stars + 1 credible-engineer retweet naming Dmitry**, not 5K stars. See the metrics table later in this doc and read the "Good" column as the actual target.

The rest of the playbook (timing, comment cadence, cross-channel sequencing) is mechanics, unchanged.

---

## When to launch

**Not on the calendar. On a milestone.** Launching to a quiet release damages the brand more than no launch at all — first impression is one-shot, and the persona evaluates by recency of activity.

Trigger conditions, all required:

- [ ] Identity migration (CELL-12) is shipped: `devcell-sh/devcell`, `brew install devcell-sh/tap/devcell`, no `DimmKirr/` in any reference
- [ ] OBSTACLES S-tier #2 is fixed: first run is consented, stack picker default is `dev` (under 3GB)
- [ ] OBSTACLES S-tier #3 is fixed: README has version + last-commit + cadence badges
- [ ] OBSTACLES A-tier #6 is fixed: first-session config-survived banner is visible
- [ ] At least one comparison or postmortem post is published on the site (gives HN commenters something to link to that isn't just the README)
- [ ] Fresh-machine first-use time measured and documented: < 10 minutes
- [ ] At least three real users beyond the maintainer have used `cell claude` for ≥ one real session (not synthetic tests)

If any of the above is false on launch day, **delay**. There is no benefit to launching unready.

---

## Pre-launch checklist (T-7 days)

The week before:

- [ ] Tag a release (`v0.X.Y`) and write its CHANGELOG. HN comments will link to specific versions.
- [ ] Make sure GitHub repo description, topics, and `about` panel are filled. Topics: `claude-code`, `agent`, `sandbox`, `container`, `docker`, `cli`, `nix`, `mcp`.
- [ ] Pin the README to the new release.
- [ ] Verify `brew install devcell-sh/tap/devcell` works on a clean macOS install (Intel + Apple Silicon, both supported Docker engines).
- [ ] Verify Linux installation path works on at least Ubuntu 24.04 and Fedora 41.
- [ ] Confirm `devcell.sh` site is live, has analytics (Plausible), and points to the new install command.
- [ ] Set up Google Search Console for `devcell.sh` and submit the sitemap. (Won't show data for weeks but starts the clock.)
- [ ] Open the FAQ-this-launch-might-trigger doc and pre-write answers to: "Why not just use Docker / devcontainers / e2b / Daytona / Anthropic's docs?" "Does my code leave my machine?" "Why Nix?" "What's the image size?" "What about Codex/Gemini?" "Is this enterprise-ready?"
- [ ] Notify two or three friendly users that you're launching, so they can be early commenters with substantive answers (not shills — actual users with real opinions).

---

## The HN post

### Timing

**Tuesday or Wednesday, 8–10am Pacific Time** (= 11am–1pm ET, 4–6pm UK, 5–7pm CET). This is the documented sweet spot — US East Coast engineers are at their desks, EU is in their afternoon dip, West Coast hasn't fully ramped. Avoid Mondays (HN front page is still saturated from the weekend tail) and Fridays (front page rolls over fast into a low-attention weekend).

Avoid:

- The week of major Anthropic, OpenAI, or Google announcements (your post drowns)
- The week of major US holidays (low traffic)
- The first or last week of a quarter (people are busy)
- Any day with a heated political news cycle (HN community is distracted)

### Title format

> **Show HN: DevCell – a local container sandbox for Claude Code, Codex, and Gemini CLI**

Rules:

- Begins `Show HN:` (signals self-launch, not third-party news)
- Contains the canonical noun (`DevCell`) once, lowercase command (`devcell`) not in title
- Names at least one of the agents the persona uses (Claude Code)
- States the category claim ("container sandbox") explicitly
- Length: ≤ 80 chars

**Do not:**
- Start with "I built X" (HN finds this self-indulgent)
- Use marketing words ("revolutionary," "easy," "powerful")
- Hide what the project actually is

### Body (the first comment)

HN convention: the submitter posts a substantive first comment immediately. This is *not* the same as the submission body — it's a top-level comment that anchors the discussion. ~150-250 words.

Skeleton:

```
Hey HN. DevCell is the cell my Claude Code lives in, turned into a CLI.
A cell is a persistent boundary — one $HOME, one network, one secrets
scope — that spawns a container per project I work on. Built it after
[specific moment — agent did something unexpected to host files, or a
teammate's auto-approve setup made me nervous, or whatever the real
catalyst was].

What it does:
  brew install devcell-sh/tap/devcell
  cd my-project
  devcell claude
That gives you Claude Code in a Nix-based container, with your .claude/,
MCP servers, hooks, and skills mounted in, and your Max/Pro subscription
auth passed through from your host browser.

What I learned along the way:
[ONE technical observation that would be interesting to a senior engineer.
Image size tradeoffs, Nix vs Debian for reproducibility, how MCP merge
actually works, the Max/Pro auth pass-through trick. ONE thing. The
comment is not a feature list.]

Stuff that's intentionally not there:
- No cloud, no managed plane, no API key
- No editor binding, no devcontainer.json
- Free, Apache 2.0, no paid tier (yet)

Image is ~[N] GB for the default 'dev' stack, [~M] for 'ultimate'.
First run takes ~[T] minutes. Repo: devcell-sh/devcell. Happy to answer
anything.
```

Constraints:

- One specific technical observation, not a feature list
- Honest numbers (image size, first-run time, what's not there)
- No marketing words
- Mention the install command verbatim — people will paste it
- Apache 2.0 in the body for license-sensitive readers

### Don't

- Don't ask for stars in the post
- Don't post on multiple platforms simultaneously (HN rules; also splits attention)
- Don't submit from a brand-new account (HN flags fresh accounts on Show HN posts)
- Don't reply with "thanks!" — reply with substance or don't reply

---

## The first 6 hours

This is where launches succeed or die. The HN ranking algorithm rewards comment-per-upvote ratio in the first 90 minutes. Be present.

Cadence:

| Time post-launch | Action |
|---|---|
| 0:00 | Submit. Add your top-level comment within 1 minute. |
| 0:00–0:15 | Triage: read every comment, plan replies in order of substance |
| 0:15–6:00 | Reply within 5–15 minutes per comment. Substantive, not defensive. |
| 1:00 | Cross-post link (NOT the same submission) to: Anthropic Discord (#showcase or equivalent), r/ClaudeAI, your own personal Twitter/X if you have one. Phrasing: *"I built a thing, HN comments would be useful: \[link\]"* |
| 6:00 | If on front page: stay present another 6 hours. If not: assess what bounced and write a post-mortem (privately first). |

Reply tactics:

- **Honest critique:** thank specifically, agree where you can, disagree without flinching, name the tradeoff
- **Comparison ("why not X?"):** name X's actual niche from COMPETITORS.md, then state the wedge. Do not denigrate X.
- **Feature request:** if it's reasonable, file a Linear ticket *in the reply* (link). Demonstrates the project is real.
- **"Looks like a hobby project":** name the cadence, the test suite, the contribution policy. Link to recent commits.
- **"Why a container?":** point to Anthropic's own docs that say "run this in a container," then describe what specifically goes wrong without one (host SSH, dotfiles, prod creds, auto-approve blast radius)
- **Trolls / hostile bait:** don't engage. The cost of one bad reply > the cost of ignoring one bad commenter.

**Hard rule:** never reply when angry. If a reply would take more than ~5 minutes to write calmly, don't write it.

---

## Cross-channel sequencing

Each channel gets one mention, ever, per launch. After this sequence, don't re-promote — distribution is from now on either word-of-mouth or search.

| Channel | Timing relative to HN | Phrasing template |
|---|---|---|
| Anthropic developer Discord (`#showcase` or `#community-projects`) | T+1h | *"Posted on HN, comments would help: \[link\]. Local Claude Code container, Apache 2.0."* |
| r/ClaudeAI | T+1h | Same — as a self-post linking the HN thread, not the repo |
| r/commandline (Reddit) | T+24h, only if HN was a hit | Brief, technical, no marketing |
| Lobsters | T+24h, only if a Lobsters invite is available | Title format same as HN; community is harsher, be more terse |
| Anthropic OSS partnerships / DevRel contact | T+48h, only after HN data is in hand | Direct email with launch data ("400 GitHub stars in 48h, here's the thread, would love to be in the community sandboxing solutions list") |
| Personal Twitter/X | T+0 to T+24h | Single tweet, link to HN thread, no thread |

Do not:

- Email lists you've collected
- LinkedIn (the persona does not read LinkedIn for tools)
- Paid promotion of any kind
- Cross-post the same content verbatim across channels (HN finds this gauche)

---

## What counts as success

Launch day metrics, in priority order:

| Metric | "Bad" | "OK" | "Good" | "Front page" |
|---|---|---|---|---|
| HN front page time | <30 min | 30 min–2h | 2–6h | 6h+ |
| HN upvotes after 24h | <50 | 50–150 | 150–400 | 400+ |
| HN comments after 24h | <10 | 10–40 | 40–100 | 100+ |
| GitHub stars added in 48h | <50 | 50–200 | 200–500 | 500+ |
| Unique repo visitors in 7d | <500 | 500–2K | 2K–10K | 10K+ |
| Homebrew installs in 7d | <50 | 50–250 | 250–1K | 1K+ |
| Press / DevRel pickup | none | 1 retweet from a credible engineer | 1 blog mention | Anthropic links it |

Any column right of "OK" means the positioning landed. "Bad" across the board means the post didn't surface — try again in 4–8 weeks with a tighter title and a clearer wedge, not the same post.

---

## Post-launch (Day 2–14)

The fortnight after launch is where retention is won or lost. New issue filers, first-time installers, contributors — every interaction is a sample of how DevCell handles the people the launch attracted.

Daily for two weeks:

- Triage GitHub issues within 24 hours, even if the answer is "this is intended behavior"
- Triage Discord/Reddit mentions within 24 hours
- Tag any "I tried it but X" issue as a candidate for an OBSTACLES.md update — these are the new S-tier candidates
- Ship something visible (release, blog post, comment thread, bug fix) every 48 hours — the persona is checking commit cadence

At day 14, write the post-launch retro privately (do not publish). What worked, what bounced, what to keep doing. Update OBSTACLES.md if the data shifts the ranking.

---

## What changes if Anthropic launches a competing primitive during the same window

This is the single highest-impact risk: Anthropic announces `claude code --sandbox` natively the same week as our HN post. (Probability low, impact catastrophic.) The playbook:

- Do not delete the post or panic
- Within the existing HN thread, add a comment within 60 minutes acknowledging the news, pointing to COMPETITORS.md §Anthropic, and naming the wedge (env + MCP + auth + subscription pass-through, not the agent process)
- Update README within 6 hours with the comparison
- Do not pivot positioning. The wedge thesis is durable; the launch was for awareness, not category-creation

---

## The one-line summary

> Don't launch until the S-tier obstacles are fixed and three real users have completed first-run successfully. Then post on Tue/Wed AM PST, be present for 6 hours, respond substantively, never beg for stars. Measure stars-at-48h and installs-at-7d as the primary signal. If Anthropic launches a competitor the same week, retreat to the env-and-MCP wedge — do not panic.

----

# Changes needed

## 2026-10-12T16:30:00Z Initial assessment

### Title update

The working draft title references "Nix-pinned MCP catalog" but
POSITIONING.md (Jun 2026) updated the pitch to "Claude Code with the
lights on." Two candidates:

1. "Show HN: DevCell -- Claude Code with the lights on (a local
   sandbox with a real Chromium you can watch)"
2. "Show HN: DevCell -- a local container sandbox for Claude Code,
   Codex, and Gemini CLI"

Decision from earlier session: #1 for HN launch, "blast radius" for
README. Title needs final lock-in.

### Channels and influencer shortlist (from marketing/CHANNELS.md, Jun 2026)

Cross-channel sequencing around the HN launch:

| Channel | Stage | Content | Persona |
|---------|-------|---------|---------|
| HN (Show HN) | Consideration | Technical write-up, be present 6h | Senior eng |
| Twitter/X | Awareness | Demo GIF, "I was scared to use auto-approve" narrative | Both |
| Reddit r/ClaudeAI | Consideration | "How I run Claude Code without fear" post | Both |
| Reddit r/vibecoding | Awareness | Organic mention in tool threads | Vibe coder |
| Discord (AI coding servers) | Consideration | Word of mouth, helpful not promotional | Both |
| YouTube | Awareness | 3-min setup + first run walkthrough (if made) | Vibe coder |

Influencer shortlist (Feb 2026 research, needs refresh):

| Name | Platform | Fit | Priority |
|------|----------|-----|----------|
| Fireship | YouTube (4.1M) | 100-second explainer format | Tier 1 (hard, high reward) |
| Theo (t3.gg) | Twitter/X (243K) | Opinionated dev tool takes | Tier 1 |
| Nate's Newsletter | Substack (20-50K est.) | Already writes Claude Code guides | Tier 2 (best fit) |
| Jack Herrington | YouTube (50-100K est.) | Practical fullstack builder | Tier 2 |

Outreach hooks:
- Technical: "What happens when you run --dangerously-skip-permissions?
  We made a sandbox so you don't have to find out the hard way."
- Non-technical: "I was terrified to let Claude run freely. Here's
  what I use now."

### SEO targets (from marketing/CHANNELS.md)

Low-volume, high-intent queries to own with a single well-optimized
page:
- "claude code auto approve safe"
- "docker sandbox for ai agents"
- "run claude code without risk"
- "isolate claude code from host"
- "claude code container"
- "agent isolation docker"
- "gemini cli sandbox"
- "codex container"

Note: "claude code container" is declining (Anthropic + Docker own
SERP). Focus on queries they don't own.

### Stale references in this doc

- References `DimmKirr/` and `cell` command in places (from Jun 2026).
  Should be `devcell-sh/` and `devcell` throughout.
- Companion links at top reference `./PERSONA.md` etc. which are
  marketing/ paths, not .product/ paths. Update to .product/ links.
- Launch trigger checklist item "Identity migration (CELL-12)" is
  DONE (commit c0a52b5, bb4a754).

### Demo asset status

The demo (15-30s silent loop of Claude Code driving headed Chromium)
gates the launch. No demo = postpone. Status: NOT PRODUCED. Linear:
CELL-274.
