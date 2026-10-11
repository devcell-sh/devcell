package main

import (
	"sort"
	"testing"
)

func TestParseContainerNames(t *testing.T) {
	input := "devcell-devcell-271\ndevcell-devcell-246\n"
	names := parseContainerNames(input)
	if len(names) != 2 {
		t.Fatalf("expected 2 names, got %d: %v", len(names), names)
	}
	if names[0] != "devcell-271" || names[1] != "devcell-246" {
		t.Errorf("unexpected names: %v", names)
	}
}

func TestParseContainerNames_IgnoresNonDevcell(t *testing.T) {
	input := "devcell-devcell-271\nsome-other-container\nryuk-abc123\n"
	names := parseContainerNames(input)
	if len(names) != 1 || names[0] != "devcell-271" {
		t.Errorf("expected [devcell-271], got: %v", names)
	}
}

func TestParseContainerNames_IgnoresSidecars(t *testing.T) {
	input := "devcell-myproject-3\ndevcell-myproject-3-gw\n"
	names := parseContainerNames(input)
	if len(names) != 1 || names[0] != "myproject-3" {
		t.Errorf("expected [myproject-3], got: %v", names)
	}
}

func TestParseContainerNames_Empty(t *testing.T) {
	names := parseContainerNames("")
	if len(names) != 0 {
		t.Errorf("expected empty, got: %v", names)
	}
}

func TestParseContainerNames_ArbitraryCellName(t *testing.T) {
	input := "devcell-myproject-3\ndevcell-foo-bar-0\n"
	names := parseContainerNames(input)
	if len(names) != 2 {
		t.Fatalf("expected 2 names, got %d: %v", len(names), names)
	}
	if names[0] != "myproject-3" {
		t.Errorf("names[0] = %q, want myproject-3", names[0])
	}
	if names[1] != "foo-bar-0" {
		t.Errorf("names[1] = %q, want foo-bar-0", names[1])
	}
}

func TestResolveAppArg_FullNamePassthrough(t *testing.T) {
	got := resolveAppArg("devcell-271")
	if got != "devcell-271" {
		t.Errorf("expected devcell-271, got %q", got)
	}
}

func TestResolveAppArg_SuffixNoMatch(t *testing.T) {
	got := resolveAppArg("999")
	if got != "999" {
		t.Errorf("expected 999, got %q", got)
	}
}

func TestSelectCellOptions_Sorted(t *testing.T) {
	apps := map[string]string{
		"devcell-271":  "27189",
		"devcell-246":  "24689",
		"myproject-42": "4289",
	}
	var names []string
	for name := range apps {
		names = append(names, name)
	}
	sort.Strings(names)
	expected := []string{"devcell-246", "devcell-271", "myproject-42"}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("names[%d] = %q, want %q", i, name, expected[i])
		}
	}
}
