package tart

import (
	"strings"
	"testing"
)

func TestSSHEnablementScript(t *testing.T) {
	script := GenerateSSHEnablementScript()
	if !strings.Contains(script, "systemsetup -setremotelogin on") {
		t.Fatalf("expected script to contain 'systemsetup -setremotelogin on', got %q", script)
	}
}

func TestSSHKeyScript(t *testing.T) {
	key := "ssh-ed25519 AAAA..."
	script := GenerateSSHKeyScript(key)
	if !strings.Contains(script, key) {
		t.Fatalf("expected script to contain the public key, got %q", script)
	}
	if !strings.Contains(script, "chmod 600") {
		t.Fatalf("expected script to contain 'chmod 600', got %q", script)
	}
}

func TestSudoersScript(t *testing.T) {
	script := GenerateSudoersScript("devcell")
	if !strings.Contains(script, "devcell ALL=(ALL) NOPASSWD: ALL") {
		t.Fatalf("expected script to contain sudoers entry, got %q", script)
	}
}

func TestNixInstallScript(t *testing.T) {
	script := GenerateNixInstallScript()
	if !strings.Contains(script, "nixos.org/nix/install") {
		t.Fatal("expected script to use official Nix installer")
	}
	if !strings.Contains(script, "--daemon") {
		t.Fatal("expected script to use multi-user (--daemon) mode")
	}
	if !strings.Contains(script, "--yes") {
		t.Fatal("expected script to pass --yes for unattended install")
	}
	if !strings.Contains(script, "set -e") {
		t.Fatal("expected script to use set -e")
	}
	if !strings.Contains(script, "nix --version") {
		t.Fatal("expected script to verify nix is available after install")
	}
	if strings.Contains(script, "determinate") {
		t.Fatal("script must NOT use the Determinate installer (conflicts with nix-darwin)")
	}
}

func TestNixDarwinActivateScript(t *testing.T) {
	script := GenerateNixDarwinActivateScript("ultimate", "/Volumes/nixhome")
	if !strings.Contains(script, "nix-darwin") {
		t.Fatalf("expected script to use nix-darwin, got %q", script)
	}
	if !strings.Contains(script, "nixhome#ultimate") {
		t.Fatalf("expected script to contain 'nixhome#ultimate', got %q", script)
	}
	if !strings.Contains(script, "nix-command flakes") {
		t.Fatal("expected script to enable nix-command and flakes experimental features")
	}
	if !strings.Contains(script, "HOME=/var/root") {
		t.Fatal("expected script to set HOME=/var/root for sudo context")
	}
}

func TestNixDarwinActivateScript_HandsOffEtcFiles(t *testing.T) {
	script := GenerateNixDarwinActivateScript("ultimate", "/Volumes/nixhome")
	for _, f := range []string{"/etc/bashrc", "/etc/zshrc", "/etc/zshenv", "/etc/zprofile", "/etc/nix/nix.conf", "/etc/shells"} {
		if !strings.Contains(script, f) {
			t.Errorf("activate script must hand off %s to nix-darwin", f)
		}
	}
}

func TestNixDarwinActivateScript_GitSafeDirectory(t *testing.T) {
	// nix's libgit2 fetcher (running as root, HOME=/var/root) refuses the
	// VirtioFS-mounted flake repo: "repository path ... is not owned by
	// current user". The script must mark it safe in root's gitconfig
	// BEFORE the nix run line.
	script := GenerateNixDarwinActivateScript("ultimate", "/Volumes/nixhome")
	safeIdx := strings.Index(script, "safe")
	if safeIdx < 0 || !strings.Contains(script, "/var/root/.gitconfig") {
		t.Fatalf("expected git safe.directory setup in /var/root/.gitconfig, got %q", script)
	}
	nixRunIdx := strings.Index(script, "nix run nix-darwin")
	if nixRunIdx >= 0 && safeIdx > nixRunIdx {
		t.Fatal("safe.directory setup must come before the nix run line")
	}
}

