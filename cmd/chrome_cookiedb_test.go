package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// flushCookieDb is the helper invoked between Phase 1 (interactive login) exit
// and Phase 2 (headless CDP extraction) launch. Two jobs:
//   1. Force sqlite WAL checkpoint on Cookies.db so Phase 2's chromium sees a
//      consistent view (no -wal lag).
//   2. Sanity-check mtime against phase1Start so a "user closed without logging
//      in" scenario fails loud instead of saving a stale storage-state.json.

func TestFlushCookieDb_NoCookiesDb_ReturnsNoFreshnessSignal(t *testing.T) {
	profile := t.TempDir()
	mkdirAll(t, filepath.Join(profile, "Default"))
	// No Cookies file at all.

	res := flushCookieDb(profile, time.Now())

	if res.cookiesExist {
		t.Errorf("cookiesExist = true, want false (no Cookies file present)")
	}
	if res.fresh {
		t.Errorf("fresh = true with no Cookies file, want false")
	}
}

func TestFlushCookieDb_StaleCookiesDb_ReportsNotFresh(t *testing.T) {
	profile := t.TempDir()
	cookiesPath := filepath.Join(profile, "Default", "Cookies")
	mkdirAll(t, filepath.Dir(cookiesPath))
	writeFile(t, cookiesPath, []byte("fake sqlite"))
	// Set mtime to 1 hour ago.
	pastMtime := time.Now().Add(-time.Hour)
	if err := os.Chtimes(cookiesPath, pastMtime, pastMtime); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	// phase1Start = now → Cookies.db mtime is BEFORE phase1Start → stale.
	res := flushCookieDb(profile, time.Now())

	if !res.cookiesExist {
		t.Errorf("cookiesExist = false, want true (file is present)")
	}
	if res.fresh {
		t.Errorf("fresh = true, want false (mtime was an hour ago)")
	}
}

func TestFlushCookieDb_FreshCookiesDb_ReportsFresh(t *testing.T) {
	profile := t.TempDir()
	cookiesPath := filepath.Join(profile, "Default", "Cookies")
	mkdirAll(t, filepath.Dir(cookiesPath))
	// phase1Start in the past, Cookies.db touched after.
	phase1Start := time.Now().Add(-time.Minute)
	writeFile(t, cookiesPath, []byte("fake sqlite"))
	// File's mtime is now → after phase1Start → fresh.

	res := flushCookieDb(profile, phase1Start)

	if !res.fresh {
		t.Errorf("fresh = false, want true (mtime > phase1Start)")
	}
}

// Even when sqlite3 binary is missing, the helper must NOT fail the whole
// extraction — checkpoint is best-effort, freshness check still proceeds.
func TestFlushCookieDb_MissingSqlite3_DoesNotPanic(t *testing.T) {
	profile := t.TempDir()
	cookiesPath := filepath.Join(profile, "Default", "Cookies")
	mkdirAll(t, filepath.Dir(cookiesPath))
	writeFile(t, cookiesPath, []byte("fake sqlite"))

	// Force PATH that won't find sqlite3.
	origPath := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", origPath) })
	os.Setenv("PATH", "/nonexistent")

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("flushCookieDb panicked: %v", r)
		}
	}()

	res := flushCookieDb(profile, time.Now().Add(-time.Minute))
	if !res.cookiesExist {
		t.Errorf("cookiesExist = false, want true (file is present even if checkpoint failed)")
	}
}

// helpers
func mkdirAll(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o700); err != nil {
		t.Fatalf("mkdir %s: %v", p, err)
	}
}

func writeFile(t *testing.T, p string, content []byte) {
	t.Helper()
	if err := os.WriteFile(p, content, 0o600); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
}
