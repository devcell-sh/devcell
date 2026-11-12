package docker_test

import (
	"os"
	"strings"
	"testing"

	"github.com/devcell-sh/devcell/internal/cfg"
	"github.com/devcell-sh/devcell/internal/config"
	"github.com/devcell-sh/devcell/internal/engine/docker"
)

func peerArgv(cellCfg cfg.CellConfig) []string {
	e := makeEnv()
	c := config.Load("/tmp/peertest", e)
	spec := docker.RunSpec{
		Config:  c,
		CellCfg: cellCfg,
		Binary:  "claude",
	}
	return docker.BuildArgv(spec,
		docker.FSFunc(func(string) error { return os.ErrNotExist }),
		func(string) (string, error) { return "", os.ErrNotExist },
	)
}

func TestBuildArgv_PeerMemoryEnabled(t *testing.T) {
	on := true
	argv := peerArgv(cfg.CellConfig{
		Agents: cfg.AgentsSection{PeerMemory: &on},
	})
	if !hasArg(argv, "DEVCELL_PEER_MEMORY=true") {
		t.Error("expected DEVCELL_PEER_MEMORY=true in argv")
	}
	found := false
	for _, a := range argv {
		if strings.Contains(a, "DEVCELL_MCP_ENABLED") && strings.Contains(a, "peer-memory") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected peer-memory in DEVCELL_MCP_ENABLED")
	}
}

func TestBuildArgv_PeerMemoryDisabled(t *testing.T) {
	argv := peerArgv(cfg.CellConfig{})
	for _, a := range argv {
		if strings.Contains(a, "DEVCELL_PEER_MEMORY") {
			t.Errorf("DEVCELL_PEER_MEMORY should not be set when peer_memory is off, found %q", a)
		}
		if strings.Contains(a, "peer-memory") {
			t.Errorf("peer-memory should not appear in argv when peer_memory is off, found %q", a)
		}
	}
}

func TestBuildArgv_PeerMemoryMergesWithExistingMcp(t *testing.T) {
	on := true
	argv := peerArgv(cfg.CellConfig{
		Agents: cfg.AgentsSection{PeerMemory: &on},
		Mcp:    cfg.McpSection{Enabled: []string{"playwright"}},
	})
	found := false
	for _, a := range argv {
		if strings.Contains(a, "DEVCELL_MCP_ENABLED") {
			if strings.Contains(a, "playwright") && strings.Contains(a, "peer-memory") {
				found = true
			}
			break
		}
	}
	if !found {
		t.Error("expected both playwright and peer-memory in DEVCELL_MCP_ENABLED")
	}
}
