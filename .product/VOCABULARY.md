# Vocabulary

Single source of truth for terms used across product, code, docs, and
marketing. If a term appears in any other `.product/` doc, this is
where it's defined. Cross-reference instead of redefining.

Reconciled against AGENTS.md and codebase as of 2026-10-12 (commit
bb4a754: terminology rename to workspace/cellID/cellName).

---

## The seven runtime entities

| Entity | Definition |
|--------|-----------|
| **workspace** | A persistent grouping and boundary: one shared `$HOME` (`~/.devcell/<workspaceName>/`), one network, one secrets scope. May host many projects. May be running or stopped. Defaults to `main`; override with `DEVCELL_WORKSPACE` or inherit from `TMUX_SESSION_NAME`. |
| **project** | A host directory with code, mounted into a container. |
| **cell** | The engine-agnostic runtime unit for one (workspace, project) pair. Composed from workspace name + project name + cell ID. |
| **stack** | The image variant a container is built from (`base`, `dev`, `ultimate`, plus `go`/`node`/`python`/`fullstack`). |
| **module** | A toggleable Nix capability composed into a stack (e.g. `desktop`, `scraping`, `android`). |
| **engine** | How a cell is run. One of `docker`, `tart`, `winkit`. Set with `--engine` or `[cell] engine`, or derived from guest. |
| **guest** | The OS image inside a cell. One of `linux`, `macos`, `winpe`, `windows-full` (internal). Set with `--os` or `[cell] os`. |

## Identity chain

```
workspaceName (DEVCELL_WORKSPACE)
  > projectName (dir basename)
    > cellId (DEVCELL_CELL_ID)
      > cellName (project + cellId)
        > container name (devcell-<cellName>)
```

## Relations

- workspace **1 : N** project (a workspace hosts many projects)
- (workspace, project) **1 : 1** cell
- cell **1 : 1** container (at any moment; many over time)
- container **N : 1** stack
- container **N : M** module

## CLI verbs

| Command | What it does |
|---------|-------------|
| `devcell claude\|codex\|gemini\|opencode` | Start/attach agent in cell for (current workspace, cwd project) |
| `devcell shell [-- cmd]` | Interactive shell or one-off command inside the cell |
| `devcell init [--stack S]` | Scaffold `devcell.toml` in the current project |
| `devcell build [--stack S]` | Build or rebuild the image for a stack |
| `devcell start` / `devcell stop` | Background lifecycle for the container/VM |
| `devcell modules list` | List available nix modules with sizes and MCP servers |
| `devcell auth chrome [app]` | Browser login: export cookies for in-cell Playwright |
| `devcell models` | Detect local/cloud LLMs, show capability ratings |
| `devcell serve` | HTTP API server with optional RDP workspace feed |
| `devcell vnc` / `devcell rdp` | GUI access to the running cell |

## Env vars

### Input (host to container)

| Var | Purpose | Fallback |
|-----|---------|----------|
| `DEVCELL_WORKSPACE` | Explicit workspace identity | `TMUX_SESSION_NAME`, then `main` |
| `DEVCELL_CELL_ID` | Cell ID within workspace | `DEVCELL_BUNK` (deprecated), then tmux window |

### Output (inside a container)

| Var | Value |
|-----|-------|
| `CELL_NAME` | Cell name |
| `APP_NAME` | Project basename |
| `WORKSPACE` | Mount path inside container (`/<APP_NAME>`) |
| `DEVCELL_PROFILE` | Active stack name |
| `DEVCELL_HOST_PROJECT_DIR` | Host-side absolute path to the project |
| `DEVCELL_CONTAINER` | `1` (always set inside a cell) |
| `IS_SANDBOX` | `1` (always set inside a cell) |

### Deprecated env vars

| New | Old (deprecated) |
|-----|-----------------|
| `DEVCELL_WORKSPACE` | `DEVCELL_CELL_NAME` |
| `DEVCELL_CELL_ID` | `DEVCELL_BUNK` |

Both old vars still work (emitted alongside new ones) but should not
appear in new docs or code.

## Paths

| Path | Purpose |
|------|---------|
| `~/.devcell/<workspaceName>/` | Workspace's persistent host-side `$HOME` |
| `/<projectName>/` | Project mount inside the container |
| `/opt/devcell/` | Nix environment root (stable, never remounted) |

## Brand identity

