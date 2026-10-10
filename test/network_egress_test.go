// network_egress_test.go: CELL-623 groundwork. Two cheap docker-level
// contracts the proxy-sidecar design rests on, checked against alpine
// rather than a cell image so they run in seconds and belong to the short
// bucket (docs/testing.md).
//
//  1. Proxy env set at launch (`docker run -e`) reaches every process in
//     the container: pid 1, a daemon the entrypoint backgrounds (the
//     s6-svscan / nix-daemon analogue), `docker exec` sessions, their
//     nested children, and non-root exec sessions.
//  2. A container whose only network is a `docker network create
//     --internal` network can reach dual-homed sidecars by name over HTTP
//     and ICMP, and nothing else: no external DNS, no external IP, no TCP
//     to the internet, and not even the sidecar's own egress-side address.
//     No proxy is involved here; this pins the deny-by-default floor the
//     sidecar is later added on top of.
//
// Run standalone to skip the suite's TestMain probes:
//
//	go test -v -count=1 ./test/network_egress_test.go
//
// Needs the docker CLI, a reachable daemon and a local alpine:latest
// (`docker pull alpine:latest`); never pulls or builds.

package container_test

import (
	"context"
	"fmt"
	"os"
	osexec "os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// egressTestImage is the shell container; alpine has no httpd applet, so
// sidecars use egressSidecarImage.
const (
	egressTestImage    = "alpine:latest"
	egressSidecarImage = "busybox:1.36"
)

// proxyEnv is the variable set the docker engine will pass with -e in proxy
// mode. The values are fake: nothing here connects to a proxy.
var proxyEnv = map[string]string{
	"HTTP_PROXY":  "http://devcell-proxy-test:3128",
	"HTTPS_PROXY": "http://devcell-proxy-test:3128",
	"ALL_PROXY":   "http://devcell-proxy-test:3128",
	"NO_PROXY":    "localhost,127.0.0.1,::1",
	"http_proxy":  "http://devcell-proxy-test:3128",
	"https_proxy": "http://devcell-proxy-test:3128",
	"all_proxy":   "http://devcell-proxy-test:3128",
	"no_proxy":    "localhost,127.0.0.1,::1",
}

// requireEgressTestDocker skips when the docker CLI, daemon or alpine image
// is missing. It returns the daemon's server version for feature gating.
func requireEgressTestDocker(t *testing.T) string {
	t.Helper()
	if _, err := osexec.LookPath("docker"); err != nil {
		t.Skip("docker CLI missing; install Docker to run the egress tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := osexec.CommandContext(ctx, "docker", "version", "--format", "{{.Server.Version}}").CombinedOutput()
	if err != nil {
		t.Skipf("Docker daemon unavailable; start Docker to run the egress tests: %v: %s", err, out)
	}
	for _, img := range []string{egressTestImage, egressSidecarImage} {
		if err := osexec.CommandContext(ctx, "docker", "image", "inspect", img).Run(); err != nil {
			t.Skipf("%s missing; run 'docker pull %s' to enable the egress tests", img, img)
		}
	}
	return strings.TrimSpace(string(out))
}

// dockerCmd runs a docker CLI command and returns combined output.
func dockerCmd(t *testing.T, timeout time.Duration, args ...string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	out, err := osexec.CommandContext(ctx, "docker", args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// mustDocker fails the test when a setup command fails.
func mustDocker(t *testing.T, args ...string) string {
	t.Helper()
	out, err := dockerCmd(t, 30*time.Second, args...)
	if err != nil {
		t.Fatalf("docker %s: %v: %s", strings.Join(args, " "), err, out)
	}
	return out
}

// egressTestName builds a unique resource name so parallel runs never clash.
func egressTestName(kind string) string {
	return fmt.Sprintf("devcell-egress-test-%s-%d-%d", kind, os.Getpid(), time.Now().UnixNano())
}

// removeContainer registers best-effort `docker rm -f` cleanup.
func removeContainer(t *testing.T, name string) {
	t.Cleanup(func() { _, _ = dockerCmd(t, 15*time.Second, "rm", "-f", name) })
}

// parseEnviron turns NUL- or newline-separated KEY=VALUE output into a map.
func parseEnviron(s string) map[string]string {
	env := map[string]string{}
	for _, line := range strings.FieldsFunc(s, func(r rune) bool { return r == 0 || r == '\n' }) {
		if k, v, ok := strings.Cut(line, "="); ok {
			env[k] = v
		}
	}
	return env
}

// TestNetworkProxyEnv_ReachesEveryProcess: the proxy env passed at launch
// must be visible from every vantage point a tool can run at inside a cell.
// Contract source: internal/engine/docker/runner.go (-e argv) and
// internal/s6/s6.go (entrypoint backgrounds s6-svscan with no env scrub).
func TestNetworkProxyEnv_ReachesEveryProcess(t *testing.T) {
	requireEgressTestDocker(t)

	name := egressTestName("env")
	removeContainer(t, name)
	args := []string{"run", "-d", "--rm", "--pull=never", "--network=none", "--name", name}
	for k, v := range proxyEnv {
		args = append(args, "-e", k+"="+v)
	}
	// pid 1 is sh; it backgrounds a long-lived child before parking, the
	// way the cell entrypoint backgrounds s6-svscan (and s6 spawns
	// nix-daemon) before exec'ing into the session.
	args = append(args, egressTestImage, "sh", "-c", "sleep 2147483647 & exec sleep infinity")
	mustDocker(t, args...)

	daemonEnviron := `pid=$(pgrep -f 'sleep 2147483647' | head -1) && tr '\0' '\n' < /proc/$pid/environ`

	cases := []struct {
		name string
		exec []string // docker exec argv after the container name
	}{
		{"pid1", []string{"sh", "-c", `tr '\0' '\n' < /proc/1/environ`}},
		{"entrypoint_backgrounded_daemon", []string{"sh", "-c", daemonEnviron}},
		{"docker_exec_root", []string{"env"}},
		{"docker_exec_nested_child", []string{"sh", "-c", "sh -c env"}},
		{"docker_exec_non_root", []string{"-u", "nobody", "env"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			argv := append([]string{"exec"}, nil...)
			// -u must precede the container name.
			if len(tc.exec) > 0 && tc.exec[0] == "-u" {
				argv = append(argv, tc.exec[0], tc.exec[1], name)
				argv = append(argv, tc.exec[2:]...)
			} else {
				argv = append(argv, name)
				argv = append(argv, tc.exec...)
			}
			out, err := dockerCmd(t, 15*time.Second, argv...)
			if err != nil {
				t.Fatalf("docker %s: %v: %s", strings.Join(argv, " "), err, out)
			}
			got := parseEnviron(out)
			for k, want := range proxyEnv {
				if got[k] != want {
					t.Errorf("%s: %s=%q, want %q", tc.name, k, got[k], want)
				}
			}
		})
	}
}

// internalNetworkDNSFixed reports whether the daemon refuses to forward DNS
// from --internal networks (CVE-2024-29018, fixed in 26.0.0, 25.0.4 and
// 23.0.11). Older daemons leak external DNS, which is the tunnelling bypass
// the design relies on the engine to close.
func internalNetworkDNSFixed(version string) bool {
	parts := strings.SplitN(version, ".", 3)
	if len(parts) < 2 {
		return false
	}
	major, _ := strconv.Atoi(parts[0])
	minor, _ := strconv.Atoi(parts[1])
	patch := 0
	if len(parts) == 3 {
		p := strings.TrimFunc(parts[2], func(r rune) bool { return r < '0' || r > '9' })
		patch, _ = strconv.Atoi(p)
	}
	switch {
	case major >= 26:
		return true
	case major == 25:
		return minor > 0 || patch >= 4
	case major == 23:
		return minor > 0 || patch >= 11
	}
	return false
}

// TestNetworkLockdown_InternalNetworkOnlyReachesSidecars pins the floor the
// proxy sidecar is built on: a container on an --internal network alone can
// talk to dual-homed sidecars and to nothing else. Contract source:
// .scratch/cell-623/DESIGN.md section 3 (topology).
func TestNetworkLockdown_InternalNetworkOnlyReachesSidecars(t *testing.T) {
	serverVersion := requireEgressTestDocker(t)

	internal := egressTestName("internal")
	egress := egressTestName("egress")
	mustDocker(t, "network", "create", "--internal", internal)
	t.Cleanup(func() { _, _ = dockerCmd(t, 15*time.Second, "network", "rm", internal) })
	mustDocker(t, "network", "create", egress)
	t.Cleanup(func() { _, _ = dockerCmd(t, 15*time.Second, "network", "rm", egress) })

	// Two sidecars, each serving its own marker over HTTP, dual-homed onto
	// the internal network (first leg) and the egress network (second leg).
	type sidecar struct {
		name, marker, egressIP string
		port                   int
	}
	sidecars := []*sidecar{
		{name: egressTestName("sidecar-a"), marker: "sidecar-a", port: 8080},
		{name: egressTestName("sidecar-b"), marker: "sidecar-b", port: 8081},
	}
	for _, s := range sidecars {
		removeContainer(t, s.name)
		mustDocker(t, "run", "-d", "--rm", "--pull=never", "--network", internal, "--name", s.name,
			egressSidecarImage, "sh", "-c",
			fmt.Sprintf("mkdir -p /www && echo %s > /www/index.html && exec busybox httpd -f -p %d -h /www", s.marker, s.port))
		mustDocker(t, "network", "connect", egress, s.name)
		s.egressIP = mustDocker(t, "inspect", "-f",
			fmt.Sprintf(`{{(index .NetworkSettings.Networks %q).IPAddress}}`, egress), s.name)
		if s.egressIP == "" {
			t.Fatalf("%s has no address on %s", s.name, egress)
		}
		// Readiness: httpd answers on its own loopback.
		deadline := time.Now().Add(10 * time.Second)
		for {
			if _, err := dockerCmd(t, 5*time.Second, "exec", s.name, "wget", "-qO-", "-T", "2",
				fmt.Sprintf("http://127.0.0.1:%d/", s.port)); err == nil {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s httpd never became ready", s.name)
			}
			time.Sleep(200 * time.Millisecond)
		}
	}

	// The negative cases below only mean something if the host itself has
	// egress; a sidecar must reach the internet through its second leg.
	if out, err := dockerCmd(t, 15*time.Second, "exec", sidecars[0].name, "sh", "-c", "nc -w 5 1.1.1.1 443 </dev/null"); err != nil {
		t.Skipf("sidecar has no internet egress (nc 1.1.1.1:443: %v: %s); the deny cases would pass vacuously", err, out)
	}

	shell := egressTestName("shell")
	removeContainer(t, shell)
	mustDocker(t, "run", "-d", "--rm", "--pull=never", "--network", internal, "--name", shell,
		egressTestImage, "sleep", "infinity")

	cases := []struct {
		name     string
		cmd      []string
		wantOK   bool
		wantBody string // substring the output must contain when wantOK
		skip     string // non-empty: skip with this reason
	}{
		{name: "http_sidecar_a_by_name", cmd: []string{"wget", "-qO-", "-T", "3", "http://" + sidecars[0].name + ":8080/"}, wantOK: true, wantBody: "sidecar-a"},
		{name: "http_sidecar_b_by_name", cmd: []string{"wget", "-qO-", "-T", "3", "http://" + sidecars[1].name + ":8081/"}, wantOK: true, wantBody: "sidecar-b"},
		{name: "ping_sidecar_a", cmd: []string{"ping", "-c", "1", "-W", "2", sidecars[0].name}, wantOK: true},
		{name: "http_external_domain", cmd: []string{"wget", "-qO-", "-T", "3", "https://example.com/"}, wantOK: false},
		{name: "ping_external_ip", cmd: []string{"ping", "-c", "1", "-W", "2", "1.1.1.1"}, wantOK: false},
		{name: "tcp_external_ip_443", cmd: []string{"sh", "-c", "nc -w 3 1.1.1.1 443 </dev/null"}, wantOK: false},
		{name: "http_sidecar_a_via_egress_leg_ip", cmd: []string{"wget", "-qO-", "-T", "3", fmt.Sprintf("http://%s:8080/", sidecars[0].egressIP)}, wantOK: false},
		{name: "dns_external_name", cmd: []string{"nslookup", "example.com"}, wantOK: false,
			skip: func() string {
				if internalNetworkDNSFixed(serverVersion) {
					return ""
				}
				return "docker " + serverVersion + " forwards DNS from --internal networks (CVE-2024-29018); need >= 26.0"
			}()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.skip != "" {
				t.Skip(tc.skip)
			}
			argv := append([]string{"exec", shell}, tc.cmd...)
			out, err := dockerCmd(t, 20*time.Second, argv...)
			ok := err == nil
			if ok != tc.wantOK {
				t.Fatalf("%s: reachable=%v, want %v\n$ %s\n%s", tc.name, ok, tc.wantOK, strings.Join(tc.cmd, " "), out)
			}
			if tc.wantOK && tc.wantBody != "" && !strings.Contains(out, tc.wantBody) {
				t.Fatalf("%s: body %q does not contain %q", tc.name, out, tc.wantBody)
			}
			t.Logf("%s: reachable=%v (%s)", tc.name, ok, lastLine(out))
		})
	}
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return lines[len(lines)-1]
}
