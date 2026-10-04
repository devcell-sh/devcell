package main

import (
	"errors"
	"strings"
	"testing"
)

func TestFlagUsageError_KnownFlagShowsItsHelp(t *testing.T) {
	err := flagUsageError(rootCmd, errors.New("flag needs an argument: --engine"))
	msg := err.Error()
	for _, want := range []string{
		"flag needs an argument: --engine",
		"--engine string",
		"execution engine",
		"cell --help",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("message must contain %q, got:\n%s", want, msg)
		}
	}
	var ue *usageError
	if !errors.As(err, &ue) {
		t.Errorf("want *usageError so Execute skips the version banner, got %T", err)
	}
}

func TestFlagUsageError_UnknownFlagOnlyPointsAtHelp(t *testing.T) {
	err := flagUsageError(buildCmd, errors.New("unknown flag: --bogus"))
	msg := err.Error()
	if !strings.HasPrefix(msg, "unknown flag: --bogus") {
		t.Errorf("message must start with the original error, got:\n%s", msg)
	}
	if !strings.Contains(msg, "cell build --help") {
		t.Errorf("message must point at the subcommand's help, got:\n%s", msg)
	}
	if strings.Contains(msg, " string ") {
		t.Errorf("no flag help line expected for an unknown flag, got:\n%s", msg)
	}
}
