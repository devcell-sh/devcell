package cfg_test

import (
	"strings"
	"testing"

	"github.com/DimmKirr/devcell/internal/cfg"
)

// A typo or a key devcell no longer reads must fail loudly, not be ignored.
func TestLoadFile_UnknownKeysAreError(t *testing.T) {
	p := writeTOML(t, t.TempDir(), "test.toml", `
[cell]
stack = "base"
stak = "go"

[mcp]
docker_host = "unix:///var/run/docker.sock"

[onepassword]
documents = ["x"]
`)
	_, err := cfg.LoadLayered("", p, func(string) string { return "" })
	if err == nil {
		t.Fatal("want an error for unknown keys")
	}
	for _, want := range []string{p, "cell.stak, mcp.docker_host, onepassword "} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
	if strings.Contains(err.Error(), "onepassword.documents") {
		t.Errorf("an unknown table must be reported once, got %q", err)
	}
}

// Free-form tables (env, mise, packages, provider maps) accept any key.
func TestLoadFile_FreeFormTablesAcceptAnyKey(t *testing.T) {
	p := writeTOML(t, t.TempDir(), "test.toml", `
[env]
ANY_VAR = "x"

[mise]
some_setting = "1"

[packages.npm]
"any-tool" = "^1"

[llm.providers.vllm]
base_url = "http://gpu:8000/v1"
`)
	if _, err := cfg.LoadFile(p); err != nil {
		t.Fatal(err)
	}
}
