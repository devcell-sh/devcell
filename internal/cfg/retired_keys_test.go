package cfg_test

import (
	"path/filepath"
	"testing"

	"github.com/DimmKirr/devcell/internal/cfg"
)

// The libvirt engine was retired. Its keys ([cell] libvirt_uri,
// libvirt_path_map, qemu_project_sync) stay in CellSection as carriers so old
// configs keep loading, and warn via cfg.Deprecations. These tests pin that
// the keys still parse and merge until the fields are removed.

func TestLoadFile_LibvirtURI(t *testing.T) {
	dir := t.TempDir()
	writeTOML(t, dir, "devcell.toml", `
[cell]
libvirt_uri = "qemu+ssh://user@mac/session"
`)
	c, err := cfg.LoadFile(filepath.Join(dir, "devcell.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if got := c.Cell.LibvirtURI; got != "qemu+ssh://user@mac/session" {
		t.Errorf("LibvirtURI = %q, want %q", got, "qemu+ssh://user@mac/session")
	}
}

func TestMerge_LibvirtURIProjectWins(t *testing.T) {
	global := cfg.CellConfig{Cell: cfg.CellSection{LibvirtURI: "qemu+tcp://global/session"}}
	project := cfg.CellConfig{Cell: cfg.CellSection{LibvirtURI: "qemu+tcp://project/session"}}
	if got := cfg.Merge(global, project).Cell.LibvirtURI; got != "qemu+tcp://project/session" {
		t.Errorf("merged LibvirtURI = %q, want project value", got)
	}
}

func TestMerge_LibvirtURIGlobalKeptWhenProjectUnset(t *testing.T) {
	global := cfg.CellConfig{Cell: cfg.CellSection{LibvirtURI: "qemu+tcp://global/session"}}
	if got := cfg.Merge(global, cfg.CellConfig{}).Cell.LibvirtURI; got != "qemu+tcp://global/session" {
		t.Errorf("merged LibvirtURI = %q, want global value preserved", got)
	}
}

func TestLoadFile_LibvirtPathMap(t *testing.T) {
	dir := t.TempDir()
	writeTOML(t, dir, "devcell.toml", `
[cell.libvirt_path_map]
"/devcell-155" = "/Users/dmitry/dev/dimmkirr/devcell"
"/home/dmitry" = "/Users/dmitry"
`)
	c, err := cfg.LoadFile(filepath.Join(dir, "devcell.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if got := c.Cell.LibvirtPathMap["/devcell-155"]; got != "/Users/dmitry/dev/dimmkirr/devcell" {
		t.Errorf("LibvirtPathMap[/devcell-155] = %q", got)
	}
	if got := c.Cell.LibvirtPathMap["/home/dmitry"]; got != "/Users/dmitry" {
		t.Errorf("LibvirtPathMap[/home/dmitry] = %q", got)
	}
}

func TestMerge_LibvirtPathMapAccumulates(t *testing.T) {
	global := cfg.CellConfig{Cell: cfg.CellSection{LibvirtPathMap: map[string]string{
		"/home/dmitry": "/Users/dmitry",
	}}}
	project := cfg.CellConfig{Cell: cfg.CellSection{LibvirtPathMap: map[string]string{
		"/devcell-155": "/Users/dmitry/dev/dimmkirr/devcell",
	}}}
	m := cfg.Merge(global, project).Cell.LibvirtPathMap
	if m["/home/dmitry"] != "/Users/dmitry" || m["/devcell-155"] != "/Users/dmitry/dev/dimmkirr/devcell" {
		t.Errorf("merged map must accumulate both entries, got %v", m)
	}
}

func TestLoadFile_QemuProjectSync(t *testing.T) {
	dir := t.TempDir()
	writeTOML(t, dir, "devcell.toml", `
[cell]
qemu_project_sync = "off"
`)
	c, err := cfg.LoadFile(filepath.Join(dir, "devcell.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Cell.QemuProjectSync != "off" {
		t.Errorf("QemuProjectSync = %q, want off", c.Cell.QemuProjectSync)
	}
}
