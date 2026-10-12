package cfg

import "testing"

func TestOpenPolicy_Label(t *testing.T) {
	p := OpenPolicy()
	if p.Label() != "open" {
		t.Errorf("Label() = %q, want %q", p.Label(), "open")
	}
}

func TestOpenPolicy_AllowsEverything(t *testing.T) {
	p := OpenPolicy()
	if p.ForceProxy() {
		t.Error("open policy should not force proxy")
	}
	if p.DenyPorts() {
		t.Error("open policy should not deny ports")
	}
	if p.DenyDirectInternet() {
		t.Error("open policy should not deny direct internet")
	}
	if p.DenyHostAccess() {
		t.Error("open policy should not deny host access")
	}
}

func TestOpenPolicy_DockerAllowsEverything(t *testing.T) {
	dp, ok := OpenPolicy().(DockerPolicy)
	if !ok {
		t.Fatal("open policy must implement DockerPolicy")
	}
	if dp.DropAllCaps() {
		t.Error("open policy should not drop all caps")
	}
	if dp.DenyPrivileged() {
		t.Error("open policy should not deny privileged")
	}
	if dp.DenyDockerSock() {
		t.Error("open policy should not deny docker socket")
	}
	if dp.DenyWireguard() {
		t.Error("open policy should not deny wireguard")
	}
	if dp.DenyKVM() {
		t.Error("open policy should not deny KVM")
	}
}

func TestOpenPolicy_VMAllowsEverything(t *testing.T) {
	vp, ok := OpenPolicy().(VMPolicy)
	if !ok {
		t.Fatal("open policy must implement VMPolicy")
	}
	if vp.RestrictNetwork() {
		t.Error("open policy should not restrict network")
	}
	if vp.IsolatedVNet() {
		t.Error("open policy should not use isolated vnet")
	}
	if vp.PublishProxyPort() {
		t.Error("open policy should not publish proxy port")
	}
}

func TestSolitaryPolicy_Label(t *testing.T) {
	p := SolitaryPolicy()
	if p.Label() != "solitary" {
		t.Errorf("Label() = %q, want %q", p.Label(), "solitary")
	}
}

func TestSolitaryPolicy_ForcesProxy(t *testing.T) {
	p := SolitaryPolicy()
	if !p.ForceProxy() {
		t.Error("solitary policy must force proxy")
	}
	if !p.DenyDirectInternet() {
		t.Error("solitary policy must deny direct internet")
	}
	if !p.DenyHostAccess() {
		t.Error("solitary policy must deny host access")
	}
	if !p.DenyPorts() {
		t.Error("solitary policy must deny ports")
	}
}

func TestSolitaryPolicy_DefaultPresets(t *testing.T) {
	p := SolitaryPolicy()
	presets := p.DefaultPresets()
	if len(presets) == 0 {
		t.Fatal("solitary policy must have default presets")
	}
	want := map[string]bool{"claude": true, "github": true, "npm": true, "devcell": true}
	for k := range want {
		found := false
		for _, p := range presets {
			if p == k {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("default presets missing %q", k)
		}
	}
}

func TestSolitaryPolicy_DockerLockdown(t *testing.T) {
	dp, ok := SolitaryPolicy().(DockerPolicy)
	if !ok {
		t.Fatal("solitary policy must implement DockerPolicy")
	}
	if !dp.DropAllCaps() {
		t.Error("solitary must drop all caps")
	}
	if len(dp.SafeCaps()) == 0 {
		t.Error("solitary must have safe caps to add back")
	}
	if !dp.DenyPrivileged() {
		t.Error("solitary must deny privileged")
	}
	if !dp.DenyDockerSock() {
		t.Error("solitary must deny docker socket")
	}
	if !dp.DenyWireguard() {
		t.Error("solitary must deny wireguard")
	}
	if !dp.DenyKVM() {
		t.Error("solitary must deny KVM")
	}
	if !dp.DenyTUN() {
		t.Error("solitary must deny TUN")
	}

	denied := dp.DeniedCaps()
	for _, cap := range []string{"NET_ADMIN", "SYS_ADMIN", "BPF"} {
		if !denied[cap] {
			t.Errorf("solitary must deny cap %s", cap)
		}
	}

	mounts := dp.DeniedMountPaths()
	if len(mounts) == 0 {
		t.Error("solitary must have denied mount paths")
	}
}

func TestSolitaryPolicy_VMLockdown(t *testing.T) {
	vp, ok := SolitaryPolicy().(VMPolicy)
	if !ok {
		t.Fatal("solitary policy must implement VMPolicy")
	}
	if !vp.RestrictNetwork() {
		t.Error("solitary must restrict VM network")
	}
	if !vp.IsolatedVNet() {
		t.Error("solitary must use isolated vnet")
	}
	if !vp.PublishProxyPort() {
		t.Error("solitary must publish proxy port for VM access")
	}
}

func TestSolitaryPolicy_DeniedCapsMatchLegacy(t *testing.T) {
	dp := SolitaryPolicy().(DockerPolicy)
	denied := dp.DeniedCaps()
	legacy := map[string]bool{
		"NET_ADMIN": true, "SYS_ADMIN": true, "SYS_PTRACE": true,
		"SYS_MODULE": true, "NET_BROADCAST": true, "BPF": true,
		"PERFMON": true, "SYS_RAWIO": true,
	}
	for cap := range legacy {
		if !denied[cap] {
			t.Errorf("denied caps missing legacy cap %s", cap)
		}
	}
	for cap := range denied {
		if !legacy[cap] {
			t.Errorf("denied caps has extra cap %s not in legacy", cap)
		}
	}
}
