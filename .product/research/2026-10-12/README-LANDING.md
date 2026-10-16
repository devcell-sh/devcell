# README Landing Page Audit

Line-by-line review of README.md against product reality, published catalog, and marketing positioning (.scratch/marketing/).

Verified against:
- Codebase: `cmd/*.go`, `internal/`
- Published catalog: `devcell.sh/schema/v0.0.0/devcell-sh/home/{modules,stacks,packages}.json`
- Marketing: `.scratch/marketing/{POSITIONING,PERSONA,COMPETITORS}.md`

---

## Broken links

| Line | Link | Status | Fix |
|---|---|---|---|
| 29, 46, 219 | `./MIGRATION.md` (3 references) | **MISSING**: file does not exist in repo | Remove references or create the file |
| 135 | `https://devcell.sh/docs/cell` | **404** | Remove or point to `devcell --help` only |
| 170 | `https://github.com/devcell-sh/home` | 200, but AGENTS.md says nixhome moved to `community-home` | **Contradictory**: `devcell-sh/home` returns 200 (repo exists), but `devcell-sh/community-home` returns 404. Verify which is the canonical repo. If `home` is still correct, update AGENTS.md; if `community-home` is correct, it may be private or not yet created. |

All other links verified OK: Docker Desktop, testcontainers-go, schema URLs (modules/stacks/packages.json all return 200).

---

## Factual errors (must fix)

### 1. Line 5, 17: Missing Gemini CLI
README says: `"Run Claude Code, Codex, or OpenCode"`
Line 17: `"Works with devcell codex and devcell opencode too."`
Reality: `cmd/gemini.go` exists, `devcell gemini` is a supported command. Marketing (Bucket 2) explicitly calls out multi-agent as a differentiator.
**Fix:** Add Gemini to both lines.

### 2. Line 5: "12+ MCP servers" is now wrong
README says: `"12+ MCP servers"`
Reality: Published catalog has **25 unique MCP servers** across all modules. Even counting only those in `dev`+`ultimate` stacks, the actual count exceeds 12.
**Fix:** Update to actual count or say "25+ MCP servers."

### 3. Line 29, 46, 219: MIGRATION.md does not exist
Three separate references link to `./MIGRATION.md`. The file is not in the repo.
**Fix:** Remove references or create the file.

### 4. Line 46: Desktop description says "Fluxbox"
README stack table says `desktop` module has "Fluxbox."
Reality: Published catalog says `"IceWM/Fluxbox WM"` (IceWM is primary).
**Fix:** Update to "IceWM/Fluxbox" or just "IceWM."

### 5. Line 56: `ultimate` description is vague and incomplete
README says: `"General development, cloud tools, browser automation, desktop and Draw.io"`
Reality: `ultimate` stack includes modules: `build, desktop, go, graphics, infra, nixos, node, project-management, python, scraping`. That's Go, Node, Python runtimes, C/C++ build tools, IaC (Terraform/OpenTofu/AWS), Inkscape/GIMP/Draw.io, Linear/Hubstaff/Atlassian, Playwright, plus desktop GUI.
**Fix:** Name key tools like other rows do.

### 6. Line 63: Module table says `android` adds emulator on "x86_64 only"
This matches the catalog. OK.

### 7. Line 64: Module table lists `desktop` as "VNC, RDP, Fluxbox, PulseAudio"
Reality: Catalog says `"IceWM/Fluxbox WM, Xvfb display, VNC + RDP servers, PulseAudio, screenshot tools"`.
**Fix:** Update to include IceWM.

### 8. Line 65: Module table lists `scraping` as "Playwright stealth scripts, anti-fingerprint Chromium config"
Reality: Catalog MCP servers are `playwright` and `playwright-android`. Description says `"anti-bot Chromium for Cloudflare/Kasada-grade sites"`. The module name in the catalog is `scraping`, which matches.
Note: module is available as add-on AND baked into `dev`, `ultimate`, `bbb`, `node`, `python`, `fullstack`. The README module table implies it's add-on only.

### 9. Line 66: Module table lists `infra` as "Cloud CLI tools: AWS, GCP, Azure"
Reality: Catalog says `"IaC + Cloud: Terraform/OpenTofu, AWS CLI v2, Helm, Packer, Porter"`. **No GCP, no Azure** in the catalog.
**Fix:** Remove GCP and Azure claims, or add them to the catalog. The MCPs are: aws-api, cloudwatch, opentofu, notion-api.

### 10. Lines 93-108: MCP server table has mismatches
README lists these MCP servers with names/descriptions. Cross-check against catalog:

| README name | Catalog name | README description | Match? |
|---|---|---|---|
| OpenTofu | `opentofu` | "IaC provider/module docs" | OK |
| Yahoo Finance | `yahoo-finance` | "Stock data, financials, options" | OK |
| EdgarTools | `edgartools` | "SEC filings: 10-K, 10-Q, 8-K, XBRL" | OK |
| FRED API | `mcp-fredapi` | "800K+ US economic time series" | OK |
| Google Maps | `google-maps` | "Geocoding, routing, places, elevation, weather" | OK |
| TripIt | `tripit` | "Trip itinerary management" | OK |
| Inoreader | `inoreader` | "RSS feeds, articles, search, tagging" | OK |
| KiCad | `kicad-mcp` | "PCB analysis, netlist extraction, DRC, BOM" | OK |
| Inkscape | `inkscape-mcp` | "SVG vector graphics and DOM operations" | OK |
| Linear | `linear-server` | "Project and issue management" | OK |
| Notion | `notion-api` | "Database and page management" | OK |
| MCP-NixOS | ??? | "Nix package search and docs" | **NOT IN CATALOG** |

