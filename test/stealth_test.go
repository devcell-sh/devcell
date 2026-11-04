package container_test

// stealth_test.go: runtime stealth verification of patchright-mcp-cell.
//
// One container, one comprehensive probe page, one MCP launch. The probe
// collects ALL stealth signals in a single Chromium session: sentinel,
// navigator, client hints, worker consistency, brands, codecs, permissions,
// timezone, and WebGL. Results are asserted per-signal and published to
// test/results/ as JSON.
//
// Runs headed (Xvfb :99) with the bundled patchright chromium — the same
// path production uses via 'cell claude'.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/devcell-sh/devcell/internal/testutil"
	"github.com/testcontainers/testcontainers-go"
)

const stealthDir = "/tmp/stealth"

// stealthProbeServer serves the comprehensive probe, a worker script, and
// a results endpoint. One Python process, one page, all signals.
const stealthProbeServer = `#!/usr/bin/env python3
import json, sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

DIR = '/tmp/stealth'

WORKER_JS = (
    "self.onmessage = async () => {\n"
    "  const r = { platform: navigator.platform, ua: navigator.userAgent };\n"
    "  r.intlTz = Intl.DateTimeFormat().resolvedOptions().timeZone;\n"
    "  if (typeof navigator.userAgentData !== 'undefined') {\n"
    "    try {\n"
    "      const h = await navigator.userAgentData.getHighEntropyValues(['architecture','platform','bitness']);\n"
    "      r.heaArch = h.architecture;\n"
    "      r.heaPlatform = h.platform;\n"
    "      r.heaBitness = h.bitness;\n"
    "    } catch (e) { r.heaError = String(e); }\n"
    "  } else { r.heaError = 'navigator.userAgentData undefined in worker'; }\n"
    "  self.postMessage(r);\n"
    "};\n"
)

PAGE_HTML = (
    '<!DOCTYPE html><html><head><title>stealth probe</title></head><body>'
    '<video id="v"></video>'
    '<canvas id="c" width="64" height="64"></canvas>'
    '<pre id="result">pending</pre>'
    '<script>'
    '(async function(){'
    '  const out = {};'

    # sentinel
    '  out.cellStealth = typeof window.__cellStealth;'

    # navigator
    '  out.platform = navigator.platform;'
    '  out.webdriverType = typeof navigator.webdriver;'
    '  out.webdriver = navigator.webdriver;'
    '  out.hasChrome = typeof window.chrome === "object" && window.chrome !== null;'
    '  out.hasChromeRuntime = !!(window.chrome && window.chrome.runtime);'
    '  out.ua = navigator.userAgent;'

    # client hints + brands
    '  if (typeof navigator.userAgentData !== "undefined") {'
    '    out.brands = navigator.userAgentData.brands;'
    '    try {'
    '      const h = await navigator.userAgentData.getHighEntropyValues('
    '        ["architecture","platform","platformVersion","bitness","fullVersionList"]);'
    '      out.heaArch = h.architecture;'
    '      out.heaPlatform = h.platform;'
    '      out.heaBitness = h.bitness;'
    '      out.fullVersionList = h.fullVersionList;'
    '    } catch (e) { out.heaError = String(e); }'
    '  } else { out.heaError = "navigator.userAgentData undefined"; }'

    # worker
    '  try {'
    '    const w = new Worker("/worker.js");'
    '    out.worker = await new Promise(function(resolve){'
    '      const t = setTimeout(function(){ resolve({ error: "worker timeout" }); }, 5000);'
    '      w.onmessage = function(e){ clearTimeout(t); resolve(e.data); };'
    '      w.onerror = function(e){ clearTimeout(t); resolve({ error: String(e.message || e) }); };'
    '      w.postMessage("go");'
    '    });'
    '  } catch (e) { out.worker = { error: String(e) }; }'

    # codecs
    '  var v = document.getElementById("v");'
    '  out.h264Baseline = v.canPlayType("video/mp4; codecs=\\"avc1.42E01E\\"");'
    '  out.h264Main     = v.canPlayType("video/mp4; codecs=\\"avc1.4d4015\\"");'
    '  out.aacLC         = v.canPlayType("audio/mp4; codecs=\\"mp4a.40.2\\"");'
    '  out.vp9           = v.canPlayType("video/webm; codecs=\\"vp9\\"");'

    # permissions
    '  out.perms = {};'
    '  if (navigator.permissions && navigator.permissions.query) {'
    '    var names = ["geolocation","notifications","camera","microphone",'
    '      "clipboard-read","clipboard-write","background-sync","payment-handler",'
    '      "persistent-storage","push","midi","accelerometer","gyroscope",'
    '      "magnetometer","ambient-light-sensor"];'
    '    for (var i = 0; i < names.length; i++) {'
    '      var n = names[i];'
    '      try {'
    '        var p = await navigator.permissions.query({ name: n });'
    '        out.perms[n] = { state: p.state, ok: true };'
    '      } catch (e) {'
    '        out.perms[n] = { state: null, ok: false, error: String(e).slice(0, 200) };'
    '      }'
    '    }'
    '  } else { out.permsError = "navigator.permissions missing"; }'

    # timezone
    '  out.intlTz = Intl.DateTimeFormat().resolvedOptions().timeZone;'
    '  out.tzOffset = new Date().getTimezoneOffset();'

    # webgl
    '  var gl = document.getElementById("c").getContext("webgl")'
    '        || document.getElementById("c").getContext("experimental-webgl");'
    '  if (gl) {'
    '    var dbg = gl.getExtension("WEBGL_debug_renderer_info");'
    '    out.webglVendor = dbg ? gl.getParameter(dbg.UNMASKED_VENDOR_WEBGL) : "no_ext";'
    '    out.webglRenderer = dbg ? gl.getParameter(dbg.UNMASKED_RENDERER_WEBGL) : "no_ext";'
    '  } else { out.webglError = "no WebGL"; }'

    # submit
    '  document.getElementById("result").textContent = JSON.stringify(out, null, 2);'
    '  try {'
    '    await fetch("/results", { method:"POST", headers:{"Content-Type":"application/json"}, body: JSON.stringify(out) });'
    '    document.title = "probe DONE";'
    '  } catch (e) { document.title = "probe FAIL: " + String(e); }'
    '})();'
    '</script></body></html>'
)

ACCEPT_CH = 'Sec-CH-UA-Arch, Sec-CH-UA-Bitness, Sec-CH-UA-Platform-Version, Sec-CH-UA-Platform, Sec-CH-UA-Model, Sec-CH-UA-Full-Version-List'
CRITICAL_CH = 'Sec-CH-UA-Arch, Sec-CH-UA-Bitness, Sec-CH-UA-Platform-Version, Sec-CH-UA-Full-Version-List'

def log_request(handler):
    entry = {
        'url': handler.path,
        'method': handler.command,
        'headers': {k.lower(): v for k, v in handler.headers.items()},
    }
    try:
        with open(DIR + '/headers.json', 'a') as f:
            f.write(json.dumps(entry) + '\n')
    except Exception:
        pass

class Handler(BaseHTTPRequestHandler):
    def log_message(self, format, *args): pass

    def do_GET(self):
        log_request(self)
        if self.path == '/probe':
            self.send_response(200)
            self.send_header('Content-Type', 'text/html')
            self.send_header('Accept-CH', ACCEPT_CH)
            self.send_header('Critical-CH', CRITICAL_CH)
            self.end_headers()
            self.wfile.write(PAGE_HTML.encode('utf-8'))
        elif self.path == '/worker.js':
            self.send_response(200)
            self.send_header('Content-Type', 'application/javascript')
            self.end_headers()
            self.wfile.write(WORKER_JS.encode('utf-8'))
        else:
            self.send_error(404)

    def do_POST(self):
        log_request(self)
        if self.path == '/results':
            length = int(self.headers.get('Content-Length', '0'))
            body = self.rfile.read(length) if length else b''
            try:
                with open(DIR + '/results.json', 'wb') as f:
                    f.write(body)
            except Exception as e:
                print('write results failed:', e, file=sys.stderr)
            self.send_response(204)
            self.end_headers()
        else:
            self.send_error(404)

server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
port = server.server_address[1]
with open(DIR + '/port', 'w') as f:
    f.write(str(port))
print('ready ' + str(port), flush=True)
server.serve_forever()
`

