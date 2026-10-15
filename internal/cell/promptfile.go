package cell

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/devcell-sh/devcell/internal/cfg"
	"github.com/devcell-sh/devcell/internal/config"
	"github.com/devcell-sh/devcell/internal/ux"
)

// promptDirName is the project-relative directory holding generated prompt
// files. It sits under .devcell/ because that tree is gitignored build
// output — these files are transport, regenerated on every launch, never
// hand-edited.
const promptDirName = "prompts"

// OverlayPromptFile is the generated file carrying container context plus the
// resolved operator prompt — everything that layers *on top of* whichever base
// prompt is in effect.
const OverlayPromptFile = "additional-systemprompt.md"

// BasePromptFile is the generated file carrying the base prompt — the one
// that REPLACES Claude Code's built-in prompt.
const BasePromptFile = "system-prompt.md"

// WriteOverlayPrompt assembles the overlay and materializes it, returning the
// container path for --append-system-prompt-file.
//
// Container context is always present, so this file is always written even
// when no append prompt is configured.
func WriteOverlayPrompt(c config.Config, cellCfg cfg.CellConfig, opts ResolveOpts) (string, error) {
	content, err := AssembleOverlayPrompt(c, cellCfg, opts)
	if err != nil {
		return "", err
	}
	return WritePromptFile(c, OverlayPromptFile, content)
}

// WriteBasePrompt materializes the base prompt, returning the container path
// for --system-prompt-file — or "" when no base is configured.
//
// The empty return is the switch that keeps this opt-in: with no base set,
// the caller emits no flag and Claude Code's stock prompt stays in effect.
// The content is written verbatim; container context belongs on the overlay.
func WriteBasePrompt(c config.Config, opts ResolveOpts) (string, error) {
	content, err := ResolveSystemPrompt(opts)
	if err != nil {
		return "", err
	}
	if content == "" {
		return "", nil
	}
	return WritePromptFile(c, BasePromptFile, content)
}

// WritePromptFile materializes prompt content to disk and returns the path
// the *container* will read it at.
//
// Prompts used to travel as a single argv element. That capped them at
// MAX_ARG_STRLEN (128 KiB on Linux) and published their full text to
// `ps aux` and `docker inspect`. Writing a file and passing its path
// removes both limits — claude reads it via --system-prompt-file /
// --append-system-prompt-file.
//
// Files are namespaced per cell. .devcell/ is per project, but a container is
// per (cell, project) pair and ContainerContext embeds the cell name, so a
// shared path would let one cell boot with another cell's container context.
//
// The returned path is the container-side path. Prompt files live under the
// workspace home (not the project dir) because the project bind mount
// inherits the host directory's permissions. On Linux with a UID mismatch
// (host UID != container UID) and a private home dir (0700), the container
// session user cannot traverse the project mount at all.
//
// The workspace home is mounted at /home/<HostUser>, and the entrypoint
// chowns it to the session user, so files there are always accessible.
func WritePromptFile(c config.Config, name, content string) (string, error) {
	cell := c.WorkspaceName
	if cell == "" {
		cell = "main"
	}

	hostDir := filepath.Join(c.WorkspaceHome, promptDirName, cell)
	if err := os.MkdirAll(hostDir, 0o755); err != nil {
		return "", fmt.Errorf("create prompt dir %s: %w", hostDir, err)
	}

	hostPath := filepath.Join(hostDir, name)
	if err := os.WriteFile(hostPath, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write prompt file %s: %w", hostPath, err)
	}

	ensureWorldReadable(hostDir, hostPath)

	containerPath := path.Join("/home/"+c.HostUser, promptDirName, cell, name)

	ux.Debugf("prompt file: host=%s container=%s", hostPath, containerPath)
	if info, err := os.Stat(hostPath); err == nil {
		ux.Debugf("prompt file: size=%d mode=%o", info.Size(), info.Mode().Perm())
	}

	return containerPath, nil
}

// ensureWorldReadable makes sure the file and its parent directories are
// accessible by any UID. Inside the container the session user's UID may
// differ from the host UID that wrote the file (e.g. macOS 501 vs Linux
// 1000), so the "other" permission bits must allow traversal and reading.
func ensureWorldReadable(dir, file string) {
	fixDir := func(p string) {
		info, err := os.Stat(p)
		if err != nil {
			return
		}
		m := info.Mode()
		if m&0o005 != 0o005 {
			_ = os.Chmod(p, m|0o005)
		}
	}
	fixDir(dir)
	fixDir(filepath.Dir(dir))
	fixDir(filepath.Dir(filepath.Dir(dir)))

	info, err := os.Stat(file)
	if err != nil {
		return
	}
	m := info.Mode()
	if m&0o004 == 0 {
		_ = os.Chmod(file, m|0o004)
	}
}
