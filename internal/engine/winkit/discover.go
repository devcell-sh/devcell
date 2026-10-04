package winkit

import (
	"os"
	"path/filepath"

	gowinkit "github.com/devcell-sh/go-winkit"
	"github.com/devcell-sh/go-winkit/vm/vmstate"
)

// VMPorts are the host ports a running VM forwards.
type VMPorts struct {
	SSHPort uint16
	VNCPort uint16
	RDPPort uint16
}

// DiscoveredVM is a running Windows VM found in a cell's instance dir.
type DiscoveredVM struct {
	CellName  string
	PID       int
	ImagePath string
	Ports     VMPorts
}

// DiscoverRunningVMs lists the live VMs winkit has registered under each
// cell's instance dir (~/.devcell/<cell>/windows/, see InstanceDir), which
// devcell passes to winkit.Start as the state dir.
func DiscoverRunningVMs(home string) []DiscoveredVM {
	entries, err := os.ReadDir(filepath.Join(home, ".devcell"))
	if err != nil {
		return nil
	}

	var vms []DiscoveredVM
	for _, entry := range entries {
		cellName := entry.Name()
		// cache/ holds media and windows/ holds templates; neither is a cell.
		if !entry.IsDir() || cellName == "cache" || cellName == "windows" {
			continue
		}
		states, err := gowinkit.Status(InstanceDir(home, cellName))
		if err != nil {
			continue
		}
		for _, st := range states {
			if !vmstate.IsAlive(st.PID) {
				continue
			}
			vms = append(vms, DiscoveredVM{
				CellName:  cellName,
				PID:       st.PID,
				ImagePath: st.ImagePath,
				Ports:     VMPorts{SSHPort: st.SSHPort, VNCPort: st.VNCPort, RDPPort: st.RDPPort},
			})
		}
	}
	return vms
}

// RunningVMForImage returns a live VM, in any cell, booted from image.
func RunningVMForImage(home, image string) (DiscoveredVM, bool) {
	abs, err := filepath.Abs(image)
	if err != nil {
		abs = image
	}
	for _, v := range DiscoverRunningVMs(home) {
		if v.ImagePath == abs {
			return v, true
		}
	}
	return DiscoveredVM{}, false
}
