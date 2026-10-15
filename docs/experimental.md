# Experimental features

An experimental feature is off until you name it. Three places turn one on,
and they are unioned:

| Where          | Setting                                      |
|----------------|----------------------------------------------|
| command line   | `devcell claude --experimental=ui-cage`     |
| environment    | `DEVCELL_EXPERIMENTAL=ui-cage`              |
| `devcell.toml` | `[cell]` `experimental = ["ui-cage"]`       |

Several names are comma-separated: `--experimental=ui-cage,ui-boot-popup`.
In `devcell.toml` the key takes a list or the same comma-separated string.
`devcell experimental` prints the registry with what is on in the current
directory.

Names are `<group>-<slug>`. The group is the area the feature touches: `ui`
for the terminal, `engine` for how a cell runs.

## Features

<!-- experiments:begin -->
Off by default. Turn one on with `--experimental=<name>`, `DEVCELL_EXPERIMENTAL=<name>` or `[cell] experimental = ["<name>"]`; several are comma-separated. Experimental features may change or go away without a deprecation cycle.

| Feature | What it does | Docs | Tracking |
|---|---|---|---|
| `ui-cage` | one-line bottom bar (workspace, cell name, clocks) drawn by a host tmux session around the agent | [docs/cage.md](../docs/cage.md) | CELL-655 |
| `ui-boot-popup` | boot panel in a centred tmux popup, closed before the agent starts (needs ui-cage) | [docs/cage.md](../docs/cage.md) | CELL-657 |
| `engine-windows` | Windows guest (--os windows or winpe): a Windows PE + WSL VM run by the winkit engine on vz or QEMU | [README.md#engines-and-guests](../README.md#engines-and-guests) | - |
<!-- experiments:end -->

## Guarantees

- Nothing is on by default.
- An unknown name stops devcell with the list of known names, from any of
  the three sources. A typo cannot leave a feature off.
- A code path that needs an experiment fails with the ways to enable it, for
  example `devcell claude --os windows` without `engine-windows`.
- Experimental features may change or go away without the
  [deprecation cycle](deprecation.md) that stable keys and flags get.

## Lifecycle

An experiment is **active** while it can be enabled. When it ships, it is
marked **released** and the setting becomes an error that says so: drop the
name. When it is dropped, it is **abandoned** and fails the same way. Both
stay registered so an old `devcell.toml` explains itself instead of hitting
"unknown experimental feature". Released and abandoned names are removed from
the registry one minor version later.

## Adding one

1. Register it in `internal/experiments/experiments.go`: a package-level
   `Experiment` with `Name`, `Summary`, `Doc` and `Ticket`, appended to `all`.
2. Guard the code path with `set.Enabled(experiments.X)` or
   `experiments.Require(set, experiments.X, "what")`. `cmd` resolves the set
   once with `experimental(cellCfg.Cell)`.
3. Run `task docs:experimental`. It rewrites the table above, the one in
   README.md, `web/src/data/experimental.json` and the website page; the
   `internal/experiments` test fails until the files match.
4. Mark the feature in prose with `<sup>e</sup>` linking here (README) or
   the `<Exp name="..."/>` component (website).
