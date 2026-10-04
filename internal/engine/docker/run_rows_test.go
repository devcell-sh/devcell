package docker

import "testing"

func TestSystemPromptRow(t *testing.T) {
	name, detail := systemPromptRow(false, "/p/.devcell/prompts/x.md")
	if name != "System prompt configured" || detail != "" {
		t.Errorf("non-debug: got %q/%q", name, detail)
	}
	name, detail = systemPromptRow(true, "/p/.devcell/prompts/x.md")
	if name != "System prompt" || detail != "/p/.devcell/prompts/x.md" {
		t.Errorf("debug: got %q/%q", name, detail)
	}
}
