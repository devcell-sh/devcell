package main

import (
	"strings"
	"testing"
)

func TestDoctorCmd_RequiresExperiment(t *testing.T) {
	// Without the ui-doctor experiment enabled, the command should fail
	// with a RequiredError mentioning the experiment name.
	cmd := doctorCmd
	cmd.SetArgs([]string{})
	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("doctor should require ui-doctor experiment")
	}
	if !strings.Contains(err.Error(), "ui-doctor") {
		t.Errorf("error should mention ui-doctor, got: %v", err)
	}
}