| Name | Usage |
|------|-------|
| **DevCell** | Product name in prose |
| **devcell** | CLI binary, package name |
| **devcell.sh** | Product domain |
| **devcell-sh** | GitHub org (product) |
| **kirr.io** | Services brand (founder) |
| **kirr-io** | GitHub org (services) |

Cross-linked at footer level, never at URL level. Product brand is
neutral and standalone; services brand carries founder identity.

## Retired words

Do not use these as devcell-layer terms:

| Word | Why retired | Replacement |
|------|------------|-------------|
| ~~session~~ | tmux, Claude Code, and HTTP all own this word | cell, container |
| ~~bunk~~ | Old cell ID concept | cell ID (`DEVCELL_CELL_ID`) |

Note: `workspace` also appears in `internal/serve/` for the MS-TSWP
RDP protocol, where it is the protocol's own term. `WorkspaceResource`
is pending rename to `Cell`.

## Remote execution terms

| Term | Definition |
|------|-----------|
| **host** (remote) | An SSH-reachable machine where a cell runs when `--host ssh://user@machine` is passed. The local machine remains the project source; the remote host runs the container. Not to be confused with "host" meaning "the local machine" in container context. |
| **remote cell** | A cell whose container runs on a remote host rather than the local machine. The project directory is mounted via a reverse SFTP tunnel: the remote container reads/writes local files over the SSH connection. |
| **reverse SFTP tunnel** | The mechanism that exposes the local project directory to a remote container. The SSH connection carries an SFTP subsystem in the reverse direction: the remote container mounts local files without copying them. Distinct from copy-based sync (Mutagen, rsync, Syncthing). |

## When to use which word

| Sentence | Right word | Wrong word |
|----------|-----------|------------|
| "Your workspace is running" | workspace | cell, container |
| "Stop the current container" | container | cell, workspace |
| "Each workspace has its own `$HOME`" | workspace | cell, container |
| "The project is mounted at `/<APP_NAME>`" | project | workspace |
| "Two parallel agents share a workspace" | workspace | session |
| "The `main` workspace hosts three projects" | workspace, project | session |
| "A cell's container exited with code 1" | cell, container | session, instance |
| "Built from the `dev` stack" | stack | profile, image |

## The one-line cheat sheet

> A **workspace** is the boundary. Many **projects** live inside it.
> Each (workspace, project) pair is a **cell** that runs one
> **container**. The container is built from a **stack** with optional
> **modules**.

----

# Changes needed

## 2026-10-14: Remote execution terms added

Added "Remote execution terms" section with: host (remote), remote
cell, reverse SFTP tunnel. These terms support CELL-654 (`--host
ssh://user@host`). The "host" term is overloaded: in container
context it means "the local machine," in remote context it means
"the SSH target." The new section disambiguates. When CELL-654 ships,
add `--host` to the CLI verbs table and add example sentences to
"When to use which word."

## 2026-10-12T16:30:00Z Initial assessment

### Code-to-doc alignment

- `WorkspaceResource` in `internal/serve/` is pending rename to
  `Cell`. Track in code. When done, remove the note from "Retired
  words" section
- Verify `devcell serve` help text uses "workspace" consistently
  (RDP protocol uses "workspace" as its own term, which is now also
  our term: may cause confusion in that context)

### README alignment

- Current README terminology section says "spawns one cell" but
  should say "spawns one or more cells" (a project can have multiple
  cell IDs)
- Current README does not mention `engine` or `guest` entities.
  Decide whether to add or keep README simple

### CLI verb drift

- Verify `devcell halt` and `devcell destroy` exist in `cmd/`. These
  appear in marketing/VOCABULARY.md but may not be implemented
- `devcell login` was renamed to `devcell auth chrome`. Verify all
  docs and README reflect the current command name
- `devcell cells` and `devcell containers` listed in marketing/
  VOCABULARY.md: verify they exist or remove from vocabulary

### Brand identity (from marketing/IDENTITY.md, Jun 2026)

- `github.com/devcell` org: currently held by an empty dormant org.
  File a GitHub name-release request citing active project, matching
  domain, and zero activity. Fallback: `devcell-sh` (current, ships).
- Identity migration from DimmKirr to devcell-sh: DONE (c0a52b5,
  bb4a754). CLI is `devcell`, tap is `devcell-sh/tap/devcell`, repo
  is `devcell-sh/devcell`.
- `cell` kept as undocumented alias for power users (per IDENTITY.md).
  Verify alias still works in `cmd/`.
