package peer

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestStripFrontmatter(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"no frontmatter", "just body text", "just body text"},
		{"with frontmatter", "---\nname: foo\ntype: feedback\n---\n\nBody here.", "Body here."},
		{"unclosed frontmatter", "---\nname: foo\nno closing", "---\nname: foo\nno closing"},
		{"empty body", "---\nname: foo\n---\n", ""},
		{"empty input", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripFrontmatter(tt.in)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseFrontmatterField(t *testing.T) {
	content := "---\nname: my-memory\ndescription: a test\nmetadata:\n  type: feedback\n---\n\nBody."
	if got := parseFrontmatterField(content, "name"); got != "my-memory" {
		t.Errorf("name: got %q, want %q", got, "my-memory")
	}
	if got := parseFrontmatterField(content, "description"); got != "a test" {
		t.Errorf("description: got %q, want %q", got, "a test")
	}
	if got := parseFrontmatterField("no frontmatter", "name"); got != "" {
		t.Errorf("no frontmatter: got %q, want empty", got)
	}
}

func TestClaudeReader_ReadAll(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "MEMORY.md", "- [Foo](foo.md) - index entry\n")
	writeFile(t, dir, "foo.md", "---\nname: foo\n---\n\nFoo body.")
	writeFile(t, dir, "bar.md", "---\nname: bar\n---\n\nBar body.")
	writeFile(t, dir, "not-md.txt", "should be ignored")

	r := &ClaudeReader{MemoryDir: dir, ProjectDir: "/home/user/dev/myproject"}
	entries := r.ReadAll()

	if len(entries) == 0 {
		t.Fatal("ReadAll returned empty")
	}
	bodies := entriesToBodies(entries)
	if !containsAll(bodies, "Foo body.", "Bar body.") {
		t.Errorf("missing expected bodies in: %s", bodies)
	}
	if containsAny(bodies, "index entry", "should be ignored") {
		t.Errorf("should not contain MEMORY.md or non-md files: %s", bodies)
	}
	for _, e := range entries {
		if e.Agent != "claude" {
			t.Errorf("agent should be claude, got %q", e.Agent)
		}
		if e.Timestamp == 0 {
			t.Errorf("timestamp should be set for %q", e.Name)
		}
	}
}

func TestClaudeReader_Search(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "feedback.md", "---\nname: feedback-style\n---\n\nUse terse responses.")
	writeFile(t, dir, "project.md", "---\nname: project-arch\n---\n\nThe API uses gRPC.")

	r := &ClaudeReader{MemoryDir: dir, ProjectDir: "/home/user/dev/myproject"}

	entries := r.Search("terse")
	bodies := entriesToBodies(entries)
	if len(entries) == 0 || !containsAll(bodies, "terse responses") {
		t.Errorf("search 'terse': got %q", bodies)
	}
	if containsAny(bodies, "gRPC") {
		t.Errorf("search 'terse' should not return gRPC entry: %s", bodies)
	}
}

func TestClaudeReader_MissingDir(t *testing.T) {
	r := &ClaudeReader{MemoryDir: "/nonexistent/path", ProjectDir: "/nonexistent"}
	entries := r.ReadAll()
	if len(entries) != 0 {
		t.Errorf("missing dir: got %d entries, want 0", len(entries))
	}
}

func TestClaudeReader_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	r := &ClaudeReader{MemoryDir: dir, ProjectDir: "/tmp/test"}
	entries := r.ReadAll()
	if len(entries) != 0 {
		t.Errorf("empty dir: got %d entries, want 0", len(entries))
	}
}

func TestClaudeReader_SkipsFrontmatterOnlyFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "empty-body.md", "---\nname: empty\n---\n")

	r := &ClaudeReader{MemoryDir: dir, ProjectDir: "/tmp/test"}
	entries := r.ReadAll()
	if len(entries) != 0 {
		t.Errorf("frontmatter-only file: got %d entries, want 0", len(entries))
	}
}

func TestParseClaudeMemories(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "MEMORY.md", "index\n")
	writeFile(t, dir, "style.md", "---\nname: coding-style\n---\n\nPrefer short functions.")
	writeFile(t, dir, "no-name.md", "No frontmatter at all.")

	memories, err := ParseClaudeMemories(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(memories) != 2 {
		t.Fatalf("got %d memories, want 2", len(memories))
	}
	found := map[string]string{}
	for _, m := range memories {
		found[m.Name] = m.Body
	}
	if found["coding-style"] != "Prefer short functions." {
		t.Errorf("coding-style body: %q", found["coding-style"])
	}
	if _, ok := found["no-name"]; !ok {
		t.Error("expected no-name.md to use filename as name")
	}
}

