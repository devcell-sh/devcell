package cfg

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

// MigrationChange is one rewrite MigrateTOML applied.
type MigrationChange struct {
	Key    string // the deprecated key as the user wrote it, e.g. "[cell] qemu_ssh_port"
	Action string // what happened to it, e.g. `renamed to [cell] winkit_ssh_port`
}

// MigrationResult is the outcome of MigrateTOML: the rewritten text and
// what changed. Changes is empty when nothing was deprecated.
type MigrationResult struct {
	Before  string
	After   string
	Changes []MigrationChange
}

// MigrateTOML rewrites every deprecated key in src to its current spelling,
// keeping comments, ordering and unrelated lines. The result is loaded the
// same way LoadFile loads a config, and an error is returned (with no
// result) when it fails to parse or still carries a deprecation, so callers
// never write a config that is worse than the one they started from.
//
// Rewrites follow the Deprecations table plus the forms it cannot express:
// [[volumes]] tables collapse into [cell] volumes, use_ollama and
// use_openrouter become [llm] provider, a "provider/" prefix on [llm] model
// is split off, and [op], [packages.npm] and [llm.models.providers] are
// renamed as whole tables. A key already present under its new name wins;
// arrays under both names are concatenated.
func MigrateTOML(src string) (MigrationResult, error) {
	// A config that does not load today cannot be migrated safely: the
	// rewrite would guess at conflicting values. The load error names the
	// problem, so the user fixes that first.
	if _, err := loadBytes([]byte(src), ""); err != nil {
		return MigrationResult{}, fmt.Errorf("config does not load, fix it before migrating: %w", err)
	}
	doc := parseTOMLDoc(src)
	var changes []MigrationChange
	llmTouched := false
	note := func(key, action string) {
		changes = append(changes, MigrationChange{Key: key, Action: action})
		if strings.HasPrefix(key, "[llm") {
			llmTouched = true
		}
	}

	// Table renames first: later key moves address the new table names.
	for _, r := range tableRenames {
		if doc.renameTable(r.from, r.to) {
			note("["+strings.Join(r.from, ".")+"]", "renamed to ["+strings.Join(r.to, ".")+"]")
		}
	}
	if vals := doc.removeVolumeTables(); len(vals) > 0 {
		doc.setOrMergeArray([]string{"cell"}, "volumes", vals)
		note("[[volumes]]", "moved to [cell] volumes")
	}
	for _, d := range Deprecations {
		if !doc.has(d.Path) {
			continue
		}
		switch {
		case d.Replacement == "(none)":
			doc.remove(d.Path)
			note(d.Name, "removed")
		case d.Path[0] == "llm" && (d.Path[1] == "use_ollama" || d.Path[1] == "use_openrouter"):
			provider := strings.TrimPrefix(d.Path[1], "use_")
			if e := doc.entry(d.Path); e != nil && strings.TrimSpace(stripComment(e.value)) == "true" {
				if !doc.has([]string{"llm", "provider"}) {
					doc.section([]string{"llm"}, true).insertAfter(e, "provider", fmt.Sprintf("%q", provider))
				}
			}
			doc.remove(d.Path)
			note(d.Name, fmt.Sprintf("replaced by [llm] provider = %q", provider))
		case len(d.Path) == 1 || d.Replacement == "[llm.providers]":
			// whole-table forms handled above (tableRenames); nothing left to do
		default:
			to := parseKeyName(d.Replacement)
			if to == nil {
				continue
			}
			doc.move(d.Path, to)
			note(d.Name, "renamed to "+d.Replacement)
		}
	}
	if e := doc.entry([]string{"llm", "model"}); e != nil {
		if pfx, rest, ok := splitProviderPrefix(stripComment(e.value)); ok {
			p := doc.entry([]string{"llm", "provider"})
			switch {
			case p == nil:
				// The prefix identifies the model's home provider but does
				// not activate it (matches runtime semantics in migrateLLM).
				e.setValue(fmt.Sprintf("%q", rest))
				note(`[llm] model = "`+pfx+`/..."`, fmt.Sprintf("prefix stripped, model = %q (provider stays default)", rest))
			case strings.Trim(strings.TrimSpace(stripComment(p.value)), `"`) == pfx:
				e.setValue(fmt.Sprintf("%q", rest))
				note(`[llm] model = "`+pfx+`/..."`, fmt.Sprintf("prefix dropped, model = %q", rest))
			}
		}
	}
	// A migrated [llm] that still says nothing about the provider gets it
	// spelled out as "default": the legacy keys left it implicit, and an
	// explicit value is what the current syntax asks for.
	if e := doc.entry([]string{"llm", "model"}); llmTouched && e != nil && doc.entry([]string{"llm", "provider"}) == nil {
		doc.section([]string{"llm"}, true).insertBefore(e, "provider", fmt.Sprintf("%q", LLMProviderDefault))
		note("[llm] provider", fmt.Sprintf("set provider = %q (the old keys did not name one)", LLMProviderDefault))
	}
	if n := doc.refreshComments(); n > 0 {
		note("comments", fmt.Sprintf("%d commented-out %s rewritten to the current syntax", n, pluralize(n, "example")))
	}
	doc.dropEmptySections()

	res := MigrationResult{Before: src, After: doc.String(), Changes: changes}
	if len(changes) == 0 {
		res.After = src
		return res, nil
	}
	c, err := loadBytes([]byte(res.After), "")
	if err != nil {
		return MigrationResult{}, fmt.Errorf("migrated config does not load: %w", err)
	}
	if len(c.DeprecatedUses) > 0 {
		names := make([]string, len(c.DeprecatedUses))
		for i, u := range c.DeprecatedUses {
			names[i] = u.Name
		}
		return MigrationResult{}, fmt.Errorf("migration left deprecated keys in place: %s", strings.Join(names, ", "))
	}
	return res, nil
}

