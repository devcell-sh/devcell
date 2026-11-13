package scan

import "testing"

func TestMergeBuild_CombinesResults(t *testing.T) {
	d := DockerResult{
		ClosureErr: nil,
	}
	n := NixResult{NixOK: true}

	b := MergeBuild(d, n)
	if !b.NixOK {
		t.Error("NixOK should be true")
	}
	if b.ClosureErr != nil {
		t.Error("ClosureErr should be nil")
	}
}

func TestMergeBuild_PropagatesNixNotOK(t *testing.T) {
	d := DockerResult{}
	n := NixResult{NixOK: false}

	b := MergeBuild(d, n)
	if b.NixOK {
		t.Error("NixOK should be false when nix scan failed")
	}
}

func TestCacheResult_Count(t *testing.T) {
	r := CacheResult{
		OldCaches: []string{"/a", "/b"},
		OldWinkit: []string{"/c"},
		HasMackit: true,
		HasOldUnif: false,
		HasUnified: true,
	}
	if got := r.Count(); got != 5 {
		t.Errorf("Count() = %d, want 5", got)
	}
}

func TestCacheResult_AllDirs(t *testing.T) {
	r := CacheResult{
		OldCaches:    []string{"/a"},
		OldWinkit:    []string{"/b"},
		OldMackit:    "/mac",
		HasMackit:    true,
		OldUnified:   "/old-uni",
		HasOldUnif:   true,
		UnifiedCache: "/uni",
		HasUnified:   false,
	}
	dirs := r.AllDirs()
	if len(dirs) != 4 {
		t.Errorf("AllDirs() = %v, want 4 entries", dirs)
	}
}

func TestDirSize_EmptyOnNonexistent(t *testing.T) {
	if got := DirSize("/nonexistent-path-for-test"); got != 0 {
		t.Errorf("DirSize on nonexistent = %d, want 0", got)
	}
}

func TestDirNonEmpty_FalseOnNonexistent(t *testing.T) {
	if DirNonEmpty("/nonexistent-path-for-test") {
		t.Error("DirNonEmpty should return false for nonexistent path")
	}
}

func TestMustCwd_ReturnsNonEmpty(t *testing.T) {
	cwd := MustCwd()
	if cwd == "" {
		t.Error("MustCwd() returned empty string")
	}
}
