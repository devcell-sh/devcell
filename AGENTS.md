# devcell — Project Instructions

## Terminology

The runtime model has seven entities. Use these words consistently in code, comments, error messages, log output, docs, and prose — a rename in one layer without the others is what made the old naming ambiguous.

- **cell** — a named, persistent identity, and a boundary: one shared `$HOME` (`~/.devcell/<cellName>/`), one network, one secrets scope. May host many projects. May be running or stopped. Defaults to `main`; override with `DEVCELL_CELL_NAME`, or inherit from `TMUX_SESSION_NAME`.
- **project** — a host directory with code, mounted into a container.
- **container** — the running docker instance for one (cell, project) pair. Ephemeral.
- **stack** — the image variant a container is built from.
- **module** — a toggleable Nix capability composed into a stack.
- **engine**: how a cell is run. One of `docker`, `tart`, `winkit` (go-winkit, which uses vz on macOS hosts and qemu on Linux hosts). Set with `--engine` or `[cell] engine`, or derived from the guest.
- **guest**: the OS image inside a cell. One of `linux`, `macos`, `winpe` (Windows PE + WSL1), `windows-full` (full Windows + WSL2, internal only for now). Set with `--os` or `[cell] os`. Each engine runs only some guests; the capability table in `internal/engine` is the one place that says which.

### Retired words

Do not use these as devcell-layer terms:

- ~~**session**~~ — tmux owns this word.
- ~~**workspace**~~ — survives only in `internal/serve/` for the MS-TSWP RDP protocol, where it is the protocol's own term. `WorkspaceResource` is still pending a rename to `Cell`.

## Git Policy

- Do NOT create commits automatically. Always ask the user to commit.
- Do NOT push to remote unless the user explicitly asks.

## TDD

Every behavioral change to `cmd/`, `internal/`, or `nixhome/modules/llm/*.nix` lands with a test that was failing before the change. Write the failing test first, implement the minimum to pass, then refactor.

Applies to: a new flag, env var, TOML key, or CLI subcommand (`cmd/*_test.go`); a new `internal/*` function with observable behavior (same package); a new MCP server or docker argv field (`internal/engine/docker/*_test.go`); a new system-prompt source (`internal/cell/*_test.go`); a change to another engine's argv, options or capability table (`internal/engine/**/*_test.go`).

No new test required for: pure refactors, docs, dependency bumps, nix module additions (nixhome now lives in `devcell-sh/community-home`), or entrypoint shell fragments (covered by `test/`).

Integration tests in `test/` split into short and long buckets by image-build cost: see [docs/testing.md](docs/testing.md).

## Deprecation workflow

Never remove or rename a user-facing TOML key, command or flag in one step. See [docs/deprecation.md](docs/deprecation.md) for the full process.

## Nix environment layout

- Nix is owned by the `devcell` user, home at `/opt/devcell` — stable, never remounted.
- The session user is `$HOST_USER`, home at `/home/$HOST_USER`, created at startup by the entrypoint.
- Nix profile path is `/opt/devcell/.local/state/nix/profiles/profile` — home-manager's native path, updated on every `home-manager switch`.
- The entrypoint copies `/opt/devcell/` dotfiles to `/home/$HOST_USER/` with `sed "s|/opt/devcell|$HOME|g"` to redirect write paths.
- Use `ln -sfT` (not `ln -sf`) when replacing a symlink-to-directory; `-T` prevents creating the link *inside* the target.
- `ENV USER=devcell` is required in the nix stage — `nix.sh` checks `[ -n "$USER" ]` and silently no-ops if empty.
- `$HOME/.config/nix/nix.conf` must carry `experimental-features = nix-command flakes` at BUILD time.

## Architecture detection in Dockerfiles

Do NOT use `ARG TARGETARCH=amd64` — the docker driver doesn't set it for host-platform builds. Use `ARCH=$(uname -m)` in `RUN` steps.

## Nix module edits

Nix modules (nixhome) now live in the standalone `devcell-sh/community-home` repo. Edits to `.nix` files in this repo are limited to `flake.nix` (the Go package build).

Escaping inside `writeShellScriptBin` (`''...''` strings) is the usual culprit:

- `${VAR}` must be `''${VAR}` (otherwise Nix interpolates it)
- `''` (empty shell string) must be `''''`
- `$VAR` without braces passes through as-is

## Go module and generated-docs hygiene

The CI **Deploy Site** workflow compiles all `cmd/*.go` together with `cmd/gendoc.go`. Three things must hold or `go build` exits 1:

1. Run `go mod tidy && go build ./...` after any dependency or import change, and commit the result. A green local build is NOT enough — CI starts from a clean module cache, so a missing `go.sum` entry only surfaces there.
2. Build-time-only tooling deps must stay anchored in `cmd/tools.go` (`//go:build tools`). `cmd/gendoc.go` is `//go:build ignore`, so `go mod tidy` can't see its `cobra/doc` import and would prune the transitive deps. Anchor any other build-ignored tool's deps there too.
3. `api/swagger/` is gitignored (swagger output) but `cmd/serve.go` imports it, so any workflow compiling `serve.go` must run `task swagger:generate` first.

After changing `go.mod`/`go.sum`, run `task nix:sync` — it resolves `flake.nix`'s `vendorHash` and stages it. The pre-commit hook only verifies.

## Disk space

If a build fails with "no space left on device":

1. Prune build cache first (safe): `docker buildx prune -af`
2. If still insufficient, **ask the user to stop old containers — never stop them yourself.** Each pins a ~13 GB untagged image with almost no layer sharing, so 2–3 usually frees ~20 GB. Then `docker image prune`.