// ErrReadOnly marks a MigrateFile error for a file that has changes but
// cannot be written (a read-only bind mount inside a cell, say). Nothing is
// written and no backup is made; callers can skip the file with a hint
// instead of failing.
var ErrReadOnly = errors.New("file is read-only")

// ReadOnlySymlinkError is the ErrReadOnly for a path that is a symlink into
// a location that cannot be written: home-manager links
// ~/.config/devcell/devcell.toml into /nix/store, so the file cannot be
// rewritten in place on the host either. Target names the link's
// destination so the user can be pointed at the source that generates it.
type ReadOnlySymlinkError struct {
	Path   string
	Link   string // the first hop, what `readlink` prints
	Target string // the fully resolved file
}

func (e *ReadOnlySymlinkError) Error() string {
	return fmt.Sprintf("%s: symlink to read-only %s", e.Path, e.Target)
}

func (e *ReadOnlySymlinkError) Unwrap() error { return ErrReadOnly }

// HomeManaged reports whether the link goes through a home-manager output
// in the nix store. home-manager links in two hops, first into
// <hash>-home-manager-files/ and from there to the per-file store path,
// so the fully resolved Target does not name home-manager; the first hop
// does.
func (e *ReadOnlySymlinkError) HomeManaged() bool {
	for _, p := range []string{e.Link, e.Target} {
		if strings.Contains(p, "/nix/store/") && strings.Contains(p, "home-manager-files") {
			return true
		}
	}
	return false
}

