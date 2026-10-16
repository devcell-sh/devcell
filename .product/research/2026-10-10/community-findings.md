# Community Findings: Who Needs This and What Do They Call It?

**Date:** 2026-10-11
**Purpose:** Inform product naming (devcell vs agentcell vs X)

---

## The Market Split (confirmed, 2025-2026)

The market that was once "cloud development environments" has bifurcated:

| Segment | For whom | Lifecycle | Examples |
|---|---|---|---|
| **AI Agent Sandboxes** | Agents (autonomous code execution) | Seconds to hours, then destroyed | E2B, Modal, Daytona (post-pivot), Cloudflare Sandboxes |
| **Cloud Dev Environments** | Human developers (interactive coding) | Days to months, persistent | Codespaces, Gitpod, old Daytona |

**Signal:** Daytona pivoted entirely from CDE to AI sandbox in early 2025, raised $24M Series A in Feb 2026. Their docs still mix old CDE terminology with new sandbox terms.

**Growth:** AI agent sandbox segment growing at 40%+ CAGR. E2B went from 40K sandboxes/month (early 2024) to 15M+/month (early 2025). Modal raised $355M at $4.65B valuation (May 2026). Traditional CDEs are mature/stagnant.

---

## Three Personas Buying Sandboxes

### Persona 1: Developer using AI coding agents (devcell's user)
> "As a developer who runs Claude Code / Codex daily, I want an isolated environment that persists my setup so that agents can't wreck my host but I don't lose my tools and config between sessions."

- **Scale:** 90% of professional devs use AI coding agents weekly (JetBrains 2026). Claude Code used by 66% of agent users (Stack Overflow 2026).
- **Need:** persistent, local, safe. NOT ephemeral, NOT cloud-hosted, NOT API-driven.
- **Word they use:** "sandbox", "devcontainer", "container", "isolated environment"
- **Pain:** "Running Claude with --dangerously-skip-permissions on my host is scary." "I want full auto-approve without risk."
- **Buys:** CLI tools, Docker-based, self-hosted.

### Persona 2: Platform builder / AI engineer
> "As a platform builder, I need an SDK to create/destroy sandboxes programmatically so my product can execute user/agent code safely at scale."

- **Scale:** Smaller but high-value. Building products ON TOP of sandboxes.
- **Need:** SDK, sub-second cold starts, API, managed infrastructure, millions of sandboxes.
- **Word they use:** "sandbox", "runtime", "execution environment"
- **Buys:** E2B, Modal, Daytona. API-first, managed, cloud.

### Persona 3: ML / Data science team
> "As an ML engineer, I need GPU-backed execution environments for training and inference."

- **Scale:** Niche but high spend.
- **Need:** GPUs, Python runtime, serverless scaling.
- **Buys:** Modal (dominant), cloud GPUs.

---

## Vocabulary Analysis

What word do users actually use for "the thing I run my agent in"?

| Term | Who uses it | Context |
|---|---|---|
| **sandbox** | Everyone. THE market word. | "AI agent sandbox", "code sandbox", "sandbox environment" |
| **environment** | CDE segment, traditional devs | "dev environment", "development environment" |
| **container** | Implementation-specific | "Docker container", "run in a container" |
| **devcontainer** | VS Code ecosystem | "devcontainer.json", ".devcontainer/" |
| **workspace** | Daytona (dropped it), VS Code, Codespaces | Legacy CDE term, being replaced by "sandbox" |
| **cell** | Nobody in this market | Only devcell uses this term |
| **machine** | Fly.io ("Fly Machines") | Infrastructure-layer term |
| **runtime** | Cloudflare, WASM context | "sandbox runtime" |

**"Sandbox" is the universal term.** It's used across all segments, all personas, all competitors. No other word comes close.

**"Cell" is a neologism.** Zero market usage outside devcell. This is either a brand asset (unique, ownable) or a liability (unfamiliar, requires explanation).

---

## Where devcell Actually Sits

devcell does NOT fit neatly into either segment:

| Dimension | AI Agent Sandbox (E2B/Modal/Daytona) | CDE (Codespaces/Gitpod) | devcell |
|---|---|---|---|
| For | Agents (programmatic) | Humans (interactive) | Humans using agents |
| Lifecycle | Ephemeral | Persistent | Persistent |
| Hosting | Cloud | Cloud | Local (self-hosted) |
| Interface | SDK/API | Browser IDE | CLI |
| Cold start | <200ms | Seconds | Minutes (first build), seconds (after) |
| Scale | Millions of sandboxes | Dozens per org | One per project per developer |
| Isolation | MicroVM / container | Container | Container / VM |

