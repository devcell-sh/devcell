# Personas

DevCell serves one primary persona. Anti-personas are defined to keep
marketing focused.

Linked from [FEATURES.md](./FEATURES.md) (what they see) and
[COMPETITION.md](./COMPETITION.md) (what they compare against).

---

## Primary persona: Alex ("The Aware Power User")

Alex does not need to be sold on sandboxing. Alex needs to find us
before someone else builds it.

### Snapshot

| | |
|---|---|
| **Role** | Senior / staff engineer, tech lead, or solo founder shipping production code |
| **Experience** | 8-20 years writing code; comfortable in a terminal; reads man pages voluntarily |
| **Location** | Distributed: North America, EU, increasingly LATAM/SEA |
| **Income** | $150K-$400K (employed) or pre-revenue to low-7-figure ARR (founder) |

**Why 8-20yr:** SO 2026 shows 16+ yr developers are the most
AI-favorable (69% positive sentiment). This cohort shares tools with
peers, generating the social credit that is DevCell's success metric.
The 2-8yr cohort uses AI agents heavily but does not drive word-of-mouth
among senior engineers.

### Tools they already use

- **Agent:** Claude Code daily (Max or Pro plan), Codex CLI for second
  opinions, Gemini CLI for long-context tasks
- **Editor:** Neovim, VS Code, Zed, or Cursor, but the agent runs in a
  terminal, not in the editor
- **Shell:** zsh / fish + tmux or Zellij + starship
- **Secrets:** 1Password CLI, sometimes Doppler / Infisical
- **Runtime mgmt:** mise, asdf, or direct nix
- **Container exposure:** uses Docker daily, has touched Dev Containers,
  knows what a volume mount is

They are not afraid of CLI flags. They are afraid of destructive side
effects from non-deterministic agents.

### State when they find us

Post-realization, pre-solution. They have already had one of these
moments:

1. Read Anthropic's docs that say "run this in a container"
2. Watched an agent `rm` something they did not expect
3. Hit `--dangerously-skip-permissions` and immediately regretted it
4. Reviewed a teammate's auto-approve YOLO setup and thought "no way"
5. Saw a Twitter/HN post about an agent leaking SSH keys or pushing to
   the wrong repo

They search: `claude code container`, `claude code sandbox`, `claude
code safe auto approve`, `agent isolation docker`.

### What they want (in their words)

- "I want to let the agent just run."
- "I want to walk away from my desk."
- "I don't want to babysit every `bash` call."
- "I don't want it touching my SSH keys."
- "I don't want to maintain a Dockerfile per project."
- "I want my MCP servers and `.claude/` to keep working."
- "I want my Claude Max subscription to work: I'm not paying API rates."
- "I want to run this on a beefy remote box, not my laptop."

### What they do NOT want

- A new IDE or a new agent
- A SaaS dashboard or an API key to manage
- A "platform" with seats and SSO
- To learn Nix
- To watch a 20-minute YouTube intro
- To file a sales-led demo request

### 90-second evaluation

In order (fail any of the first four and they close the tab):

1. Is it real? (recent commits, version number, stars trending)
2. Does it support my agent? (Claude Code mentioned by name in hero)
3. Does it preserve my config? (MCP servers + `.claude/` carry over?)
4. Does it work with Max/Pro? (or am I paying API rates?)
5. How big is it? (image size, install time, opt-in to weight)
6. Can I get out? (project still looks normal, no lock-in)

### What they will pay for (eventually)

Today: nothing. Clean OSS CLI, `brew install`, trust.
Tomorrow: team-scoped cells, hosted cell runners, shared MCP secrets,
audit logs. Once their company asks "how do we let our engineers do this
safely at scale?"

---

## TAM sizing

| Tier | Estimate | Notes |
|---|---|---|
| Claude Code / Codex / Gemini CLI users globally | 500K-1M | GitHub stars + Anthropic disclosures |
| Daily power users, multi-project | 100K-200K | The "Alex" pool |
| Aware of sandbox concern | 20K-50K | Addressable market |
| Actively shopping for a fix right now | 2K-5K | Serviceable market this quarter |