func TestGrantSSHdFDAScript(t *testing.T) {
	script := GenerateGrantSSHdFDAScript()

	if !strings.Contains(script, "/usr/libexec/sshd-keygen-wrapper") {
		t.Fatalf("expected script to target sshd-keygen-wrapper, got %q", script)
	}
	if !strings.Contains(script, "codesign -dr-") {
		t.Fatalf("expected script to extract code signing requirement, got %q", script)
	}
	if !strings.Contains(script, "csreq -r- -b") {
		t.Fatalf("expected script to generate csreq blob, got %q", script)
	}
	if !strings.Contains(script, "kTCCServiceSystemPolicyAllFiles") {
		t.Fatalf("expected script to grant Full Disk Access, got %q", script)
	}
	if !strings.Contains(script, "INSERT OR REPLACE") {
		t.Fatalf("expected script to use INSERT OR REPLACE (entry may exist with auth_value=0), got %q", script)
	}
	if !strings.Contains(script, "auth_value") {
		t.Fatalf("expected script to reference auth_value column, got %q", script)
	}
	if !strings.Contains(script, "killall tccd") {
		t.Fatalf("expected script to restart tccd, got %q", script)
	}
	if !strings.Contains(script, "com.apple.TCC/TCC.db") {
		t.Fatalf("expected script to target system TCC.db, got %q", script)
	}
	if !strings.Contains(script, "logger -t devcell-tcc") {
		t.Fatalf("expected script to use logger for serial console visibility, got %q", script)
	}
	if !strings.Contains(script, "grant-sshd-fda.starting") {
		t.Fatalf("expected script to emit starting sentinel, got %q", script)
	}
	if !strings.Contains(script, "grant-sshd-fda.ready") {
		t.Fatalf("expected script to emit ready sentinel, got %q", script)
	}
	if !strings.Contains(script, "grant-sshd-fda.failed") {
		t.Fatalf("expected script to emit failed sentinel on error, got %q", script)
	}
	if !strings.Contains(script, "SELECT auth_value FROM access") {
		t.Fatalf("expected script to verify grant with SELECT query, got %q", script)
	}
}

func TestVerifySSHdFDAScript(t *testing.T) {
	script := GenerateVerifySSHdFDAScript()

	if !strings.Contains(script, "/usr/libexec/sshd-keygen-wrapper") {
		t.Fatalf("expected script to check sshd-keygen-wrapper, got %q", script)
	}
	if !strings.Contains(script, "TCC_FDA_GRANTED") {
		t.Fatalf("expected script to emit TCC_FDA_GRANTED on success, got %q", script)
	}
	if !strings.Contains(script, "TCC_FDA_DENIED") {
		t.Fatalf("expected script to emit TCC_FDA_DENIED on failure, got %q", script)
	}
	if !strings.Contains(script, "com.apple.TCC/TCC.db") {
		t.Fatalf("expected script to query system TCC.db, got %q", script)
	}
}

func TestProvisionStepsOnline(t *testing.T) {
	cfg := InitConfig{CellName: "main", Stack: "ultimate", Username: "admin"}
	steps := ProvisionSteps(cfg, "ssh-ed25519 AAAA", false)

	if len(steps) != 9 {
		t.Fatalf("expected 9 online provisioning steps, got %d", len(steps))
	}

	names := make([]string, len(steps))
	for i, s := range steps {
		names[i] = s.Name
	}

	expected := []string{
		"Enable SSH",
		"Inject SSH key",
		"Configure passwordless sudo",
		"Mount home volume",
		"Prepare nix disk",
		"Install Nix",
		"Swap nix to external disk",
		"Mount nixhome",
	}
	for _, want := range expected {
		found := false
		for _, got := range names {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected step %q in online provisioning, got %v", want, names)
		}
	}

	// Verify order: Prepare nix disk → Install Nix → Swap nix to external disk
	var prepIdx, installIdx, swapIdx int
	for i, name := range names {
		if name == "Prepare nix disk" {
			prepIdx = i
		}
		if name == "Install Nix" {
			installIdx = i
		}
		if name == "Swap nix to external disk" {
			swapIdx = i
		}
	}
	if prepIdx >= installIdx {
		t.Fatalf("Prepare nix disk (idx %d) must come before Install Nix (idx %d)", prepIdx, installIdx)
	}
	if installIdx >= swapIdx {
		t.Fatalf("Install Nix (idx %d) must come before Swap nix (idx %d)", installIdx, swapIdx)
	}

	for _, name := range names {
		if name == "Create Nix volume" || name == "Install Lix" || name == "Mount nix volume" {
			t.Fatalf("old step %q should not appear in provisioning", name)
		}
	}
}