// MigrateFile runs MigrateTOML on path. With write set, a changed file is
// first copied to "<path>.bak-<UTC timestamp>" and then overwritten; the
// backup path is returned. A missing file, a dry run, or a file with
// nothing to migrate returns no backup. A file that cannot be opened for
// writing returns ErrReadOnly (a *ReadOnlySymlinkError when the path is a
// symlink) before any backup is made; the result still carries the changes
// so callers can show what the user has to apply by hand.
func MigrateFile(path string, write bool) (MigrationResult, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return MigrationResult{}, "", err
	}
	res, err := MigrateTOML(string(data))
	if err != nil {
		return MigrationResult{}, "", fmt.Errorf("%s: %w", path, err)
	}
	if !write || len(res.Changes) == 0 {
		return res, "", nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return MigrationResult{}, "", err
	}
	// Probe writability first so a read-only file leaves no stray backup.
	if f, err := os.OpenFile(path, os.O_WRONLY, 0); err != nil {
		if errors.Is(err, os.ErrPermission) || errors.Is(err, syscall.EROFS) {
			if target, lerr := filepath.EvalSymlinks(path); lerr == nil && target != path {
				link, _ := os.Readlink(path)
				return res, "", &ReadOnlySymlinkError{Path: path, Link: link, Target: target}
			}
			return res, "", fmt.Errorf("%s: %w", path, ErrReadOnly)
		}
		return MigrationResult{}, "", err
	} else {
		_ = f.Close()
	}
	backup := fmt.Sprintf("%s.bak-%s", path, time.Now().UTC().Format("20060102T150405Z"))
	if err := os.WriteFile(backup, data, info.Mode().Perm()); err != nil {
		return MigrationResult{}, "", fmt.Errorf("write backup: %w", err)
	}
	if err := os.WriteFile(path, []byte(res.After), info.Mode().Perm()); err != nil {
		return MigrationResult{}, "", fmt.Errorf("write %s: %w", path, err)
	}
	return res, backup, nil
}

func pluralize(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

// tableRenames are deprecations of a whole table, applied to it and every
// subtable. Kept next to the Deprecations entries that describe them.
var tableRenames = []struct{ from, to []string }{
	{[]string{"op"}, []string{"secrets", "onepassword"}},
	{[]string{"packages", "npm"}, []string{"packages", "node"}},
	{[]string{"llm", "models", "providers"}, []string{"llm", "providers"}},
}

// parseKeyName turns a Deprecation.Replacement such as "[cell] winkit_ssh_port"
// into its path. Whole-table replacements ("[llm.providers]") return nil.
func parseKeyName(s string) []string {
	m := replacementRe.FindStringSubmatch(s)
	if m == nil {
		return nil
	}
	return append(strings.Split(m[1], "."), m[2])
}

var replacementRe = regexp.MustCompile(`^\[([A-Za-z0-9_.\-]+)\]\s+([A-Za-z0-9_\-]+)$`)

func splitProviderPrefix(v string) (pfx, rest string, ok bool) {
	v = strings.TrimSpace(v)
	if len(v) < 2 || v[0] != '"' || v[len(v)-1] != '"' {
		return "", "", false
	}
	pfx, rest, ok = strings.Cut(v[1:len(v)-1], "/")
	if !ok || (pfx != LLMProviderOllama && pfx != LLMProviderOpenRouter) {
		return "", "", false
	}
	return pfx, rest, true
}

// ── minimal comment-preserving TOML document model ─────────────────────

type tomlEntry struct {
	key   string
	value string   // raw text after "=" on the first line, comment included
	lines []string // the entry's raw lines: first line plus any continuation
}

func (e *tomlEntry) setKey(k string) {
	e.key = k
	e.lines[0] = keyLineRe.ReplaceAllString(e.lines[0], "${1}"+k+"${3}${4}")
}

func (e *tomlEntry) setValue(v string) {
	e.value = v
	e.lines = []string{e.key + " = " + v}
}

type tomlItem struct {
	entry *tomlEntry
	raw   string // comment or blank line when entry is nil
}

type tomlSection struct {
	header string   // raw header line, "" for the root
	path   []string // nil for the root
	array  bool     // [[table]]
	items  []tomlItem
}

type tomlDoc struct {
	sections []*tomlSection
}

var (
	headerRe  = regexp.MustCompile(`^\s*(\[\[?)\s*([^\]]+?)\s*\]\]?\s*(#.*)?$`)
	keyLineRe = regexp.MustCompile(`^(\s*)([A-Za-z0-9_\-]+|"[^"]*")(\s*=\s*)(.*)$`)
)

func parseTOMLDoc(src string) *tomlDoc {
	lines := strings.Split(src, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1] // trailing newline is re-added by String
	}
	doc := &tomlDoc{}
	cur := &tomlSection{}
	doc.sections = append(doc.sections, cur)
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if m := headerRe.FindStringSubmatch(line); m != nil {
			cur = &tomlSection{header: line, path: splitTablePath(m[2]), array: m[1] == "[["}
			doc.sections = append(doc.sections, cur)
			continue
		}
		if m := keyLineRe.FindStringSubmatch(line); m != nil {
			e := &tomlEntry{key: strings.Trim(m[2], `"`), value: m[4], lines: []string{line}}
			depth := bracketDepth(m[4])
			for depth > 0 && i+1 < len(lines) {
				i++
				e.lines = append(e.lines, lines[i])
				depth += bracketDepth(lines[i])
			}
			cur.items = append(cur.items, tomlItem{entry: e})
			continue
		}
		cur.items = append(cur.items, tomlItem{raw: line})
	}
	return doc
}

