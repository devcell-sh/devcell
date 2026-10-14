package docker_test

import (
	"strings"
	"testing"
	"time"

	"github.com/devcell-sh/devcell/internal/engine/docker"
)

func TestBootTimeoutError_ContainsContainerAndDuration(t *testing.T) {
	err := docker.BootTimeoutErrorExport("cell-myproject-run", 90*time.Second)
	msg := err.Error()

	if !strings.Contains(msg, "cell-myproject-run") {
		t.Errorf("error should contain container name; got: %s", msg)
	}
	if !strings.Contains(msg, "1m30s") {
		t.Errorf("error should contain duration; got: %s", msg)
	}
	if !strings.Contains(msg, "docker logs") {
		t.Errorf("error should contain troubleshooting hint; got: %s", msg)
	}
}
