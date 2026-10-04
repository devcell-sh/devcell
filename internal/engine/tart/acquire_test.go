package tart

import (
	"strings"
	"testing"
	"time"
)

func TestAcquireInputs_Defaults(t *testing.T) {
	a := AcquireInputs{}
	a.ApplyDefaults()

	if a.SSHHost != "localhost" {
		t.Errorf("SSHHost = %q, want localhost", a.SSHHost)
	}
	if a.SSHPort != 22 {
		t.Errorf("SSHPort = %d, want 22", a.SSHPort)
	}
	if a.SSHTimeout != 120*time.Second {
		t.Errorf("SSHTimeout = %v, want 120s", a.SSHTimeout)
	}
}

func TestAcquireInputs_Validate_External(t *testing.T) {
	a := AcquireInputs{ExternalVM: true}
	if err := a.Validate(); err != nil {
		t.Errorf("external VM should not require VMName: %v", err)
	}
}

func TestAcquireInputs_Validate_MissingVMName(t *testing.T) {
	a := AcquireInputs{}
	err := a.Validate()
	if err == nil {
		t.Fatal("expected error for missing VMName")
	}
	if !strings.Contains(err.Error(), "VMName") {
		t.Errorf("error = %q, want mention of VMName", err)
	}
}

func TestAcquireInputs_Validate_OK(t *testing.T) {
	a := AcquireInputs{VMName: "test-vm"}
	if err := a.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAcquireInputs_Validate_OK_WithTemplate(t *testing.T) {
	a := AcquireInputs{VMName: "test-vm", TemplateName: "devcell-tart-ultimate"}
	if err := a.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAcquireResult_ManagedVsExternal(t *testing.T) {
	managed := AcquireResult{Managed: true, SSHHost: "localhost", SSHPort: 22}
	if !managed.Managed {
		t.Error("expected Managed=true")
	}

	external := AcquireResult{Managed: false, SSHHost: "10.0.0.5", SSHPort: 2222}
	if external.Managed {
		t.Error("expected Managed=false")
	}
	if external.SSHHost != "10.0.0.5" {
		t.Errorf("SSHHost = %q, want 10.0.0.5", external.SSHHost)
	}
}
