package main_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// dryRunWithLLMConfig writes toml as the global devcell.toml and returns the
// dry-run argv of `cell <args> --dry-run`.
func dryRunWithLLMConfig(t *testing.T, toml string, args ...string) string {
	t.Helper()
	home := scaffoldedHome(t)
	if err := os.WriteFile(filepath.Join(home, ".config", "devcell", "devcell.toml"), []byte("[cell]\n"+toml), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binaryPath, append(args, "--dry-run")...)
	cmd.Dir = home
	cmd.Env = append(os.Environ(), "DEVCELL_BUNK=1", "HOME="+home, "OPENROUTER_API_KEY=sk-or-test-key")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v --dry-run failed: %v\noutput: %s", args, err, out)
	}
	return string(out)
}

func opencodeConfig(t *testing.T, argv string) map[string]any {
	t.Helper()
	var parsed map[string]any
	if err := json.Unmarshal([]byte(extractEnvFromArgv(argv, "OPENCODE_CONFIG_CONTENT")), &parsed); err != nil {
		t.Fatalf("invalid OPENCODE_CONFIG_CONTENT: %v\n%s", err, argv)
	}
	return parsed
}

func TestLLM_ProviderBaseURLOverridesDefault(t *testing.T) {
	ollama := `[llm]
provider = "ollama"
model = "qwen3:8b"
[llm.providers.ollama]
base_url = "http://gpu-box:11434"
`
	openrouter := `[llm]
provider = "openrouter"
[llm.providers.openrouter]
base_url = "https://or-proxy.example.com/api/v1"
`
	for _, tc := range []struct {
		name, toml string
		args       []string
		want       string
	}{
		{"claude ollama", ollama, []string{"claude"}, "ANTHROPIC_BASE_URL=http://gpu-box:11434"},
		{"codex ollama", ollama, []string{"codex"}, "CODEX_OSS_BASE_URL=http://gpu-box:11434/v1"},
		{"claude openrouter", openrouter, []string{"claude"}, "ANTHROPIC_BASE_URL=https://or-proxy.example.com/api"},
		{"codex openrouter", openrouter, []string{"codex"}, "model_providers.openrouter.base_url=https://or-proxy.example.com/api/v1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if argv := dryRunWithLLMConfig(t, tc.toml, tc.args...); !strings.Contains(argv, tc.want) {
				t.Errorf("want %q in argv:\n%s", tc.want, argv)
			}
		})
	}
}

func TestLLM_OpencodeProviderBaseURL(t *testing.T) {
	argv := dryRunWithLLMConfig(t, `[llm.providers.ollama]
base_url = "http://gpu-box:11434"
models = ["qwen3:8b"]
`, "opencode")
	prov := opencodeConfig(t, argv)["provider"].(map[string]any)["ollama"].(map[string]any)
	if got := prov["options"].(map[string]any)["baseURL"]; got != "http://gpu-box:11434/v1" {
		t.Errorf("ollama baseURL = %v, want http://gpu-box:11434/v1", got)
	}
}

// opencode wants "provider/model"; devcell composes it from the two keys.
func TestLLM_OpencodeModelComposedFromProvider(t *testing.T) {
	for _, tc := range []struct{ toml, want string }{
		{"[llm]\nprovider = \"ollama\"\nmodel = \"qwen3:8b\"\n", "ollama/qwen3:8b"},
		{"[llm]\nprovider = \"openrouter\"\nmodel = \"deepseek/deepseek-v4-pro\"\n", "openrouter/deepseek/deepseek-v4-pro"},
		// default: no rerouting, so the model is opencode's own ID, verbatim.
		{"[llm]\nprovider = \"default\"\nmodel = \"anthropic/claude-sonnet-4-5\"\n", "anthropic/claude-sonnet-4-5"},
	} {
		if got := opencodeConfig(t, dryRunWithLLMConfig(t, tc.toml, "opencode"))["model"]; got != tc.want {
			t.Errorf("%q: model = %v, want %s", tc.toml, got, tc.want)
		}
	}
}

// A --ollama run must not send the configured OpenRouter model to ollama.
func TestLLM_FlagProviderIgnoresOtherProvidersModel(t *testing.T) {
	argv := dryRunWithLLMConfig(t, "[llm]\nprovider = \"openrouter\"\nmodel = \"deepseek/deepseek-v4-pro\"\n", "claude", "--ollama")
	if strings.Contains(argv, "ANTHROPIC_MODEL=deepseek/deepseek-v4-pro") {
		t.Errorf("openrouter model leaked into --ollama run:\n%s", argv)
	}
}
