# Cage: bottom bar around the cell (CELL-655) and boot popup (CELL-657)

The cage pins a one-line bar to the bottom row of the terminal while
devcell boots a cell and runs an agent (claude, codex, opencode, shell):

```
 main  devcell-437                                 UTC 21:09 | ET 17:09
```

Left: the workspace name, then the cell name, as two coloured elements.
Right: the status panel with clocks. The bar is a tmux status
line. tmux runs on the host; devcell re-executes itself inside the tmux
window, so the boot panel and then the agent both render under the bar. It is
display-only: the tmux prefix key is `None`, every keystroke reaches the
agent. The mouse is not captured (`mouse off`): selection and the wheel
stay with your terminal or outer tmux.

## Turning it on

The cage is experimental and off by default. Enable it with the shared
experimental gate, any of these, unioned:

| Where            | Setting                                        |
|------------------|------------------------------------------------|
| command line     | `devcell claude --experimental=ui-cage`       |
| `devcell.toml`   | `[cell]` `experimental = ["ui-cage"]`         |
| environment      | `DEVCELL_EXPERIMENTAL=ui-cage`                |

Several features are comma-separated: `--experimental=ui-cage,ui-boot-popup`. In
`devcell.toml` the key takes a list or the same comma-separated string:
`experimental = ["ui-cage", "ui-boot-popup"]` or `experimental = "ui-cage,ui-boot-popup"`.
The registry of experimental features is [experimental.md](experimental.md).
An unknown name stops devcell with the list of known features. The cage is
skipped when stdin or stdout is not a terminal and on `--dry-run`. `tmux`
must be installed on the host.

## Boot popup (`ui-boot-popup`, needs `ui-cage`)

With `--experimental=ui-cage,ui-boot-popup` the boot panel is not printed into
the agent window. A centred tmux popup shows it, and the popup is closed
before the agent is attached:

```
          ┌─ main  devcell-437 ──────────────────────────┐
          │ Prepare      ■ ■ ■ ■ ■ ■                      │
          │ Configure    ■ ■ ■ ■ ■                        │
          │ Boot         ■ ■ ■ ■ ■                        │
          │ Services     ■ ■ ■ ■ ■                        │
          └───────────────────────────────────────────────┘
 main  devcell-437                                 UTC 21:09 | ET 17:09
```

The identity line above the panel is dropped inside the cage since the
bar already shows it.

The popup is sized before the boot starts. Width is 76 columns, or the
terminal minus a 4-column margin. Height is estimated from the panel the
engine will draw: one row per group (docker draws four, five with the GUI
on, see `docker.BootGroupCount`), plus the two border rows, the rule under
the groups, and two rows for the "press any key" line a failed boot prints
under the rule, clamped to the terminal minus four rows
(`cage.PopupHeight`). Step and warning text is drawn on the group rows by
the streak layout, so nothing else grows below the rule.

Sequencing is enforced by ownership and acknowledgements, not timing:

- The window process owns the boot and never draws on the tty while the
  popup is up: its stdin is `/dev/null`, stdout and stderr go to
  `~/.devcell/<workspace>/.local/state/devcell/boot-<cellName>.log`. tmux is
  the only writer to the terminal; `devcell boot-ui` draws only inside the
  popup.
- Events reach `boot-ui` over a unix socket; the window process buffers them
  from the start and replays on connect, so a late popup loses nothing. EOF
  on either side means the peer is gone.
- The agent is attached only after the boot succeeded, `boot-ui`
  acknowledged the finish by closing the socket (after a short dwell so the
  completed panel is readable) and the popup client exited.
- A failed boot keeps the popup up with the reason until a key is pressed;
  the agent is never started. Ctrl+C in the popup is forwarded to the boot;
  a second Ctrl+C within two seconds force-quits the popup.
- If no `boot-ui` connects within 1.5 s (or tmux is older than 3.3, or the
  terminal is smaller than 50x12) the buffered events replay into the inline
  panel on the restored tty, exactly as without the feature.
- The flake-trust question is asked before the popup opens, because no
  prompt can be answered while the popup owns the keyboard.

## Theme

The bar has three elements, each with its own colours: the workspace name,
the cell name (`<project>-<cellID>`) and the status panel with the clocks.
`bar_*` colours the gap between them. `[theme]` in `devcell.toml` picks a
preset and overrides single colours:

```toml
[theme]
name = "devcell"          # devcell (cool tone, default), brand, nord, catppuccin-mocha, tokyo-night
workspace_bg = "#38bdf8"  # workspace name element
workspace_fg = "#0f172a"
cell_bg = "#334155"       # cell name element
cell_fg = "#e2e8f0"
status_bg = "#1e293b"     # status panel (clocks), right-aligned
status_fg = "#cbd5e1"
bar_bg = "#0f172a"        # the bar between elements
bar_fg = "#94a3b8"
accent = "#818cf8"        # copy-mode selection, messages
separator = " | "         # between status panel blocks
```

Colours are tmux colours: `#rrggbb`, `colour0`..`colour255`, or a name such
as `brightblue`. A bad colour or unknown preset is rejected before the cell
boots, because tmux would otherwise stop reading the config at that line.

## How it works

1. `devcell` resolves the theme, then writes
   `~/.devcell/<workspace>/.local/state/devcell/cage-<cellName>.tmux.conf`
   from the embedded template `internal/cage/cage.tmux.conf.tmpl`.
2. It runs `tmux -u -f <conf> -L devcell-cage-<cellName> new-session -n <agent>
   -e DEVCELL_CAGE=1 -e DEVCELL_WORKSPACE=<ws> -e DEVCELL_CELL_ID=<id> '<devcell> <same args>'`.
   The per-cell socket keeps the cage apart from your own tmux server; the
   pinned env keeps the inner run on the same cell (inside tmux, `TMUX_PANE`
   would otherwise point at the cage's pane).
3. The inner devcell sees `DEVCELL_CAGE=1`, skips the wrapping, boots the
   cell and attaches the agent as usual.
4. When the agent exits the window closes and tmux exits. tmux would print
   `[exited]` at that point and has no option to silence it, so devcell runs
   the client with stdout on `/dev/null`: tmux draws through the tty on
   stdin, and stdout only ever carries that message.

The generated config enables `allow-passthrough on` (Kitty graphics),
`extended-keys on` (shift+enter and friends), `set-clipboard on` (OSC 52),
`focus-events on`, `escape-time 0`, `default-terminal tmux-256color` and RGB,
hyperlink and underline-style terminal features.

## Trade-offs

- Scrollback: the cage runs full-screen, so the terminal's own scrollback
  does not see the agent's output, and with no prefix key and no mouse the
  cage's own history is not reachable either. Agents scroll their own views.
- The agent's exit code is not propagated; tmux exits 0 once the session ends.
- Running inside an outer tmux shows two status lines (the outer one and the
  cage). The inner prefix is `None`, so there is no key clash.

## Changing the bar

`internal/cage/cage.tmux.conf.tmpl` is the template; `clocks` in
`internal/cage/cage.go` lists the right-hand blocks; `presets` in
`internal/cage/theme.go` holds the palettes. `TestCage_RendersBarInTmux`
runs the generated config under a pty and checks the rendered bottom row;
`TestTmuxConf_AllPresetsLoadInTmux` loads every preset in tmux.
