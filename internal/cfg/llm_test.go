package cfg_test

import (
	"strings"
	"testing"

	"github.com/DimmKirr/devcell/internal/cfg"
)

func TestLoadFile_LLMProviderModelProviders(t *testing.T) {
	dir := t.TempDir()
	p := writeTOML(t, dir, "test.toml", `
[llm]
provider = "openrouter"
model = "deepseek/deepseek-v4-pro"

[llm.providers.ollama]
base_url = "http://host.docker.internal:11434"
models = ["qwen3:8b"]

[llm.providers.openrouter]
models = ["moonshotai/kimi-k3"]
`)
	c, err := cfg.LoadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.LLM.Provider != "openrouter" || c.LLM.Model != "deepseek/deepseek-v4-pro" {
		t.Errorf("provider/model = %q/%q", c.LLM.Provider, c.LLM.Model)
	}
	if got := c.LLM.Providers["ollama"]; got.BaseURL != "http://host.docker.internal:11434" || strings.Join(got.Models, ",") != "qwen3:8b" {
		t.Errorf("ollama provider = %+v", got)
	}
	if len(c.DeprecatedUses) != 0 {
		t.Errorf("new keys must not warn, got %v", deprecatedNames(c))
	}
}

func TestLoadFile_LLMLegacyKeysMigrate(t *testing.T) {
	dir := t.TempDir()
	p := writeTOML(t, dir, "test.toml", `
[llm]
use_ollama = true

[llm.models]
default = "ollama/qwen3:8b"

[llm.models.providers.ollama]
models = ["qwen3:8b"]
`)
	c, err := cfg.LoadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.LLM.Provider != "ollama" || c.LLM.Model != "qwen3:8b" {
		t.Errorf("provider/model = %q/%q", c.LLM.Provider, c.LLM.Model)
	}
	if strings.Join(c.LLM.Providers["ollama"].Models, ",") != "qwen3:8b" {
		t.Errorf("providers = %+v", c.LLM.Providers)
	}
	want := "[llm] use_ollama,[llm.models] default,[llm.models.providers]"
	if got := strings.Join(deprecatedNames(c), ","); got != want {
		t.Errorf("deprecated = %q, want %q", got, want)
	}
}

func TestLoadFile_LLMUseOpenRouterMigrates(t *testing.T) {
	dir := t.TempDir()
	p := writeTOML(t, dir, "test.toml", "[llm]\nuse_openrouter = true\n")
	c, err := cfg.LoadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.LLM.Provider != "openrouter" {
		t.Errorf("provider = %q, want openrouter", c.LLM.Provider)
	}
}

func TestLoadFile_LLMNewKeysWinOverLegacy(t *testing.T) {
	dir := t.TempDir()
	p := writeTOML(t, dir, "test.toml", `
[llm]
model = "new-model"

[llm.providers.ollama]
models = ["new"]

[llm.models]
default = "old-model"

[llm.models.providers.ollama]
models = ["old"]

[llm.models.providers.lmstudio]
models = ["kept"]
`)
	c, err := cfg.LoadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.LLM.Model != "new-model" {
		t.Errorf("model = %q, want new-model", c.LLM.Model)
	}
	if strings.Join(c.LLM.Providers["ollama"].Models, ",") != "new" {
		t.Errorf("ollama = %+v, new key must win", c.LLM.Providers["ollama"])
	}
	if strings.Join(c.LLM.Providers["lmstudio"].Models, ",") != "kept" {
		t.Errorf("legacy-only provider must be kept, got %+v", c.LLM.Providers)
	}
}

func TestLoadFile_LLMConflictingProviderIsError(t *testing.T) {
	for name, body := range map[string]string{
		"both legacy flags": "[llm]\nuse_ollama = true\nuse_openrouter = true\n",
		"flag vs provider":  "[llm]\nprovider = \"openrouter\"\nuse_ollama = true\n",
		"unknown provider":  "[llm]\nprovider = \"gpt\"\n",
	} {
		t.Run(name, func(t *testing.T) {
			p := writeTOML(t, t.TempDir(), "test.toml", body)
			if _, err := cfg.LoadFile(p); err == nil {
				t.Errorf("expected an error for %q", body)
			}
		})
	}
}