func TestProvisionStepsWithHostNix(t *testing.T) {
	cfg := InitConfig{CellName: "main", Stack: "ultimate", Username: "admin", HasHostNix: true}
	steps := ProvisionSteps(cfg, "ssh-ed25519 AAAA", false)

	if len(steps) != 10 {
		t.Fatalf("expected 10 online provisioning steps with host nix, got %d", len(steps))
	}

	names := make([]string, len(steps))
	for i, s := range steps {
		names[i] = s.Name
	}

	var swapIdx, substIdx, nixhomeIdx int
	for i, name := range names {
		if name == "Swap nix to external disk" {
			swapIdx = i
		}
		if name == "Configure host nix substituter" {
			substIdx = i
		}
		if name == "Mount nixhome" {
			nixhomeIdx = i
		}
	}
	if substIdx == 0 {
		t.Fatalf("expected 'Configure host nix substituter' step, got %v", names)
	}
	if substIdx <= swapIdx {
		t.Fatalf("host nix substituter (idx %d) must come after swap (idx %d)", substIdx, swapIdx)
	}
	if substIdx >= nixhomeIdx {
		t.Fatalf("host nix substituter (idx %d) must come before nixhome (idx %d)", substIdx, nixhomeIdx)
	}
}

func TestHostNixSubstituterScript(t *testing.T) {
	script := GenerateHostNixSubstituterScript()

	for _, want := range []string{
		"hostnix",
		"host-nix-root",
		"db.sqlite",
		"extra-substituters",
		"local?root=",
		"set -e",
		"nix-daemon",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("host nix substituter script should contain %q", want)
		}
	}
}

func TestProvisionStepsOnlineNixStepUsesOfficial(t *testing.T) {
	cfg := InitConfig{Stack: "ultimate", Username: "admin"}
	steps := ProvisionSteps(cfg, "ssh-ed25519 AAAA", false)

	for _, s := range steps {
		if s.Name == "Install Nix" {
			if !strings.Contains(s.Command, "nixos.org/nix/install") {
				t.Fatalf("Install Nix step should use official installer, got %q", s.Command)
			}
			if strings.Contains(s.Command, "determinate") {
				t.Fatal("Install Nix step must NOT use Determinate installer")
			}
			return
		}
	}
	t.Fatal("expected an 'Install Nix' step in online provisioning")
}

func TestProvisionStepsOffline(t *testing.T) {
	cfg := InitConfig{Stack: "ultimate", Username: "test"}
	steps := ProvisionSteps(cfg, "ssh-ed25519 AAAA", true)

	var hasPrep, hasInstallNix, hasSwap bool
	names := make([]string, len(steps))
	for i, s := range steps {
		names[i] = s.Name
		if s.Name == "Prepare nix disk" {
			hasPrep = true
		}
		if s.Name == "Install Nix" {
			hasInstallNix = true
		}
		if s.Name == "Swap nix to external disk" {
			hasSwap = true
		}
		if s.Name == "Create Nix volume" || s.Name == "Install Lix" || s.Name == "Mount nix volume" {
			t.Fatalf("old step %q should not appear in offline provisioning", s.Name)
		}
	}
	if !hasPrep {
		t.Fatal("expected offline provisioning to include 'Prepare nix disk' step")
	}
	if !hasInstallNix {
		t.Fatal("expected offline provisioning to include 'Install Nix' step")
	}
	if !hasSwap {
		t.Fatal("expected offline provisioning to include 'Swap nix to external disk' step")
	}
}

func TestNixDiskPrepScript(t *testing.T) {
	script := GenerateNixDiskPrepScript("main")

	for _, want := range []string{
		"DevcellNix",
		"diskutil eraseDisk JHFS+",
		".devcell.json",
		"set -e",
		`"main"`,
		"diskutil unmount",
		"Physical Store",
		"physical store for boot disk",
		"disk image",
		"BOOT_PHYS",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("prep script should contain %q", want)
		}
	}

	if strings.Contains(script, `"Nix Store"`) {
		t.Error("prep script must NOT reference old label 'Nix Store'")
	}
}