func TestClaudeMemoryDir(t *testing.T) {
	got := ClaudeMemoryDir("/home/user", "/CELL/Users/user/dev/myproject")
	want := "/home/user/.claude/projects/-CELL-Users-user-dev-myproject/memory"
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestCodexMemoryDB_PrefersV2(t *testing.T) {
	home := t.TempDir()
	codexDir := filepath.Join(home, ".codex")
	os.MkdirAll(codexDir, 0o755)

	// No DB files: falls back to v1 path
	got := CodexMemoryDB(home)
	want := filepath.Join(home, ".codex", "memories_1.sqlite")
	if got != want {
		t.Errorf("no DBs: got %s, want %s", got, want)
	}

	// Only v1 exists: returns v1
	os.WriteFile(filepath.Join(codexDir, "memories_1.sqlite"), []byte{}, 0o644)
	got = CodexMemoryDB(home)
	if got != want {
		t.Errorf("v1 only: got %s, want %s", got, want)
	}

	// Both exist: returns v2
	os.WriteFile(filepath.Join(codexDir, "memories_v2_1.sqlite"), []byte{}, 0o644)
	got = CodexMemoryDB(home)
	wantV2 := filepath.Join(home, ".codex", "memories_v2_1.sqlite")
	if got != wantV2 {
		t.Errorf("both exist: got %s, want %s", got, wantV2)
	}
}

func TestMultiReader_ReadAll(t *testing.T) {
	a := &stubMemReader{all: []MemoryEntry{{Agent: "claude", Body: "Claude memory"}}}
	b := &stubMemReader{all: []MemoryEntry{{Agent: "codex", Body: "Codex memory"}}}
	m := &MultiReader{Readers: []MemoryReader{a, b}}

	entries := m.ReadAll()
	bodies := entriesToBodies(entries)
	if !containsAll(bodies, "Claude memory", "Codex memory") {
		t.Errorf("ReadAll missing content: %s", bodies)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
}

func TestMultiReader_ReadAll_SkipsEmpty(t *testing.T) {
	a := &stubMemReader{}
	b := &stubMemReader{all: []MemoryEntry{{Agent: "codex", Body: "Codex memory"}}}
	m := &MultiReader{Readers: []MemoryReader{a, b}}

	entries := m.ReadAll()
	if len(entries) != 1 || entries[0].Body != "Codex memory" {
		t.Errorf("got %v, want single Codex entry", entries)
	}
}

func TestMultiReader_Search(t *testing.T) {
	a := &stubMemReader{search: map[string][]MemoryEntry{"test": {{Agent: "claude", Body: "Claude test memory"}}}}
	b := &stubMemReader{search: map[string][]MemoryEntry{"test": {{Agent: "codex", Body: "Codex test memory"}}}}
	m := &MultiReader{Readers: []MemoryReader{a, b}}

	entries := m.Search("test")
	bodies := entriesToBodies(entries)
	if !containsAll(bodies, "Claude test memory", "Codex test memory") {
		t.Errorf("Search missing content: %s", bodies)
	}
}

func TestMultiReader_AllEmpty(t *testing.T) {
	a := &stubMemReader{}
	b := &stubMemReader{}
	m := &MultiReader{Readers: []MemoryReader{a, b}}

	if entries := m.ReadAll(); len(entries) != 0 {
		t.Errorf("all empty ReadAll: got %d entries", len(entries))
	}
	if entries := m.Search("anything"); len(entries) != 0 {
		t.Errorf("all empty Search: got %d entries", len(entries))
	}
}

type stubMemReader struct {
	all    []MemoryEntry
	search map[string][]MemoryEntry
}

func (r *stubMemReader) ReadAll() []MemoryEntry { return r.all }
func (r *stubMemReader) Search(q string) []MemoryEntry {
	if r.search != nil {
		return r.search[q]
	}
	return nil
}

func TestCodexReader_MissingDB(t *testing.T) {
	r := &CodexReader{DBPath: "/nonexistent/memories_1.sqlite"}
	entries := r.ReadAll()
	if len(entries) != 0 {
		t.Errorf("missing db: got %d entries, want 0", len(entries))
	}
}

func TestCodexReader_ProjectScope(t *testing.T) {
	if _, err := exec.LookPath("sqlite3"); err != nil {
		t.Skip("sqlite3 not available")
	}

	dir := t.TempDir()
	memDB := filepath.Join(dir, "memories_v2_1.sqlite")
	stateDB := filepath.Join(dir, "state_5.sqlite")

	// Create state DB with threads table (cwd per thread).
	run(t, "sqlite3", stateDB,
		"CREATE TABLE threads (id TEXT PRIMARY KEY, cwd TEXT);"+
			"INSERT INTO threads VALUES ('t1', '/home/user/dev/myproject');"+
			"INSERT INTO threads VALUES ('t2', '/home/user/dev/other-project');"+
			"INSERT INTO threads VALUES ('t3', '');") // empty cwd

	// Create memory DB with stage1_outputs.
	run(t, "sqlite3", memDB,
		"CREATE TABLE stage1_outputs (thread_id TEXT, source_updated_at INTEGER, rollout_slug TEXT, rollout_summary TEXT, raw_memory TEXT);"+
			"INSERT INTO stage1_outputs VALUES ('t1', 100, 'mem-a', 'memory from myproject', '');"+
			"INSERT INTO stage1_outputs VALUES ('t2', 200, 'mem-b', 'memory from other-project', '');"+
			"INSERT INTO stage1_outputs VALUES ('t3', 300, 'mem-c', 'memory with no cwd', '');")

	t.Run("unscoped returns all", func(t *testing.T) {
		r := &CodexReader{DBPath: memDB, StateDB: stateDB}
		entries := r.ReadAll()
		if len(entries) != 3 {
			t.Errorf("unscoped: got %d entries, want 3", len(entries))
		}
	})

	t.Run("scoped returns only matching project", func(t *testing.T) {
		r := &CodexReader{DBPath: memDB, StateDB: stateDB, ProjectDir: "/CELL/Users/user/dev/myproject"}
		entries := r.ReadAll()
		if len(entries) != 1 {
			t.Fatalf("scoped: got %d entries, want 1", len(entries))
		}
		if entries[0].Name != "mem-a" {
			t.Errorf("scoped: got name %q, want mem-a", entries[0].Name)
		}
	})

	t.Run("scoped search filters by project and keyword", func(t *testing.T) {
		r := &CodexReader{DBPath: memDB, StateDB: stateDB, ProjectDir: "/home/user/dev/other-project"}
		entries := r.Search("other")
		if len(entries) != 1 {
			t.Fatalf("scoped search: got %d entries, want 1", len(entries))
		}
		if entries[0].Name != "mem-b" {
			t.Errorf("scoped search: got name %q, want mem-b", entries[0].Name)
		}
	})

	t.Run("scoped excludes entries with empty cwd", func(t *testing.T) {
		r := &CodexReader{DBPath: memDB, StateDB: stateDB, ProjectDir: "/home/user/dev/myproject"}
		entries := r.ReadAll()
		for _, e := range entries {
			if e.Name == "mem-c" {
				t.Error("scoped should exclude entries with empty cwd")
			}
		}
	})

	t.Run("scoped with no state DB falls back to post-filter", func(t *testing.T) {
		r := &CodexReader{DBPath: memDB, ProjectDir: "/home/user/dev/myproject"}
		entries := r.ReadAll()
		// Without state DB, cwd is always empty, so all get excluded by post-filter.
		if len(entries) != 0 {
			t.Errorf("scoped no state: got %d entries, want 0", len(entries))
		}
	})
}

func run(t *testing.T, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}

func entriesToBodies(entries []MemoryEntry) string {
	var parts []string
	for _, e := range entries {
		parts = append(parts, e.Body)
	}
	return strings.Join(parts, "\n---\n")
}

func TestClaudeReader_EnvelopeFields(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "style.md", "---\nname: coding-style\n---\n\nPrefer short functions.")

	r := &ClaudeReader{MemoryDir: dir, ProjectDir: "/CELL/Users/user/dev/myproject"}
	entries := r.ReadAll()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Agent != "claude" {
		t.Errorf("agent: got %q, want claude", e.Agent)
	}
	if e.Name != "coding-style" {
		t.Errorf("name: got %q, want coding-style", e.Name)
	}
	if e.ProjectName != "myproject" {
		t.Errorf("project_name: got %q, want myproject", e.ProjectName)
	}
	if e.ProjectDir != "/CELL/Users/user/dev/myproject" {
		t.Errorf("project_dir: got %q", e.ProjectDir)
	}
	if e.MemoryDir != dir {
		t.Errorf("memory_dir: got %q, want %q", e.MemoryDir, dir)
	}
	if e.Timestamp == 0 {
		t.Error("timestamp should be nonzero")
	}
	if e.Body != "Prefer short functions." {
		t.Errorf("body: got %q", e.Body)
	}
}

func TestNormalizeProjectName(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"/CELL/Users/dmitry/dev/devcell-sh/devcell", "devcell"},
		{"/home/user/projects/my-app", "my-app"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := normalizeProjectName(tt.in); got != tt.want {
			t.Errorf("normalizeProjectName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !contains(s, sub) {
			return false
		}
	}
	return true
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if contains(s, sub) {
			return true
		}
	}
	return false
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && searchString(s, sub)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
