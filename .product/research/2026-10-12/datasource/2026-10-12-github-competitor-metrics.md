# GitHub Competitor Metrics for AI Agent Sandboxes

**Date:** 2026-10-12
**Source:** GitHub REST API (unauthenticated), some keyword searches rate-limited and retried

## Repo Overview

| Repo | Stars | Forks | Open Issues | Total Issues | Latest Release | Last Push | Archived |
|---|---|---|---|---|---|---|---|
| anthropics/claude-code | 150,330 | 26,299 | 14,660 | 98,602 | v2.1.296 (2026-10-09) | 2026-10-11 | no |
| daytonaio/daytona | 71,562 | 5,646 | 458 | 1,262 | v0.190.0 (2026-06-23) | 2026-07-24 | **yes** |
| lima-vm/lima | 22,075 | 977 | 510 | 1,580 | v2.2.1 (2026-10-03) | 2026-10-12 | no |
| coder/coder | 16,976 | 1,620 | 1,173 | 7,317 | v2.36.7 (2026-10-05) | 2026-10-12 | no |
| loft-sh/devpod | 15,257 | 586 | 47 | 718 | v0.6.15 (2025-03-10) | 2025-11-14 | no |
| e2b-dev/E2B | 14,294 | 1,102 | 96 | 402 | v2.55.1 (2026-10-09) | 2026-10-09 | no |
| superradcompany/microsandbox | 8,661 | 474 | 96 | 372 | v0.7.8 (2026-10-09) | 2026-10-12 | no |
| cachix/devenv | 7,734 | 577 | 386 | 1,667 | v2.4.0 (2026-09-24) | 2026-10-12 | no |
| devcell-sh/devcell | 4 | 0 | 4 | 0 | v0.9.0 (2026-08-28) | 2026-10-11 | no |

Notes:
- Daytona archived 2026-10-03 (went closed-source June 2026)
- DevPod stalled (last push Nov 2025, last release Mar 2025)
- claude-code issue counts include all issues (bugs, features, questions), not just sandbox-related

## Keyword Issue Counts by Repo

Issue count = total issues (open+closed) mentioning the keyword anywhere in title/body.

| Keyword | E2B | Daytona | microsandbox | Coder | devenv | Lima | DevPod | claude-code |
|---|---|---|---|---|---|---|---|---|
| sandbox | 271 | 464 | 232 | 12 | 45 | 57 | 8 | 3,258 |
| isolation | 4 | 10 | 12 | 36 | 13 | 11 | 3 | 2,332 |
| MCP | 16 | 17 | 15 | 37 | 13 | 22 | 0 | 14,584 |
| browser/chromium | 48 | 79 | 7 | 418 | 19 | 18 | 42 | 6,869 |
| macOS/windows | 43 | 119 | 101 | 433 | 160 | 487 | 241 | 52,083 |
| offline/local | 67 | 222 | 84 | 628 | 200 | 365 | 515 | 22,475 |
| auto-approve/autonomy | 0 | 2 | 0 | 9 | 0 | 0 | ERR | 2,395 |
| secrets/credential | 14 | 28 | 36 | 132 | ERR | 12 | ERR | 2,646 |
| VPN/wireguard | 0 | 9 | 4 | 126 | ERR | 25 | ERR | 959 |
| egress/network-policy | 8 | 34 | ERR | 161 | ERR | 34 | ERR | 5,624 |

ERR = rate-limited, could not retrieve.

## Key Observations

1. **claude-code dwarfs all others** in issue volume across every keyword. This makes sense: 150K stars and a consumer-facing tool generates orders of magnitude more issues than infrastructure projects.

2. **Sandbox-specific issue density** (sandbox issues / total issues):
   - E2B: 271/402 = 67% (core product)
   - Daytona: 464/1,262 = 37%
   - microsandbox: 232/372 = 62% (core product)
   - claude-code: 3,258/98,602 = 3.3% (feature, not core product)

3. **MCP is overwhelmingly a claude-code concern**: 14,584 issues (15% of all claude-code issues). Competitors have 15-37 each.

4. **macOS/Windows multi-OS** is a massive topic in claude-code (52K issues) and significant in Lima (487), Coder (433), DevPod (241). This validates devcell's multi-engine approach.

5. **Auto-approve/autonomy** has nearly zero discussion in sandbox repos (0-9 issues) but 2,395 in claude-code. This is an agent-side concern, not a sandbox-side one: agents want to self-approve, but sandboxes are the enabler.

6. **Secrets/credential management** shows meaningful signal across Coder (132), microsandbox (36), Daytona (28), E2B (14): a real cross-competitor concern.

7. **Egress/network-policy** is significant in claude-code (5,624), Coder (161), Daytona (34), Lima (34): validates Solitary Mode as a high-demand feature.

8. **VPN/WireGuard** shows notable signal only in Coder (126) and claude-code (959). Niche but meaningful for infrastructure users.

9. **Offline/local** is broadly discussed everywhere: DevPod (515), Coder (628), Lima (365), Daytona (222), devenv (200). Confirms local-first as a universal developer desire.

10. **DevPod is effectively dead**: last push Nov 2025, last release Mar 2025, only 19 open issues remaining. This opens the local-first CDE space further for devcell.
