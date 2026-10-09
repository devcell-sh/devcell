// Package s6 provides shared s6 service management primitives used by all
// three devcell engines (Docker/Linux, Tart/macOS, WinKit/Windows).
//
// Services are defined in the community-home repo (modules/s6/) and compiled
// at image build time by s6-rc-compile into a binary database. At boot,
// s6-svscan starts the supervision tree and s6-rc brings services up in
// dependency order with parallel execution and readiness gating.
package s6

import "fmt"

// Standard paths, shared across all engines.
const (
	EnvDir      = "/etc/s6/env"
	SourceDir   = "/etc/s6-rc/source"
	CompiledDir = "/etc/s6-rc/compiled"
	ScanDir     = "/run/service"
	LiveDir     = "/run/s6-rc"
	ReadyFile   = "/run/devcell-session-ready"

	// ServicesDir is the legacy path where nix renderers install service
	// source definitions. Kept as an alias for callers that log it.
	// Deprecated: use SourceDir for s6-rc source, CompiledDir for compiled DB.
	ServicesDir = SourceDir
)

// SessionEnv is the standard env var set written to the s6 envdir before
// activating session services. Every s6 service script reads these via
// `s6-envdir /etc/s6/env`.
type SessionEnv struct {
	HostUser    string // host $USER, e.g. "dmitry"
	SessionHome string // session user's home, e.g. "/Users/dmitry" or "/home/dmitry"
	DevcellHome string // nix profile owner's home, e.g. "/Users/devcell" or "/opt/devcell"
}

// DarwinSessionEnv builds a SessionEnv for macOS (Tart engine).
// cellUser is the session user (e.g. "dmitry"), vmUser owns the nix
// profile (e.g. "devcell").
func DarwinSessionEnv(cellUser, vmUser string) SessionEnv {
	return SessionEnv{
		HostUser:    cellUser,
		SessionHome: "/Users/" + cellUser,
		DevcellHome: "/Users/" + vmUser,
	}
}

// LinuxSessionEnv builds a SessionEnv for Docker/Linux containers.
func LinuxSessionEnv(hostUser string) SessionEnv {
	return SessionEnv{
		HostUser:    hostUser,
		SessionHome: "/home/" + hostUser,
		DevcellHome: "/opt/devcell",
	}
}

// WSL1SessionEnv builds a SessionEnv for WinKit's WSL1 distro.
func WSL1SessionEnv(hostUser string) SessionEnv {
	home := "/home/" + hostUser
	if hostUser == "root" {
		home = "/root"
	}
	return SessionEnv{
		HostUser:    hostUser,
		SessionHome: home,
		DevcellHome: "/opt/devcell",
	}
}

// WSL1ExecPATH returns the PATH string for commands executed in a WSL1
// distro. Mirrors Docker's EntrypointSnippet PATH layout but omits
// Docker-specific entries (/run/wrappers/bin, devcell-tools).
func WSL1ExecPATH(env SessionEnv) string {
	return env.SessionHome + "/go/bin:" +
		env.SessionHome + "/.local/state/nix/profiles/profile/bin:" +
		env.SessionHome + "/.local/state/nix/profiles/project/bin:" +
		env.SessionHome + "/.local/share/mise/shims:" +
		env.SessionHome + "/.local/bin:" +
		env.DevcellHome + "/.local/state/nix/profiles/profile/bin:" +
		env.DevcellHome + "/.local/bin:" +
		"/nix/var/nix/profiles/default/bin:" +
		"/usr/local/bin:/usr/bin:/bin"
}

// LinuxExecPATH returns the PATH string for docker exec sessions.
// Mirrors the PATH set by EntrypointSnippet so exec'd processes have
// the same tool visibility as the entrypoint-launched CMD.
func LinuxExecPATH(env SessionEnv) string {
	return env.SessionHome + "/go/bin:" +
		env.SessionHome + "/.local/state/nix/profiles/profile/bin:" +
		env.SessionHome + "/.local/state/nix/profiles/project/bin:" +
		env.SessionHome + "/.local/share/mise/shims:" +
		env.SessionHome + "/.local/bin:" +
		"/run/wrappers/bin:" +
		"/nix/var/nix/profiles/devcell-tools/bin:" +
		env.DevcellHome + "/.local/state/nix/profiles/profile/bin:" +
		env.DevcellHome + "/.local/bin:" +
		"/nix/var/nix/profiles/default/bin:" +
		"/usr/local/bin:/usr/bin:/bin"
}

