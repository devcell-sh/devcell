package cfg

import (
	"testing"
)

func TestImpliedModules_WireguardEnabled(t *testing.T) {
	c := CellConfig{
		Wireguard: []WireguardEntry{{Name: "wg0", Enabled: true, Config: "[Interface]\n[Peer]"}},
	}
	got := ImpliedModules(c)
	if len(got) != 1 || got[0] != "wireguard" {
		t.Errorf("ImpliedModules with wireguard enabled = %v, want [wireguard]", got)
	}
}

func TestImpliedModules_WireguardDisabled(t *testing.T) {
	c := CellConfig{
		Wireguard: []WireguardEntry{{Name: "wg0", Enabled: false}},
	}
	got := ImpliedModules(c)
	if len(got) != 0 {
		t.Errorf("ImpliedModules with wireguard disabled = %v, want []", got)
	}
}

func TestImpliedModules_NoWireguard(t *testing.T) {
	c := CellConfig{}
	got := ImpliedModules(c)
	if len(got) != 0 {
		t.Errorf("ImpliedModules with no wireguard = %v, want []", got)
	}
}

func TestApplyImpliedModules_AddsWireguard(t *testing.T) {
	c := CellConfig{
		Wireguard: []WireguardEntry{{Name: "wg0", Enabled: true, Config: "[Interface]\n[Peer]"}},
	}
	c.Cell.Modules = []string{"go"}
	ApplyImpliedModules(&c)
	found := false
	for _, m := range c.Cell.Modules {
		if m == "wireguard" {
			found = true
		}
	}
	if !found {
		t.Errorf("ApplyImpliedModules did not add wireguard, modules = %v", c.Cell.Modules)
	}
}

func TestApplyImpliedModules_NoDuplicate(t *testing.T) {
	c := CellConfig{
		Wireguard: []WireguardEntry{{Name: "wg0", Enabled: true, Config: "[Interface]\n[Peer]"}},
	}
	c.Cell.Modules = []string{"go", "wireguard"}
	ApplyImpliedModules(&c)
	count := 0
	for _, m := range c.Cell.Modules {
		if m == "wireguard" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("ApplyImpliedModules duplicated wireguard, modules = %v", c.Cell.Modules)
	}
}

func TestApplyImpliedModules_NoopWhenNotNeeded(t *testing.T) {
	c := CellConfig{}
	c.Cell.Modules = []string{"go"}
	ApplyImpliedModules(&c)
	if len(c.Cell.Modules) != 1 || c.Cell.Modules[0] != "go" {
		t.Errorf("ApplyImpliedModules changed modules when it shouldn't, modules = %v", c.Cell.Modules)
	}
}
