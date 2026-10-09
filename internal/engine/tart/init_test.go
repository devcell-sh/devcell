package tart

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitConfig_Defaults(t *testing.T) {
	c := InitConfig{HomeDir: "/home/test"}
	c.ApplyDefaults()

	if c.Username != "admin" {
		t.Errorf("Username = %q, want admin (Cirrus Labs default)", c.Username)
	}
	if c.Password != "admin" {
		t.Errorf("Password = %q, want admin (Cirrus Labs default)", c.Password)
	}
	if c.CPUs != 4 {
		t.Errorf("CPUs = %d, want 4", c.CPUs)
	}
	if c.MemoryGB != 4 {
		t.Errorf("MemoryGB = %d, want 4", c.MemoryGB)
	}
	if c.DiskGB != 64 {
		t.Errorf("DiskGB = %d, want 64", c.DiskGB)
	}
	if c.SSHPort != 22 {
		t.Errorf("SSHPort = %d, want 22", c.SSHPort)
	}
	if c.Stack != "base" {
		t.Errorf("Stack = %q, want base", c.Stack)
	}
	if c.CellName != "main" {
		t.Errorf("CellName = %q, want main", c.CellName)
	}
}

func TestInitConfig_Validate(t *testing.T) {
	c := InitConfig{CellName: "test"}
	if err := c.Validate(); err == nil {
		t.Error("expected error for missing HomeDir")
	}

	c = InitConfig{HomeDir: "/home/test"}
	if err := c.Validate(); err == nil {
		t.Error("expected error for missing CellName")
	}

	c = InitConfig{HomeDir: "/home/test", CellName: "main"}
	if err := c.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGenerateSSHKeyPair(t *testing.T) {
	dir := t.TempDir()
	pubKey, err := GenerateSSHKeyPair(dir)
	if err != nil {
		t.Fatalf("GenerateSSHKeyPair: %v", err)
	}

	if !strings.HasPrefix(pubKey, "ssh-ed25519 ") {
		t.Errorf("pubKey should start with 'ssh-ed25519 ', got %q", pubKey[:30])
	}

	privPath := filepath.Join(dir, "id_ed25519")
	if _, err := os.Stat(privPath); err != nil {
		t.Errorf("private key not found: %v", err)
	}

	pubPath := filepath.Join(dir, "id_ed25519.pub")
	if _, err := os.Stat(pubPath); err != nil {
		t.Errorf("public key not found: %v", err)
	}

	// Check file permissions
	info, _ := os.Stat(privPath)
	if info.Mode().Perm() != 0600 {
		t.Errorf("private key permissions = %o, want 0600", info.Mode().Perm())
	}
}

func TestCollectSSHPubKeys(t *testing.T) {
	dir := t.TempDir()

	// Write some .pub files
	key1 := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAItest1 user@host\n"
	key2 := "ssh-rsa AAAAB3NzaC1yc2EAAAAtest2 user@host\n"
	os.WriteFile(filepath.Join(dir, "id_ed25519.pub"), []byte(key1), 0644)
	os.WriteFile(filepath.Join(dir, "id_rsa.pub"), []byte(key2), 0644)

	// Write a private key (should be ignored: no .pub suffix)
	os.WriteFile(filepath.Join(dir, "id_ed25519"), []byte("PRIVATE"), 0600)

	// Write known_hosts (should be ignored)
	os.WriteFile(filepath.Join(dir, "known_hosts"), []byte("host key data"), 0644)

	got := CollectSSHPubKeys(dir)
	if !strings.Contains(got, "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAItest1") {
		t.Error("missing ed25519 key")
	}
	if !strings.Contains(got, "ssh-rsa AAAAB3NzaC1yc2EAAAAtest2") {
		t.Error("missing rsa key")
	}
	if strings.Contains(got, "PRIVATE") {
		t.Error("should not include private key files")
	}
	if strings.Contains(got, "host key data") {
		t.Error("should not include known_hosts")
	}
	// Each key on its own line, no trailing blank lines
	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d: %q", len(lines), got)
	}
}

func TestCollectSSHPubKeys_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	got := CollectSSHPubKeys(dir)
	if got != "" {
		t.Errorf("expected empty string for dir with no .pub files, got %q", got)
	}
}