// WriteEnvScript returns a shell snippet that writes env to EnvDir.
// Each variable becomes a single file (s6 envdir format).
func WriteEnvScript(env SessionEnv) string {
	return fmt.Sprintf(`sudo mkdir -p %s
echo %q | sudo tee %s/HOST_USER > /dev/null
echo %q | sudo tee %s/SESSION_HOME > /dev/null
echo %q | sudo tee %s/DEVCELL_HOME > /dev/null`,
		EnvDir,
		env.HostUser, EnvDir,
		env.SessionHome, EnvDir,
		env.DevcellHome, EnvDir)
}

// EntrypointSnippet returns a shell snippet for Docker's init to activate
// s6 session services. Runs as root (no sudo), does not source nix-daemon
// profile (container image PATH is already set up).
//
// Boot sequence:
//  1. Write s6 envdir vars
//  2. Create session user
//  3. Start s6-svscan supervision tree (background)
//  4. s6-rc-init: link compiled DB into live state
//  5. s6-rc change user: bring up all services (parallel deps, readiness)
//  6. Drop to session user via gosu
func EntrypointSnippet(env SessionEnv) string {
	return fmt.Sprintf(`S6_ENV=%q
S6_COMPILED=%q
S6_SCAN=%q
S6_LIVE=%q
HOST_USER=%q
SESSION_HOME=%q
DEVCELL_HOME=%q

# Explicit PATH so the script does not depend on the image ENV.
# Session-user paths (go/bin, mise shims, user nix profiles) are included
# so the CMD after privilege drop inherits them even when it is not a shell.
export PATH="$SESSION_HOME/go/bin:$SESSION_HOME/.local/state/nix/profiles/profile/bin:$SESSION_HOME/.local/state/nix/profiles/project/bin:$SESSION_HOME/.local/share/mise/shims:$SESSION_HOME/.local/bin:/run/wrappers/bin:/nix/var/nix/profiles/devcell-tools/bin:$DEVCELL_HOME/.local/state/nix/profiles/profile/bin:$DEVCELL_HOME/.local/bin:/nix/var/nix/profiles/default/bin:/usr/local/bin:/usr/bin:/bin"

_notify() {
  local _name="$1" _msg="${2:-}"
  # Sentinel files (backward compat with BootDirWatcher)
  if [ -n "$DEVCELL_BOOT_DIR" ] && [ -d "$DEVCELL_BOOT_DIR" ]; then
    if [ -n "$_msg" ]; then
      echo "$_msg" > "$DEVCELL_BOOT_DIR/$_name" 2>/dev/null || true
    else
      touch "$DEVCELL_BOOT_DIR/$_name" 2>/dev/null || true
    fi
  fi
  # JSONL system event to docker logs (suppressed when PID 1 stdout is a TTY)
  case "$(readlink /proc/1/fd/1 2>/dev/null)" in
    /dev/pts/*) return 0 ;;
  esac
  local _svc="${_name%%.*}" _status="${_name#*.}"
  local _ts; _ts=$(date -u +%%Y-%%m-%%dT%%H:%%M:%%SZ)
  if [ -n "$_msg" ]; then
    printf '{"ts":"%%s","scope":"system","service":"%%s","status":"%%s","msg":"%%s"}\n' "$_ts" "$_svc" "$_status" "$_msg"
  else
    printf '{"ts":"%%s","scope":"system","service":"%%s","status":"%%s"}\n' "$_ts" "$_svc" "$_status"
  fi
}

_notify entrypoint.ready

# 1. Write s6 envdir vars
mkdir -p "$S6_ENV"
echo "$HOST_USER" > "$S6_ENV/HOST_USER"
echo "$SESSION_HOME" > "$S6_ENV/SESSION_HOME"
echo "$DEVCELL_HOME" > "$S6_ENV/DEVCELL_HOME"
echo "$DEVCELL_BOOT_DIR" > "$S6_ENV/DEVCELL_BOOT_DIR"

# 2. Create session user (gosu prerequisite)
if ! id "$HOST_USER" >/dev/null 2>&1; then
  chmod 644 /etc/passwd /etc/shadow /etc/group 2>/dev/null || true
  useradd -m -s /bin/zsh "$HOST_USER" 2>/dev/null || true
  echo "$HOST_USER ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers
fi

# 3. Essential directories
mkdir -p "$SESSION_HOME/.local/bin" "$SESSION_HOME/go/bin" "$SESSION_HOME/tmp"
chown -h "$HOST_USER" "$SESSION_HOME/.local" "$SESSION_HOME/.local/bin" \
    "$SESSION_HOME/go" "$SESSION_HOME/go/bin" "$SESSION_HOME/tmp" 2>/dev/null || true

# 4. s6 services via s6-rc (idiomatic: svscan + rc-init + rc change)
if [ -d "$S6_COMPILED" ]; then
  _notify s6.starting

  mkdir -p "$S6_SCAN"

  # Redirect service output so longruns don't flood the agent terminal.
  S6_LOG="/var/log/s6"
  mkdir -p "$S6_LOG"

  if ! command -v s6-svscan >/dev/null 2>&1; then
    _notify s6.warn "s6 binaries not on PATH, skipping activation"
  else
    # Start supervision tree (s6-svscan supervises all longruns)
    s6-svscan "$S6_SCAN" >"$S6_LOG/svscan.log" 2>&1 &

    # Initialize s6-rc live state. -t 5000: bail after 5s if svscan
    # hasn't created the supervision tree (prevents infinite hang).
    if s6-rc-init -t 5000 -c "$S6_COMPILED" -l "$S6_LIVE" "$S6_SCAN"; then
      # Bring up all services in dependency order (parallel, readiness-aware).
      # -t 30000: 30s global timeout so boot never hangs on a service
      # that crash-loops or can't reach readiness.
      if s6-rc -l "$S6_LIVE" -t 30000 -u change user >>"$S6_LOG/rc-change.log" 2>&1; then
        :
      else
        _notify s6.warn "Service activation failed (non-fatal, see $S6_LOG/)"
      fi
    else
      _notify s6.warn "rc-init failed (non-fatal, see $S6_LOG/)"
    fi
  fi
  # Let the session user query service status via s6-svstat.
  # supervise/ dirs default to 0700/root; 0711 lets non-root enter
  # without listing, matching s6's read-only status model.
  for d in "$S6_SCAN"/*/supervise; do
    chmod 0711 "$d" 2>/dev/null || true
  done
  _notify s6.ready
fi

# 5. Fix ownership for root-created files, signal ready
find "$SESSION_HOME" -maxdepth 2 -user root -not -path "*/tmp/*" \
  -exec chown "$HOST_USER" {} + 2>/dev/null || true
touch %q

_notify boot.ready

# 6. Drop to session user, hand off to CMD
exec gosu "$HOST_USER" "$@"`,
		EnvDir, CompiledDir, ScanDir, LiveDir,
		env.HostUser, env.SessionHome, env.DevcellHome,
		ReadyFile)
}

