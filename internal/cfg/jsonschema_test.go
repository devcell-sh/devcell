package cfg

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

// The published devcell.toml JSON Schema (web/public/schema/devcell.json,
// served at https://devcell.sh/schema/devcell.json) is generated from
// CellConfig so editors validating `#:schema` never drift from the Go
// loader. `task schema:generate` (a dep of cell:build) writes it.

func decodeSchema(t *testing.T, opts SchemaOptions) map[string]any {
	t.Helper()
	raw, err := JSONSchema(opts)
	if err != nil {
		t.Fatalf("JSONSchema: %v", err)
	}
	var s map[string]any
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("schema is not valid JSON: %v\n%s", err, raw)
	}
	return s
}

// prop walks properties.a.properties.b... and returns the leaf schema.
func prop(t *testing.T, s map[string]any, path ...string) map[string]any {
	t.Helper()
	cur := s
	for _, p := range path {
		props, _ := cur["properties"].(map[string]any)
		next, ok := props[p].(map[string]any)
		if !ok {
			t.Fatalf("path %v: key %q missing in %v", path, p, keys(props))
		}
		cur = next
	}
	return cur
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestJSONSchema_Envelope(t *testing.T) {
	s := decodeSchema(t, SchemaOptions{})
	if s["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
		t.Errorf("$schema = %v", s["$schema"])
	}
	if s["$id"] != SchemaURLFor("v0.0.0") {
		t.Errorf("$id = %v, want %s", s["$id"], SchemaURLFor("v0.0.0"))
	}
	if s["version"] != "v0.0.0" {
		t.Errorf("unversioned generation must publish as %s, got version=%v", DefaultSchemaVersion, s["version"])
	}
	if s["type"] != "object" || s["additionalProperties"] != false {
		t.Errorf("root must be a closed object: type=%v additionalProperties=%v", s["type"], s["additionalProperties"])
	}
	for _, section := range []string{"cell", "docker", "build", "nix", "llm", "git", "ports", "op", "secrets", "aws", "mcp", "stealth", "gui", "env", "mise", "volumes", "packages", "wireguard"} {
		prop(t, s, section)
	}
}

func TestJSONSchema_OmitsTomlDashFields(t *testing.T) {
	s := decodeSchema(t, SchemaOptions{})
	props := s["properties"].(map[string]any)
	for _, k := range []string{"deprecateduses", "DeprecatedUses"} {
		if _, ok := props[k]; ok {
			t.Errorf("toml:\"-\" field leaked into schema as %q", k)
		}
	}
}

func TestJSONSchema_TypeMapping(t *testing.T) {
	s := decodeSchema(t, SchemaOptions{})
	cases := []struct {
		path []string
		typ  string
	}{
		{[]string{"cell", "registry"}, "string"},
		{[]string{"cell", "gui"}, "boolean"}, // *bool
		{[]string{"cell", "tart_ssh_port"}, "integer"},
		{[]string{"cell", "modules"}, "array"},
		{[]string{"packages", "node"}, "object"},
		{[]string{"wireguard"}, "array"},
		{[]string{"secrets", "onepassword"}, "object"},
	}
	for _, c := range cases {
		if got := prop(t, s, c.path...)["type"]; got != c.typ {
			t.Errorf("%v: type = %v, want %s", c.path, got, c.typ)
		}
	}

	if items := prop(t, s, "cell", "modules")["items"].(map[string]any); items["type"] != "string" {
		t.Errorf("cell.modules items = %v, want string", items)
	}
	if ap := prop(t, s, "packages", "node")["additionalProperties"].(map[string]any); ap["type"] != "string" {
		t.Errorf("packages.node additionalProperties = %v, want string", ap)
	}
	// Nested struct inside a map: [llm.models.providers.<name>] base_url
	providers := prop(t, s, "llm", "models", "providers")["additionalProperties"].(map[string]any)
	if _, ok := providers["properties"].(map[string]any)["base_url"]; !ok {
		t.Errorf("llm.models.providers.* should declare base_url: %v", providers)
	}
	// Nested struct inside a slice: [[wireguard]] name/enabled/config
	wg := prop(t, s, "wireguard")["items"].(map[string]any)
	for _, k := range []string{"name", "enabled", "config"} {
		if _, ok := wg["properties"].(map[string]any)[k]; !ok {
			t.Errorf("wireguard items missing %q", k)
		}
	}
	if wg["additionalProperties"] != false {
		t.Errorf("struct items must be closed objects: %v", wg)
	}
}

func TestJSONSchema_DeprecatedKeysMarked(t *testing.T) {
	s := decodeSchema(t, SchemaOptions{})
	for _, path := range [][]string{{"cell", "qemu_ssh_port"}, {"op"}, {"llm", "models", "default"}} {
		p := prop(t, s, path...)
		if p["deprecated"] != true {
			t.Errorf("%v should be deprecated: %v", path, p)
		}
	}
	if d, _ := prop(t, s, "cell", "qemu_ssh_port")["description"].(string); !bytes.Contains([]byte(d), []byte("winkit_ssh_port")) {
		t.Errorf("deprecated key description should name the replacement, got %q", d)
	}
	if p := prop(t, s, "cell", "winkit_ssh_port"); p["deprecated"] != nil {
		t.Errorf("live key must not be marked deprecated: %v", p)
	}
}

func TestJSONSchema_UsesFieldDescriptions(t *testing.T) {
	s := decodeSchema(t, SchemaOptions{Descriptions: map[string]string{"CellSection.Registry": "container registry"}})
	if got := prop(t, s, "cell", "registry")["description"]; got != "container registry" {
		t.Errorf("description = %v", got)
	}
}

func TestFieldComments_TrailingAndDoc(t *testing.T) {
	src := `package cfg

type Demo struct {
	A string ` + "`toml:\"a\"`" + ` // trailing comment for A
	// doc comment for B
	B int ` + "`toml:\"b\"`" + `
	C bool
}
`
	got, err := FieldComments([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"Demo.A": "trailing comment for A", "Demo.B": "doc comment for B"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
	if _, ok := got["Demo.C"]; ok {
		t.Errorf("uncommented field should be absent, got %q", got["Demo.C"])
	}
}

// The committed artifact must match what the generator emits; a stale
// file means someone edited CellConfig without `task schema:generate`.
func TestJSONSchema_PublishedFileInSync(t *testing.T) {
	const published = "../../web/public/schema/v0.0.0/devcell.json"
	have, err := os.ReadFile(published)
	if err != nil {
		t.Fatalf("read %s: %v (run `task schema:generate`)", published, err)
	}
	src, err := os.ReadFile("cfg.go")
	if err != nil {
		t.Fatal(err)
	}
	descs, err := FieldComments(src)
	if err != nil {
		t.Fatal(err)
	}
	opts := SchemaOptions{Version: DefaultSchemaVersion, Descriptions: descs}
	if err := opts.LoadCatalog("../../web/public/schema/v0.0.0/devcell-sh/home"); err != nil {
		t.Fatal(err)
	}
	want, err := JSONSchema(opts)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.TrimSpace(have), bytes.TrimSpace(want)) {
		t.Errorf("%s is stale: run `task schema:generate` and commit the result", published)
	}
}

func TestJSONSchema_CatalogHints(t *testing.T) {
	s := decodeSchema(t, SchemaOptions{Modules: []string{"go", "infra"}, Stacks: []string{"base", "go"}})
	items := prop(t, s, "cell", "modules")["items"].(map[string]any)
	if ex, _ := items["examples"].([]any); len(ex) != 2 || ex[0] != "go" {
		t.Errorf("cell.modules items examples = %v, want catalog names", items["examples"])
	}
	if _, strict := items["enum"]; strict {
		t.Errorf("modules must stay open (custom homes add modules); got enum")
	}
	if ex, _ := prop(t, s, "cell", "stack")["examples"].([]any); len(ex) != 2 || ex[1] != "go" {
		t.Errorf("cell.stack examples = %v", ex)
	}
	links := prop(t, s, "cell", "modules")["x-taplo"].(map[string]any)["links"].(map[string]any)
	if links["key"] != CatalogURLFor("v0.0.0")+"/modules.json" {
		t.Errorf("modules key link = %v", links["key"])
	}
}

func TestJSONSchema_PackageNamesCarryNixAttrPattern(t *testing.T) {
	s := decodeSchema(t, SchemaOptions{})
	for _, path := range [][]string{{"cell", "packages"}, {"packages", "nix", "unstable"}} {
		items := prop(t, s, path...)["items"].(map[string]any)
		if items["pattern"] != validNixAttr.String() {
			t.Errorf("%v items pattern = %v, want loader regex", path, items["pattern"])
		}
	}
	links := prop(t, s, "cell", "packages")["x-taplo"].(map[string]any)["links"].(map[string]any)
	if links["key"] != "https://search.nixos.org/packages" {
		t.Errorf("packages key link = %v", links["key"])
	}
}

func TestSchemaOptions_LoadCatalog(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(dir+"/modules.json", []byte(`{"infra": {}, "go": {}}`), 0o644)
	os.WriteFile(dir+"/stacks.json", []byte(`{"go": {}, "base": {}}`), 0o644)
	var opts SchemaOptions
	if err := opts.LoadCatalog(dir); err != nil {
		t.Fatal(err)
	}
	if len(opts.Modules) != 2 || opts.Modules[0] != "go" || len(opts.Stacks) != 2 || opts.Stacks[0] != "base" {
		t.Errorf("sorted names expected, got modules=%v stacks=%v", opts.Modules, opts.Stacks)
	}
	if err := (&SchemaOptions{}).LoadCatalog(t.TempDir()); err != nil {
		t.Errorf("missing catalog should be tolerated (schema still valid without hints): %v", err)
	}
}

func TestJSONSchema_VersionedURLs(t *testing.T) {
	s := decodeSchema(t, SchemaOptions{Version: "v1.2.3"})
	if s["version"] != "v1.2.3" {
		t.Errorf("version = %v", s["version"])
	}
	if s["$id"] != "https://devcell.sh/schema/v1.2.3/devcell.json" {
		t.Errorf("$id = %v", s["$id"])
	}
	links := prop(t, s, "cell", "stack")["x-taplo"].(map[string]any)["links"].(map[string]any)
	if links["key"] != "https://devcell.sh/schema/v1.2.3/devcell-sh/home/stacks.json" {
		t.Errorf("catalog link must be versioned too: %v", links["key"])
	}
}