func splitTablePath(s string) []string {
	parts := strings.Split(s, ".")
	for i := range parts {
		parts[i] = strings.Trim(strings.TrimSpace(parts[i]), `"`)
	}
	return parts
}

// bracketDepth counts unbalanced [ and { outside strings and comments.
func bracketDepth(s string) int {
	depth := 0
	inStr := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inStr != 0:
			if c == '\\' && inStr == '"' {
				i++
			} else if c == inStr {
				inStr = 0
			}
		case c == '"' || c == '\'':
			inStr = c
		case c == '#':
			return depth
		case c == '[' || c == '{':
			depth++
		case c == ']' || c == '}':
			depth--
		}
	}
	return depth
}

func stripComment(s string) string {
	inStr := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inStr != 0:
			if c == '\\' && inStr == '"' {
				i++
			} else if c == inStr {
				inStr = 0
			}
		case c == '"' || c == '\'':
			inStr = c
		case c == '#':
			return s[:i]
		}
	}
	return s
}

func pathEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func pathHasPrefix(p, prefix []string) bool {
	return len(p) >= len(prefix) && pathEq(p[:len(prefix)], prefix)
}

// section returns the table at path, creating it at the end when create is
// set and it does not exist. Array tables are never returned.
func (d *tomlDoc) section(path []string, create bool) *tomlSection {
	for _, s := range d.sections {
		if !s.array && pathEq(s.path, path) {
			return s
		}
	}
	if !create {
		return nil
	}
	s := &tomlSection{header: "[" + strings.Join(path, ".") + "]", path: path}
	if last := d.sections[len(d.sections)-1]; len(last.items) > 0 || last.header != "" {
		s.items = nil
		// blank separator goes on the previous section so the new header
		// sits after an empty line
		last.items = append(last.items, tomlItem{raw: ""})
	}
	d.sections = append(d.sections, s)
	return s
}

func (s *tomlSection) find(key string) *tomlEntry {
	for _, it := range s.items {
		if it.entry != nil && it.entry.key == key {
			return it.entry
		}
	}
	return nil
}

func (s *tomlSection) removeEntry(e *tomlEntry) {
	for i, it := range s.items {
		if it.entry == e {
			s.items = append(s.items[:i], s.items[i+1:]...)
			return
		}
	}
}

func (s *tomlSection) append(e *tomlEntry) {
	// Insert after the last entry so trailing blank lines stay trailing.
	idx := len(s.items)
	for i := len(s.items) - 1; i >= 0; i-- {
		if s.items[i].entry != nil {
			idx = i + 1
			break
		}
		if strings.TrimSpace(s.items[i].raw) != "" {
			idx = i + 1
			break
		}
	}
	s.items = append(s.items[:idx], append([]tomlItem{{entry: e}}, s.items[idx:]...)...)
}

