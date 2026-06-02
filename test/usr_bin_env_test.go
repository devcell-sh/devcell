// usr_bin_env_test.go — pure image must ship /usr/bin/env.
//
// Without /usr/bin/env, any external script with the canonical
// `#!/usr/bin/env <interp>` shebang dies at exec time with
// `bad interpreter: No such file or directory` — the kernel resolves the
// shebang's first token as a literal absolute path, never via $PATH, so no
// PATH manipulation can rescue it. Claude Code plugin hooks, most PyPI/npm
// CLIs, and any user script following the standard shebang convention hit
// this on pure (nix2container) cells.
//
// Debian-based impure builds inherit /usr/bin/env from apt coreutils; the
// pure path builds from scratch and never materializes /usr/bin/ — fixed by
// adding pkgs.dockerTools.usrBinEnv to copyToRoot.

package container_test

import (
	"strings"
	"testing"
)

// TestImageNix_StagesUsrBinEnv asserts the pure image build pulls in
// pkgs.dockerTools.usrBinEnv via copyToRoot. Without this, scripts using
// `#!/usr/bin/env ...` shebangs fail in pure cells.
func TestImageNix_StagesUsrBinEnv(t *testing.T) {
	imgNix := readNixhomeFile(t, "packages/image.nix")

	if !strings.Contains(imgNix, "dockerTools.usrBinEnv") {
		t.Fatal(`image.nix doesn't include pkgs.dockerTools.usrBinEnv — pure cells will lack /usr/bin/env, breaking every script with a "#!/usr/bin/env ..." shebang (Claude hook plugins, PyPI/npm CLIs, etc.) with "bad interpreter: No such file or directory"`)
	}

	// Pin its presence in the copyToRoot block specifically — being referenced
	// elsewhere (a comment, dead code) wouldn't materialize the symlink into
	// the image rootfs.
	copyToRootStart := strings.Index(imgNix, "copyToRoot = [")
	if copyToRootStart == -1 {
		t.Fatal("copyToRoot anchor missing — image.nix shape changed; update this test")
	}
	closeOffset := strings.Index(imgNix[copyToRootStart:], "];")
	if closeOffset == -1 {
		t.Fatal("copyToRoot block not terminated — image.nix shape changed")
	}
	block := imgNix[copyToRootStart : copyToRootStart+closeOffset]
	if !strings.Contains(block, "dockerTools.usrBinEnv") {
		t.Fatal("pkgs.dockerTools.usrBinEnv is referenced in image.nix but NOT inside the copyToRoot list — symlink won't reach the image rootfs")
	}
}