// stealthProbeClient drives patchright-mcp-cell over MCP stdio, navigates
// to /probe, and waits for the page to POST results.
const stealthProbeClient = `#!/usr/bin/env python3
import subprocess, json, os, sys, time

DIR = '/tmp/stealth'
mcp_args = sys.argv[1:]

with open(DIR + '/port') as f:
    port = f.read().strip()
url = 'http://127.0.0.1:' + port + '/probe'
print('probe url:', url, flush=True)

env = dict(os.environ)
env['PLAYWRIGHT_MCP_USER_DATA_DIR'] = '/tmp/pw-stealth-probe'

proc = subprocess.Popen(
    ['patchright-mcp-cell'] + mcp_args,
    stdin=subprocess.PIPE, stdout=subprocess.PIPE,
    stderr=open(DIR + '/mcp-stderr.log', 'w'),
    env=env)

def send(msg):
    proc.stdin.write((json.dumps(msg) + '\n').encode())
    proc.stdin.flush()

def recv():
    line = proc.stdout.readline()
    if not line:
        raise RuntimeError('EOF from patchright-mcp-cell stdout')
    return json.loads(line)

try:
    send({'jsonrpc':'2.0','id':1,'method':'initialize','params':{
        'protocolVersion':'2024-11-05','capabilities':{},
        'clientInfo':{'name':'stealth-probe','version':'0'}}})
    r = recv()
    print('init:', r.get('result',{}).get('serverInfo',{}).get('name','?'), flush=True)

    send({'jsonrpc':'2.0','id':2,'method':'tools/call','params':{
        'name':'browser_navigate','arguments':{'url':url}}})
    r = recv()
    if 'error' in r:
        print('navigate ERROR:', r.get('error'), file=sys.stderr)
        sys.exit(2)
    print('navigate: ok', flush=True)

    results = DIR + '/results.json'
    deadline = time.time() + 45
    while time.time() < deadline:
        if os.path.exists(results) and os.path.getsize(results) > 10:
            print('DONE', flush=True)
            sys.exit(0)
        time.sleep(0.5)

    print('ERROR: probe page did not POST results within 45s', file=sys.stderr)
    sys.exit(3)
finally:
    proc.terminate()
    try: proc.wait(timeout=5)
    except Exception: proc.kill()
`