func TestLoadFile_LLMMatchingLegacyFlagIsNotError(t *testing.T) {
	p := writeTOML(t, t.TempDir(), "test.toml", "[llm]\nprovider = \"ollama\"\nuse_ollama = true\n")
	c, err := cfg.LoadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.LLM.Provider != "ollama" {
		t.Errorf("provider = %q", c.LLM.Provider)
	}
}

func TestLoadLayered_LLMProjectOverridesGlobal(t *testing.T) {
	dir := t.TempDir()
	global := writeTOML(t, dir, "global.toml", `
[llm]
provider = "ollama"
model = "qwen3:8b"

[llm.providers.ollama]
models = ["qwen3:8b"]
`)
	project := writeTOML(t, dir, "project.toml", `
[llm]
provider = "openrouter"

[llm.providers.openrouter]
models = ["moonshotai/kimi-k3"]
`)
	c, err := cfg.LoadLayered(global, project, func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	if c.LLM.Provider != "openrouter" {
		t.Errorf("provider = %q, project must win", c.LLM.Provider)
	}
	if c.LLM.Model != "" {
		t.Errorf("model = %q, the global ollama model must not follow a switch to openrouter", c.LLM.Model)
	}
	if len(c.LLM.Providers) != 2 {
		t.Errorf("providers must accumulate, got %+v", c.LLM.Providers)
	}
}

func TestLoadFile_LLMModelPrefixStripsButDoesNotSetProvider(t *testing.T) {
	p := writeTOML(t, t.TempDir(), "test.toml", "[llm]\nmodel = \"ollama/qwen3:8b\"\n")
	c, err := cfg.LoadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.LLM.ActiveProvider() != "default" || c.LLM.Model != "qwen3:8b" {
		t.Errorf("provider/model = %q/%q, want default/qwen3:8b (prefix stripped, provider unchanged)", c.LLM.ActiveProvider(), c.LLM.Model)
	}
	if got := strings.Join(deprecatedNames(c), ","); got != `[llm] model = "<provider>/<model>"` {
		t.Errorf("deprecated = %q", got)
	}
}

func TestLoadFile_LLMModelMatchingPrefixIsStripped(t *testing.T) {
	p := writeTOML(t, t.TempDir(), "test.toml", "[llm]\nprovider = \"openrouter\"\nmodel = \"openrouter/moonshotai/kimi-k3\"\n")
	c, err := cfg.LoadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.LLM.Model != "moonshotai/kimi-k3" {
		t.Errorf("model = %q, want moonshotai/kimi-k3", c.LLM.Model)
	}
	if len(c.DeprecatedUses) != 1 {
		t.Errorf("want one prefix warning, got %v", deprecatedNames(c))
	}
}

// OpenRouter model IDs are vendor/model, including vendors that share a name
// with a provider ("anthropic/..."). They must pass through untouched.
func TestLoadFile_LLMModelIsProvidersOwnID(t *testing.T) {
	for _, m := range []string{"deepseek/deepseek-v4-pro", "anthropic/claude-sonnet-4.5"} {
		p := writeTOML(t, t.TempDir(), "test.toml", "[llm]\nprovider = \"openrouter\"\nmodel = \""+m+"\"\n")
		c, err := cfg.LoadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if c.LLM.Model != m || len(c.DeprecatedUses) != 0 {
			t.Errorf("model = %q (warnings %v), want %q untouched", c.LLM.Model, deprecatedNames(c), m)
		}
	}
}

func TestLoadFile_LLMInvalidProviderSetupIsError(t *testing.T) {
	for name, body := range map[string]string{
		"prefix for another provider": "[llm]\nprovider = \"openrouter\"\nmodel = \"ollama/qwen3:8b\"\n",
		"custom provider no base_url": "[llm.providers.vllm]\nmodels = [\"x\"]\n",
	} {
		t.Run(name, func(t *testing.T) {
			p := writeTOML(t, t.TempDir(), "test.toml", body)
			if _, err := cfg.LoadFile(p); err == nil {
				t.Errorf("expected an error for %q", body)
			}
		})
	}
}

func TestLoadFile_LLMCustomProviderWithBaseURL(t *testing.T) {
	p := writeTOML(t, t.TempDir(), "test.toml", "[llm.providers.vllm]\nbase_url = \"http://gpu:8000/v1\"\nmodels = [\"x\"]\n")
	if _, err := cfg.LoadFile(p); err != nil {
		t.Fatal(err)
	}
}

func TestLoadLayered_LLMProviderSwitchDropsGlobalModel(t *testing.T) {
	dir := t.TempDir()
	global := writeTOML(t, dir, "global.toml", "[llm]\nprovider = \"ollama\"\nmodel = \"qwen3:8b\"\n")
	for _, tc := range []struct{ project, model string }{
		{"[llm]\nprovider = \"openrouter\"\n", ""},
		{"[llm]\nprovider = \"ollama\"\n", "qwen3:8b"},
		{"[llm]\nprovider = \"openrouter\"\nmodel = \"x-ai/grok-4.6\"\n", "x-ai/grok-4.6"},
	} {
		project := writeTOML(t, dir, "project.toml", tc.project)
		c, err := cfg.LoadLayered(global, project, func(string) string { return "" })
		if err != nil {
			t.Fatal(err)
		}
		if c.LLM.Model != tc.model {
			t.Errorf("project %q: model = %q, want %q", tc.project, c.LLM.Model, tc.model)
		}
	}
}

func TestLLMSection_ModelFor(t *testing.T) {
	l := cfg.LLMSection{Provider: "openrouter", Model: "deepseek/deepseek-v4-pro"}
	if got := l.ModelFor("openrouter"); got != "deepseek/deepseek-v4-pro" {
		t.Errorf("ModelFor(openrouter) = %q", got)
	}
	if got := l.ModelFor("ollama"); got != "" {
		t.Errorf("ModelFor(ollama) = %q, the model belongs to openrouter", got)
	}
	if got := (cfg.LLMSection{Model: "claude-opus-5-5"}).ModelFor("default"); got != "claude-opus-5-5" {
		t.Errorf("empty provider must mean default, got %q", got)
	}
}

// The go-winkit scenario: [llm.models] default = "ollama/..." without an
// explicit provider or use_ollama. The prefix identifies the model's home
// provider but must not activate it.
func TestLoadFile_LLMLegacyModelsDefaultWithoutProviderStaysDefault(t *testing.T) {
	dir := t.TempDir()
	p := writeTOML(t, dir, "test.toml", `
[llm.models]
default = "ollama/glm-4.7-flash:latest"

[llm.models.providers.ollama]
models = ["glm-4.7-flash:latest", "llama3.2:3b"]
`)
	c, err := cfg.LoadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.LLM.ActiveProvider() != "default" {
		t.Errorf("provider = %q, want default (no explicit provider set)", c.LLM.ActiveProvider())
	}
	if c.LLM.Model != "glm-4.7-flash:latest" {
		t.Errorf("model = %q, want glm-4.7-flash:latest (prefix stripped)", c.LLM.Model)
	}
	if got := c.LLM.Providers["ollama"]; len(got.Models) != 2 {
		t.Errorf("ollama provider models = %v, want 2 entries", got.Models)
	}
}

func TestLoadFile_LLMAnthropicProviderPointsToDefault(t *testing.T) {
	p := writeTOML(t, t.TempDir(), "test.toml", "[llm]\nprovider = \"anthropic\"\n")
	_, err := cfg.LoadFile(p)
	if err == nil || !strings.Contains(err.Error(), `"default"`) {
		t.Errorf("err = %v, want a hint to use \"default\"", err)
	}
}

// A project turns off a global provider override with provider = "default".
func TestLoadLayered_LLMProjectDefaultOverridesGlobalProvider(t *testing.T) {
	dir := t.TempDir()
	global := writeTOML(t, dir, "global.toml", "[llm]\nprovider = \"openrouter\"\nmodel = \"deepseek/deepseek-v4-pro\"\n")
	project := writeTOML(t, dir, "project.toml", "[llm]\nprovider = \"default\"\n")
	c, err := cfg.LoadLayered(global, project, func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	if c.LLM.ActiveProvider() != "default" || c.LLM.Model != "" {
		t.Errorf("provider/model = %q/%q, want default with no model", c.LLM.ActiveProvider(), c.LLM.Model)
	}
}
