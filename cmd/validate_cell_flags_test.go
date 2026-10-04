package main

import "testing"

// Agent commands set DisableFlagParsing, so a cell string flag with no value
// never reaches cobra's "flag needs an argument" check. validateCellFlags
// is that check for them.
func TestValidateCellFlags(t *testing.T) {
	for _, tc := range []struct {
		args    []string
		wantErr string
	}{
		{args: nil},
		{args: []string{"claude", "-p", "hi"}},
		{args: []string{"claude", "--engine", "docker"}},
		{args: []string{"--engine=docker", "claude"}},
		{args: []string{"--os", "linux", "--format", "json", "claude"}},
		{args: []string{"claude", "--resume", "--dry-run"}}, // agent flags pass through untouched
		{args: []string{"claude", "--engine"}, wantErr: "flag needs an argument: --engine"},
		{args: []string{"claude", "--engine="}, wantErr: "flag needs an argument: --engine"},
		{args: []string{"claude", "--engine", "--dry-run"}, wantErr: "flag needs an argument: --engine"},
		{args: []string{"claude", "--os"}, wantErr: "flag needs an argument: --os"},
		{args: []string{"--os=", "claude"}, wantErr: "flag needs an argument: --os"},
		{args: []string{"claude", "--cell-name", "-c"}, wantErr: "flag needs an argument: --cell-name"},
		{args: []string{"claude", "--format"}, wantErr: "flag needs an argument: --format"},
		{args: []string{"--engine", "docker", "claude", "--base-image"}, wantErr: "flag needs an argument: --base-image"},
	} {
		err := validateCellFlags(tc.args)
		switch {
		case tc.wantErr == "" && err != nil:
			t.Errorf("%q: unexpected error %v", tc.args, err)
		case tc.wantErr != "" && err == nil:
			t.Errorf("%q: want error %q, got nil", tc.args, tc.wantErr)
		case tc.wantErr != "" && err.Error() != tc.wantErr:
			t.Errorf("%q: error = %q, want %q", tc.args, err, tc.wantErr)
		}
	}
}
