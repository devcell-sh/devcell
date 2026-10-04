package winkit

import (
	"strconv"

	"github.com/DimmKirr/devcell/internal/config"
)

// AllocatedPorts holds the bunk-derived port strings for SSH, VNC, and RDP.
type AllocatedPorts struct {
	SSHPort string
	VNCPort string
	RDPPort string
}

// vncBasePort is QEMU's display :0. QEMU takes VNC as a display number
// (port - 5900), so lower ports cannot be expressed.
const vncBasePort = 5900

// AllocatePorts computes SSH, VNC, and RDP ports using the same bunk-based
// scheme as the Docker runner. portPrefix is SESSION_PORT_PREFIX + bunk.
// The taken map (may be nil) lists ports already in use by other processes.
func AllocatePorts(portPrefix string, taken map[int]struct{}) AllocatedPorts {
	if taken == nil {
		taken = map[int]struct{}{}
	}
	return AllocatedPorts{
		SSHPort: config.ResolveAvailablePort(config.ClampPort(portPrefix+"22"), taken),
		VNCPort: config.ResolveAvailablePort(hoistBelowVNCBase(config.ClampPort(portPrefix+"50")), taken),
		RDPPort: config.ResolveAvailablePort(config.ClampPort(portPrefix+"89"), taken),
	}
}

// hoistBelowVNCBase moves a port under vncBasePort up by 10000, the same
// shift ResolveAvailablePort applies to privileged ports, so the
// last-digits-by-pane pattern survives.
func hoistBelowVNCBase(port string) string {
	n, err := strconv.Atoi(port)
	if err != nil || n >= vncBasePort {
		return port
	}
	return strconv.Itoa(n + 10000)
}

// SSHPortUint16 returns the SSH port as uint16.
func (p AllocatedPorts) SSHPortUint16() uint16 {
	n, _ := strconv.Atoi(p.SSHPort)
	return uint16(n)
}

// VNCPortUint16 returns the VNC port as uint16.
func (p AllocatedPorts) VNCPortUint16() uint16 {
	n, _ := strconv.Atoi(p.VNCPort)
	return uint16(n)
}

// RDPPortUint16 returns the RDP port as uint16.
func (p AllocatedPorts) RDPPortUint16() uint16 {
	n, _ := strconv.Atoi(p.RDPPort)
	return uint16(n)
}
