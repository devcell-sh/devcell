# Contributing to DevCell

## Quick start

```bash
git clone https://github.com/DimmKirr/devcell.git
cd devcell
task test
```

`task test` runs short-mode unit tests against a pre-built image. For full integration tests (rebuilds the image first): `go test -v -timeout 600s ./test/...`

## Where things live

| Directory | What's there |
|---|---|
| `cmd/` | CLI commands (cobra). One file per subcommand. |
| `internal/engine/` | Engine implementations: `docker/`, `tart/`, `winkit/`. The `Engine` interface in `engine.go` is the extension point. |
| `internal/cfg/` | Config parsing, TOML schema, deprecation checks. |
| `internal/cell/` | Cell identity resolution, git identity. |
| `internal/serve/` | HTTP API server (OpenAI Responses API, SSE). |
| `internal/ux/` | Terminal UI: boot panel, spinners, tables. |
| `test/` | Integration tests (testcontainers-go). See `docs/testing.md`. |
| `examples/` | Example projects with `.devcell.toml` (tested in CI). |

Nix home-manager modules live in the separate [devcell-sh/community-home](https://github.com/devcell-sh/home) repo.

## Rules

**TDD**: every behavioral change to `cmd/` or `internal/` lands with a test that was failing before the change. Write the failing test first, implement the minimum to pass, then refactor.

**Deprecation**: never remove or rename a user-facing TOML key, command, or flag in one step. See `docs/deprecation.md`.

**Go module hygiene**: run `go mod tidy && go build ./...` after any dependency or import change. After changing `go.mod`/`go.sum`, run `task nix:sync` to update `flake.nix` vendorHash.

## PR expectations

Small, focused PRs are usually reviewed within a few days. One logical change per PR. If a PR touches multiple features, split it.
