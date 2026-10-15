package experiments

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Markers around the generated block in README.md and docs/experimental.md.
const (
	BeginMarker = "<!-- experiments:begin -->"
	EndMarker   = "<!-- experiments:end -->"
)

// Markdown renders the experiments table that README.md, docs/experimental.md
// and the website share. Doc links are prefixed with linkBase: "" for a
// file at the repo root, "../" from docs/, a GitHub blob URL for the site.
func Markdown(linkBase string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Off by default. Turn one on with `%s=<name>`, `%s=<name>` or `%s = [\"<name>\"]`; several are comma-separated. Experimental features may change or go away without a deprecation cycle.\n\n", Flag, Env, Key)
	b.WriteString("| Feature | What it does | Docs | Tracking |\n|---|---|---|---|\n")
	for _, x := range all {
		doc, ticket := "-", "-"
		if x.Doc != "" {
			doc = fmt.Sprintf("[%s](%s%s)", x.Doc, linkBase, x.Doc)
		}
		if x.Ticket != "" {
			ticket = x.Ticket
		}
		state := ""
		if x.State != Active {
			state = " (" + x.State.String() + ")"
		}
		fmt.Fprintf(&b, "| `%s`%s | %s | %s | %s |\n", x.Name, state, x.Summary, doc, ticket)
	}
	return b.String()
}

// JSON renders the registry for the website (web/src/data/experimental.json).
func JSON() ([]byte, error) {
	type entry struct {
		Name    string `json:"name"`
		Group   string `json:"group"`
		Summary string `json:"summary"`
		Doc     string `json:"doc"`
		Ticket  string `json:"ticket"`
		State   string `json:"state"`
	}
	var out []entry
	for _, x := range all {
		out = append(out, entry{x.Name, x.Group(), x.Summary, x.Doc, x.Ticket, x.State.String()})
	}
	return json.MarshalIndent(out, "", "  ")
}

// Block returns the text between the markers in doc.
func Block(doc string) (string, bool) {
	_, rest, ok := strings.Cut(doc, BeginMarker)
	if !ok {
		return "", false
	}
	block, _, ok := strings.Cut(rest, EndMarker)
	return block, ok
}

// ReplaceBlock swaps the text between the markers in doc for body.
func ReplaceBlock(doc, body string) (string, error) {
	head, rest, ok := strings.Cut(doc, BeginMarker)
	if !ok {
		return "", errors.New("missing " + BeginMarker)
	}
	_, tail, ok := strings.Cut(rest, EndMarker)
	if !ok {
		return "", errors.New("missing " + EndMarker)
	}
	return head + BeginMarker + "\n" + strings.TrimSpace(body) + "\n" + EndMarker + tail, nil
}