const stealthDBusAddrFile = stealthDir + "/dbus-addr"
const stealthPulseAudioConf = "/opt/devcell/.local/state/nix/profiles/profile/etc/pulse/default.pa"

func startStealthCell(t *testing.T) testcontainers.Container {
	t.Helper()
	c := startContainer(t, map[string]string{
		"HOST_USER":        hostUser,
		"APP_NAME":         "stealth",
		"USER_WORKING_DIR": "/tmp/stealth-wd",
	})
	if _, code := containerExec(t, c, []string{"sh", "-c", "command -v patchright-mcp-cell"}); code != 0 {
		t.Skip("patchright-mcp-cell not on PATH; set DEVCELL_TEST_IMAGE to a scraping stack")
	}
	if _, code := containerExec(t, c, []string{"sh", "-c", "command -v python3"}); code != 0 {
		t.Skip("python3 not on PATH")
	}
	containerExec(t, c, []string{"mkdir", "-p", stealthDir})
	ctx := context.Background()
	if err := c.CopyToContainer(ctx, []byte(stealthProbeServer), stealthDir+"/server.py", 0o755); err != nil {
		t.Fatalf("copy probe server: %v", err)
	}
	if err := c.CopyToContainer(ctx, []byte(stealthProbeClient), stealthDir+"/client.py", 0o755); err != nil {
		t.Fatalf("copy probe client: %v", err)
	}
	containerExec(t, c, []string{"bash", "-c",
		"nohup python3 " + stealthDir + "/server.py > " + stealthDir + "/server.log 2>&1 &"})
	if _, code := containerExec(t, c, []string{"bash", "-c",
		"for i in 1 2 3 4 5 6 7 8 9 10; do [ -f " + stealthDir + "/port ] && exit 0; sleep 0.5; done; exit 1",
	}); code != 0 {
		log, _ := containerExec(t, c, []string{"cat", stealthDir + "/server.log"})
		t.Fatalf("probe server did not start:\n%s", log)
	}
	port, _ := containerExec(t, c, []string{"cat", stealthDir + "/port"})
	t.Logf("probe server listening on port %s", port)
	return c
}

