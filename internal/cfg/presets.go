package cfg

import (
	"fmt"
	"sort"
	"strings"
)

// Presets maps preset names to their domain lists.
var Presets = map[string][]string{
	"claude": {
		"api.anthropic.com",
		"claude.ai",
		"claude.com",
		"platform.claude.com",
		"mcp-proxy.anthropic.com",
		"downloads.claude.ai",
		"code.claude.com",
		"storage.googleapis.com",
	},
	"codex": {
		"api.openai.com",
		"chatgpt.com",
		"auth.openai.com",
		"*.oaistatic.com",
		"*.oaiusercontent.com",
		"platform.openai.com",
	},
	"gemini": {
		"generativelanguage.googleapis.com",
		"oauth2.googleapis.com",
		"accounts.google.com",
	},
	"opencode": {
		"opencode.ai",
		"models.dev",
	},
	"openrouter": {
		"openrouter.ai",
	},
	"github": {
		"github.com",
		"api.github.com",
		"codeload.github.com",
		"*.githubusercontent.com",
		"ghcr.io",
	},
	"npm": {
		"registry.npmjs.org",
	},
	"pypi": {
		"pypi.org",
		"files.pythonhosted.org",
	},
	"go": {
		"proxy.golang.org",
		"sum.golang.org",
		"storage.googleapis.com",
	},
	"cargo": {
		"index.crates.io",
		"static.crates.io",
	},
	"nix": {
		"cache.nixos.org",
		"channels.nixos.org",
	},
	"mise": {
		"mise-versions.jdx.dev",
		"nodejs.org",
	},
	"docker": {
		"registry-1.docker.io",
		"auth.docker.io",
		"production.cloudflare.docker.com",
		"hub.docker.com",
	},
	"devcell": {
		"devcell.sh",
		"host.docker.internal",
	},
	"1password": {
		"*.1password.com",
		"*.1passwordservices.com",
		"*.1passwordusercontent.com",
	},
}

// presetIncludes maps preset names to other presets they implicitly include.
var presetIncludes = map[string][]string{
	"nix":  {"github"},
	"mise": {"github"},
}

// ExpandPresets expands a list of preset names into a deduplicated, sorted
// domain list. Returns an error if any preset name is unknown.
func ExpandPresets(names []string) ([]string, error) {
	seen := make(map[string]bool)
	var domains []string

	var expand func(name string) error
	expand = func(name string) error {
		if seen[name] {
			return nil
		}
		seen[name] = true
		ds, ok := Presets[name]
		if !ok {
			return fmt.Errorf("unknown network proxy preset: %q (known: %s)", name, knownPresetNames())
		}
		domains = append(domains, ds...)
		for _, inc := range presetIncludes[name] {
			if err := expand(inc); err != nil {
				return err
			}
		}
		return nil
	}

	for _, name := range names {
		if err := expand(name); err != nil {
			return nil, err
		}
	}

	deduped := dedupStrings(domains)
	sort.Strings(deduped)
	return deduped, nil
}

func knownPresetNames() string {
	names := make([]string, 0, len(Presets))
	for k := range Presets {
		names = append(names, k)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

func dedupStrings(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(s))
	out := make([]string, 0, len(s))
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}