func (s *tomlSection) insertAfter(anchor *tomlEntry, key, value string) {
	s.insertAt(anchor, 1, key, value)
}

func (s *tomlSection) insertBefore(anchor *tomlEntry, key, value string) {
	s.insertAt(anchor, 0, key, value)
}

func (s *tomlSection) insertAt(anchor *tomlEntry, offset int, key, value string) {
	e := &tomlEntry{key: key, value: value, lines: []string{key + " = " + value}}
	for i, it := range s.items {
		if it.entry == anchor {
			at := i + offset
			s.items = append(s.items[:at], append([]tomlItem{{entry: e}}, s.items[at:]...)...)
			return
		}
	}
	s.append(e)
}

func (s *tomlSection) entryCount() int {
	n := 0
	for _, it := range s.items {
		if it.entry != nil {
			n++
		}
	}
	return n
}

func (d *tomlDoc) entry(path []string) *tomlEntry {
	if len(path) < 1 {
		return nil
	}
	s := d.section(path[:len(path)-1], false)
	if s == nil {
		return nil
	}
	return s.find(path[len(path)-1])
}

func (d *tomlDoc) has(path []string) bool {
	if d.entry(path) != nil {
		return true
	}
	for _, s := range d.sections {
		if len(s.path) > 0 && pathHasPrefix(s.path, path) {
			return true
		}
	}
	return false
}

func (d *tomlDoc) remove(path []string) {
	if e := d.entry(path); e != nil {
		d.section(path[:len(path)-1], false).removeEntry(e)
		return
	}
	// whole table (and subtables)
	kept := d.sections[:0]
	for _, s := range d.sections {
		if len(s.path) > 0 && pathHasPrefix(s.path, path) {
			continue
		}
		kept = append(kept, s)
	}
	d.sections = kept
}

// move relocates the entry at from to the key at to. Same table: the key is
// renamed in place. Different table: the entry is appended there. When to
// already exists, inline arrays are concatenated and anything else keeps
// the existing value.
func (d *tomlDoc) move(from, to []string) {
	e := d.entry(from)
	if e == nil {
		return
	}
	src := d.section(from[:len(from)-1], false)
	newKey := to[len(to)-1]
	if pathEq(from[:len(from)-1], to[:len(to)-1]) {
		e.setKey(newKey)
		return
	}
	dst := d.section(to[:len(to)-1], true)
	src.removeEntry(e)
	if existing := dst.find(newKey); existing != nil {
		if a, ok := inlineArrayItems(existing); ok {
			if b, ok := inlineArrayItems(e); ok {
				existing.setValue("[" + strings.Join(append(a, b...), ", ") + "]")
			}
		}
		return
	}
	e.setKey(newKey)
	dst.append(e)
}

// inlineArrayItems returns the comma-separated items of an array value,
// one per element, comments dropped, when the value is an array.
func inlineArrayItems(e *tomlEntry) ([]string, bool) {
	var b strings.Builder
	for _, l := range e.lines {
		b.WriteString(stripComment(l))
		b.WriteString(" ")
	}
	text := b.String()
	if i := strings.Index(text, "="); i >= 0 && e.lines[0] != "" && keyLineRe.MatchString(e.lines[0]) {
		text = text[i+1:]
	}
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "[") || !strings.HasSuffix(text, "]") {
		return nil, false
	}
	var out []string
	for _, p := range strings.Split(text[1:len(text)-1], ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out, true
}

func (d *tomlDoc) setOrMergeArray(table []string, key string, vals []string) {
	s := d.section(table, true)
	if existing := s.find(key); existing != nil {
		if a, ok := inlineArrayItems(existing); ok {
			existing.setValue("[" + strings.Join(append(a, vals...), ", ") + "]")
		}
		return
	}
	s.append(&tomlEntry{key: key, value: "[" + strings.Join(vals, ", ") + "]", lines: []string{key + " = [" + strings.Join(vals, ", ") + "]"}})
}

