package docker

// systemPromptRow names the "System prompt" checklist row. The generated
// prompt path is an implementation detail, so outside --debug the row is a
// plain "System prompt configured" with no detail.
func systemPromptRow(verbose bool, path string) (name, detail string) {
	if verbose {
		return "System prompt", path
	}
	return "System prompt configured", ""
}
