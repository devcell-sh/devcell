package main

import (
	"strings"

	"github.com/DimmKirr/devcell/internal/cfg"
)

// llmDefaultBaseURLs are the API roots of the built-in providers, as seen
// from inside the container, without the /v1 suffix. Claude Code takes the
// root (it appends /v1 itself); OpenAI-compat clients take root + "/v1".
var llmDefaultBaseURLs = map[string]string{
	cfg.LLMProviderOllama:     "http://host.docker.internal:11434",
	cfg.LLMProviderLMStudio:   "http://host.docker.internal:1234",
	cfg.LLMProviderOpenRouter: "https://openrouter.ai/api",
}

// llmBaseURL returns the API root of a built-in provider: its
// [llm.providers.<name>] base_url (written with or without /v1), else the
// default.
func llmBaseURL(l cfg.LLMSection, provider string) string {
	u := strings.TrimSuffix(strings.TrimRight(l.Providers[provider].BaseURL, "/"), "/v1")
	if u == "" {
		return llmDefaultBaseURLs[provider]
	}
	return u
}
