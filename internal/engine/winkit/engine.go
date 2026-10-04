// Package winkit is the winkit engine: Windows guests built and booted by
// go-winkit (vz on macOS hosts, qemu on Linux hosts). It runs two guests,
// chosen by engine.Cell.Guest:
//
//   - engine.WindowsPE: a WinPE boot volume with a WSL1 nix distro (winkit
//     stage pe-wsl). The default.
//   - engine.WindowsFull: a full Windows install with a WSL1 nix distro
//     (winkit stage full-wsl).
//
// Both import the distro as "winkit" and run the agent in it with
// `wsl -d winkit`. They differ in the build stage, the template directory,
// and the SSH channel the agent command runs over (see guest).
package winkit

import (
	"fmt"

	"github.com/devcell-sh/go-winkit/gosshd"
	"github.com/devcell-sh/go-winkit/unattend"

	"github.com/DimmKirr/devcell/internal/engine"
)

func init() {
	engine.Register(engine.Winkit, Engine{})
}

// Engine implements engine.Engine for engine.Winkit.
type Engine struct{}

var _ engine.Engine = Engine{}

// guest is a Windows guest winkit builds and boots, resolved from
// engine.Cell.Guest by guestFor. The zero value is WindowsPE.
type guest struct {
	// full selects a full Windows install (winkit stage full-wsl) over a
	// WinPE boot volume (pe-wsl).
	full bool
}

var (
	guestPE   = guest{}
	guestFull = guest{full: true}
)

// guestFor resolves the guest a cell runs. An empty Guest is the engine's
// default; guests winkit cannot run are an error.
func guestFor(g engine.Guest) (guest, error) {
	if g == "" {
		g, _ = engine.DefaultGuest(engine.Winkit)
	}
	switch g {
	case engine.WindowsPE:
		return guestPE, nil
	case engine.WindowsFull:
		return guestFull, nil
	}
	return guest{}, fmt.Errorf("the winkit engine cannot run guest %q (supported: %s, %s)", g, engine.WindowsPE, engine.WindowsFull)
}

// label names the guest in dry-run and prompt output.
func (g guest) label() string {
	if g.full {
		return "Windows+WSL1"
	}
	return "PE+WSL1"
}

// sshChannel is the SSH endpoint on the host that the agent command runs
// over in a booted guest.
type sshChannel struct {
	port     uint16
	user     string
	password string
}

// channel returns the SSH endpoint the agent command runs over, given the
// host port winkit forwards to the guest's gosshd.
//
// PE runs it over gosshd itself: WinPE has no user accounts, and the WSL1
// distro is registered for the SYSTEM session gosshd runs in. A full install
// registers the distro for the Windows user at first logon, and WSL
// registrations are per user, so gosshd's SYSTEM session cannot see it.
// There the command runs over the Windows OpenSSH the image ships, as that
// user; winkit.Start forwards it on sshPort+100.
func (g guest) channel(sshPort uint16) sshChannel {
	if g.full {
		u := unattend.DefaultConfig()
		return sshChannel{port: sshPort + 100, user: u.Username, password: u.Password}
	}
	return sshChannel{port: sshPort, user: gosshd.DefaultUser, password: gosshd.DefaultPassword}
}