func TestNixStoreSwapScript(t *testing.T) {
	script := GenerateNixStoreSwapScript("main")

	for _, want := range []string{
		"DevcellNix",
		"rsync -a /nix/",
		"diskutil mount -mountPoint /nix",
		"nix-daemon",
		"set -e",
		"com.devcell.mount-nix",
		"fstab",
		// The installer's boot-time APFS mount daemon must be REMOVED, not just
		// booted out — bootout doesn't survive reboot, so cloned VMs would get
		// the tiny APFS "Nix Store" volume mounted over the JHFS+ DevcellNix.
		"rm -f /Library/LaunchDaemons/org.nixos.darwin-store.plist",
		// The installer's fstab entry is "UUID=... /nix apfs ..." — it contains
		// no "Nix Store" text, so the old sed pattern never matched it.
		"/nix[[:space:]]*apfs",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("swap script should contain %q", want)
		}
	}

	if strings.Contains(script, "eraseDisk") {
		t.Error("swap script should NOT format the disk (prep step does that)")
	}
	if strings.Contains(script, `"Nix Store"`) {
		t.Error("swap script must NOT reference old label 'Nix Store'")
	}
}

// TestGenerateNixShadowRepairScript covers the runtime repair for templates
// built before the swap-script fix: the installer's darwin-store daemon
// re-mounts its APFS volume over /nix at clone boot, shadowing the JHFS+
// store that holds nix-darwin profiles and s6.
func TestGenerateNixShadowRepairScript(t *testing.T) {
	script := GenerateNixShadowRepairScript()

	for _, want := range []string{
		"df /nix",                // detect which device serves /nix
		"Nix Store",              // identify the installer's APFS volume by name
		"diskutil unmount",       // remove the shadow
		"org.nixos.darwin-store", // kill the persistence vector
		"rm -f /Library/LaunchDaemons/org.nixos.darwin-store.plist",
		"/nix[[:space:]]*apfs", // purge the installer fstab entry
		"DevcellNix",           // ensure the JHFS+ disk ends up serving /nix
		"nix-daemon",           // daemon must be kicked after the swap
	} {
		if !strings.Contains(script, want) {
			t.Errorf("shadow repair script should contain %q", want)
		}
	}
	if strings.Contains(script, "set -e") {
		t.Error("repair script must be best-effort — set -e would abort the session on a healthy VM")
	}
}

func TestVirtioFSMountScript(t *testing.T) {
	script := GenerateVirtioFSMountScript("myshare", "/Volumes/myshare")
	if !strings.Contains(script, "myshare") {
		t.Fatalf("expected script to contain tag 'myshare', got %q", script)
	}
	if !strings.Contains(script, "/Volumes/myshare") {
		t.Fatalf("expected script to contain mount point '/Volumes/myshare', got %q", script)
	}
	if !strings.Contains(script, `My Shared Files/myshare`) {
		t.Fatal("expected mount script to check Apple automount path")
	}
	if !strings.Contains(script, "mount_virtiofs") {
		t.Fatal("expected mount script to try mount_virtiofs as fallback")
	}
	if !strings.Contains(script, "set -e") {
		t.Fatal("expected mount script to use set -e for early exit on failure")
	}
	if !strings.Contains(script, "ln -sfn") {
		t.Fatal("expected mount script to symlink from automount path")
	}
}

func TestProjectMountScript(t *testing.T) {
	script := GenerateProjectMountScript("project", "admin", "/Users/admin/dev/acme/devcell")
	if !strings.Contains(script, "set -e") {
		t.Fatal("expected script to use set -e")
	}
	if !strings.Contains(script, "/Users/admin/dev/acme/devcell") {
		t.Fatalf("expected script to mount at the full mirrored path, got %q", script)
	}
	if !strings.Contains(script, "mount_virtiofs") {
		t.Fatal("expected script to try mount_virtiofs as fallback")
	}
	if !strings.Contains(script, "project") {
		t.Fatal("expected script to reference VirtioFS tag 'project'")
	}
	if !strings.Contains(script, `My Shared Files/project`) {
		t.Fatal("expected script to check Apple automount path")
	}
	// Parent dirs must be created as the session user, not root — otherwise
	// the user can't create siblings under ~/dev/... later.
	if !strings.Contains(script, "sudo -u admin mkdir -p") {
		t.Fatal("expected parent dirs created as the session user")
	}
}