// removeVolumeTables drops every [[volumes]] table and returns their mount
// values, quoted as written.
func (d *tomlDoc) removeVolumeTables() []string {
	var vals []string
	kept := d.sections[:0]
	for _, s := range d.sections {
		if s.array && pathEq(s.path, []string{"volumes"}) {
			if e := s.find("mount"); e != nil {
				vals = append(vals, strings.TrimSpace(stripComment(e.value)))
			}
			continue
		}
		kept = append(kept, s)
	}
	d.sections = kept
	return vals
}

// renameTable rewrites the header of the table at from and of every
// subtable. When the target already exists its entries are merged into it.
func (d *tomlDoc) renameTable(from, to []string) bool {
	changed := false
	for _, s := range d.sections {
		if len(s.path) == 0 || !pathHasPrefix(s.path, from) || s.array {
			continue
		}
		newPath := append(append([]string{}, to...), s.path[len(from):]...)
		if dst := d.section(newPath, false); dst != nil && dst != s {
			for _, it := range s.items {
				if it.entry != nil {
					dst.append(it.entry)
				}
			}
			s.items = nil
			s.path = nil
			s.header = ""
			changed = true
			continue
		}
		s.header = "[" + strings.Join(newPath, ".") + "]"
		s.path = newPath
		changed = true
	}
	// drop the emptied husks left by merges
	kept := d.sections[:0]
	for i, s := range d.sections {
		if i > 0 && s.header == "" && len(s.items) == 0 {
			continue
		}
		kept = append(kept, s)
	}
	d.sections = kept
	return changed
}

// dropEmptySections removes tables left with no keys and no comments.
func (d *tomlDoc) dropEmptySections() {
	kept := d.sections[:0]
	for i, s := range d.sections {
		if i > 0 && s.entryCount() == 0 {
			hasText := false
			for _, it := range s.items {
				if strings.TrimSpace(it.raw) != "" {
					hasText = true
					break
				}
			}
			if !hasText {
				// keep at most the trailing blank so the previous
				// section still ends with one empty line
				continue
			}
		}
		kept = append(kept, s)
	}
	d.sections = kept
}

func (d *tomlDoc) String() string {
	var b strings.Builder
	for _, s := range d.sections {
		if s.header != "" {
			b.WriteString(s.header)
			b.WriteString("\n")
		}
		for _, it := range s.items {
			if it.entry != nil {
				for _, l := range it.entry.lines {
					b.WriteString(l)
					b.WriteString("\n")
				}
				continue
			}
			b.WriteString(it.raw)
			b.WriteString("\n")
		}
	}
	out := b.String()
	// collapse runs of blank lines left by removals
	for strings.Contains(out, "\n\n\n") {
		out = strings.ReplaceAll(out, "\n\n\n", "\n\n")
	}
	return strings.TrimRight(out, "\n") + "\n"
}

// ── commented-out examples ────────────────────────────────────────────
//
// Scaffolded configs document every option as a commented example, and an
// example written for an old release keeps teaching the old spelling after
// the live keys were migrated. refreshComments rewrites such lines in place
// with the same rules the live keys follow: retired keys are dropped,
// renamed keys and tables get their new name, [[volumes]] blocks collapse
// into one `volumes = [...]` line, and a key that moved to another table
// gets that table's header in front of it. Lines that are not of the form
// `# key = value` or `# [table]`, or that are already current, are left
// alone. The count of rewritten lines is returned.

// renamedCommentTables extends tableRenames with [llm.models], whose only
// key moved to [llm]; a live [llm.models] table is dropped as empty, a
// commented one is renamed so its example keys land in the right place.
var renamedCommentTables = append(tableRenames, struct{ from, to []string }{[]string{"llm", "models"}, []string{"llm"}})

var commentLineRe = regexp.MustCompile(`^(\s*#\s?)(.*)$`)

