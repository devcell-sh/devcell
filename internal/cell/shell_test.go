package cell_test

import (
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/DimmKirr/devcell/internal/cell"
)

func TestShellQuote(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", "''"},
		{"claude", "claude"},
		{"--flag=value", "--flag=value"},
		{"/run/secrets:mode=700,noexec", "/run/secrets:mode=700,noexec"},
		{"user@host", "user@host"},
		{"50%", "50%"},
		{"a+b", "a+b"},
		{"hello world", "'hello world'"},
		{"it's", `'it'\''s'`},
		{"'", `''\'''`},
		{"$HOME", "'$HOME'"},
		{"a;b", "'a;b'"},
		{"*", "'*'"},
		{"~/x", "'~/x'"},
		{`say "hi"`, `'say "hi"'`},
		{`back\slash`, `'back\slash'`},
		{"line\nbreak", "'line\nbreak'"},
		{"café", "'café'"},
	}
	for _, tc := range cases {
		if got := cell.ShellQuote(tc.in); got != tc.want {
			t.Errorf("ShellQuote(%q) = %s, want %s", tc.in, got, tc.want)
		}
	}
}

func TestShellJoin(t *testing.T) {
	got := cell.ShellJoin([]string{"echo", "it's here", "", "plain"})
	want := `echo 'it'\''s here' '' plain`
	if got != want {
		t.Errorf("ShellJoin = %s, want %s", got, want)
	}
}

// TestShellJoin_RoundTripsThroughSh proves the output is one word per input
// when a real POSIX shell parses it, including nested quoting.
func TestShellJoin_RoundTripsThroughSh(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh not installed")
	}
	words := []string{
		"plain", "", "with space", "it's", "''", `"dq"`, `back\slash`,
		"$HOME", "`id`", "a;b|c&d", "*?[x]", "~/tilde", "tab\there", "new\nline",
		cell.ShellJoin([]string{"nested", "it's", "a b"}),
	}
	out, err := exec.Command(sh, "-c", `printf '%s\0' `+cell.ShellJoin(words)).Output()
	if err != nil {
		t.Fatalf("sh: %v", err)
	}
	got := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
	if !reflect.DeepEqual(got, words) {
		t.Errorf("round trip mismatch:\n got %q\nwant %q", got, words)
	}
}
