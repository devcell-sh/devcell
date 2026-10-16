# Community Findings

*Research date: 2026-02-28*
*Sources: r/ClaudeAI, r/vibecoding, r/LocalLLaMA, Reddit search, WebSearch*

---

## Persona Ratio Estimate

| Community    | Developer % | Wannabe % | Notes                                                                      |
|--------------|-------------|-----------|----------------------------------------------------------------------------|
| r/ClaudeAI   | ~70%        | ~30%      | Technical vocabulary dominant; but wannabe voices visible in fear posts    |
| r/vibecoding | ~30%        | ~70%      | 89k members; wannabe-dominant; "building without knowing code" is the norm |
| r/LocalLLaMA | ~85%        | ~15%      | Heavily technical; ML/infra focus                                          |

**Verdict:** By Reddit volume, developers are the larger presence. But r/vibecoding's 89k membership and rapid growth
suggests the wannabe audience is scaling faster. The wannabe's fear vocabulary is louder and more emotionally expressive
in posts that go viral.

---

## Q1 Answer: Which Persona Is Larger?

Developers dominate by post count. Wannabes dominate by anxiety expression and viral horror story engagement. *
*Recommendation: lead with developer headline ("Claude Code, without the blast radius.") — it earns technical
credibility and resonates with the larger base, while the hero description converts wannabes via outcome framing.**

---

## Q2 Answer: Wannabe Fear Vocabulary (Exact Phrases)

These are verbatim phrases from non-technical posters (r/vibecoding, r/ClaudeAI replies, Reddit search):

- **"harm to your machine"** — exact phrase used in Claude Code's own help text; non-technical users paraphrase it back
  directly
- **"one oopsie and I lose days of work"**
- **"I'm nervous about letting an AI loose on my computer"**
- **"what if it deletes something important?"**
- **"Don't let Claude do anything you don't understand"** — advice given in community threads; reflects the wannabe's
  baseline anxiety
- **"I'm scared to turn off the approvals"**
- **"it's going to mess up my whole computer"**

**Key insight:** Wannabes do NOT say "blast radius." They say "mess up my computer" and "delete something." The fear is
loss, not blast radius. Frame accordingly in description copy.

---

## Developer Fear Vocabulary (Exact Phrases)

- **"--dangerously-skip-permissions"** — the flag name is itself a warning; developers mention it by name
- **"blast radius"** — used naturally; this is their word
- **"it has access to my SSH keys"**
- **"pushed to the wrong repo"**
- **"dropped a table"**
- **"touched files outside the project"**
- **"mess up my filesystem"**

---

## Q3 Answer: "Container" vs "Sandbox" Terminology

| Term                     | Developer use           | Wannabe use                               | Verdict                                                 |
|--------------------------|-------------------------|-------------------------------------------|---------------------------------------------------------|
| "container"              | Yes — frequent, natural | Rare; some use it after Docker experience | Developer vocabulary; safe to use for developer persona |
| "sandbox"                | Yes                     | Yes — wannabes use it naturally           | **Safe for both personas; prefer this on homepage**     |
| "VM" / "virtual machine" | Yes                     | Rarely                                    | Developer only                                          |
| "isolated environment"   | Yes                     | Rarely                                    | Developer only                                          |
| "cell"                   | Not common              | Not common                                | Requires explanation — but resonates once explained     |

**Verdict:** Use **"sandbox"** on the homepage. It works for both personas. Reserve "container" for GitHub README and
technical docs. Never use "VM" in hero copy.

---

## Q4 Answer: Precipitating Events — The Three Viral Horror Stories

These are the documented incidents that seeded fear in communities:

| Incident                  | Date     | Platform                     | Format                | Engagement             | Summary                                                                                     |
|---------------------------|----------|------------------------------|-----------------------|------------------------|---------------------------------------------------------------------------------------------|
| **rm -rf home**           | Dec 2025 | Hacker News                  | Top-level post        | ~197 points            | User ran Claude Code with auto-approve; agent executed `rm -rf ~/` on host; lost everything |
| **Wolak firmware wipe**   | Oct 2025 | Reddit / r/ClaudeAI          | Post with screenshots | High upvotes           | Agent ran recursive delete on firmware project directory, propagated to mounted paths       |
| **McAulay 11GB deletion** | Jan 2026 | Twitter/X + Reddit crosspost | Thread + screenshots  | Viral; 1k+ engagements | AI agent deleted 11GB of project files in a single session; user had no recovery path       |

**Precipitating event pattern:** Wannabes don't search for a sandbox unprompted. They search after reading one of these
posts, or seeing a quote from one shared in their feed. The horror story is the trigger; it creates acute, emotional
recall ("that could be me").

**Where stories break:** Hacker News (developer amplification) → Reddit crossposts → Twitter/X threads. The virality
path is HN → Reddit → Twitter.

**Implication for awareness content:** Seed at HN and r/ClaudeAI first; Twitter/X amplifies the viral wave.

---

## Anthropic's Own Framing (Highest Authority Signal)

Anthropic engineers explicitly wrote in Claude Code documentation:

> *"Run this in a container, not your actual machine."*

This is the highest-credibility validation of devcell's concept. Anthropic's own team recommends containerized execution
for safety — devcell makes that recommendation actionable.

Also notable: Anthropic shipped a native `/sandbox` command (OS-level via Seatbelt on macOS, bubblewrap on Linux). This
validates the market but uses a fundamentally different approach (process-level, not Docker). devcell's Docker-based
approach gives a harder boundary and is more portable.

---

## Community Vocabulary Summary

| Concept        | Developer says                                       | Wannabe says                                              |
|----------------|------------------------------------------------------|-----------------------------------------------------------|
| The risk       | "blast radius", "SSH keys", "wrong repo"             | "mess up my computer", "delete something", "lose my work" |
| The tool flag  | "--dangerously-skip-permissions"                     | "turn off the approvals", "let it run freely"             |
| Isolation term | "container", "sandbox", "VM"                         | "sandbox" (only)                                          |
| What they want | "full-cycle autonomous dev without approval prompts" | "let it just build without me worrying"                   |

---

## Horror Story Sources (for awareness content seeding)

- **Primary platform:** Hacker News — developers post, developers amplify; gets crossposted
- **Secondary:** Reddit r/ClaudeAI — moderate visibility, high anxiety engagement
- **Tertiary:** Twitter/X — fastest spread after an HN/Reddit post goes viral
- **Format that spreads:** Story post with specific data loss numbers ("11GB", "3 years of work") > vague warnings
- **Implication:** Seed awareness content with concrete, specific framing — numbers and specificity drive shares

---

## What Surprised Us

1. **"sandbox" is already the dominant community term** — devcell doesn't need to fight for this word
2. **Wannabes are already searching for solutions after viral posts** — there is active demand, not latent
3. **Anthropic's own documentation validates the product concept** — use this as social proof
4. **The wannabe audience (r/vibecoding, 89k) is growing faster than developer audience** — timeline for headline
   choice: start with developer, re-test wannabe within 90 days
