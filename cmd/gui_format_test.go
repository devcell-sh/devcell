package main

// White-box tests for renderCellList (`cell rdp --list`, `cell vnc --list`).
// Package main for access to unexported symbols.

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/DimmKirr/devcell/internal/gui"
	"github.com/DimmKirr/devcell/internal/ux"
)

// captureStdoutMain redirects os.Stdout during fn and returns what was written.
// (package main equivalent of ux_test's captureStdout)
func captureStdoutMain(fn func()) string {
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// guiFormatCases are the per-protocol fixtures every renderCellList test runs
// against: port is a docker cell's host port, vmPort a winkit VM's.
var guiFormatCases = []struct {
	p         gui.Protocol
	port      string
	vmPort    string
	emptyText string // exact text-mode output when no cell is running
	urlPrefix string
}{
	{gui.RDP, "3389", "40589", "No running cell containers with RDP found.\n", "rdp://"},
	{gui.VNC, "5900", "40550", "No running cell containers found.\n", "vnc://"},
}

func ep(host, port, engine string) gui.CellEndpoint {
	return gui.CellEndpoint{Host: host, Port: port, Engine: engine}
}

// renderCellListOutput renders m for p in the given output format and
// returns what was printed. The format is reset to "text" after the test.
func renderCellListOutput(t *testing.T, p gui.Protocol, format string, m map[string]gui.CellEndpoint) string {
	t.Helper()
	ux.OutputFormat = format
	t.Cleanup(func() { ux.OutputFormat = "text" })
	return captureStdoutMain(func() { renderCellList(p, m) })
}

func TestGUIFormat_JSONFormat(t *testing.T) {
	for _, tc := range guiFormatCases {
		t.Run(string(tc.p), func(t *testing.T) {
			m := map[string]gui.CellEndpoint{"devcell-42-run": ep("127.0.0.1", tc.port, "docker")}

			out := renderCellListOutput(t, tc.p, "json", m)

			var result []map[string]string
			if err := json.Unmarshal([]byte(out), &result); err != nil {
				t.Fatalf("not valid JSON: %v\noutput: %q", err, out)
			}
			if len(result) != 1 {
				t.Fatalf("want 1 entry, got %d", len(result))
			}
			if result[0]["app_name"] != "devcell-42-run" {
				t.Errorf("want app_name=devcell-42-run, got %q", result[0]["app_name"])
			}
			if !strings.Contains(result[0]["address"], tc.port) {
				t.Errorf("want address to contain port %s, got %q", tc.port, result[0]["address"])
			}
		})
	}
}

func TestGUIFormat_EmptyMapJSON(t *testing.T) {
	for _, tc := range guiFormatCases {
		t.Run(string(tc.p), func(t *testing.T) {
			out := renderCellListOutput(t, tc.p, "json", map[string]gui.CellEndpoint{})

			var result []map[string]string
			if err := json.Unmarshal([]byte(out), &result); err != nil {
				t.Fatalf("empty list should produce valid JSON array: %v\noutput: %q", err, out)
			}
			if len(result) != 0 {
				t.Errorf("want empty array, got %d entries", len(result))
			}
		})
	}
}

func TestGUIFormat_EmptyMapText(t *testing.T) {
	for _, tc := range guiFormatCases {
		t.Run(string(tc.p), func(t *testing.T) {
			out := renderCellListOutput(t, tc.p, "text", map[string]gui.CellEndpoint{})

			if !strings.Contains(out, "No running") {
				t.Errorf("text empty message should contain 'No running', got: %q", out)
			}
			if out != tc.emptyText {
				t.Errorf("text empty message = %q, want %q", out, tc.emptyText)
			}
		})
	}
}

func TestGUIFormat_TextContainsCellNameAndPort(t *testing.T) {
	for _, tc := range guiFormatCases {
		t.Run(string(tc.p), func(t *testing.T) {
			m := map[string]gui.CellEndpoint{"cell-abc-run": ep("127.0.0.1", tc.port, "docker")}

			out := renderCellListOutput(t, tc.p, "text", m)

			if !strings.Contains(out, "cell-abc-run") {
				t.Errorf("text output should contain app name, got: %q", out)
			}
			if !strings.Contains(out, tc.port) {
				t.Errorf("text output should contain port, got: %q", out)
			}
		})
	}
}

func TestGUIFormat_URLIncludedInJSON(t *testing.T) {
	for _, tc := range guiFormatCases {
		t.Run(string(tc.p), func(t *testing.T) {
			m := map[string]gui.CellEndpoint{"cell-1-run": ep("127.0.0.1", tc.port, "docker")}

			out := renderCellListOutput(t, tc.p, "json", m)

			var result []map[string]string
			if err := json.Unmarshal([]byte(out), &result); err != nil {
				t.Fatalf("not valid JSON: %v", err)
			}
			url := result[0]["url"]
			if !strings.Contains(url, tc.port) {
				t.Errorf("url should contain port %s, got %q", tc.port, url)
			}
			if !strings.HasPrefix(url, tc.urlPrefix) {
				t.Errorf("url should start with %q, got %q", tc.urlPrefix, url)
			}
		})
	}
}

// L0: winkit VM entries ("winkit-<cell>") render correctly; renderCellList is
// pure (no I/O).

func TestGUIFormat_VMEntryText(t *testing.T) {
	for _, tc := range guiFormatCases {
		t.Run(string(tc.p), func(t *testing.T) {
			m := map[string]gui.CellEndpoint{"winkit-main": ep("127.0.0.1", tc.vmPort, "winkit")}

			out := renderCellListOutput(t, tc.p, "text", m)

			if !strings.Contains(out, "winkit-main") {
				t.Errorf("text output must contain VM app name, got: %q", out)
			}
			if !strings.Contains(out, tc.vmPort) {
				t.Errorf("text output must contain VM %s port, got: %q", tc.p.Label(), out)
			}
		})
	}
}

func TestGUIFormat_VMEntryJSON(t *testing.T) {
	for _, tc := range guiFormatCases {
		t.Run(string(tc.p), func(t *testing.T) {
			m := map[string]gui.CellEndpoint{"winkit-main": ep("127.0.0.1", tc.vmPort, "winkit")}

			out := renderCellListOutput(t, tc.p, "json", m)

			var result []map[string]string
			if err := json.Unmarshal([]byte(out), &result); err != nil {
				t.Fatalf("not valid JSON: %v\noutput: %q", err, out)
			}
			if len(result) != 1 {
				t.Fatalf("want 1 entry, got %d", len(result))
			}
			if result[0]["app_name"] != "winkit-main" {
				t.Errorf("want app_name=winkit-main, got %q", result[0]["app_name"])
			}
			if !strings.Contains(result[0]["address"], tc.vmPort) {
				t.Errorf("want address to contain port %s, got %q", tc.vmPort, result[0]["address"])
			}
		})
	}
}

func TestGUIFormat_MixedDockerAndVM(t *testing.T) {
	for _, tc := range guiFormatCases {
		t.Run(string(tc.p), func(t *testing.T) {
			m := map[string]gui.CellEndpoint{
				"cell-myproject-3-run": ep("127.0.0.1", tc.port, "docker"),
				"winkit-main":          ep("127.0.0.1", tc.vmPort, "winkit"),
			}

			out := renderCellListOutput(t, tc.p, "text", m)

			if !strings.Contains(out, "cell-myproject-3-run") {
				t.Errorf("text output must contain docker app name, got: %q", out)
			}
			if !strings.Contains(out, "winkit-main") {
				t.Errorf("text output must contain VM app name, got: %q", out)
			}
		})
	}
}

func TestGUIFormat_TartVMEntry(t *testing.T) {
	m := map[string]gui.CellEndpoint{
		"tart-main": ep("192.168.64.5", "5900", "tart"),
	}
	out := renderCellListOutput(t, gui.VNC, "text", m)
	if !strings.Contains(out, "tart-main") {
		t.Errorf("text output must contain tart VM name, got: %q", out)
	}
	if !strings.Contains(out, "192.168.64.5:5900") {
		t.Errorf("text output must contain tart VM address, got: %q", out)
	}
}

func TestGUIFormat_TartVMEntryJSON(t *testing.T) {
	m := map[string]gui.CellEndpoint{
		"tart-main": ep("192.168.64.5", "5900", "tart"),
	}
	out := renderCellListOutput(t, gui.VNC, "json", m)

	var result []map[string]string
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("not valid JSON: %v\noutput: %q", err, out)
	}
	if len(result) != 1 {
		t.Fatalf("want 1 entry, got %d", len(result))
	}
	if result[0]["app_name"] != "tart-main" {
		t.Errorf("want app_name=tart-main, got %q", result[0]["app_name"])
	}
	if result[0]["address"] != "192.168.64.5:5900" {
		t.Errorf("want address=192.168.64.5:5900, got %q", result[0]["address"])
	}
	url := result[0]["url"]
	if !strings.Contains(url, "192.168.64.5") {
		t.Errorf("url should contain VM IP, got %q", url)
	}
}
