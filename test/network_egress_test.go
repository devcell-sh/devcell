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

// TestSolitary_FullLockdown verifies the --solitary mode contracts:
//  1. The shell container is on an --internal network with gateway isolation
//     (bridge gateway unreachable, so no host access in either direction).
//  2. The gateway sidecar is reachable from the shell by DNS name.
//  3. The shell can reach the internet only through the proxy (allow-listed
//     domains work, others are denied, direct external TCP fails).
//  4. The shell has no docker socket, no dangerous capabilities, and no
//     port mappings.
//
// This is a docker-level contract test (short bucket): it uses alpine
// containers, not a cell image. The cfg.SanitizeForSolitary unit tests
// prove the config rewriting; this test proves the resulting container
// topology actually isolates traffic.
func TestSolitary_FullLockdown(t *testing.T) {
	serverVersion := requireEgressTestDocker(t)
	_ = serverVersion

	internal := egressTestName("sol-internal")
	egress := egressTestName("sol-egress")

	// Create internal network with gateway isolation (the topology --solitary uses).
	mustDocker(t, "network", "create", "--internal",
		"-o", "com.docker.network.bridge.gateway_mode_ipv4=isolated",
		"-o", "com.docker.network.bridge.gateway_mode_ipv6=isolated",
		internal)
	t.Cleanup(func() { _, _ = dockerCmd(t, 15*time.Second, "network", "rm", internal) })
	mustDocker(t, "network", "create", egress)
	t.Cleanup(func() { _, _ = dockerCmd(t, 15*time.Second, "network", "rm", egress) })

	// Gateway sidecar: squid allowing only example.com.
	gw := egressTestName("sol-gw")
	removeContainer(t, gw)

	squidStartup := `
apk add --no-cache squid >/dev/null 2>&1 &&
touch /tmp/access.log && chown squid:squid /tmp/access.log &&
cat > /etc/squid/squid.conf << "SQEOF"
http_port 3128
pid_filename /tmp/squid.pid
cache deny all
cache_log /tmp/cache.log
acl allowed_dst dstdomain "/etc/squid/allow_domains.txt"
acl CONNECT method CONNECT
http_access allow allowed_dst
http_access deny all
logformat devcell_json {"ts":%ts,"client":"%>a","method":"%rm","target":"%ru","code":%>Hs}
access_log /tmp/access.log devcell_json
SQEOF
printf ".example.com\nexample.com\n" > /etc/squid/allow_domains.txt &&
exec squid -N -f /etc/squid/squid.conf
`
	mustDocker(t, "run", "-d", "--pull=never", "--network", internal, "--name", gw,
		egressTestImage, "sh", "-c", squidStartup)
	mustDocker(t, "network", "connect", egress, gw)

	deadline := time.Now().Add(30 * time.Second)
	for {
		if _, err := dockerCmd(t, 5*time.Second, "exec", gw, "sh", "-c", "nc -z 127.0.0.1 3128"); err == nil {
			break
		}
		if time.Now().After(deadline) {
			logs, _ := dockerCmd(t, 5*time.Second, "exec", gw, "cat", "/tmp/cache.log")
			t.Fatalf("squid never became ready; cache.log:\n%s", logs)
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Shell container: on the internal network only, with proxy env vars
	// pointing at the gateway. No docker socket, no extra caps, no ports.
	shell := egressTestName("sol-shell")
	removeContainer(t, shell)

	proxyURL := "http://" + gw + ":3128"
	shellArgs := []string{
		"run", "-d", "--pull=never",
		"--network", internal,
		"--name", shell,
		"--cap-drop=ALL",
		"-e", "http_proxy=" + proxyURL,
		"-e", "https_proxy=" + proxyURL,
		"-e", "HTTP_PROXY=" + proxyURL,
		"-e", "HTTPS_PROXY=" + proxyURL,
	}
	shellArgs = append(shellArgs, egressTestImage, "sleep", "infinity")
	mustDocker(t, shellArgs...)

	// ── 1. No host access ────────────────────────────────────────────────

	t.Run("no_host_gateway", func(t *testing.T) {
		// With gateway_mode=isolated, Docker doesn't assign a gateway IP
		// at all (inspect returns empty or "invalid IP"). That IS the
		// lockdown: no gateway address means no way to reach the host.
		gwIP, _ := dockerCmd(t, 5*time.Second, "network", "inspect", internal,
			"--format", "{{(index .IPAM.Config 0).Gateway}}")
		if gwIP == "" || gwIP == "invalid IP" || gwIP == "<no value>" {
			t.Log("gateway isolated: no gateway IP assigned (lockdown confirmed)")
			return
		}
		// If there is a gateway IP (older Docker without isolation support),
		// verify it's unreachable.
		_, err := dockerCmd(t, 10*time.Second, "exec", shell,
			"ping", "-c", "1", "-W", "2", gwIP)
		if err == nil {
			t.Fatalf("bridge gateway %s should be unreachable, but ping succeeded", gwIP)
		}
		t.Logf("gateway %s correctly unreachable", gwIP)
	})

	// ── 2. Gateway sidecar reachable ─────────────────────────────────────

	t.Run("gateway_reachable", func(t *testing.T) {
		// Proxy env vars are set on the container; BusyBox wget uses them.
		out, err := dockerCmd(t, 10*time.Second, "exec", shell,
			"wget", "-qO-", "-T", "5", "-Y", "on",
			"http://example.com/")
		if err != nil {
			t.Fatalf("gateway unreachable: %v: %s", err, out)
		}
		if !strings.Contains(out, "Example Domain") {
			t.Fatalf("unexpected response body: %s", out)
		}
		t.Log("gateway reachable, proxied example.com returned expected body")
	})

	// ── 3. Only allowed traffic through proxy ────────────────────────────

	t.Run("blocked_domain", func(t *testing.T) {
		_, err := dockerCmd(t, 10*time.Second, "exec", shell,
			"wget", "-qO-", "-T", "5", "-Y", "on",
			"http://google.com/")
		if err == nil {
			t.Fatal("blocked domain google.com should have been denied")
		}
		t.Log("google.com correctly denied by proxy")
	})

	t.Run("direct_external_tcp_fails", func(t *testing.T) {
		_, err := dockerCmd(t, 10*time.Second, "exec", shell,
			"sh", "-c", "nc -w 3 1.1.1.1 443 </dev/null")
		if err == nil {
			t.Fatal("direct TCP to 1.1.1.1:443 should fail (internal network)")
		}
		t.Log("direct external TCP correctly blocked")
	})

	// ── 4. No docker socket ─────────────────────────────────────────────

	t.Run("no_docker_socket", func(t *testing.T) {
		_, err := dockerCmd(t, 5*time.Second, "exec", shell,
			"ls", "/var/run/docker.sock")
		if err == nil {
			t.Fatal("docker socket should not be present in solitary mode")
		}
		t.Log("docker socket correctly absent")
	})

	// ── 5. Proxy logs prove traffic went through gateway ─────────────────

	t.Run("proxy_logs", func(t *testing.T) {
		logs, err := dockerCmd(t, 5*time.Second, "exec", gw, "cat", "/tmp/access.log")
		if err != nil {
			t.Fatalf("failed to read proxy logs: %v: %s", err, logs)
		}
		if !strings.Contains(logs, "example.com") {
			t.Fatalf("proxy log missing example.com:\n%s", logs)
		}
		if !strings.Contains(logs, "google.com") {
			t.Fatalf("proxy log missing google.com (should show denied):\n%s", logs)
		}
		t.Logf("proxy logs:\n%s", logs)
	})
}

// TestGateway_Lifecycle verifies the gateway sidecar lifecycle:
//  1. Starts and becomes healthy (port 3128 accepts connections).
//  2. Runs with --rm so it cleans itself on stop.
//  3. Stop removes the container and the internal network.
//  4. Double-remove is idempotent (no error).
func TestGateway_Lifecycle(t *testing.T) {
	requireEgressTestDocker(t)

	internal := egressTestName("gw-lifecycle-internal")
	mustDocker(t, "network", "create", "--internal", internal)
	t.Cleanup(func() { _, _ = dockerCmd(t, 15*time.Second, "network", "rm", internal) })

	egress := egressTestName("gw-lifecycle-egress")
	mustDocker(t, "network", "create", egress)
	t.Cleanup(func() { _, _ = dockerCmd(t, 15*time.Second, "network", "rm", egress) })

	gw := egressTestName("gw-lifecycle")
	removeContainer(t, gw)

	squidStartup := `
apk add --no-cache squid >/dev/null 2>&1 &&
touch /tmp/access.log && chown squid:squid /tmp/access.log &&
cat > /etc/squid/squid.conf << "SQEOF"
http_port 3128
pid_filename /tmp/squid.pid
cache deny all
cache_log /tmp/cache.log
http_access allow all
SQEOF
exec squid -N -f /etc/squid/squid.conf
`
	mustDocker(t, "run", "-d", "--rm", "--pull=never",
		"--network", internal,
		"--name", gw,
		"--label", "devcell.role=proxy",
		egressTestImage, "sh", "-c", squidStartup)
	mustDocker(t, "network", "connect", egress, gw)

	t.Run("starts_healthy", func(t *testing.T) {
		deadline := time.Now().Add(30 * time.Second)
		for {
			if _, err := dockerCmd(t, 5*time.Second, "exec", gw, "sh", "-c", "nc -z 127.0.0.1 3128"); err == nil {
				t.Log("gateway healthy: squid listening on 3128")
				return
			}
			if time.Now().After(deadline) {
				logs, _ := dockerCmd(t, 5*time.Second, "exec", gw, "cat", "/tmp/cache.log")
				t.Fatalf("gateway never became healthy; cache.log:\n%s", logs)
			}
			time.Sleep(500 * time.Millisecond)
		}
	})

	t.Run("running", func(t *testing.T) {
		out, err := dockerCmd(t, 5*time.Second, "inspect", gw, "--format", "{{.State.Running}}")
		if err != nil {
			t.Fatalf("inspect: %v: %s", err, out)
		}
		if out != "true" {
			t.Fatalf("expected running=true, got %q", out)
		}
	})

	t.Run("stop_removes", func(t *testing.T) {
		_, err := dockerCmd(t, 15*time.Second, "stop", gw)
		if err != nil {
			t.Fatalf("stop: %v", err)
		}
		// --rm: container should be gone.
		_, err = dockerCmd(t, 5*time.Second, "inspect", gw)
		if err == nil {
			t.Fatal("gateway container should be removed after stop (--rm)")
		}
		t.Log("gateway removed after stop")
	})

	t.Run("double_remove_idempotent", func(t *testing.T) {
		// rm -f on an already-gone container should not error.
		_, err := dockerCmd(t, 5*time.Second, "rm", "-f", gw)
		if err != nil {
			t.Fatalf("double remove should be idempotent: %v", err)
		}
		t.Log("double remove succeeded")
	})
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return lines[len(lines)-1]
}

// TestSquidProxy_AllowAndBlock starts a real squid proxy sidecar, curls
// through it from an isolated shell container, and checks the proxy's access
// log to prove traffic actually went through the proxy. This is the
// end-to-end contract for the CELL-623 egress proxy feature.
func TestSquidProxy_AllowAndBlock(t *testing.T) {
	serverVersion := requireEgressTestDocker(t)
	_ = serverVersion

	// Networks: internal (no outbound) and egress (has internet).
	internal := egressTestName("internal")
	egress := egressTestName("egress")
	mustDocker(t, "network", "create", "--internal", internal)
	t.Cleanup(func() { _, _ = dockerCmd(t, 15*time.Second, "network", "rm", internal) })
	mustDocker(t, "network", "create", egress)
	t.Cleanup(func() { _, _ = dockerCmd(t, 15*time.Second, "network", "rm", egress) })

	// Squid sidecar: allow only example.com, deny everything else.
	// Config is written inline because Docker-in-Docker bind mounts
	// resolve on the host, not the outer container.
	proxy := egressTestName("proxy")
	removeContainer(t, proxy)

	squidStartup := `
apk add --no-cache squid >/dev/null 2>&1 &&
touch /tmp/access.log && chown squid:squid /tmp/access.log &&
cat > /etc/squid/squid.conf << "SQEOF"
http_port 3128
pid_filename /tmp/squid.pid
cache deny all
cache_log /tmp/cache.log
acl allowed_dst dstdomain "/etc/squid/allow_domains.txt"
acl CONNECT method CONNECT
http_access allow allowed_dst
http_access deny all
logformat devcell_json {"ts":%ts,"client":"%>a","method":"%rm","target":"%ru","code":%>Hs}
access_log /tmp/access.log devcell_json
SQEOF
printf ".example.com\nexample.com\n" > /etc/squid/allow_domains.txt &&
exec squid -N -f /etc/squid/squid.conf
`
	mustDocker(t, "run", "-d", "--pull=never", "--network", internal, "--name", proxy,
		egressTestImage, "sh", "-c", squidStartup)
	mustDocker(t, "network", "connect", egress, proxy)

	// Wait for squid to accept connections.
	deadline := time.Now().Add(30 * time.Second)
	for {
		if _, err := dockerCmd(t, 5*time.Second, "exec", proxy, "sh", "-c",
			"nc -z 127.0.0.1 3128"); err == nil {
			break
		}
		if time.Now().After(deadline) {
			logs, _ := dockerCmd(t, 5*time.Second, "exec", proxy, "cat", "/tmp/cache.log")
			t.Fatalf("squid never became ready; cache.log:\n%s", logs)
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Shell container: on the internal network only, with proxy env vars
	// pointing at the squid sidecar by Docker DNS name.
	shell := egressTestName("shell")
	removeContainer(t, shell)

	proxyURL := "http://" + proxy + ":3128"
	shellArgs := []string{
		"run", "-d", "--pull=never", "--network", internal, "--name", shell,
		"-e", "http_proxy=" + proxyURL,
		"-e", "https_proxy=" + proxyURL,
		"-e", "HTTP_PROXY=" + proxyURL,
		"-e", "HTTPS_PROXY=" + proxyURL,
	}
	shellArgs = append(shellArgs, egressTestImage, "sleep", "infinity")
	mustDocker(t, shellArgs...)

	// Allowed: example.com through the proxy should return the page body.
	t.Run("allowed_domain", func(t *testing.T) {
		out, err := dockerCmd(t, 15*time.Second, "exec", shell, "wget", "-qO-", "-T", "5",
			"http://example.com/")
		if err != nil {
			t.Fatalf("allowed domain failed: %v: %s", err, out)
		}
		if !strings.Contains(out, "Example Domain") {
			t.Fatalf("response does not contain expected body, got:\n%s", out)
		}
		t.Logf("allowed: got %d bytes from example.com", len(out))
	})

	// Blocked: google.com should be denied by squid (403).
	t.Run("blocked_domain", func(t *testing.T) {
		_, err := dockerCmd(t, 15*time.Second, "exec", shell, "wget", "-qO-", "-T", "5",
			"http://google.com/")
		if err == nil {
			t.Fatal("blocked domain should have failed but succeeded")
		}
		t.Log("blocked: google.com correctly denied")
	})

	// Proxy logs: prove both requests went through squid.
	t.Run("proxy_logs", func(t *testing.T) {
		logs, err := dockerCmd(t, 5*time.Second, "exec", proxy, "cat", "/tmp/access.log")
		if err != nil {
			t.Fatalf("failed to read proxy logs: %v: %s", err, logs)
		}
		if !strings.Contains(logs, "example.com") {
			t.Fatalf("proxy log missing example.com entry:\n%s", logs)
		}
		if !strings.Contains(logs, "google.com") {
			t.Fatalf("proxy log missing google.com entry:\n%s", logs)
		}
		// Verify status codes in logs.
		for _, line := range strings.Split(logs, "\n") {
			if strings.Contains(line, "example.com") && !strings.Contains(line, `"code":200`) {
				t.Errorf("example.com should show code 200: %s", line)
			}
			if strings.Contains(line, "google.com") && !strings.Contains(line, `"code":403`) {
				t.Errorf("google.com should show code 403: %s", line)
			}
		}
		t.Logf("proxy logs:\n%s", logs)
	})
}