**Missing from README (in catalog but not listed):**
- `android` (Android MCP)
- `any-chat-completions` (call chat providers via MCP)
- `atlassian` (Jira/Confluence)
- `aws-api` (AWS API)
- `cloudwatch` (CloudWatch)
- `firefly-iii` (personal finance)
- `gimp-mcp` (GIMP raster graphics)
- `hubstaff-mcp` (time tracking)
- `mailslurp` (email testing)
- `mastodon` (social media)
- `n8n` (workflow automation)
- `playwright` (browser automation)
- `playwright-android` (mobile browser automation)
- `plex` (media server)

**In README but not in catalog:**
- `MCP-NixOS`: not found in published modules.json. May be installed separately or removed.

**Fix:** Update the table to match the catalog. Decide which are "featured" vs module-specific.

### 11. Line 126: Stale env var name
README says: `$DEVCELL_PROJECT_DIR` (`$WORKSPACE` still set for compatibility)
After terminology rename: `DEVCELL_PROJECT_DIR` is current; deprecated alias is `WORKSPACE`.
The phrasing is actually correct but the format is inconsistent with the rename. `$WORKSPACE` was the old *variable name*, not just an alias. The sentence structure implies `$WORKSPACE` = `$DEVCELL_PROJECT_DIR` which is correct.
**Verdict:** Technically OK but wording could be clearer. Low priority.

### 12. Line 170: nixhome repo link
`devcell-sh/home` returns 200 (repo exists).
AGENTS.md says nixhome lives in `devcell-sh/community-home`, but that URL returns 404.
**Fix:** Verify which is canonical. If `home` is correct, update AGENTS.md. If `community-home`, the repo may be private.

### 13. Line 203: Dev section references `nixhome/modules/`
README says: `"The image is built from composable Nix home-manager modules (nixhome/modules/)"`
Reality: This path doesn't exist in this repo. AGENTS.md says nixhome is in a separate repo.
**Fix:** Update to reference the external repo with a link.

### 14. Line 221: "Each project spawns one cell" is wrong
README says: `"a workspace is the boundary; many projects live inside it; each project spawns one cell"`
Reality: A project can have multiple cells via `DEVCELL_CELL_ID`. The identity chain is: workspace > project > cellID.
**Fix:** Change to "each project spawns one or more cells" or "each (workspace, project) pair has one default cell, optionally more via cell ID."

---

## Positioning issues (should fix)

### 15. Line 3: Hook leads with fear
`"Your AI agent can rm -rf / and you're fine."` frames DevCell as a security tool.
Positioning says lead with "lights on" (the environment, Bucket 3).
**Recommendation:** Keep as a secondary hook, lead with value.

### 16. Line 29: "4 primary stacks" but table shows 8
`base`, `dev`, `ultimate`, `bbb` are called primary. Table also lists `go`, `node`, `python`, `fullstack`, `electronics` without clear legacy marking.
**Fix:** Add "(legacy)" suffix in the table rows, or move legacy stacks to a collapsed section.

### 17. Line 42: Comparison closer uses old framing
`"Use DevCell when the agent needs full auto-approve in a cell that can't touch your host."`
Positioning says: "room with the lights on" not "wall."
**Fix:** Rewrite to mention MCPs, persistence, environment.

### 18. Line 92: MCP section misses Nix-pinning differentiator
`"Baked into the image and auto-merged into each agent's config at container startup."`
Bucket 1 differentiator is Nix pinning. The section doesn't mention it.
**Fix:** Add "Nix-pinned to commit hashes in flake.lock" to the description.

### 19. Line 30: Model ranking is secondary
SWE-Bench ranking in "What you get" dilutes the pitch. Positioning says this is "useful for retention, irrelevant for acquisition."
**Recommendation:** Move to a separate section or the dev details.

---

## Taste (consider)

### 20. Line 24: Solitary Mode too implementation-heavy
"A squid gateway sidecar proxies all egress" is internal detail.
**Recommendation:** Simplify to "locks down outbound traffic to allow-listed domains."

### 21. Line 25: Naming specific bot-detection bypasses
"pass Cloudflare and Kasada" reads adversarial.
**Recommendation:** Reword to "anti-fingerprint browser sessions" without naming specific vendors.

### 22. Lines 46-57: `bbb` stack reads as kitchen-sink
"Wine, QEMU, Swift, publishing, security/RE, Inkscape/GIMP..." A user questions focus.
**Recommendation:** Describe as "everything-in-one personal stack" or similar framing.

### 23. Lines 93-108: Non-coding MCP servers in the table
TripIt, Google Maps, FRED alongside KiCad and Linear.
**Recommendation:** Group by domain (dev tools, data, personal) or note which module enables each.

---

## Summary

| Category | Count |
|---|---|
| Broken links | 3 (MIGRATION.md x3, devcell.sh/docs/cell, nixhome ambiguity) |
| Factual errors | 14 |
| Positioning issues | 5 |
| Taste | 4 |
| **Total** | **26** |