func ensureStealthXvfb(t *testing.T, c testcontainers.Container) {
	t.Helper()
	containerExec(t, c, []string{"bash", "-c",
		"DISPLAY=:99 xset q >/dev/null 2>&1 && exit 0; " +
			"rm -f /tmp/.X99-lock /tmp/.X11-unix/X99; " +
			"setsid Xvfb :99 -screen 0 1920x1080x24 +extension GLX +render < /dev/null > /tmp/Xvfb.log 2>&1 &"})
	if _, code := containerExec(t, c, []string{"bash", "-c",
		"for i in $(seq 1 40); do DISPLAY=:99 xset q >/dev/null 2>&1 && exit 0; sleep 0.25; done; cat /tmp/Xvfb.log; exit 1",
	}); code != 0 {
		t.Fatal("Xvfb did not come up on :99")
	}
}

func ensureStealthDBus(t *testing.T, c testcontainers.Container) {
	t.Helper()
	if _, code := containerExec(t, c, []string{"bash", "-c",
		"[ -f " + stealthDBusAddrFile + " ] && cat " + stealthDBusAddrFile + " | xargs -I{} dbus-send --bus={} --dest=org.freedesktop.DBus --print-reply / org.freedesktop.DBus.ListNames >/dev/null 2>&1",
	}); code == 0 {
		return
	}
	containerExec(t, c, []string{"bash", "-c", "dbus-uuidgen > /etc/machine-id 2>/dev/null || true"})
	dbusConf := "/opt/devcell/.local/state/nix/profiles/profile/share/dbus-1/session.conf"
	out, code := containerExec(t, c, []string{"bash", "-c",
		"dbus-daemon --config-file=" + dbusConf + " --fork --print-address"})
	if code != 0 {
		t.Fatalf("dbus-daemon failed: %s", out)
	}
	addr := strings.TrimSpace(out)
	containerExec(t, c, []string{"bash", "-c", "echo '" + addr + "' > " + stealthDBusAddrFile})
	t.Logf("D-Bus session: %s", addr)
}

func ensureStealthPulseAudio(t *testing.T, c testcontainers.Container) {
	t.Helper()
	if _, code := containerExec(t, c, []string{"bash", "-c", "pgrep pulseaudio >/dev/null 2>&1"}); code == 0 {
		return
	}
	containerExec(t, c, []string{"bash", "-c", "mkdir -p /tmp/runtime-pulse"})
	out, code := containerExec(t, c, []string{"bash", "-c",
		"XDG_RUNTIME_DIR=/tmp/runtime-pulse pulseaudio --exit-idle-time=-1 --daemonize=yes --log-level=error " +
			"-F " + stealthPulseAudioConf + " --disallow-module-loading=0 2>&1; echo exit=$?"})
	if code != 0 {
		t.Logf("PulseAudio start warning (non-fatal): %s", out)
	}
}

