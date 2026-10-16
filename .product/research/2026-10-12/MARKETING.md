# DevCell Project Marketing Improvement Plan

Based on repo-scout analysis of 10 trending GitHub repos (Oct 2026) and the AIO audit
from `.scratch/aio-audit/workflow-result.json`.

AIO (AI Optimization) covers agent-facing surfaces: pkg.go.dev, error strings, package
doc comments. This document covers human-facing surfaces: README visual appeal, badges,
GitHub metadata, community signals, and contribution onboarding.

Sources: trendshift.io, gitnova.dev, GitHub API.
Data: `.scratch/repo-scout/2026-10-11T07-41-03Z/`

## Patterns of Successful Repos

| Pattern | Count | Examples |
|---|---|---|
| Centered logo/banner in README first line | 5/10 | photocraft, moli, openGym, e2e, HowToLiveBetter |
| Badge row (CI, version, license, Discord) | 6/10 | rea, photocraft, HowToLiveBetter, openGym, e2e, skills |
| One bold value-prop sentence | 6/10 | rea, photocraft, Strata, skills, openGym, Compositor |
| GitHub description set (action verb + audience, ~12 words) | 10/10 | all |
| Topics/tags (median 9, range 0-20) | 6/10 have 7+ | rea (20), openGym (14) |
| Homepage URL set | 8/10 | all except Strata, Compositor |
| Permissive license (MIT/Apache) | 8/10 | MIT (5), Apache (3) |
| CONTRIBUTING.md or link | 4/10 | AnyPS5, openGym |
| Screenshot or video in README | 3/10 | Strata, photocraft |

Consensus first-screen formula: centered logo, one bold sentence, badge row, then
install or feature list.

## Recommendations

### Quick wins (effort: lines, one PR)

| # | Change | Where | Why |
|---|--------|-------|-----|
| 1 | Set GitHub description + topics on DimmKirr/devcell | GitHub repo settings | go.mod canonical path shows blank to agents and pkg.go.dev. 10/10 trending repos have a description. |
| 3 | Add badge row (CI status, latest release, license, Go Report Card) | README.md after title | 6/10 trending repos show badges. DevCell has 0. Signals active maintenance. |
| 6 | Add Dependabot config for gomod + github-actions | .github/dependabot.yml | e2e has `dependabot[bot]` in top contributors. Signals active dependency management. |
| 7 | Add `pull_request:` trigger to CI | .github/workflows/build.dev.yml | Fork PRs currently get no automatic test run. e2e and openGym both show CI badges. |
| 9 | Expand topics to 10+ | GitHub repo settings | rea has 20. Add: sandbox, mcp, claude-code, codex, opencode, nix, ai-sandbox, devcontainer, docker, macos-vm, windows-vm |
| 11 | Add URL pointers to unsupported-case errors | internal/engine/capability.go:169,176,212; internal/serve/responses.go:491 | 0/55 errors carry a doc URL. Append `; see https://github.com/DimmKirr/devcell/issues`. |
| 12 | Improve timeout error messages | cmd/chrome.go:414; internal/engine/docker/df_collect.go:62,66 | Some timeouts lack port, duration, or last probe error. |

### Medium effort (section or file, separate PRs)

| # | Change | Where | Why |
|---|--------|-------|-----|
| 2 | Add banner/logo image to README top | README.md line 1 | 5/10 trending repos open with a centered image. Visual anchor increases dwell time. |
| 4 | Add CONTRIBUTING.md | repo root | No clone steps, no "where things live" map, no PR expectations. No contribution path = no external PRs. |
| 5 | Add CHANGELOG.md | repo root | 25 semver tags exist but no changelog. Agents checking a `go get` cache see nothing dated. Backfill from GitHub Releases. |
| 8 | Add package doc comment to cmd/ | cmd/main.go or cmd/root.go | pkg.go.dev command page is blank. `go doc ./cmd` prints nothing. |
| 10 | Add ExampleXxx test functions | internal/engine/example_test.go, internal/cell/example_test.go | 0 example functions = blank pkg.go.dev Examples tab. |

### Ongoing effort (process)

| # | Change | Where | Why |
|---|--------|-------|-----|
| 13 | Add doc comments to undocumented exported symbols | ~20 files across internal/ | Only 74/209 (35%) exported funcs have doc comments. Lowest priority but highest volume. |

## Already OK

- README first screen has quickstart (title + one-paragraph + install code block)
- go.mod module path matches GitHub origin
- Engine extension point documented (internal/engine/engine.go)
- 25 semver tags, latest v0.9.0 (42 days old)
- Apache 2.0 license
- examples/ directory with 5 programs, tested in CI
- AGENTS.md / CLAUDE.md present

## Suggested GitHub description

> Isolated containers for AI coding agents (Claude Code, Codex, OpenCode). Your agent
> can rm -rf / and you're fine.

## Suggested topics

```
devcell sandbox docker container ai-agents claude-code codex opencode
mcp nix devcontainer macos-vm windows-vm developer-tools isolation
```