func (d *tomlDoc) refreshComments() int {
	n := 0
	for _, s := range d.sections {
		// ctxOld is the table a commented key belongs to as the user wrote
		// it, ctxNew the same table under its current name.
		ctxOld, ctxNew := s.path, s.path
		var volumes []string   // mounts from a run of `# [[volumes]]` blocks
		var volumesAt int      // item index where that run started
		var volumesInCell bool // the run sits where [cell] keys would land
		volumesPending := false
		var out []tomlItem
		// A run of commented [[volumes]] blocks counts as one example: it
		// becomes a single `volumes = [...]` line, under a [cell] header
		// unless the surrounding context already is [cell].
		flushVolumes := func() {
			if !volumesPending {
				return
			}
			items := []tomlItem{{raw: "# volumes = [" + strings.Join(volumes, ", ") + "]"}}
			if !volumesInCell {
				items = append([]tomlItem{{raw: "# [cell]"}}, items...)
			}
			out = append(out[:volumesAt], append(items, out[volumesAt:]...)...)
			volumes, volumesPending = nil, false
			n++
		}
		for _, it := range s.items {
			if it.entry != nil {
				flushVolumes()
				out = append(out, it)
				continue
			}
			m := commentLineRe.FindStringSubmatch(it.raw)
			if m == nil {
				flushVolumes()
				out = append(out, it)
				continue
			}
			prefix, text := m[1], m[2]
			if h := headerRe.FindStringSubmatch(text); h != nil {
				path := splitTablePath(h[2])
				if h[1] == "[[" && pathEq(path, []string{"volumes"}) {
					if !volumesPending {
						volumesAt, volumesPending = len(out), true
						volumesInCell = pathEq(ctxNew, []string{"cell"})
					}
					ctxOld, ctxNew = path, path
					continue
				}
				flushVolumes()
				ctxOld, ctxNew = path, path
				for _, r := range renamedCommentTables {
					if pathHasPrefix(path, r.from) {
						ctxNew = append(append([]string{}, r.to...), path[len(r.from):]...)
						break
					}
				}
				if pathEq(ctxOld, ctxNew) {
					out = append(out, it)
				} else {
					out = append(out, tomlItem{raw: prefix + h[1] + strings.Join(ctxNew, ".") + strings.Repeat("]", len(h[1])) + trailing(h[3])})
					n++
				}
				continue
			}
			k := keyLineRe.FindStringSubmatch(text)
			if k == nil {
				if volumesPending && strings.TrimSpace(text) == "" {
					continue // the blank `#` between two [[volumes]] blocks
				}
				flushVolumes()
				out = append(out, it)
				continue
			}
			key, value := strings.Trim(k[2], `"`), k[4]
			if volumesPending && pathEq(ctxOld, []string{"volumes"}) && key == "mount" {
				volumes = append(volumes, strings.TrimSpace(stripComment(value)))
				continue
			}
			flushVolumes()
			dep := findDeprecation(append(append([]string{}, ctxOld...), key))
			if dep == nil {
				out = append(out, it)
				continue
			}
			n++
			if dep.Replacement == "(none)" {
				continue
			}
			to := parseKeyName(dep.Replacement)
			if to == nil {
				out = append(out, it)
				n--
				continue
			}
			newKey, table := to[len(to)-1], to[:len(to)-1]
			if key == "use_ollama" || key == "use_openrouter" {
				value = fmt.Sprintf("%q", strings.TrimPrefix(key, "use_"))
			}
			if !pathEq(table, ctxNew) {
				out = append(out, tomlItem{raw: prefix + "[" + strings.Join(table, ".") + "]"})
			}
			out = append(out, tomlItem{raw: prefix + newKey + k[3] + value})
		}
		flushVolumes()
		s.items = out
	}
	return n
}

// trailing keeps a header's trailing comment, with its separating space.
func trailing(c string) string {
	if c == "" {
		return ""
	}
	return " " + c
}

func findDeprecation(path []string) *Deprecation {
	for i := range Deprecations {
		if pathEq(Deprecations[i].Path, path) {
			return &Deprecations[i]
		}
	}
	return nil
}
