//go:build rdp_probe

// resolution_probe_test.go — fast L0-style probe that runs against a live
// xrdp endpoint on 127.0.0.1:3389 (the local cell's own RDP server). Unlike
// test/gui_test.go::TestRdp_ClientResolutionRequest this does NOT spin up
// a testcontainer or rebuild the image — it just shells out to xfreerdp +
// xdotool against the already-running xrdp.
//
// Run:
//
//	go test -tags rdp_probe -v -count=1 -run TestResolution ./internal/rdp/
//
// Skips cleanly when xfreerdp / xdotool / Xvfb / 127.0.0.1:3389 are
// unavailable, so safe to leave in tree.
package rdp_test

import (
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	probeXrdpAddr      = "127.0.0.1:3389"
	probeRdpUser       = "dmitry"
	probeRdpPassword   = "rdp"
	probeClientDisplay = ":98"
)

// TestResolution_ClientRequestedSizeIsIgnored asserts the current broken
// behavior: xfreerdp /size:WxH requests a specific session size, but the
// Xvfb-backed xrdp+libvnc pipeline always returns 1920x1080. Once the
// Xvnc replacement lands, flip the assertion to actual==requested.
func TestResolution_ClientRequestedSizeIsIgnored(t *testing.T) {
	requireBinary(t, "xfreerdp")
	requireBinary(t, "xdotool")
	requireBinary(t, "Xvfb")
	requireTCP(t, probeXrdpAddr)

	// Roomy second Xvfb as the xfreerdp client display.
	xvfb := exec.Command("Xvfb", probeClientDisplay,
		"-screen", "0", "3200x1800x24", "+extension", "RANDR")
	if err := xvfb.Start(); err != nil {
		t.Fatalf("Xvfb %s: %v", probeClientDisplay, err)
	}
	t.Cleanup(func() {
		_ = xvfb.Process.Kill()
		_, _ = xvfb.Process.Wait()
	})
	time.Sleep(1 * time.Second)

	cases := []struct {
		name       string
		reqW, reqH int
		note       string
	}{
		{"tiny_800x600_4_3", 800, 600, "small VGA-era 4:3"},
		{"laptop_1366x768_16_9", 1366, 768, "common cheap-laptop 16:9"},
		{"hd_1600x900_16_9", 1600, 900, "between client sizes"},
		{"matches_1920x1080_16_9", 1920, 1080, "exactly server framebuffer"},
		{"qhd_2560x1440_16_9", 2560, 1440, "iPad-Pro / external monitor"},
		{"uhd_3840x2160_16_9", 3840, 2160, "4K — large mismatch"},
	}
	const wantW, wantH = 1920, 1080 // Xvfb framebuffer, see 50-gui.sh:11

	type result struct {
		req, got string
		match    bool
	}
	var collected []result

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Kick any stale xfreerdp from the prior subtest so x11vnc
			// (-nevershared) reliably accepts the next attempt.
			_ = exec.Command("pkill", "-9", "-f", "xfreerdp.*"+probeXrdpAddr).Run()
			time.Sleep(2 * time.Second)

			sizeArg := fmt.Sprintf("/size:%dx%d", tc.reqW, tc.reqH)
			xfree := exec.Command("xfreerdp",
				"/v:"+probeXrdpAddr,
				"/u:"+probeRdpUser,
				"/p:"+probeRdpPassword,
				"/cert:ignore",
				sizeArg,
			)
			xfree.Env = append(xfree.Environ(), "DISPLAY="+probeClientDisplay)
			stderrPipe, _ := xfree.StderrPipe()
			if err := xfree.Start(); err != nil {
				t.Fatalf("xfreerdp start: %v", err)
			}
			t.Cleanup(func() {
				_ = xfree.Process.Kill()
				_, _ = xfree.Process.Wait()
			})
			// Drain stderr in background so the child doesn't block.
			var stderrBuf strings.Builder
			go func() {
				buf := make([]byte, 4096)
				for {
					n, err := stderrPipe.Read(buf)
					if n > 0 {
						stderrBuf.Write(buf[:n])
					}
					if err != nil {
						return
					}
				}
			}()

			// FreeRDP 3.x needs ~9s to fully negotiate + map its window.
			time.Sleep(9 * time.Second)

			w, h := freerdpWindowGeometry(t, probeClientDisplay)
			t.Logf("requested %4dx%-4d (%s) → FreeRDP window %dx%d",
				tc.reqW, tc.reqH, tc.note, w, h)

			// Pull "DesktopSize" / "ProtocolType" hints from FreeRDP stderr
			// for additional diagnostic context.
			for _, line := range strings.Split(stderrBuf.String(), "\n") {
				if strings.Contains(line, "DesktopWidth") ||
					strings.Contains(line, "DesktopHeight") ||
					strings.Contains(line, "Capabilities") {
					t.Logf("  freerdp: %s", strings.TrimSpace(line))
				}
			}

			collected = append(collected, result{
				req:   fmt.Sprintf("%dx%d", tc.reqW, tc.reqH),
				got:   fmt.Sprintf("%dx%d", w, h),
				match: w == tc.reqW && h == tc.reqH,
			})

			if w != wantW || h != wantH {
				t.Errorf("expected fixed %dx%d (Xvfb framebuffer), got %dx%d — "+
					"if the Xvnc replacement has landed, flip this to actual==requested.",
					wantW, wantH, w, h)
			}
		})
	}

	// Summary table for easy data extraction
	t.Run("data_summary", func(t *testing.T) {
		t.Logf("==== Resolution probe summary (server framebuffer = %dx%d) ====", wantW, wantH)
		t.Logf("  %-12s | %-12s | %s", "REQUESTED", "ACTUAL", "MATCHES?")
		t.Logf("  %s", strings.Repeat("-", 42))
		for _, r := range collected {
			mark := "NO  (server forced)"
			if r.match {
				mark = "yes (= server)"
			}
			t.Logf("  %-12s | %-12s | %s", r.req, r.got, mark)
		}
	})
}

func requireBinary(t *testing.T, name string) {
	t.Helper()
	if _, err := exec.LookPath(name); err != nil {
		t.Skipf("skipping: %s not on PATH", name)
	}
}

func requireTCP(t *testing.T, addr string) {
	t.Helper()
	c, err := net.DialTimeout("tcp", addr, 1*time.Second)
	if err != nil {
		t.Skipf("skipping: nothing listening on %s (%v)", addr, err)
	}
	_ = c.Close()
}

func freerdpWindowGeometry(t *testing.T, display string) (int, int) {
	t.Helper()
	cmd := exec.Command("xdotool", "search", "--name", "FreeRDP")
	cmd.Env = append(cmd.Environ(), "DISPLAY="+display)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("xdotool search: %v (out=%q)", err, string(out))
	}
	wid := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
	if wid == "" {
		t.Fatalf("no FreeRDP window on %s", display)
	}
	cmd = exec.Command("xdotool", "getwindowgeometry", "--shell", wid)
	cmd.Env = append(cmd.Environ(), "DISPLAY="+display)
	out, err = cmd.Output()
	if err != nil {
		t.Fatalf("xdotool getwindowgeometry: %v (out=%q)", err, string(out))
	}
	return parseShellVar(string(out), "WIDTH"),
		parseShellVar(string(out), "HEIGHT")
}

func parseShellVar(out, key string) int {
	for _, line := range strings.Split(out, "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), key+"="); ok {
			n, _ := strconv.Atoi(strings.TrimSpace(v))
			return n
		}
	}
	return 0
}