**devcell is a third thing:** a local, persistent, CLI-first sandbox for developers who work WITH agents daily. Not a cloud sandbox API. Not a cloud IDE. A personal sandbox.

---

## Competitor Taglines (how they position)

| Product | Tagline |
|---|---|
| E2B | "Cloud Runtime for AI Agents" |
| Daytona | "Infrastructure for Agentic Code" |
| Modal | "Run AI Models and Batch Jobs" |
| Codespaces | "Your instant dev environment" |
| Gitpod | "Developer environments for professional software teams" |
| Docker Sandboxes | "AI coding agents, safely contained" |
| Trail of Bits devcontainer | "Run Claude Code safely" |
| Fly.io Sprites | "Dedicated servers for AI agents" |
| **devcell** | **"Agentic Coding, Without the Blast Radius"** |

devcell's tagline is already well-positioned. It says:
1. Agentic (this is for agent workflows)
2. Coding (this is for developers)
3. Without the Blast Radius (safety/isolation)

---

## The Naming Question: "dev" cell or "X" cell?

### Option analysis

| Name | Emphasizes | Audience signal | Risk |
|---|---|---|---|
| **devcell** | The developer | "This is a dev tool" | Limits to "dev" when product could serve CI, testing, education |
| **agentcell** | The AI agent | "This is agent infrastructure" | Competes directly with E2B/Daytona. devcell doesn't have their SDK/API/scale. |
| **codecell** | Code execution | "Run code safely" | Generic. Sounds like CodeSandbox/Codespaces. |
| **sandcell** | Sandbox metaphor | Sandbox + cell | Awkward. |
| **cellbox** | Sandbox + cell | Sounds like a physical product | No. |

### Verdict

**"devcell" is the right name.** Here's why:

1. **It's the intersection, not a segment.** devcell serves developers who use agents. The "dev" isn't limiting: it's positioning. It says "this is YOUR tool" (not the agent's tool, not the platform's tool).

2. **"agentcell" would misposition.** It implies competing with E2B/Modal/Daytona on their terms (SDK, API, ephemeral, cloud-scale). devcell doesn't do that. It would invite unfavorable comparisons.

3. **The growth IS in "dev + agent."** 90% of devs use agents weekly. The fastest-growing need is "how do I run these agents safely?" devcell answers that for the individual developer.

4. **"dev" matches the persistence model.** Agent sandboxes are ephemeral. Dev environments are persistent. devcell is persistent. "dev" signals that.

5. **The tagline already does the agent positioning.** "Agentic Coding, Without the Blast Radius" says "agents" without putting it in the product name. The product name says "dev" (who it's for), the tagline says "agentic" (what it handles).

---

## Sources

- [JetBrains Developer Ecosystem Survey 2026](https://blog.jetbrains.com/research/2026/08/ai-coding-agent-adoption-2026/)
- [Stack Overflow Developer Survey 2026](https://stackoverflow.blog/2026/10/06/the-results-of-the-2026-developer-survey-are-here/)
- [E2B vs Daytona (ZenML)](https://www.zenml.io/blog/e2b-vs-daytona)
- [Daytona pivot and Series A](https://northflank.com/blog/daytona-vs-e2b-ai-code-execution-sandboxes)
- [Modal $355M raise](https://tech-insider.org/e2b-vs-modal-vs-daytona-2026/)
- [AI Agent Sandbox Guide (Cosmonic)](https://cosmonic.com/blog/ai-sandbox-guide/)
- [Agent Sandbox Market (bex.co)](https://bex.co/blog/2026/09/25/four-companies-one-sandbox-market-agent-code-execution)
- [devcell GitHub](https://github.com/devcell-sh/devcell)
- [Claude Code Sandbox Environments (Anthropic)](https://code.claude.com/docs/en/sandbox-environments)
- [Trail of Bits devcontainer](https://github.com/trailofbits/claude-code-devcontainer)
- [AI Agent Market Size (Polaris)](https://www.polarismarketresearch.com/industry-analysis/ai-agent-market)
