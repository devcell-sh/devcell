// devcell runs an AI coding agent (Claude Code, Codex, OpenCode) inside a
// reproducible, isolated container called a cell, so the agent cannot touch
// host SSH keys, other repos, or credentials.
package main

func main() {
	Execute()
}
