# Headline research: competitor and peer taglines (vendor-named vs category)

**Date:** 2026-10-15. Headlines fetched today unless noted.

| Product | Headline / tagline | Names a vendor agent in h1? | Where agents are named | Stars / signal |
|---|---|---|---|---|
| Docker Sandboxes | "Docker Sandboxes" / "Run agents safely, local to cloud." | No | Body: "Run Claude Code, Codex, and your other agents at full speed in MicroVM environments." Logos: Kiro, Cursor, Devin, Gemini, Copilot, Warp | Docker-backed |
| Daytona | "Run AI Code." / "Secure and Elastic Infrastructure for Running Your AI-Generated Code." | No | Not in hero | $24M Series A |
| E2B | "E2B — The AI Agent Cloud" | No | Not in hero | 15M+ sandboxes/mo |
| Trail of Bits devcontainer | "Claude Code in a devcontainer" / "Sandboxed devcontainer for running Claude Code in bypass mode safely." | Yes | Claude-only | 951 stars |
| textcortex/claude-code-sandbox | "Run Claude Code safely in local Docker containers without having to approve every permission" | Yes | Claude-only, archived | 323 stars |
| mattolson/agent-sandbox | "Secure local dev environment for collaboration with AI coding agents" | No | Generic | 210 stars |
| anthropics/sandbox-runtime | "A lightweight sandboxing tool for enforcing filesystem and network restrictions" | No | Anthropic's own | 5,550 stars |
| kubernetes-sigs/agent-sandbox | "isolated, stateful, singleton" agent sandboxes | No | Platform | 4,260 stars |
| agent-infra/sandbox | "All-in-One Sandbox for AI Agents" | No | Platform | 6,162 stars |
| Omnara (Launch HN) | "Run Claude Code and Codex from anywhere" | Yes (two vendors) | Title | 147 pts |
| Runtime (Launch HN) | "Sandboxed coding agents for everyone on a team" | No | Title | 103 pts |
| Agent37 (Launch HN) | "Low-cost agent sandboxes for running agents" | No | Title | 160 pts |

## Pattern

- Platforms and infrastructure (E2B, Daytona, Docker, K8s SIG, agent-infra) use category words. Their buyer is a platform builder, not the Alex persona.
- Developer-facing local wrappers name the agent. The two best-known local wrappers for Alex (Trail of Bits, textcortex) are both Claude-named. The generic local wrapper (mattolson) has a fifth of the stars.
- Docker's hero is the template for a hedged position: category verb in the h1 ("Run agents safely"), vendor names in the first body sentence. devcell already does the inverse: vendor name in h1, category sentence in the body.
- Nobody in the set uses "agentic coding" in a hero. The phrase appears in analyst and vendor long-form content (Anthropic trends report, Google Cloud, Sourcegraph), not in taglines.

## Emotional register

- Capability/speed: E2B, Daytona, Agent37, Docker ("full speed").
- Safety/relief: Trail of Bits ("safely"), textcortex ("safely"), devvy-container ("sleep soundly"), devcell ("without the blast radius"). Relief register stays mostly unclaimed by funded players; it is where devcell sits.
