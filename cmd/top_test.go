package main

import "testing"

func TestTopCmd_Registered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"top"})
	if err != nil {
		t.Fatalf("top command not found: %v", err)
	}
	if cmd.Use != "top" {
		t.Errorf("unexpected Use: %q", cmd.Use)
	}
}

func TestTopCmd_GlobalFlag(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"top"})
	if err != nil {
		t.Fatal(err)
	}
	f := cmd.Flags().Lookup("global")
	if f == nil {
		t.Fatal("--global flag not found")
	}
}