// ActivateScript returns a shell script for Tart/macOS and WinKit engines.
// Same s6-rc boot sequence as EntrypointSnippet but with sudo (runs as
// non-root admin user) and nix-daemon profile sourcing.
func ActivateScript(env SessionEnv) string {
	return fmt.Sprintf(`set -e
export HOST_USER=%q
export SESSION_HOME=%q
export DEVCELL_HOME=%q

# Source nix-daemon profile so s6/s6-rc are on PATH (tart exec runs as admin
# whose default PATH doesn't include /run/current-system/sw/bin).
. /nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh 2>/dev/null || true
export PATH="/run/current-system/sw/bin${PATH:+:}${PATH}"

S6_ENV=%q
S6_COMPILED=%q
S6_SCAN=%q
S6_LIVE=%q

if command -v s6-rc >/dev/null 2>&1 && [ -d "$S6_COMPILED" ]; then
  sudo mkdir -p "$S6_ENV"
  echo "$HOST_USER" | sudo tee "$S6_ENV/HOST_USER" > /dev/null
  echo "$SESSION_HOME" | sudo tee "$S6_ENV/SESSION_HOME" > /dev/null
  echo "$DEVCELL_HOME" | sudo tee "$S6_ENV/DEVCELL_HOME" > /dev/null
  echo "${DEVCELL_BOOT_DIR:-}" | sudo tee "$S6_ENV/DEVCELL_BOOT_DIR" > /dev/null
  echo "s6: activating session services for $HOST_USER"

  sudo mkdir -p "$S6_SCAN"

  # Start supervision tree
  sudo s6-svscan "$S6_SCAN" &

  # Initialize s6-rc live state. -t 5000: bail if svscan not ready in 5s.
  if sudo s6-rc-init -t 5000 -c "$S6_COMPILED" -l "$S6_LIVE" "$S6_SCAN"; then
    # Bring up all services in dependency order
    if sudo s6-rc -l "$S6_LIVE" -t 30000 -u change user; then
      echo "s6: session services activated"
    else
      echo "s6: service activation failed (non-fatal)"
    fi
  else
    echo "s6: rc-init failed (non-fatal)"
  fi

  sudo touch %q
  echo "s6: session ready"
else
  echo "s6: s6-rc not available — skipping session activation"
fi`,
		env.HostUser,
		env.SessionHome,
		env.DevcellHome,
		EnvDir,
		CompiledDir, ScanDir, LiveDir,
		ReadyFile)
}
