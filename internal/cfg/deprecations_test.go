package cfg_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/DimmKirr/devcell/internal/cfg"
)

func deprecatedNames(c cfg.CellConfig) []string {
	var out []string
	for _, u := range c.DeprecatedUses {
		out = append(out, u.Name)
	}
	return out
}

func TestLoadFile_DetectsDeprecatedKeys(t *testing.T) {
	dir := t.TempDir()
	p := writeTOML(t, dir, "test.toml", `
[[volumes]]
mount = "/data"

[ports]
forward = ["3000"]

[mcp]
enabled = ["playwright"]

[op]
documents = ["prod-api-keys"]
`)
	c, err := cfg.LoadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(deprecatedNames(c), ",")
	want := "[[volumes]],[ports] forward,[mcp] enabled,[op]"
	if got != want {
		t.Errorf("deprecated = %q, want %q", got, want)
	}
	for _, u := range c.DeprecatedUses {
		if u.File != p {
			t.Errorf("%s: File = %q, want %q", u.Name, u.File, p)
		}
	}
}

func TestLoadFile_ShorthandsAreNotDeprecated(t *testing.T) {
	dir := t.TempDir()
	p := writeTOML(t, dir, "test.toml", `
[cell]
volumes = ["/data"]
ports = ["3000"]
mcps = ["playwright"]

[ports]
publish_ip = "127.0.0.1"

[secrets.onepassword]
documents = ["prod-api-keys"]
`)
	c, err := cfg.LoadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.DeprecatedUses) != 0 {
		t.Errorf("want no deprecations, got %v", deprecatedNames(c))
	}
}

func TestLoadFile_SecretsOpIsAliasForOnePassword(t *testing.T) {
	dir := t.TempDir()
	p := writeTOML(t, dir, "test.toml", `
[secrets.onepassword]
documents = ["a", "b"]

[secrets.op]
documents = ["b", "c"]
`)
	c, err := cfg.LoadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(c.Op.ResolvedDocuments(), ","); got != "a,b,c" {
		t.Errorf("ResolvedDocuments = %q, want a,b,c", got)
	}
	if len(c.DeprecatedUses) != 0 {
		t.Errorf("[secrets.op] alias must not warn, got %v", deprecatedNames(c))
	}
}

func TestLoadFile_SecretsOnePasswordDocuments(t *testing.T) {
	dir := t.TempDir()
	p := writeTOML(t, dir, "test.toml", `
[secrets.onepassword]
documents = ["prod-api-keys", "dev-secrets"]

[op]
documents = ["dev-secrets", "legacy"]
items = ["older"]
`)
	c, err := cfg.LoadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(c.Op.ResolvedDocuments(), ",")
	want := "dev-secrets,legacy,prod-api-keys,older"
	if got != want {
		t.Errorf("ResolvedDocuments = %q, want %q", got, want)
	}
}

func TestLoadLayered_SecretsOnePasswordMergesAcrossLayers(t *testing.T) {
	dir := t.TempDir()
	global := writeTOML(t, dir, "global.toml", "[op]\ndocuments = [\"global-doc\"]\n")
	project := writeTOML(t, dir, "project.toml", "[secrets.onepassword]\ndocuments = [\"project-doc\"]\n")
	c, err := cfg.LoadLayered(global, project, func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(c.Op.ResolvedDocuments(), ",")
	if got != "global-doc,project-doc" {
		t.Errorf("ResolvedDocuments = %q, want global-doc,project-doc", got)
	}
}

func TestLoadLayered_KeepsDeprecationsFromBothLayers(t *testing.T) {
	dir := t.TempDir()
	global := writeTOML(t, dir, "global.toml", "[ports]\nforward = [\"3000\"]\n")
	project := writeTOML(t, dir, "project.toml", "[mcp]\nenabled = [\"playwright\"]\n")
	c, err := cfg.LoadLayered(global, project, func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	if len(c.DeprecatedUses) != 2 {
		t.Fatalf("want 2 deprecations, got %v", deprecatedNames(c))
	}
	if c.DeprecatedUses[0].File != global || c.DeprecatedUses[1].File != project {
		t.Errorf("files = %q, %q", c.DeprecatedUses[0].File, c.DeprecatedUses[1].File)
	}
}

func TestDeprecatedUse_Warning(t *testing.T) {
	u := cfg.DeprecatedUse{
		Deprecation: cfg.Deprecation{
			Name:        "[op]",
			Replacement: "[secrets]",
			Message:     `use [secrets] instead; move op items to [secrets] op = ["..."]`,
		},
		File: "/p/.devcell.toml",
	}
	want := `/p/.devcell.toml: [op] is deprecated and will be removed in a future release: use [secrets] instead; move op items to [secrets] op = ["..."]`
	if got := u.Warning(); got != want {
		t.Errorf("Warning() =\n%q\nwant\n%q", got, want)
	}
}

func TestDeprecations_EveryEntryIsComplete(t *testing.T) {
	for _, d := range cfg.Deprecations {
		if len(d.Path) == 0 || d.Name == "" || d.Replacement == "" || d.Message == "" {
			t.Errorf("incomplete deprecation entry: %+v", d)
		}
	}
}

// Guards against typos: every table entry must name a real TOML key.
func TestDeprecations_PathsResolveToConfigFields(t *testing.T) {
	for _, d := range cfg.Deprecations {
		typ := reflect.TypeOf(cfg.CellConfig{})
		for _, part := range d.Path {
			for typ.Kind() == reflect.Slice || typ.Kind() == reflect.Ptr {
				typ = typ.Elem()
			}
			f, ok := fieldByTOMLKey(typ, part)
			if !ok {
				t.Errorf("%s: no TOML key %q in %s", d.Name, part, typ)
				break
			}
			typ = f.Type
		}
	}
}

func fieldByTOMLKey(typ reflect.Type, key string) (reflect.StructField, bool) {
	if typ.Kind() != reflect.Struct {
		return reflect.StructField{}, false
	}
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		tag := strings.Split(f.Tag.Get("toml"), ",")[0]
		if tag == key || (tag == "" && strings.EqualFold(f.Name, key)) {
			return f, true
		}
	}
	return reflect.StructField{}, false
}
