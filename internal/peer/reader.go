package peer

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// MemoryEntry is a single memory with its envelope metadata.
type MemoryEntry struct {
	Agent      string `json:"agent"`
	ProjectName string `json:"project_name"`
	ProjectDir  string `json:"project_dir"`
	MemoryDir   string `json:"memory_dir"`
	Timestamp   int64  `json:"timestamp"`
	Name        string `json:"name,omitempty"`
	Body        string `json:"body"`
}

// MemoryReader reads memories from one agent's native store.
type MemoryReader interface {
	// ReadAll returns all memories as structured entries.
	// Returns nil when the store is empty or missing.
	ReadAll() []MemoryEntry

	// Search returns memories matching the query (case-insensitive substring).
	// Returns nil when nothing matches.
	Search(query string) []MemoryEntry
}

// MultiReader combines multiple MemoryReaders, returning results from all.
type MultiReader struct {
	Readers []MemoryReader
}

func (m *MultiReader) ReadAll() []MemoryEntry {
	var entries []MemoryEntry
	for _, r := range m.Readers {
		entries = append(entries, r.ReadAll()...)
	}
	return entries
}

func (m *MultiReader) Search(query string) []MemoryEntry {
	var entries []MemoryEntry
	for _, r := range m.Readers {
		entries = append(entries, r.Search(query)...)
	}
	return entries
}

// Memory is a single parsed memory entry (used by ParseClaudeMemories).
type Memory struct {
	Name string
	Body string
}

// ClaudeReader reads Claude Code's markdown memory files.
type ClaudeReader struct {
	MemoryDir  string // e.g. ~/.claude/projects/<slug>/memory/
	ProjectDir string // original project directory (passed in, not derived from slug)
}

// ClaudeMemoryDir derives the memory directory path for a given project dir.
// Claude's slug format: the absolute path with "/" replaced by "-".
func ClaudeMemoryDir(home, projectDir string) string {
	slug := strings.ReplaceAll(projectDir, "/", "-")
	return filepath.Join(home, ".claude", "projects", slug, "memory")
}

func (r *ClaudeReader) ReadAll() []MemoryEntry {
	return r.read("")
}

func (r *ClaudeReader) Search(query string) []MemoryEntry {
	return r.read(query)
}

func (r *ClaudeReader) read(query string) []MemoryEntry {
	dirEntries, err := os.ReadDir(r.MemoryDir)
	if err != nil {
		return nil
	}
	q := strings.ToLower(query)
	projectDir := r.ProjectDir
	projectName := normalizeProjectName(projectDir)

	var entries []MemoryEntry
	for _, de := range dirEntries {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".md") {
			continue
		}
		if strings.EqualFold(de.Name(), "MEMORY.md") {
			continue
		}
		fullPath := filepath.Join(r.MemoryDir, de.Name())
		data, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}
		content := string(data)
		body := stripFrontmatter(content)
		if body == "" {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(body), q) &&
			!strings.Contains(strings.ToLower(de.Name()), q) {
			continue
		}

		name := parseFrontmatterField(content, "name")
		if name == "" {
			name = strings.TrimSuffix(de.Name(), ".md")
		}

		var ts int64
		if info, err := de.Info(); err == nil {
			ts = info.ModTime().Unix()
		}

		entries = append(entries, MemoryEntry{
			Agent:       "claude",
			ProjectName: projectName,
			ProjectDir:  projectDir,
			MemoryDir:   r.MemoryDir,
			Timestamp:   ts,
			Name:        name,
			Body:        body,
		})
	}
	return entries
}

// stripFrontmatter removes YAML frontmatter (content between --- delimiters
// at the start of the file).
func stripFrontmatter(content string) string {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "---") {
		return content
	}
	rest := content[3:]
	idx := strings.Index(rest, "---")
	if idx < 0 {
		return content
	}
	return strings.TrimSpace(rest[idx+3:])
}

// CodexReader reads Codex CLI's SQLite memory store.
// Uses the sqlite3 CLI binary to avoid a Go SQLite dependency.
type CodexReader struct {
	DBPath     string // e.g. ~/.codex/memories_v2_1.sqlite
	StateDB    string // e.g. ~/.codex/state_5.sqlite (for cwd join)
	ProjectDir string // when set, scope results to this project (matched by basename)
}

// CodexMemoryDB returns the path to the Codex memories database.
// Prefers the v2 database if it exists, falls back to v1.
func CodexMemoryDB(home string) string {
	v2 := filepath.Join(home, ".codex", "memories_v2_1.sqlite")
	if _, err := os.Stat(v2); err == nil {
		return v2
	}
	return filepath.Join(home, ".codex", "memories_1.sqlite")
}

