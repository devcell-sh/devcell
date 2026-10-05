package tart

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTartList_RunningInstances(t *testing.T) {
	raw := `[
		{"Name":"devcell-tart-base","State":"stopped","Source":"local","OS":"darwin","CPU":4,"Memory":8589934592,"Disk":53687091200},
		{"Name":"main-tart","State":"running","Source":"local","OS":"darwin","CPU":4,"Memory":8589934592,"Disk":53687091200},
		{"Name":"dev-tart","State":"running","Source":"local","OS":"darwin","CPU":2,"Memory":4294967296,"Disk":26843545600},
		{"Name":"unrelated-vm","State":"running","Source":"local","OS":"darwin","CPU":2,"Memory":4294967296,"Disk":26843545600}
	]`

	got, err := parseTartList([]byte(raw))
	require.NoError(t, err)

	// Only "main-tart" and "dev-tart" match the devcell instance pattern
	assert.Len(t, got, 2)
	assert.Contains(t, got, tartListEntry{Name: "main-tart", State: "running"})
	assert.Contains(t, got, tartListEntry{Name: "dev-tart", State: "running"})
}

func TestParseTartList_NoRunning(t *testing.T) {
	raw := `[
		{"Name":"devcell-tart-base","State":"stopped","Source":"local","OS":"darwin","CPU":4,"Memory":8589934592,"Disk":53687091200}
	]`

	got, err := parseTartList([]byte(raw))
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestParseTartList_Empty(t *testing.T) {
	got, err := parseTartList([]byte(`[]`))
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestCellNameFromInstance(t *testing.T) {
	for _, tc := range []struct {
		vmName   string
		wantCell string
		wantOK   bool
	}{
		{"main-tart", "main", true},
		{"dev-tart", "dev", true},
		{"my-project-tart", "my-project", true},
		{"devcell-tart-base", "", false},       // template, not instance
		{"devcell-tart-llm", "", false},         // template
		{"unrelated-vm", "", false},             // no -tart suffix
		{"tart", "", false},                     // just the suffix
	} {
		t.Run(tc.vmName, func(t *testing.T) {
			cell, ok := cellNameFromInstance(tc.vmName)
			assert.Equal(t, tc.wantOK, ok, "ok")
			if ok {
				assert.Equal(t, tc.wantCell, cell)
			}
		})
	}
}

func TestDiscoverRunningVMs_Integration(t *testing.T) {
	// Test with mocked CLI functions
	origList := tartListFunc
	origIP := tartIPFunc
	defer func() {
		tartListFunc = origList
		tartIPFunc = origIP
	}()

	tartListFunc = func(_ context.Context) ([]byte, error) {
		return []byte(`[
			{"Name":"main-tart","State":"running","Source":"local","OS":"darwin","CPU":4,"Memory":8589934592,"Disk":53687091200},
			{"Name":"dev-tart","State":"stopped","Source":"local","OS":"darwin","CPU":2,"Memory":4294967296,"Disk":26843545600},
			{"Name":"test-tart","State":"running","Source":"local","OS":"darwin","CPU":2,"Memory":4294967296,"Disk":26843545600}
		]`), nil
	}
	tartIPFunc = func(_ context.Context, name string) (string, error) {
		ips := map[string]string{
			"main-tart": "192.168.64.5",
			"test-tart": "192.168.64.6",
		}
		return ips[name], nil
	}

	vms := DiscoverRunningVMs(context.Background())
	require.Len(t, vms, 2)

	assert.Equal(t, "main", vms[0].CellName)
	assert.Equal(t, "192.168.64.5", vms[0].IP)
	assert.Equal(t, "5900", vms[0].VNCPort)

	assert.Equal(t, "test", vms[1].CellName)
	assert.Equal(t, "192.168.64.6", vms[1].IP)
	assert.Equal(t, "5900", vms[1].VNCPort)
}
