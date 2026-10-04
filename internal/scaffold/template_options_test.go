package scaffold_test

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/DimmKirr/devcell/internal/cfg"
	"github.com/DimmKirr/devcell/internal/scaffold"
)

// schemaKeys lists every user-facing TOML key path in cfg.CellConfig, skipping
// deprecated keys and TOML-only aliases (hm:"-"). Maps of tables use "<name>";
// arrays of tables use "[[path]]".
func schemaKeys() []string {
	var out []string
	var walk func(t reflect.Type, prefix string)
	walk = func(t reflect.Type, prefix string) {
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			k, _, _ := strings.Cut(f.Tag.Get("toml"), ",")
			if k == "" {
				k = strings.ToLower(f.Name)
			}
			if k == "-" || f.Tag.Get("hm") == "-" {
				continue
			}
			p := k
			if prefix != "" {
				p = prefix + "." + k
			}
			if isDeprecated(p) {
				continue
			}
			ft := f.Type
			for ft.Kind() == reflect.Ptr {
				ft = ft.Elem()
			}
			switch {
			case ft.Kind() == reflect.Struct:
				walk(ft, p)
			case ft.Kind() == reflect.Slice && ft.Elem().Kind() == reflect.Struct:
				walk(ft.Elem(), "[["+p+"]]")
			case ft.Kind() == reflect.Map && ft.Elem().Kind() == reflect.Struct:
				walk(ft.Elem(), p+".<name>")
			default:
				out = append(out, p)
			}
		}
	}
	walk(reflect.TypeOf(cfg.CellConfig{}), "")
	return out
}

func isDeprecated(path string) bool {
	path = strings.NewReplacer("[[", "", "]]", "").Replace(path)
	for _, d := range cfg.Deprecations {
		dp := strings.Join(d.Path, ".")
		if path == dp || strings.HasPrefix(path, dp+".") {
			return true
		}
	}
	return false
}

var (
	headerRe = regexp.MustCompile(`^(\[\[?)\s*([A-Za-z0-9_."-]+)\s*\]\]?$`)
	keyRe    = regexp.MustCompile(`^([A-Za-z0-9_-]+)\s*=`)
)

// documentedKeys returns every key path the text defines or shows commented.
func documentedKeys(text string) map[string]bool {
	seen := map[string]bool{}
	table := ""
	for _, line := range strings.Split(text, "\n") {
		s := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "#"))
		if m := headerRe.FindStringSubmatch(s); m != nil {
			table = strings.ReplaceAll(m[2], `"`, "")
			if m[1] == "[[" {
				table = "[[" + table + "]]"
			}
			seen[table] = true
			continue
		}
		if m := keyRe.FindStringSubmatch(s); m != nil {
			p := m[1]
			if table != "" {
				p = table + "." + p
			}
			seen[p] = true
		}
	}
	return seen
}

func renderInit(t *testing.T, snippet string) string {
	t.Helper()
	dir := t.TempDir()
	if err := scaffold.Scaffold(dir, snippet, "", false); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".devcell.toml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestInitTemplate_ListsEveryNonDeprecatedOption(t *testing.T) {
	seen := documentedKeys(renderInit(t, ""))
	for _, want := range schemaKeys() {
		re := regexp.MustCompile("^" + strings.ReplaceAll(regexp.QuoteMeta(want), regexp.QuoteMeta("<name>"), `[A-Za-z0-9_-]+`) + "$")
		found := false
		for k := range seen {
			if re.MatchString(k) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("init template does not show option %q", want)
		}
	}
}

func TestInitTemplate_OnlyCellUncommented(t *testing.T) {
	for i, line := range strings.Split(renderInit(t, ""), "\n") {
		s := strings.TrimSpace(line)
		if s == "" || strings.HasPrefix(s, "#") || s == "[cell]" {
			continue
		}
		t.Errorf("line %d is active, only [cell] may be: %q", i+1, s)
	}
}

func TestInitTemplate_DetectedModelsStayCommented(t *testing.T) {
	snippet := "# Detected local ollama models.\n# [llm]\n# provider = \"ollama\"\n# model = \"qwen3:8b\"\n#\n# [llm.providers.ollama]\n# models = [\"qwen3:8b\"]\n"
	out := renderInit(t, snippet)
	for i, line := range strings.Split(out, "\n") {
		if s := strings.TrimSpace(line); s != "" && !strings.HasPrefix(s, "#") && s != "[cell]" {
			t.Errorf("line %d is active, only [cell] may be: %q", i+1, s)
		}
	}
	llmDoc := strings.Index(out, "# --- AI agents")
	detected := strings.Index(out, "# Detected local ollama models.")
	git := strings.Index(out, "# --- Git identity")
	if llmDoc < 0 || detected < llmDoc || detected > git {
		t.Errorf("detected models must sit inside the LLM block, before [git]:\n%s", out)
	}
}