func stealthServiceEnv(t *testing.T, c testcontainers.Container) []string {
	t.Helper()
	var env []string
	if addr, code := containerExec(t, c, []string{"cat", stealthDBusAddrFile}); code == 0 {
		env = append(env, "DBUS_SESSION_BUS_ADDRESS="+strings.TrimSpace(addr))
	}
	env = append(env, "XDG_RUNTIME_DIR=/tmp/runtime-pulse")
	return env
}

func runStealthProbe(t *testing.T, c testcontainers.Container, env []string) (results json.RawMessage, headers string) {
	t.Helper()
	containerExec(t, c, []string{"sh", "-c",
		"rm -f " + stealthDir + "/headers.json " + stealthDir + "/results.json " + stealthDir + "/mcp-stderr.log"})

	ensureStealthXvfb(t, c)
	ensureStealthDBus(t, c)
	ensureStealthPulseAudio(t, c)

	argv := append([]string{"env"}, env...)
	argv = append(argv, "DISPLAY=:99")
	argv = append(argv, stealthServiceEnv(t, c)...)
	argv = append(argv, "python3", stealthDir+"/client.py", "--browser", "chromium")

	mcpOut, mcpCode := containerExec(t, c, argv)
	t.Logf("MCP client output:\n%s", mcpOut)
	if mcpCode != 0 {
		stderr, _ := containerExec(t, c, []string{"sh", "-c", "tail -100 " + stealthDir + "/mcp-stderr.log 2>/dev/null"})
		t.Fatalf("MCP client exited %d\nstderr:\n%s", mcpCode, stderr)
	}

	resultsRaw, code := containerExec(t, c, []string{"cat", stealthDir + "/results.json"})
	if code != 0 {
		t.Fatalf("results.json missing: probe page did not POST results")
	}
	headers, _ = containerExec(t, c, []string{"cat", stealthDir + "/headers.json"})
	return json.RawMessage(resultsRaw), headers
}

func parseHeaders(raw string) []map[string]string {
	var reqs []map[string]string
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		if line == "" {
			continue
		}
		var entry struct {
			Headers map[string]string `json:"headers"`
		}
		if json.Unmarshal([]byte(line), &entry) == nil {
			reqs = append(reqs, entry.Headers)
		}
	}
	return reqs
}

// --- signal types for the JSON report ---

type stealthSignal struct {
	Signal   string `json:"signal"`
	Pass     bool   `json:"pass"`
	Severity string `json:"severity"`
	Ticket   string `json:"ticket"`
	Details  string `json:"details"`
}

type stealthReport struct {
	Timestamp string          `json:"timestamp"`
	Arch      string          `json:"arch"`
	Platform  string          `json:"platform"`
	Signals   []stealthSignal `json:"signals"`
	Summary   struct {
		Total  int `json:"total"`
		Passed int `json:"passed"`
		Failed int `json:"failed"`
	} `json:"summary"`
}

// --- the test ---