The aware tier grows monthly as agents become more autonomous. The goal
is to own the search result for that tier before competitors do.

---

## Anti-personas (who we do NOT market to)

| Who | Why not |
|---|---|
| Agent-curious beginner ("just started learning Python") | Needs education we cannot afford to give |
| Browser-only AI user ("I use ChatGPT in the browser") | Not an agent user |
| Autocomplete user ("we use Copilot") | Autocomplete, not autonomous |
| Fearless deployer ("I run everything in prod already") | Will not pay for safety they do not feel they need |
| Enterprise buyer ("we need SOC 2 and an MSA") | Enterprise procurement, wrong motion |
| Nix enthusiast | Interesting overlap but selling Nix to them is the inverse of our pitch |
| "I'll write my own Dockerfile" engineer | They will, and that is fine |

---

## Discovery channels (ranked by likelihood)

1. **Google search** with high-intent query (`claude code docker`,
   `agent sandbox`)
2. **GitHub** via Show HN or a tweet
3. **Word of mouth** in senior-engineer Slack/Discord
4. **Anthropic's docs** linking to community sandboxing solutions
   (aspirational)
5. **HN front page** for a launch post or postmortem

NOT via: paid ads, LinkedIn, conference booths, developer-relations
content marketing.

----

# Changes needed

## 2026-10-14: Remote compute desire added

Added "I want to run this on a beefy remote box, not my laptop" to
Alex's want list. Research (2026-10-14, 68 findings) shows 64% of
developers now use non-local environments (Docker 2025), and
remote execution demand is a 4-surface strong signal. This is a
secondary want: Alex's primary pain is sandbox safety, not compute
location. Remote compute becomes relevant when Alex runs large
builds, GPU workloads, or agents on a team server. The want was
not present in the original persona because the persona was defined
before remote cell research.

## 2026-10-12T16:30:00Z Initial assessment

### Persona validation

- Alex is defined from research, not from user interviews. Validate
  against first 10 real users post-launch: do they match the 8-20yr
  profile, the tool stack, the discovery path?
- "90-second evaluation" checklist (#1-6) is untested. Run with 3
  real users watching them evaluate the README cold

### Secondary persona: Vibe Coder

- Feb 2026 PRODUCT.md had a second persona: "Developer Wannabe /
  Vibe Coder" (non-technical, AI-heavy, 2-8yr). Deprioritized in
  favor of Alex because the social credit goal depends on senior
  engineers sharing, not beginners adopting
- r/vibecoding is growing faster than r/ClaudeAI. Re-evaluate within
  90 days of launch whether vibe coders are a meaningful install
  source even if not the marketing target
- If re-added: separate landing page or section, not mixed into the
  Alex-targeted hero

### Discovery channel gaps

- Channel #4 "Anthropic's docs linking to community sandboxing
  solutions" is aspirational. No action taken to get listed. Explore:
  PR to Anthropic docs, or appear in their community showcase
- No SEO baseline established for devcell.sh. Set up Google Search
  Console and measure impression/click data for target queries before
  launch

### TAM recalibration

- "Actively shopping" tier (2K-5K) was estimated in Oct 2026. Re-
  estimate after launch using actual install/star conversion data
- Claude Code adoption (66% of agent users, SO 2026) may shift with
  Codex and Gemini CLI growth. Monitor agent market share quarterly

## 2026-10-15: Agent mix per Stack Overflow 2026

Stack Overflow 2026 (survey.stackoverflow.co/2026/ai/data), coding agents among agent users: Claude Code 65.5%, GitHub Copilot 58.7%, OpenAI Codex 29.5%, Cursor 25.5%, Google Antigravity 16.0%, Gemini Code Assist 14.6%, OpenCode 13.6%. "Gemini CLI" is not a separate line.

Gap: "Tools they already use" lists Codex and Gemini CLI as the secondary agents. Copilot is the real #2 by usage and is absent from the persona, the hero body copy, and the logo strip. Decide whether Copilot CLI is a supported agent before the HN launch; if not, say so in the FAQ. The 66% Claude Code figure in "TAM recalibration" is confirmed (65.5%) and trending up (41% -> 55% -> 65.5%), so the vendor-dependence note can stay as written.

