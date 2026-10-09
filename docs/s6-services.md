# s6-rc Service Topology

34 services managed by s6-rc. Dependencies resolved in parallel with readiness gating.

## Dependency Graph

```mermaid
graph TD
    classDef oneshot fill:#dae8fc,stroke:#6c8ebf,color:#333
    classDef longrun fill:#d5e8d4,stroke:#82b366,color:#333,font-weight:bold
    classDef readiness fill:#d5e8d4,stroke:#82b366,color:#333,font-weight:bold,stroke-width:3px
    classDef bundle fill:#e1d5e7,stroke:#9673a6,color:#333

    user["user (bundle)"]:::bundle
    base_init["base-init"]:::oneshot

    env_setup["env-setup"]:::oneshot
    secrets["secrets"]:::oneshot
    postgres_init["postgres-init"]:::oneshot
    system_fixups["system-fixups"]:::oneshot
    pulseaudio["pulseaudio"]:::longrun
    tor["tor"]:::oneshot
    wireguard["wireguard"]:::oneshot

    shell_rc["shell-rc"]:::oneshot
    postgres["postgres"]:::longrun
    gcroot["gcroot"]:::oneshot
    nix_daemon["nix-daemon ★"]:::readiness

    homedir["homedir"]:::oneshot
    gui_config["gui-config"]:::oneshot
    claude_config["claude-config"]:::oneshot
    codex_config["codex-config"]:::oneshot
    opencode_config["opencode-config"]:::oneshot
    gemini_config["gemini-config"]:::oneshot
    antigravity["antigravity"]:::oneshot
    mise["mise"]:::oneshot
    project_flake["project-flake"]:::oneshot

    chromium_cleanup["chromium-cleanup"]:::oneshot
    mcp_toggle["mcp-toggle"]:::oneshot
    mise_packages["mise-packages"]:::oneshot
    nix_packages["nix-packages"]:::oneshot

    xvfb["xvfb ★"]:::readiness
    dbus_session["dbus-session"]:::longrun
    snixembed["snixembed"]:::longrun
    window_manager["window-manager ★"]:::readiness
    wallpaper["wallpaper"]:::oneshot
    x11vnc["x11vnc"]:::longrun
    xrdp["xrdp"]:::longrun
    xrdp_chansrv["xrdp-chansrv"]:::longrun

    user --> base_init

    base_init --> env_setup
    base_init --> secrets
    base_init --> postgres_init
    base_init --> system_fixups
    base_init --> pulseaudio
    base_init --> tor
    base_init --> wireguard
    base_init --> dbus_session

    env_setup --> shell_rc
    system_fixups --> gcroot
    system_fixups --> nix_daemon
    postgres_init --> postgres

    shell_rc --> homedir
    shell_rc --> gui_config
    shell_rc --> claude_config
    shell_rc --> codex_config
    shell_rc --> opencode_config
    shell_rc --> gemini_config
    shell_rc --> antigravity
    shell_rc --> mise
    nix_daemon --> project_flake

    homedir --> chromium_cleanup
    gui_config --> xvfb
    claude_config --> mcp_toggle
    codex_config --> mcp_toggle
    opencode_config --> mcp_toggle
    mise --> mise_packages
    project_flake --> nix_packages

    xvfb --> dbus_session
    xvfb --> window_manager
    xvfb --> x11vnc
    xvfb --> xrdp

    xrdp --> xrdp_chansrv
    dbus_session --> snixembed
    window_manager --> wallpaper

    subgraph gui ["GUI Stack (DEVCELL_GUI_ENABLED=true)"]
        xvfb
        dbus_session
        snixembed
        window_manager
        wallpaper
        x11vnc
        xrdp
        xrdp_chansrv
    end

    style gui fill:none,stroke:#82b366,stroke-dasharray:8 8,color:#333
```

> Blue = oneshot, green = longrun, purple = bundle, **★** = notification-fd=3 (readiness-gated).

## Services

| Service | Type | Platform | Readiness | Notes |
|---------|------|----------|-----------|-------|
| **base-init** | oneshot | all | | `linux/up`, `darwin/up` variants |
| **env-setup** | oneshot | Linux, WinKit | | writes `/etc/s6/env/` |
| **secrets** | oneshot | all | | |
| **postgres-init** | oneshot | all | | |
| **system-fixups** | oneshot | all | | `linux/up`, `darwin/up` variants |
| **pulseaudio** | longrun | Linux, WinKit | | |
| **tor** | oneshot | all | | |
| **wireguard** | oneshot | all | | |
| **shell-rc** | oneshot | all | | |
| **postgres** | longrun | all | | |
| **gcroot** | oneshot | all | | |
| **nix-daemon** | longrun | all | notification-fd=3 | |
| **homedir** | oneshot | all | | |
| **gui-config** | oneshot | all | | |
| **claude-config** | oneshot | all | | |
| **codex-config** | oneshot | all | | |
| **opencode-config** | oneshot | all | | |
| **gemini-config** | oneshot | all | | |
| **antigravity** | oneshot | all | | |
| **mise** | oneshot | all | | |
| **project-flake** | oneshot | all | | |
| **chromium-cleanup** | oneshot | all | | |
| **mcp-toggle** | oneshot | all | | |
| **mise-packages** | oneshot | all | | |
| **nix-packages** | oneshot | all | | |
| **xvfb** | longrun | Linux, WinKit | notification-fd=3 | `:99` |
| **dbus-session** | longrun | Linux, WinKit | | writes `DBUS_SESSION_BUS_ADDRESS` to envdir |
| **window-manager** | longrun | Linux, WinKit | notification-fd=3 | check: `xdotool search --class` |
| **snixembed** | longrun | Linux, WinKit | | reads envdir for `DBUS_SESSION_BUS_ADDRESS` |
| **x11vnc** | longrun | Linux, WinKit | | `:5900` |
| **xrdp** | longrun | Linux, WinKit | | `:3389` |
| **xrdp-chansrv** | longrun | Linux, WinKit | | |
| **wallpaper** | oneshot | Linux, WinKit | | |
| **user** | bundle | all | | `contents.d/` lists all services |

## How It Works

- **Build time**: `s6-rc-compile` (via `s6-linux-renderer.nix`) compiles source definitions from `modules/s6/` into `/etc/s6-rc/compiled/`.
- **Boot time**: entrypoint starts `s6-svscan`, runs `s6-rc-init` to link the compiled DB, then `s6-rc -u change user` brings up all services in parallel dependency order.
- **Privilege model**: s6-svscan runs as root. Services drop to session user via `gosu` in their run scripts. `chmod 0711` on supervise directories lets the session user query status with `s6-svstat`.
- **Envdir** (`/etc/s6/env/`): oneshots write env vars (e.g. `DBUS_SESSION_BUS_ADDRESS`), longruns read them via `s6-envdir`.
- **Dependencies**: declared via `dependencies.d/` directories containing empty files named after deps.

## Known Issues

1. **Compiled DB missing log pipelines** (MEDIUM): per-service s6-log definitions exist in source but aren't in the compiled output. Service stdout goes to the svscan catchall log.
2. **Oneshot up scripts use shell** (LOW): s6-rc-compile expects execline. The community-home compiler wraps them, but pure s6-rc-compile would reject.
3. **xrdp.log root-only** (LOW): session user can't read connection logs. Fixed by #1.