// CodexStateDB returns the path to the Codex state database (threads table).
func CodexStateDB(home string) string {
	return filepath.Join(home, ".codex", "state_5.sqlite")
}

func (r *CodexReader) ReadAll() []MemoryEntry {
	return r.query("")
}

func (r *CodexReader) Search(q string) []MemoryEntry {
	return r.query(q)
}

func (r *CodexReader) query(filter string) []MemoryEntry {
	if _, err := os.Stat(r.DBPath); err != nil {
		return nil
	}

	scopeName := normalizeProjectName(r.ProjectDir)

	hasState := false
	if r.StateDB != "" {
		if _, err := os.Stat(r.StateDB); err == nil {
			hasState = true
		}
	}

	var sql string
	if hasState {
		safe := strings.ReplaceAll(r.StateDB, "'", "''")
		attach := fmt.Sprintf("ATTACH '%s' AS state; ", safe)
		where := "m.rollout_summary != ''"
		if filter != "" {
			f := strings.ReplaceAll(filter, "'", "''")
			where += fmt.Sprintf(" AND (m.rollout_summary LIKE '%%%s%%' OR m.raw_memory LIKE '%%%s%%')", f, f)
		}
		// SQL-level project scoping: filter by cwd basename when ProjectDir is set.
		if scopeName != "" {
			s := strings.ReplaceAll(scopeName, "'", "''")
			where += fmt.Sprintf(" AND t.cwd LIKE '%%/%s'", s)
		}
		sql = attach + fmt.Sprintf(
			"SELECT m.source_updated_at, COALESCE(t.cwd, ''), m.rollout_slug, m.rollout_summary "+
				"FROM stage1_outputs m "+
				"LEFT JOIN state.threads t ON t.id = m.thread_id "+
				"WHERE %s "+
				"ORDER BY m.source_updated_at DESC",
			where,
		)
	} else {
		where := "rollout_summary != ''"
		if filter != "" {
			f := strings.ReplaceAll(filter, "'", "''")
			where += fmt.Sprintf(" AND (rollout_summary LIKE '%%%s%%' OR raw_memory LIKE '%%%s%%')", f, f)
		}
		sql = fmt.Sprintf(
			"SELECT source_updated_at, '', rollout_slug, rollout_summary "+
				"FROM stage1_outputs "+
				"WHERE %s "+
				"ORDER BY source_updated_at DESC",
			where,
		)
	}

	out, err := exec.Command("sqlite3", "-separator", "\x1f", r.DBPath, sql).Output()
	if err != nil {
		return nil
	}
	text := strings.TrimSpace(string(out))
	if text == "" {
		return nil
	}

	var entries []MemoryEntry
	for _, line := range strings.Split(text, "\n") {
		fields := strings.SplitN(line, "\x1f", 4)
		if len(fields) < 4 || fields[3] == "" {
			continue
		}
		ts, _ := strconv.ParseInt(fields[0], 10, 64)
		cwd := fields[1]
		slug := fields[2]
		body := fields[3]

		projectName := normalizeProjectName(cwd)

		// Go-level post-filter: catches anything the SQL filter couldn't
		// (e.g. no state DB, NULL cwd from LEFT JOIN).
		if scopeName != "" && projectName != scopeName {
			continue
		}

		entries = append(entries, MemoryEntry{
			Agent:       "codex",
			ProjectName: projectName,
			ProjectDir:  cwd,
			MemoryDir:   r.DBPath,
			Timestamp:   ts,
			Name:        slug,
			Body:        body,
		})
	}
	return entries
}

// normalizeProjectName extracts a project name from a directory path,
// using the same logic as Claude: basename of the directory.
func normalizeProjectName(dir string) string {
	if dir == "" {
		return ""
	}
	return filepath.Base(dir)
}

// ParseClaudeMemories parses all markdown memory files from a directory
// into structured Memory entries. Useful for programmatic access.
func ParseClaudeMemories(dir string) ([]Memory, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var memories []Memory
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		if strings.EqualFold(entry.Name(), "MEMORY.md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		body := stripFrontmatter(string(data))
		if body == "" {
			continue
		}
		name := parseFrontmatterField(string(data), "name")
		if name == "" {
			name = strings.TrimSuffix(entry.Name(), ".md")
		}
		memories = append(memories, Memory{Name: name, Body: body})
	}
	return memories, nil
}

func parseFrontmatterField(content, field string) string {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "---") {
		return ""
	}
	rest := content[3:]
	idx := strings.Index(rest, "---")
	if idx < 0 {
		return ""
	}
	fm := rest[:idx]
	scanner := bufio.NewScanner(strings.NewReader(fm))
	prefix := field + ":"
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}
