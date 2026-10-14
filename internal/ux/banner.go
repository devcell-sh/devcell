package ux

import (
	"strings"
)

// Banner renders the cell-open header — a single-line identity strip used at
// the top of every `cell <command>` invocation:
//
//	CELL > devcell-304
//
// Workspace name is bold + brand orange; the separator and cell ID are muted.
// Two segments are suppressed when they carry no signal:
//   - workspace name empty (single-pane default `main`)
//   - cellID "0" or "" (the no-multiplexer fallback; surfacing "-0" is noise).
func Banner(cell, project, cellID string) string {
	var parts []string
	if cell != "" {
		parts = append(parts,
			StyleInfo.Bold(true).Render(cell),
			StyleMuted.Render(">"),
		)
	}
	suffix := ""
	if cellID != "" && cellID != "0" {
		suffix = StyleMuted.Render("-"+cellID)
	}
	parts = append(parts, project+suffix)
	return strings.Join(parts, " ")
}

// KV renders an aligned key-value row for the cell-open detail block.
// keyWidth is the *key column width* — the value column starts at
// `keyWidth + 2` (2-space gap). Keys longer than keyWidth still get a minimum
// 2-space gap so nothing collides. Pure — no global state.
func KV(keyWidth int, key, value string) string {
	pad := keyWidth - len(key) + 2
	if pad < 2 {
		pad = 2
	}
	return key + strings.Repeat(" ", pad) + value
}
