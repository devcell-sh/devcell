# Remote Cell Scope Expansion: Strategic Advice

Open source project advisor assessment for CELL-654 (`--host ssh://`).
Uses perspective rotation across four OSS leadership personas.

Date: 2026-10-14
Input: FEATURES.md (#6), COMPETITION.md (remote dev table), MARKET.md
(remote dev demand), research/2026-10-14/remote-cell-competitors.md.

---

## The question

Should a pre-launch AI agent sandbox CLI (devcell, ~4 stars,
Apache-2.0) add remote cell execution via reverse SFTP mount, given
that competitors are well-funded (Coder $90M) but DevPod (~10K users)
is abandoned?

## Personas selected

Four leaders whose experience maps directly to this decision:

1. **Solomon Hykes** (Docker): built the container platform everyone
   uses, failed to monetize, learned that focus and commercial clarity
   matter more than feature breadth. Directly relevant: devcell is
   container-based and this is about expanding scope.

2. **DHH** (Rails): extracts tools from production use into open
   source. Product funds the open source, not the reverse. Relevant:
   devcell is a personal project funding social credit, not a
   VC-backed company.

3. **Mitchell Hashimoto** (HashiCorp/Ghostty): lived the full arc
   from single tool (Vagrant) to platform (Terraform+Consul+Vault+...)
   to BSL relicensing. Relevant: the Vagrant-to-platform expansion
   is the exact pattern being considered here.

4. **Evan You** (Vue/Vite): indie maintainer who stayed focused,
   expanded incrementally (Vue -> Vite -> toolchain), and kept each
   project's identity sharp. Relevant: devcell is indie, pre-launch,
   and risks blurring its identity.

## Perspective rotation

### Hykes would say: "Ship it, but don't let it become the product."

Docker's failure was not that it had too many features. It was that
Docker the product lacked focus while Docker the technology powered
everyone else's business. Hykes' lesson: "We never shipped a great
commercial product. We tried to do a little bit of everything."

Applied to devcell: adding `--host ssh://` is fine IF the framing
stays "your sandbox, on a remote machine" and NOT "a remote
development environment." The moment the pitch becomes "develop
remotely with devcell," you're competing with Coder's $90M and
Microsoft's VS Code Remote. The moment it stays "sandbox a remote
GPU box the same way you sandbox your laptop," you're extending
your existing story.

Hykes would also warn about the "kleenex problem": if devcell's
sandbox identity gets diluted by remote dev features, others will
build on your mechanism (reverse SFTP) while you chase a positioning
nobody identifies with.

**Verdict:** Ship it. Keep the sandbox framing. Don't chase the
remote dev market.

### DHH would say: "Only extract what you actually use."

DHH's extraction philosophy: the best OSS comes from real production
use, not from speculative features. Rails exists because Basecamp
needed it. Kamal exists because 37signals needed deployment. Solid
Queue exists because HEY needed background jobs.

Applied to devcell: does the maintainer (Dmitry) actually use remote
cells? Does Dmitry have a beefy remote box he SSH's into for builds
or GPU work? If yes, extracting that workflow into `--host ssh://`
is the extraction pattern at its best. If no, this is speculative
product management: adding a feature because research says the market
wants it, not because you've lived the pain.

DHH would also note the timing risk. A pre-launch project adding an
unshipped XL feature (4 story points, 4 implementation phases)
before the core product has users is "building the conference room
before the office has employees." Ship the sandbox. Get 300 stars.
Then add remote cells when users ask for it or when you need it.

**Verdict:** Ship it only if you're extracting real usage. Defer
if speculative.

### Hashimoto would say: "Scope expansion killed my first company."

HashiCorp went from Vagrant (one tool, clear identity) to 8+
products covering the entire infrastructure stack. Each expansion
was justified by market demand. The aggregate effect was that
HashiCorp became a "platform" whose identity nobody could articulate
in one sentence. This diffusion contributed to the conditions where
BSL relicensing felt necessary.

Mitchell's correction with Ghostty was severe: one product, one
purpose, donated to a non-profit. He explicitly said he blamed
"venture-backed open source in general" for creating expectations
of polished, funded, opinionated projects.

Applied to devcell: at Stage 0 (pre-launch, < 100 stars), the
playbook says to solve one problem well, not to enumerate features.
The Stage Anti-pattern Matrix flags "feature creep from enthusiastic
users" as harmful at every stage. Remote cells aren't user-driven
creep (there are no users), but they are founder-driven scope
expansion at the worst possible time: before launch.

However, Hashimoto would also note that Vagrant's SSH-based approach
to VMs was its defining innovation. If reverse SFTP is to devcell
what SSH provisioning was to Vagrant: an architectural differentiator
that makes the core product better, not a separate product, then
it's a strength.

**Verdict:** Defer to post-launch unless the reverse SFTP mechanism
is architecturally foundational (like Vagrant's SSH was).

### Evan You would say: "Progressive disclosure. Ship the flag, hide the complexity."

Vue's "progressive framework" philosophy: simple things are simple,
complex things are possible. Vue starts as a drop-in script tag and
scales to a full SPA framework. Vite starts as a dev server and
scales to a production build system. Neither leads with the complex
end.

Applied to devcell: `--host ssh://user@host` is a single flag on an
existing command. It doesn't change the default experience. It doesn't
appear in the hero. It doesn't complicate the first 90 seconds. A
senior engineer discovers it in `--help` or the README's comparison
table and thinks "huh, sharp." This is exactly the progressive
disclosure pattern.

Evan You would caution against one thing: don't let the remote cell
feature drive architectural decisions that compromise the local
experience. If supporting reverse SFTP requires a daemon, a new
dependency, or changes to how local cells work, the tail is wagging
the dog.

**Verdict:** Ship it, but as a progressive feature: simple to
discover, invisible when unused.

## Agreements

All four agree on two points:

1. **Identity must stay sharp.** "Claude Code, without the blast
   radius" must remain the one-liner. Remote cells don't appear in
   the hero, the headline, or the first paragraph of the README.
2. **The mechanism matters more than the feature.** If reverse SFTP
   is genuinely novel (and the research confirms no competitor uses
   it), it's a legitimate differentiator worth owning: but only if
   the implementation is clean and the framing stays "sandbox on a
   remote box," not "remote development platform."

## Disagreements

The split is on **timing**:

- **Hykes and Evan You:** ship it now, as a quiet progressive
  feature. It costs little to add if done right, and it pre-empts
  the whitespace before a well-funded competitor claims it.
- **DHH and Hashimoto:** defer to post-launch unless you're
  extracting real usage. A 4-phase XL feature on a pre-launch
  project with no users is premature scope expansion.

## Synthesis and recommendation

**Ship it post-launch, but design the architecture now.**

Weight: DHH and Hashimoto carry more weight here because devcell is
Stage 0. Evan You's progressive disclosure argument is compelling
but assumes the feature is cheap: a 4-phase XL implementation is not
cheap. Hykes' "ship it" instinct is right for a funded company, but
devcell is a solo project optimizing for social credit, not market
share.

### Specifically:

1. **Pre-launch (now through first 100 stars):**
   - Design the SSH/SFTP architecture. Write the internal interfaces.
   - Do NOT ship `--host` in v1. The launch must tell one story:
     "Claude Code, without the blast radius."
   - Mention remote cells exactly once in the README: in a "Coming
     soon" or "Roadmap" section, one line. This signals ambition
     without promising delivery.
   - If Dmitry personally uses remote cells in his workflow, prototype
     it in a branch. The extraction pattern requires real usage.

2. **Post-launch (100-500 stars, Stage 1-2):**
   - Ship `--host ssh://` as Phase 1 (host flag) when at least 3
     users have asked for it, OR when Dmitry has used the prototype
     for 30+ days.
   - Position it as: "Your sandbox, on a remote machine."
   - Add it to the comparison table. This is where "reverse SFTP
     mount" becomes the talking point that makes senior engineers
     say "huh, sharp."

3. **Never:**
   - Don't position devcell as a "remote development platform."
   - Don't chase DevPod's stranded users as a primary market. They
     are a bonus, not a target.
   - Don't add Mutagen, Syncthing, or copy-based sync as
     alternatives. The reverse SFTP mount is the opinion. Ship one
     mechanism, own it.

### Why this ordering

The goal is social credit with senior engineers. Social credit comes
from being sharp, not broad. A pre-launch product that claims sandbox
+ remote dev + multi-OS + MCP servers + secrets + egress control
reads as aspirational vaporware. A launched product with 300 stars
that then adds `--host ssh://` with a genuinely novel mechanism
reads as a project that knows what it is and is expanding
thoughtfully.

The DHH test: "Would you extract this from your own daily workflow?"
If yes, it's ready. If you're adding it because the market research
says the demand is there, wait until you or your users feel the pull.

### Risk assessment

| Risk | Likelihood | Mitigation |
|---|---|---|
| Competitor claims reverse SFTP whitespace first | Low (no signals) | Architecture design now, ship post-launch |
| Remote cells dilute sandbox identity | Moderate | Framing discipline: "sandbox on remote," never "remote dev" |
| 4-phase implementation delays launch | High if started now | Defer implementation, design only |
| DevPod users find devcell but bounce (no remote) | Low (they'll wait) | "Roadmap" mention in README |
| Coder adds reverse SFTP / mounts | Very low | Their business model requires Mutagen licensing |

### The one-sentence answer

Remote cells are a strong post-launch feature for a pre-launch
project: design the architecture now, ship the sandbox first, add
`--host` when you or your users feel the pull.