func TestSetupSessionHomeScriptSkipsShellRcFiles(t *testing.T) {
	// Shell rc files in CellHome are generated by the LINUX container's
	// shell-rc service and carry Linux paths (/home/<user>, /opt/devcell).
	// Symlinking them into the macOS VM home breaks every shell (e.g.
	// HISTFILE=/home/dmitry/.zsh_history does not exist on macOS). The s6
	// shell-rc service generates platform-correct ones instead.
	script := GenerateSetupSessionHomeScript("dmitry")
	for _, rc := range []string{".zshenv", ".zshrc", ".profile", ".bashrc"} {
		if !strings.Contains(script, rc) {
			t.Errorf("setup home script should explicitly skip %s (platform-specific, owned by shell-rc)", rc)
		}
	}
	if !strings.Contains(script, "continue") {
		t.Error("expected a skip (continue) branch for platform-specific rc files")
	}
}

func TestProjectPathInVM(t *testing.T) {
	// Project under host home → mirror the relative path into the VM home.
	got := ProjectPathInVM("/Users/dmitry", "/Users/dmitry/dev/devcell-sh/devcell", "dmitry")
	if got != "/Users/dmitry/dev/devcell-sh/devcell" {
		t.Errorf("under-home project: got %q, want mirrored host path", got)
	}

	// Different session user still lands under that user's VM home.
	got = ProjectPathInVM("/Users/alice", "/Users/alice/work/proj", "bob")
	if got != "/Users/bob/work/proj" {
		t.Errorf("cross-user mapping: got %q, want /Users/bob/work/proj", got)
	}

	// Project outside host home → fall back to ~/<basename>.
	got = ProjectPathInVM("/Users/dmitry", "/opt/checkouts/thing", "dmitry")
	if got != "/Users/dmitry/thing" {
		t.Errorf("outside-home project: got %q, want /Users/dmitry/thing", got)
	}
}

func TestProvisionedMarkerScript(t *testing.T) {
	script := GenerateProvisionedMarkerScript()
	if !strings.Contains(script, "/private/var/devcell-provisioned") {
		t.Fatal("expected script to write /private/var/devcell-provisioned marker (boot disk, not home)")
	}
}

func TestCheckProvisionedScript(t *testing.T) {
	script := GenerateCheckProvisionedScript()
	if !strings.Contains(script, "/private/var/devcell-provisioned") {
		t.Fatal("expected script to check /private/var/devcell-provisioned marker (boot disk, not home)")
	}
}

func TestBaseProvisionSteps(t *testing.T) {
	t.Run("without host nix", func(t *testing.T) {
		cfg := InitConfig{CellName: "main", Stack: "ultimate", Username: "admin"}
		steps := BaseProvisionSteps(cfg, "ssh-ed25519 AAAA")

		if len(steps) != 7 {
			t.Fatalf("expected 7 base steps without host nix, got %d", len(steps))
		}

		wantNames := []string{
			"Enable SSH",
			"Inject SSH key",
			"Configure passwordless sudo",
			"Mount home volume",
			"Prepare nix disk",
			"Install Nix",
			"Swap nix to external disk",
		}
		for i, want := range wantNames {
			if steps[i].Name != want {
				t.Errorf("step[%d] = %q, want %q", i, steps[i].Name, want)
			}
		}
	})

	t.Run("with host nix", func(t *testing.T) {
		cfg := InitConfig{CellName: "main", Stack: "ultimate", Username: "admin", HasHostNix: true}
		steps := BaseProvisionSteps(cfg, "ssh-ed25519 AAAA")

		if len(steps) != 8 {
			t.Fatalf("expected 8 base steps with host nix, got %d", len(steps))
		}

		if steps[7].Name != "Configure host nix substituter" {
			t.Errorf("last base step = %q, want 'Configure host nix substituter'", steps[7].Name)
		}
	})

	t.Run("password auth on first two steps", func(t *testing.T) {
		cfg := InitConfig{CellName: "main", Stack: "ultimate", Username: "admin"}
		steps := BaseProvisionSteps(cfg, "ssh-ed25519 AAAA")

		if !steps[0].NeedsPassword {
			t.Error("Enable SSH should need password auth")
		}
		if !steps[1].NeedsPassword {
			t.Error("Inject SSH key should need password auth")
		}
		for _, s := range steps[2:] {
			if s.NeedsPassword {
				t.Errorf("step %q should not need password auth", s.Name)
			}
		}
	})
}

