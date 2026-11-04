package main

import "testing"

func TestListCmd_Registered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"list"})
	if err != nil {
		t.Fatalf("list command not found: %v", err)
	}
	if cmd.Use != "list" {
		t.Errorf("unexpected Use: %q", cmd.Use)
	}
}

func TestFriendlyCellName(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"devcell-foo-1", "foo-1"},
		{"devcell-nopk-backend-87", "nopk-backend-87"},
		{"other-name", "other-name"},
		{"devcell-", ""},
	}
	for _, tt := range tests {
		if got := friendlyCellName(tt.in); got != tt.want {
			t.Errorf("friendlyCellName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