func TestStealth(t *testing.T) {
	c := startStealthCell(t)

	const (
		archEnv          = "arm"
		platformEnv      = "Linux"
		expectedPlatform = "Linux aarch64"
		tzEnv            = "Europe/Prague"
	)

	// Single probe run: headed, bundled chromium, all stealth env vars.
	rawResults, rawHeaders := runStealthProbe(t, c, []string{
		"DEVCELL_STEALTH_ARCH=" + archEnv,
		"DEVCELL_STEALTH_PLATFORM=" + platformEnv,
		"TZ=" + tzEnv,
	})

	var r struct {
		CellStealth    string      `json:"cellStealth"`
		Platform       string      `json:"platform"`
		WebdriverType  string      `json:"webdriverType"`
		Webdriver      interface{} `json:"webdriver"`
		HasChrome      bool        `json:"hasChrome"`
		HasChromeRuntime bool      `json:"hasChromeRuntime"`
		UA             string      `json:"ua"`
		HeaArch        string      `json:"heaArch"`
		HeaPlatform    string      `json:"heaPlatform"`
		HeaBitness     string      `json:"heaBitness"`
		HeaError       string      `json:"heaError"`
		Brands         []struct {
			Brand   string `json:"brand"`
			Version string `json:"version"`
		} `json:"brands"`
		FullVersionList []struct {
			Brand   string `json:"brand"`
			Version string `json:"version"`
		} `json:"fullVersionList"`
		Worker struct {
			Platform string `json:"platform"`
			UA       string `json:"ua"`
			IntlTz   string `json:"intlTz"`
			HeaArch  string `json:"heaArch"`
			HeaBitness string `json:"heaBitness"`
			HeaError string `json:"heaError"`
			Error    string `json:"error"`
		} `json:"worker"`
		H264Baseline string `json:"h264Baseline"`
		H264Main     string `json:"h264Main"`
		AacLC        string `json:"aacLC"`
		VP9          string `json:"vp9"`
		Perms        map[string]struct {
			State string `json:"state"`
			OK    bool   `json:"ok"`
			Error string `json:"error"`
		} `json:"perms"`
		PermsError  string `json:"permsError"`
		IntlTz      string `json:"intlTz"`
		TzOffset    int    `json:"tzOffset"`
		WebglVendor   string `json:"webglVendor"`
		WebglRenderer string `json:"webglRenderer"`
		WebglError    string `json:"webglError"`
	}
	if err := json.Unmarshal(rawResults, &r); err != nil {
		t.Fatalf("parse results: %v\nraw: %s", err, rawResults)
	}
	t.Logf("results: %s", rawResults)

	// Extract CDP headers from the probe request log.
	var archHeader, platformHeader, bitnessHeader, secChUA string
	bitnessSeen := false
	for _, h := range parseHeaders(rawHeaders) {
		if v := h["sec-ch-ua-arch"]; v != "" {
			archHeader = strings.Trim(v, `"`)
		}
		if v := h["sec-ch-ua-platform"]; v != "" {
			platformHeader = strings.Trim(v, `"`)
		}
		if v, ok := h["sec-ch-ua-bitness"]; ok {
			bitnessHeader = strings.Trim(v, `"`)
			bitnessSeen = true
		}
		if v := h["sec-ch-ua"]; v != "" {
			secChUA = v
		}
	}

	var signals []stealthSignal
	check := func(t *testing.T, signal, severity, ticket string, pass bool, detail string) {
		t.Helper()
		signals = append(signals, stealthSignal{
			Signal: signal, Pass: pass, Severity: severity,
			Ticket: ticket, Details: detail,
		})
		if !pass {
			t.Errorf("%s", detail)
		}
	}

	// Layer 0: sentinel — stealth-init.js reached the page context.
	t.Run("Sentinel", func(t *testing.T) {
		check(t, "stealth-init.js active", "CRITICAL", "CELL-161",
			r.CellStealth == "object",
			fmt.Sprintf("typeof __cellStealth=%q, want \"object\" — init script never reached page", r.CellStealth))
	})

	// Layer 1: CDP HTTP headers.
	t.Run("Headers", func(t *testing.T) {
		check(t, "Sec-CH-UA-Arch header", "HIGH", "CELL-68",
			archHeader == archEnv,
			fmt.Sprintf("Sec-CH-UA-Arch=%q, want %q", archHeader, archEnv))
		check(t, "Sec-CH-UA-Platform header", "HIGH", "CELL-68",
			platformHeader == "" || platformHeader == platformEnv,
			fmt.Sprintf("Sec-CH-UA-Platform=%q, want %q", platformHeader, platformEnv))
		check(t, "Sec-CH-UA-Bitness header", "MEDIUM", "CELL-150",
			bitnessSeen && bitnessHeader == "64",
			fmt.Sprintf("Sec-CH-UA-Bitness=%q seen=%v, want \"64\"", bitnessHeader, bitnessSeen))
		check(t, "Sec-CH-UA contains Google Chrome", "HIGH", "CELL-667",
			secChUA != "" && strings.Contains(secChUA, "Google Chrome"),
			fmt.Sprintf("Sec-CH-UA=%q", secChUA))
	})

	// Layer 2: main-thread JS navigator spoofs.
	t.Run("Navigator", func(t *testing.T) {
		check(t, "navigator.platform consistency", "HIGH", "CELL-68",
			r.Platform == expectedPlatform,
			fmt.Sprintf("navigator.platform=%q, want %q", r.Platform, expectedPlatform))
		check(t, "navigator.webdriver = false", "HIGH", "CELL-153",
			r.WebdriverType == "boolean" && r.Webdriver == false,
			fmt.Sprintf("webdriver=%v (type=%s), want boolean false", r.Webdriver, r.WebdriverType))
		check(t, "window.chrome present, no runtime", "HIGH", "CELL-153",
			r.HasChrome && !r.HasChromeRuntime,
			fmt.Sprintf("chrome=%v runtime=%v", r.HasChrome, r.HasChromeRuntime))
		check(t, "HEA architecture", "HIGH", "CELL-68",
			r.HeaArch == archEnv,
			fmt.Sprintf("hea.architecture=%q, want %q (error=%q)", r.HeaArch, archEnv, r.HeaError))
		check(t, "HEA bitness", "HIGH", "CELL-150",
			r.HeaBitness == "64",
			fmt.Sprintf("hea.bitness=%q, want \"64\"", r.HeaBitness))
	})

	// Layer 3: Worker cross-context consistency.
	t.Run("Worker", func(t *testing.T) {
		if r.Worker.Error != "" {
			t.Fatalf("worker probe failed: %q", r.Worker.Error)
		}
		check(t, "worker platform consistency", "MEDIUM", "CELL-34",
			r.Worker.Platform == expectedPlatform,
			fmt.Sprintf("worker.platform=%q, want %q", r.Worker.Platform, expectedPlatform))
		check(t, "worker HEA architecture", "MEDIUM", "CELL-25",
			r.Worker.HeaArch == archEnv,
			fmt.Sprintf("worker.heaArch=%q, want %q (error=%q)", r.Worker.HeaArch, archEnv, r.Worker.HeaError))
		check(t, "worker HEA bitness", "MEDIUM", "CELL-150",
			r.Worker.HeaBitness == "64",
			fmt.Sprintf("worker.heaBitness=%q, want \"64\"", r.Worker.HeaBitness))
		check(t, "worker timezone consistency", "MEDIUM", "CELL-21",
			r.Worker.IntlTz == tzEnv,
			fmt.Sprintf("worker.intlTz=%q, want %q", r.Worker.IntlTz, tzEnv))
	})

	// Brands: userAgentData must include "Google Chrome".
	t.Run("Brands", func(t *testing.T) {
		hasGC := false
		for _, b := range r.Brands {
			if b.Brand == "Google Chrome" {
				hasGC = true
			}
		}
		check(t, "Google Chrome brand in userAgentData", "HIGH", "CELL-667",
			hasGC,
			fmt.Sprintf("brands=%v", r.Brands))
		hasFVL := false
		for _, b := range r.FullVersionList {
			if b.Brand == "Google Chrome" {
				hasFVL = true
			}
		}
		check(t, "Google Chrome in fullVersionList", "MEDIUM", "CELL-667",
			hasFVL,
			fmt.Sprintf("fullVersionList count=%d", len(r.FullVersionList)))
	})

	// Codecs: proprietary codec support (LOW severity, currently requires nix chromium).
	t.Run("Codecs", func(t *testing.T) {
		check(t, "H264 codec support", "LOW", "CELL-20",
			r.H264Baseline == "probably",
			fmt.Sprintf("h264=%q h264_main=%q aac=%q", r.H264Baseline, r.H264Main, r.AacLC))
		check(t, "VP9 codec (sanity)", "LOW", "CELL-20",
			r.VP9 == "probably",
			fmt.Sprintf("vp9=%q (open codec, should always work)", r.VP9))
	})

	// Permissions: real Chrome returns PermissionStatus for most queries.
	t.Run("Permissions", func(t *testing.T) {
		if r.PermsError != "" {
			t.Fatalf("permissions API missing: %s", r.PermsError)
		}
		queryable := 0
		for _, p := range r.Perms {
			if p.OK && p.State != "" {
				queryable++
			}
		}
		check(t, "permissions API queryable (>=10)", "MEDIUM", "CELL-19",
			queryable >= 10,
			fmt.Sprintf("%d/%d queryable", queryable, len(r.Perms)))

		core := []string{"geolocation", "notifications", "camera", "microphone", "push"}
		for _, name := range core {
			p, ok := r.Perms[name]
			corePass := ok && p.OK && p.State != ""
			check(t, "permission: "+name, "HIGH", "CELL-19",
				corePass,
				fmt.Sprintf("state=%q ok=%v err=%q", p.State, p.OK, p.Error))
		}
	})

	// Timezone: Intl timezone must match TZ env.
	t.Run("Timezone", func(t *testing.T) {
		check(t, "timezone matches TZ env", "HIGH", "CELL-21",
			r.IntlTz == tzEnv,
			fmt.Sprintf("intlTz=%q, want %q, offset=%d", r.IntlTz, tzEnv, r.TzOffset))
		check(t, "timezone offset non-zero", "HIGH", "CELL-21",
			r.TzOffset != 0,
			fmt.Sprintf("offset=%d (0=UTC), TZ=%s should be non-zero", r.TzOffset, tzEnv))
	})

	// WebGL: vendor/renderer must match the platform (no macOS strings on Linux).
	t.Run("WebGL", func(t *testing.T) {
		if r.WebglError != "" {
			check(t, "WebGL available", "HIGH", "CELL-70",
				false, fmt.Sprintf("WebGL error: %s", r.WebglError))
			return
		}
		isLinux := strings.Contains(r.Platform, "Linux")
		macVendor := r.WebglVendor == "Intel Inc." || strings.HasPrefix(r.WebglVendor, "Apple")
		macRenderer := strings.Contains(r.WebglRenderer, "Intel Iris") ||
			strings.Contains(r.WebglRenderer, "OpenGL Engine") ||
			strings.HasPrefix(r.WebglRenderer, "Apple")
		check(t, "WebGL vendor matches platform", "HIGH", "CELL-70",
			!(isLinux && macVendor),
			fmt.Sprintf("vendor=%q renderer=%q platform=%q", r.WebglVendor, r.WebglRenderer, r.Platform))
		check(t, "WebGL renderer matches platform", "HIGH", "CELL-70",
			!(isLinux && macRenderer),
			fmt.Sprintf("renderer=%q on platform=%q", r.WebglRenderer, r.Platform))
	})

	// Publish results.
	report := stealthReport{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Arch:      archEnv,
		Platform:  platformEnv,
		Signals:   signals,
	}
	for _, s := range signals {
		report.Summary.Total++
		if s.Pass {
			report.Summary.Passed++
		} else {
			report.Summary.Failed++
		}
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Errorf("marshal report: %v", err)
		return
	}
	outDir := testutil.TestResultsDir(t, hostBaseDirFn)
	outPath := filepath.Join(outDir, "stealth-results.json")
	if mkErr := os.MkdirAll(filepath.Dir(outPath), 0o755); mkErr != nil {
		t.Errorf("mkdir: %v", mkErr)
	}
	if wErr := os.WriteFile(outPath, data, 0o644); wErr != nil {
		t.Errorf("write: %v", wErr)
	}
	t.Logf("Report: %s", outPath)
	t.Logf("Summary: %d total, %d passed, %d failed",
		report.Summary.Total, report.Summary.Passed, report.Summary.Failed)

	fmt.Fprintf(os.Stderr, "\n=== STEALTH RESULTS ===\n")
	for _, s := range signals {
		status := "PASS"
		if !s.Pass {
			status = "FAIL"
		}
		fmt.Fprintf(os.Stderr, "  [%s] %s: %s\n", status, s.Signal, s.Details)
	}
	fmt.Fprintf(os.Stderr, "Report: %s\n", outPath)
}