func TestStackProvisionSteps(t *testing.T) {
	cfg := InitConfig{CellName: "main", Stack: "ultimate", Username: "admin"}
	steps := StackProvisionSteps(cfg)

	if len(steps) != 2 {
		t.Fatalf("expected 2 stack steps, got %d", len(steps))
	}

	if steps[0].Name != "Mount nixhome" {
		t.Errorf("step[0] = %q, want 'Mount nixhome'", steps[0].Name)
	}
	if !strings.Contains(steps[1].Name, "Activate nix-darwin") {
		t.Errorf("step[1] = %q, want it to contain 'Activate nix-darwin'", steps[1].Name)
	}
	if !strings.Contains(steps[1].Name, "ultimate") {
		t.Errorf("step[1] = %q, want it to contain stack name 'ultimate'", steps[1].Name)
	}
}

func TestProvisionSteps_IsBaseAndStackCombined(t *testing.T) {
	cfg := InitConfig{CellName: "main", Stack: "ultimate", Username: "admin", HasHostNix: true}
	pubKey := "ssh-ed25519 AAAA"

	all := ProvisionSteps(cfg, pubKey, false)
	base := BaseProvisionSteps(cfg, pubKey)
	stack := StackProvisionSteps(cfg)
	combined := append(base, stack...)

	if len(all) != len(combined) {
		t.Fatalf("ProvisionSteps returned %d steps, BaseProvisionSteps+StackProvisionSteps returned %d",
			len(all), len(combined))
	}
	for i := range all {
		if all[i].Name != combined[i].Name {
			t.Errorf("step[%d]: ProvisionSteps=%q, combined=%q", i, all[i].Name, combined[i].Name)
		}
	}
}

func TestBaseTemplateName(t *testing.T) {
	if BaseTemplateName != "devcell-tart-base" {
		t.Errorf("BaseTemplateName = %q, want %q", BaseTemplateName, "devcell-tart-base")
	}
}

func TestGenerateS6SessionActivateScript(t *testing.T) {
	script := GenerateS6SessionActivateScript("dmitry")

	if !strings.Contains(script, "s6-rc") {
		t.Fatal("expected script to use s6-rc for session service activation")
	}
	if !strings.Contains(script, "dmitry") {
		t.Fatal("expected script to reference the session user")
	}
	if !strings.Contains(script, "shell-rc") {
		t.Fatal("expected script to activate shell-rc service")
	}
	if !strings.Contains(script, "claude-config") {
		t.Fatal("expected script to activate claude-config service")
	}
	if !strings.Contains(script, "HOST_USER") {
		t.Fatal("expected script to set HOST_USER env var for service scripts")
	}
	if !strings.Contains(script, "SESSION_HOME") {
		t.Fatal("expected script to set SESSION_HOME env var for service scripts")
	}
	if !strings.Contains(script, "DEVCELL_HOME") {
		t.Fatal("expected script to set DEVCELL_HOME for darwin paths")
	}
	if !strings.Contains(script, "/Users/devcell") {
		t.Fatal("expected DEVCELL_HOME to point at /Users/devcell on darwin")
	}
	if !strings.Contains(script, S6EnvDir) {
		t.Fatalf("expected script to use S6EnvDir (%s)", S6EnvDir)
	}
	if !strings.Contains(script, S6ServicesDir) {
		t.Fatalf("expected script to use S6ServicesDir (%s)", S6ServicesDir)
	}
	if strings.Contains(script, "mkdir -p") && !strings.Contains(script, "sudo mkdir") {
		t.Fatal("mkdir under /etc/s6/ requires sudo")
	}
	// tart exec runs as admin (uid 501) — service up scripts write to the
	// session user's home and /nix/var, so they must run as root with the
	// exported env (HOST_USER, SESSION_HOME, DEVCELL_HOME) preserved.
	if !strings.Contains(script, `sudo -E "$S6_SVC/$svc/up"`) {
		t.Fatal("service up scripts must run via sudo -E (admin can't write session-user home)")
	}
}
