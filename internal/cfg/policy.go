package cfg

// Policy is the engine-agnostic isolation contract. Every engine reads it to
// decide what to allow or deny. Engine-specific details are accessed via type
// assertion to DockerPolicy or VMPolicy.
type Policy interface {
	Label() string
	ForceProxy() bool
	DenyDirectInternet() bool
	DenyHostAccess() bool
	DenyPorts() bool
	DefaultPresets() []string
}

// DockerPolicy extends Policy with container-specific isolation controls.
type DockerPolicy interface {
	Policy
	DropAllCaps() bool
	SafeCaps() []string
	DeniedCaps() map[string]bool
	DenyPrivileged() bool
	DenyDockerSock() bool
	DenyUserVolumes() bool
	DeniedMountPaths() []string
	DenyOpRun() bool
	DenyInheritEnv() bool
	DenyWireguard() bool
	DenyKVM() bool
	DenyTUN() bool
}

// VMPolicy extends Policy with hypervisor-specific isolation controls.
type VMPolicy interface {
	Policy
	RestrictNetwork() bool
	IsolatedVNet() bool
	PublishProxyPort() bool
}

// --- open policy (everything allowed) ---

type openPolicy struct{}

func OpenPolicy() Policy { return openPolicy{} }

func (openPolicy) Label() string          { return "open" }
func (openPolicy) ForceProxy() bool       { return false }
func (openPolicy) DenyDirectInternet() bool { return false }
func (openPolicy) DenyHostAccess() bool   { return false }
func (openPolicy) DenyPorts() bool        { return false }
func (openPolicy) DefaultPresets() []string { return nil }

// DockerPolicy
func (openPolicy) DropAllCaps() bool            { return false }
func (openPolicy) SafeCaps() []string            { return nil }
func (openPolicy) DeniedCaps() map[string]bool   { return nil }
func (openPolicy) DenyPrivileged() bool          { return false }
func (openPolicy) DenyDockerSock() bool          { return false }
func (openPolicy) DenyUserVolumes() bool         { return false }
func (openPolicy) DeniedMountPaths() []string    { return nil }
func (openPolicy) DenyOpRun() bool               { return false }
func (openPolicy) DenyInheritEnv() bool          { return false }
func (openPolicy) DenyWireguard() bool           { return false }
func (openPolicy) DenyKVM() bool                 { return false }
func (openPolicy) DenyTUN() bool                 { return false }

// VMPolicy
func (openPolicy) RestrictNetwork() bool  { return false }
func (openPolicy) IsolatedVNet() bool     { return false }
func (openPolicy) PublishProxyPort() bool { return false }

// --- solitary policy (maximum lockdown) ---

// solitarySafeCaps are the capabilities added back after --cap-drop=ALL.
// These are the Docker default caps minus anything that defeats network isolation.
var solitarySafeCaps = []string{
	"CHOWN",
	"DAC_OVERRIDE",
	"FOWNER",
	"FSETID",
	"KILL",
	"SETGID",
	"SETUID",
	"SETPCAP",
	"NET_BIND_SERVICE",
	"SYS_CHROOT",
	"MKNOD",
	"AUDIT_WRITE",
	"SETFCAP",
}

var solitaryDefaultPresets = []string{
	"claude", "github", "npm", "pypi", "go", "cargo", "nix", "devcell",
}

// solitaryDeniedCaps are capabilities that defeat network isolation.
var solitaryDeniedCaps = map[string]bool{
	"NET_ADMIN":     true,
	"SYS_ADMIN":     true,
	"SYS_PTRACE":    true,
	"SYS_MODULE":    true,
	"NET_BROADCAST": true,
	"BPF":           true,
	"PERFMON":       true,
	"SYS_RAWIO":     true,
}

// solitaryDeniedMountPaths are host paths that defeat network isolation.
var solitaryDeniedMountPaths = []string{
	"/var/run",
	"/run",
	"/proc",
	"/sys",
}

type solitaryPolicy struct{}

func SolitaryPolicy() Policy { return solitaryPolicy{} }

func (solitaryPolicy) Label() string          { return "solitary" }
func (solitaryPolicy) ForceProxy() bool       { return true }
func (solitaryPolicy) DenyDirectInternet() bool { return true }
func (solitaryPolicy) DenyHostAccess() bool   { return true }
func (solitaryPolicy) DenyPorts() bool        { return true }
func (solitaryPolicy) DefaultPresets() []string { return solitaryDefaultPresets }

// DockerPolicy
func (solitaryPolicy) DropAllCaps() bool            { return true }
func (solitaryPolicy) SafeCaps() []string            { return solitarySafeCaps }
func (solitaryPolicy) DeniedCaps() map[string]bool   { return solitaryDeniedCaps }
func (solitaryPolicy) DenyPrivileged() bool          { return true }
func (solitaryPolicy) DenyDockerSock() bool          { return true }
func (solitaryPolicy) DenyUserVolumes() bool         { return false }
func (solitaryPolicy) DeniedMountPaths() []string    { return solitaryDeniedMountPaths }
func (solitaryPolicy) DenyOpRun() bool               { return false }
func (solitaryPolicy) DenyInheritEnv() bool          { return false }
func (solitaryPolicy) DenyWireguard() bool           { return true }
func (solitaryPolicy) DenyKVM() bool                 { return true }
func (solitaryPolicy) DenyTUN() bool                 { return true }

// VMPolicy
func (solitaryPolicy) RestrictNetwork() bool  { return true }
func (solitaryPolicy) IsolatedVNet() bool     { return true }
func (solitaryPolicy) PublishProxyPort() bool { return true }

// compile-time interface checks
var (
	_ Policy       = openPolicy{}
	_ DockerPolicy = openPolicy{}
	_ VMPolicy     = openPolicy{}
	_ Policy       = solitaryPolicy{}
	_ DockerPolicy = solitaryPolicy{}
	_ VMPolicy     = solitaryPolicy{}
)
