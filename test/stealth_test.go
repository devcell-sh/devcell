package container_test

// stealth_test.go: runtime stealth verification of patchright-mcp-cell
// (CELL-153 and follow-ups).
//
// One container, one probe server, one subtest per signal. Each subtest
// drives patchright-mcp-cell over MCP stdio (the exact path Claude Code uses
// in production), navigates Chromium to its page on the in-container probe
// server, and asserts on the JSON the page POSTs back:
//
//	AllLayersConsistent             sentinel, CDP Sec-CH-UA-* headers,
//	                                main-thread JS and Worker JS agree
//	                                (CELL-153, CELL-161, CELL-68, CELL-150)
//	GoogleChromeBrand               "Google Chrome" brand present (DIMM-XXX)
//	H264CodecSupport                proprietary codecs supported (CELL-20)
//	PermissionsAPISupported         permissions.query answers (CELL-19)
//	TimezoneMatchesContainer        Intl timezone follows TZ (CELL-21)
//	MainThreadWebGLMatchesPlatform  no macOS WebGL strings on Linux (CELL-70)
//
// Every subtest is L2 in the pyramid described in sudo_test.go: container
// exec against an existing image. TestMcp_PatchrightUndetected in
// mcp_test.go is only a smoke check; it does not catch the CELL-161 class
// of bugs, where the init script silently aborts at parse time and every
// spoof becomes dead code.
//
// Server and client are Python (ThreadingHTTPServer), not Node: the thin
// variant does not have node on PATH but python3 is always present.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/testcontainers/testcontainers-go"
)

// stealthDir holds the probe server, client, and per-probe output inside
// the container.
const stealthDir = "/tmp/stealth"

// stealthNixChromium is the nix-profile chromium some probes pin with
// --executable-path instead of patchright's bundled browser.
const stealthNixChromium = "/opt/devcell/.local/state/nix/profiles/profile/bin/chromium"

// stealthProbeServer serves every probe page from one Python HTTP server:
//
//	GET  /probe/<name>  PAGES[<name>]; the page collects its signals and
//	                    POSTs them as JSON to /results.
//	GET  /worker.js     worker source for the all-layers page.
//	POST /results       writes the body to results.json.
//
// Every incoming request's headers are appended to headers.json (one JSON
// object per line, keys lowercased) so subtests can verify CDP-emitted
// Sec-CH-UA-* headers. Pages that need high-entropy hints advertise
// Accept-CH + Critical-CH so Chrome retries the navigation with them
// attached. runStealthProbe clears both files before each probe.
//
// Page HTML uses single-quoted JS strings + concatenation so the Python
// source can hold it without escape acrobatics.
const stealthProbeServer = `#!/usr/bin/env python3
import json, sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

DIR = '/tmp/stealth'

# Worker source for the all-layers page (GET /worker.js).
WORKER_JS = (
    "self.onmessage = async () => {\n"
    "  const r = { platform: navigator.platform, ua: navigator.userAgent };\n"
    "  r.cellStealth = typeof self.__cellStealth;\n"
    "  if (typeof navigator.userAgentData !== 'undefined') {\n"
    "    try {\n"
    "      const h = await navigator.userAgentData.getHighEntropyValues(['architecture','platform','bitness']);\n"
    "      r.hea_arch = h.architecture;\n"
    "      r.hea_platform = h.platform;\n"
    "      r.hea_bitness = h.bitness;\n"
    "    } catch (e) { r.hea_error = String(e); }\n"
    "  } else {\n"
    "    r.hea_error = 'navigator.userAgentData undefined in worker';\n"
    "  }\n"
    "  self.postMessage(r);\n"
    "};\n"
)

PAGES = {}

# all-layers: main-thread + Worker signals (AllLayersConsistent).
PAGES['all-layers'] = {
    'accept_ch': 'Sec-CH-UA-Arch, Sec-CH-UA-Bitness, Sec-CH-UA-Platform-Version, Sec-CH-UA-Platform, Sec-CH-UA-Model, Sec-CH-UA-Full-Version-List',
    'critical_ch': 'Sec-CH-UA-Arch, Sec-CH-UA-Bitness, Sec-CH-UA-Platform-Version',
    'html': (
    '<!DOCTYPE html><html><head><title>stealth all-layers probe</title></head><body>'
    '<pre id="result">pending</pre>'
    '<script>'
    '(async function(){'
    '  const out = { main: {}, worker: {} };'
    '  out.main.cellStealth = typeof window.__cellStealth;'
    '  out.main.cellStealthValue = (typeof window.__cellStealth !== "undefined") ? JSON.stringify(window.__cellStealth) : null;'
    '  out.main.platform = navigator.platform;'
    '  out.main.webdriverType = typeof navigator.webdriver;'
    '  out.main.webdriver = navigator.webdriver;'
    '  out.main.hasChrome = typeof window.chrome === "object" && window.chrome !== null;'
    '  out.main.hasChromeRuntime = !!(window.chrome && window.chrome.runtime);'
    '  out.main.ua = navigator.userAgent;'
    '  if (typeof navigator.userAgentData !== "undefined") {'
    '    try {'
    '      const hea = await navigator.userAgentData.getHighEntropyValues(["architecture","platform","platformVersion","bitness"]);'
    '      out.main.hea_arch = hea.architecture;'
    '      out.main.hea_platform = hea.platform;'
    '      out.main.hea_bitness = hea.bitness;'
    '      out.main.hea_platformVersion = hea.platformVersion;'
    '    } catch (e) { out.main.hea_error = String(e); }'
    '  } else { out.main.hea_error = "navigator.userAgentData undefined"; }'
    '  try {'
    '    const w = new Worker("/worker.js");'
    '    out.worker = await new Promise(function(resolve){'
    '      const t = setTimeout(function(){ resolve({ error: "worker timeout" }); }, 5000);'
    '      w.onmessage = function(e){ clearTimeout(t); resolve(e.data); };'
    '      w.onerror = function(e){ clearTimeout(t); resolve({ error: "worker-error: " + (e.message || String(e)) }); };'
    '      w.postMessage("go");'
    '    });'
    '  } catch (e) { out.worker = { error: "worker-spawn: " + String(e) }; }'
    '  document.getElementById("result").textContent = JSON.stringify(out, null, 2);'
    '  try {'
    '    await fetch("/results", { method:"POST", headers:{"Content-Type":"application/json"}, body: JSON.stringify(out) });'
    '    document.title = "all-layers DONE";'
    '  } catch (e) { document.title = "all-layers POST-FAILED: " + String(e); }'
    '})();'
    '</script></body></html>'
    ),
}

# brand: userAgentData brands + fullVersionList (GoogleChromeBrand).
PAGES['brand'] = {
    'accept_ch': 'Sec-CH-UA-Full-Version-List, Sec-CH-UA-Full-Version',
    'critical_ch': 'Sec-CH-UA-Full-Version-List',
    'html': (
    '<!DOCTYPE html><html><head><title>brand-probe</title></head><body>'
    '<pre id="r">pending</pre><script>(async function(){'
    '  const out = {};'
    '  out.ua = navigator.userAgent;'
    '  if (navigator.userAgentData) {'
    '    out.brands = navigator.userAgentData.brands;'
    '    try {'
    '      const h = await navigator.userAgentData.getHighEntropyValues(["fullVersionList"]);'
    '      out.fullVersionList = h.fullVersionList;'
    '    } catch (e) { out.fvl_error = String(e); }'
    '  } else { out.error = "userAgentData undefined"; }'
    '  document.getElementById("r").textContent = JSON.stringify(out, null, 2);'
    '  try {'
    '    await fetch("/results", { method:"POST", headers:{"Content-Type":"application/json"}, body: JSON.stringify(out) });'
    '    document.title = "brand DONE";'
    '  } catch (e) { document.title = "brand FAIL: " + String(e); }'
    '})();</script></body></html>'
    ),
}

# codec: canPlayType / MediaSource / mediaCapabilities (H264CodecSupport).
PAGES['codec'] = {
    'html': (
    '<!DOCTYPE html><html><head><title>codec</title></head><body>'
    '<video id="v"></video><script>'
    '(async function(){'
    '  const v = document.getElementById("v");'
    '  const out = {};'
    '  out.h264_baseline = v.canPlayType("video/mp4; codecs=\\\"avc1.42E01E\\\"");'
    '  out.h264_main     = v.canPlayType("video/mp4; codecs=\\\"avc1.4d4015\\\"");'
    '  out.aac_lc        = v.canPlayType("audio/mp4; codecs=\\\"mp4a.40.2\\\"");'
    '  out.mp3           = v.canPlayType("audio/mpeg");'
    '  out.vp9_webm      = v.canPlayType("video/webm; codecs=\\\"vp9\\\"");'
    '  out.opus_webm     = v.canPlayType("audio/webm; codecs=\\\"opus\\\"");'
    '  if (window.MediaSource) {'
    '    out.ms_h264_baseline = MediaSource.isTypeSupported("video/mp4; codecs=\\\"avc1.42E01E\\\"");'
    '    out.ms_aac_lc       = MediaSource.isTypeSupported("audio/mp4; codecs=\\\"mp4a.40.2\\\"");'
    '  }'
    '  if (navigator.mediaCapabilities) {'
    '    try {'
    '      const r = await navigator.mediaCapabilities.decodingInfo({'
    '        type: "file",'
    '        video: { contentType: "video/mp4; codecs=\\\"avc1.42E01E\\\"", width:1280, height:720, bitrate:1500000, framerate:30 }'
    '      });'
    '      out.mc_h264 = { supported: r.supported, smooth: r.smooth, powerEfficient: r.powerEfficient };'
    '    } catch (e) { out.mc_h264_err = String(e); }'
    '  }'
    '  await fetch("/results", {method:"POST", headers:{"Content-Type":"application/json"}, body: JSON.stringify(out)});'
    '  document.title = "codec DONE";'
    '})();'
    '</script></body></html>'
    ),
}

# permissions: navigator.permissions.query per name (PermissionsAPISupported).
PAGES['permissions'] = {
    'html': (
    '<!DOCTYPE html><html><head><title>perms</title></head><body><script>'
    '(async function(){'
    '  const names = ["geolocation","notifications","camera","microphone",'
    '    "clipboard-read","clipboard-write","background-sync","payment-handler",'
    '    "persistent-storage","push","midi","accelerometer","gyroscope",'
    '    "magnetometer","ambient-light-sensor"];'
    '  const out = { perms: {} };'
    '  if (!navigator.permissions || !navigator.permissions.query) {'
    '    out.error = "navigator.permissions missing";'
    '  } else {'
    '    for (const n of names) {'
    '      try {'
    '        const p = await navigator.permissions.query({ name: n });'
    '        out.perms[n] = { state: p.state, ok: true };'
    '      } catch (e) {'
    '        out.perms[n] = { state: null, ok: false, error: String(e).slice(0, 200) };'
    '      }'
    '    }'
    '  }'
    '  await fetch("/results", {method:"POST", headers:{"Content-Type":"application/json"}, body: JSON.stringify(out)});'
    '  document.title = "perms DONE";'
    '})();'
    '</script></body></html>'
    ),
}

# timezone: Intl timezone + offset, main thread and Worker (TimezoneMatchesContainer).
PAGES['timezone'] = {
    'html': (
    '<!DOCTYPE html><html><head><title>tz</title></head><body><script>'
    '(async function(){'
    '  const out = {};'
    '  out.intlTz = Intl.DateTimeFormat().resolvedOptions().timeZone;'
    '  out.tzOffset = new Date().getTimezoneOffset();'
    '  out.dateStr = new Date().toString();'
    '  out.jsEpochMs = Date.now();'
    '  try {'
    '    const w = new Worker(URL.createObjectURL(new Blob(['
    '      "self.onmessage=()=>{self.postMessage({wTz:Intl.DateTimeFormat().resolvedOptions().timeZone,wOff:new Date().getTimezoneOffset()})}"'
    '    ], {type: "application/javascript"})));'
    '    out.worker = await new Promise(function(r){'
    '      const t = setTimeout(function(){ r({error:"timeout"}); }, 3000);'
    '      w.onmessage = function(e){ clearTimeout(t); r(e.data); };'
    '      w.postMessage("go");'
    '    });'
    '  } catch (e) { out.worker = { error: String(e) }; }'
    '  await fetch("/results", {method:"POST", headers:{"Content-Type":"application/json"}, body: JSON.stringify(out)});'
    '  document.title = "tz DONE";'
    '})();'
    '</script></body></html>'
    ),
}

# webgl: WebGL vendor/renderer strings (MainThreadWebGLMatchesPlatform).
PAGES['webgl'] = {
    'html': (
    '<!DOCTYPE html><html><head><title>webgl</title></head><body>'
    '<canvas id="c" width="64" height="64"></canvas><script>'
    '(async function(){'
    '  const out = {};'
    '  const gl = document.getElementById("c").getContext("webgl") || document.getElementById("c").getContext("experimental-webgl");'
    '  if (!gl) { out.error = "no WebGL"; }'
    '  else {'
    '    const dbg = gl.getExtension("WEBGL_debug_renderer_info");'
    '    out.unmaskedVendor = dbg ? gl.getParameter(dbg.UNMASKED_VENDOR_WEBGL) : "no_ext";'
    '    out.unmaskedRenderer = dbg ? gl.getParameter(dbg.UNMASKED_RENDERER_WEBGL) : "no_ext";'
    '    out.vendor = gl.getParameter(gl.VENDOR);'
    '    out.renderer = gl.getParameter(gl.RENDERER);'
    '    out.version = gl.getParameter(gl.VERSION);'
    '    out.shadingLang = gl.getParameter(gl.SHADING_LANGUAGE_VERSION);'
    '  }'
    '  out.platform = navigator.platform;'
    '  await fetch("/results", {method:"POST", headers:{"Content-Type":"application/json"}, body: JSON.stringify(out)});'
    '  document.title = "webgl DONE";'
    '})();'
    '</script></body></html>'
    ),
}

def log_request(handler):
    entry = {
        'url': handler.path,
        'method': handler.command,
        # Lowercase keys so Go assertions can use stable indexing.
        'headers': {k.lower(): v for k, v in handler.headers.items()},
    }
    try:
        with open(DIR + '/headers.json', 'a') as f:
            f.write(json.dumps(entry) + '\n')
    except Exception:
        pass

class Handler(BaseHTTPRequestHandler):
    # Silence stderr request log spam; we have our own file log.
    def log_message(self, format, *args): pass

    def do_GET(self):
        log_request(self)
        page = PAGES.get(self.path[len('/probe/'):]) if self.path.startswith('/probe/') else None
        if page:
            self.send_response(200)
            self.send_header('Content-Type', 'text/html')
            # Accept-CH advertises high-entropy hints; Critical-CH forces
            # Chrome to retry this navigation with them attached.
            if page.get('accept_ch'):
                self.send_header('Accept-CH', page['accept_ch'])
            if page.get('critical_ch'):
                self.send_header('Critical-CH', page['critical_ch'])
            self.end_headers()
            self.wfile.write(page['html'].encode('utf-8'))
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

// stealthProbeClient is a Python MCP stdio client that launches
// patchright-mcp-cell with the argv after the probe name (identical to a
// production launch), navigates Chromium to /probe/<name>, and waits for
// the page to POST its results back to the probe server.
//
// Values are never extracted through MCP output, only read back from
// results.json. This avoids the brittle MCP-output parsing path entirely.
const stealthProbeClient = `#!/usr/bin/env python3
import subprocess, json, os, sys, time

DIR = '/tmp/stealth'
name = sys.argv[1]
mcp_args = sys.argv[2:]

with open(DIR + '/port') as f:
    port = f.read().strip()
url = 'http://127.0.0.1:' + port + '/probe/' + name
print('probe url:', url, flush=True)

env = dict(os.environ)
env['PLAYWRIGHT_MCP_USER_DATA_DIR'] = '/tmp/pw-stealth-' + name

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
        'clientInfo':{'name':'stealth-' + name,'version':'0'}}})
    r = recv()
    print('init:', r.get('result',{}).get('serverInfo',{}).get('name','?'), flush=True)

    send({'jsonrpc':'2.0','id':2,'method':'tools/call','params':{
        'name':'browser_navigate','arguments':{'url':url}}})
    r = recv()
    if 'error' in r:
        print('navigate ERROR:', r.get('error'), file=sys.stderr)
        sys.exit(2)
    print('navigate: ok', flush=True)

    # Wait up to 45s for the probe page to POST /results.
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

// stealthProbe describes how one subtest launches patchright-mcp-cell.
type stealthProbe struct {
	name string // PAGES key in stealthProbeServer; the page is GET /probe/<name>

	// headed launches without --headless under Xvfb on :99, the way
	// production runs (CELL-17). Headless disables most permission backends
	// and changes the WebGL surface.
	headed bool

	// nixChromium pins --executable-path to stealthNixChromium instead of
	// patchright's bundled chromium.
	nixChromium bool

	// env holds extra KEY=VALUE pairs for patchright-mcp-cell, which reads
	// TZ and DEVCELL_STEALTH_* at launch.
	env []string
}

// startStealthCell boots the one container shared by every stealth subtest,
// installs the probe server and client, and starts the server.
func startStealthCell(t *testing.T) testcontainers.Container {
	t.Helper()
	c := startContainer(t, map[string]string{
		"HOST_USER":        hostUser,
		"APP_NAME":         "stealth",
		"USER_WORKING_DIR": "/tmp/stealth-wd",
	})

	// Skip if patchright-mcp-cell isn't installed in this stack variant.
	if _, code := exec(t, c, []string{"sh", "-c", "command -v patchright-mcp-cell"}); code != 0 {
		t.Skip("patchright-mcp-cell not on PATH (stack without scraping module); set DEVCELL_TEST_IMAGE to a stack that includes it")
	}
	// Skip if python3 isn't available (needed for both probe server and MCP client).
	if _, code := exec(t, c, []string{"sh", "-c", "command -v python3"}); code != 0 {
		t.Skip("python3 not on PATH")
	}

	exec(t, c, []string{"mkdir", "-p", stealthDir})
	ctx := context.Background()
	if err := c.CopyToContainer(ctx, []byte(stealthProbeServer), stealthDir+"/server.py", 0o755); err != nil {
		t.Fatalf("copy probe server: %v", err)
	}
	if err := c.CopyToContainer(ctx, []byte(stealthProbeClient), stealthDir+"/client.py", 0o755); err != nil {
		t.Fatalf("copy probe client: %v", err)
	}

	exec(t, c, []string{"bash", "-c",
		"nohup python3 " + stealthDir + "/server.py > " + stealthDir + "/server.log 2>&1 &"})
	if _, code := exec(t, c, []string{"bash", "-c",
		"for i in 1 2 3 4 5 6 7 8 9 10; do [ -f " + stealthDir + "/port ] && exit 0; sleep 0.5; done; exit 1",
	}); code != 0 {
		log, _ := exec(t, c, []string{"cat", stealthDir + "/server.log"})
		t.Fatalf("probe server did not start in 5s:\n%s", log)
	}
	port, _ := exec(t, c, []string{"cat", stealthDir + "/port"})
	t.Logf("probe server listening on port %s", port)
	return c
}

// ensureStealthXvfb starts Xvfb on :99 unless it is already up.
// testcontainers' `tail -f /dev/null` Cmd bypasses the entrypoint script,
// so 50-gui.sh never runs. We must replicate what production does:
// spawn Xvfb on :99 so chromium has a real display surface (a GL surface,
// and most permission backends activate).
func ensureStealthXvfb(t *testing.T, c testcontainers.Container) {
	t.Helper()
	exec(t, c, []string{"bash", "-c",
		"DISPLAY=:99 xset q >/dev/null 2>&1 && exit 0; " +
			"rm -f /tmp/.X99-lock /tmp/.X11-unix/X99; " +
			"setsid Xvfb :99 -screen 0 1920x1080x24 +extension GLX +render < /dev/null > /tmp/Xvfb.log 2>&1 &"})
	if _, code := exec(t, c, []string{"bash", "-c",
		"for i in $(seq 1 40); do DISPLAY=:99 xset q >/dev/null 2>&1 && exit 0; sleep 0.25; done; cat /tmp/Xvfb.log; exit 1",
	}); code != 0 {
		t.Fatal("Xvfb did not come up on :99")
	}
}

// runStealthProbe drives one probe page through patchright-mcp-cell, decodes
// the JSON the page POSTed into `into`, and returns the raw request-header
// log (see parseStealthHeaders).
func runStealthProbe(t *testing.T, c testcontainers.Container, p stealthProbe, into any) (headersRaw string) {
	t.Helper()

	// Reset the previous probe's artifacts so polling sees a clean slate and
	// the header log only holds this probe's requests.
	exec(t, c, []string{"sh", "-c",
		"rm -f " + stealthDir + "/headers.json " + stealthDir + "/results.json " + stealthDir + "/mcp-stderr.log"})

	argv := append([]string{"env"}, p.env...)
	if p.headed {
		ensureStealthXvfb(t, c)
		argv = append(argv, "DISPLAY=:99")
	}
	argv = append(argv, "python3", stealthDir+"/client.py", p.name)
	if !p.headed {
		argv = append(argv, "--headless")
	}
	argv = append(argv, "--browser", "chromium")
	if p.nixChromium {
		argv = append(argv, "--executable-path", stealthNixChromium)
	}

	mcpOut, mcpCode := exec(t, c, argv)
	t.Logf("MCP client output:\n%s", mcpOut)
	if mcpCode != 0 {
		stderr, _ := exec(t, c, []string{"sh", "-c", "tail -100 " + stealthDir + "/mcp-stderr.log 2>/dev/null"})
		t.Fatalf("MCP client exited %d\nstderr:\n%s", mcpCode, stderr)
	}

	resultsRaw, code := exec(t, c, []string{"cat", stealthDir + "/results.json"})
	if code != 0 {
		t.Fatalf("%s/results.json missing: %s probe page did not POST results", stealthDir, p.name)
	}
	if err := json.Unmarshal([]byte(resultsRaw), into); err != nil {
		t.Fatalf("parse %s results: %v\nraw: %s", p.name, err, resultsRaw)
	}
	headersRaw, _ = exec(t, c, []string{"cat", stealthDir + "/headers.json"})
	return headersRaw
}

// parseStealthHeaders decodes the probe server's header log: one JSON object
// per request, header names lowercased. Returns each request's headers.
func parseStealthHeaders(raw string) []map[string]string {
	var reqs []map[string]string
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		if line == "" {
			continue
		}
		var entry struct {
			URL     string            `json:"url"`
			Method  string            `json:"method"`
			Headers map[string]string `json:"headers"`
		}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		reqs = append(reqs, entry.Headers)
	}
	return reqs
}

func TestStealth(t *testing.T) {
	c := startStealthCell(t)

	// AllLayersConsistent: CELL-153 regression test for CELL-161 (silent
	// abort) and CELL-68 (cross-layer arch mismatch). Launches
	// patchright-mcp-cell with DEVCELL_STEALTH_ARCH=arm and
	// DEVCELL_STEALTH_PLATFORM=Linux and verifies that 4 layers report values
	// consistent with those env vars:
	//
	//   Layer 0: sentinel, window.__cellStealth defined (CELL-161 silent-abort)
	//   Layer 1: CDP HTTP headers, Sec-CH-UA-Arch / Sec-CH-UA-Platform (CELL-68)
	//   Layer 2: main-thread JS, navigator.platform, webdriver, chrome.runtime,
	//            getHighEntropyValues().architecture (CELL-169, CELL-68)
	//   Layer 3: Worker JS, navigator.platform + architecture (CELL-34, CELL-25)
	//
	// Broken state (init script aborted) fails on:
	//   - sentinel: window.__cellStealth === undefined  (script aborted)
	//   - main JS: navigator.platform === "Linux x86_64"  (unspoofed)
	//   - HTTP: no Sec-CH-UA-Arch advertised  (jq merge / wrapper broken)
	//
	// Working state (init-script injection fix + CELL-68 unified config):
	//   - sentinel: typeof window.__cellStealth === "object"
	//   - main JS: navigator.platform === "Linux aarch64"
	//   - HTTP: Sec-CH-UA-Arch == "arm"
	//   - Worker: same arch + platform as main thread
	t.Run("AllLayersConsistent", func(t *testing.T) {
		const (
			archEnv          = "arm"
			platformEnv      = "Linux"
			expectedPlatform = "Linux aarch64" // platformEnv + archEnv per stealth-init.js:395
		)

		var results struct {
			Main struct {
				CellStealth        string      `json:"cellStealth"`
				CellStealthValue   string      `json:"cellStealthValue"`
				Platform           string      `json:"platform"`
				WebdriverType      string      `json:"webdriverType"`
				Webdriver          interface{} `json:"webdriver"`
				HasChrome          bool        `json:"hasChrome"`
				HasChromeRuntime   bool        `json:"hasChromeRuntime"`
				UA                 string      `json:"ua"`
				HeaArch            string      `json:"hea_arch"`
				HeaPlatform        string      `json:"hea_platform"`
				HeaBitness         string      `json:"hea_bitness"`
				HeaPlatformVersion string      `json:"hea_platformVersion"`
				HeaError           string      `json:"hea_error"`
			} `json:"main"`
			Worker struct {
				Platform    string `json:"platform"`
				UA          string `json:"ua"`
				CellStealth string `json:"cellStealth"`
				HeaArch     string `json:"hea_arch"`
				HeaPlatform string `json:"hea_platform"`
				HeaBitness  string `json:"hea_bitness"`
				HeaError    string `json:"hea_error"`
				Error       string `json:"error"`
			} `json:"worker"`
		}
		headersRaw := runStealthProbe(t, c, stealthProbe{
			name:        "all-layers",
			nixChromium: true,
			env: []string{
				"DEVCELL_STEALTH_ARCH=" + archEnv,
				"DEVCELL_STEALTH_PLATFORM=" + platformEnv,
			},
		}, &results)
		t.Logf("probe results (parsed):\n  main=%+v\n  worker=%+v", results.Main, results.Worker)

		// Find the last request with the high-entropy hints.
		var archHeader, platformHeader, bitnessHeader, uaHeader string
		bitnessSeen := false
		for _, h := range parseStealthHeaders(headersRaw) {
			if v := h["sec-ch-ua-arch"]; v != "" {
				archHeader = strings.Trim(v, `"`)
			}
			if v := h["sec-ch-ua-platform"]; v != "" {
				platformHeader = strings.Trim(v, `"`)
			}
			// Sec-CH-UA-Bitness: header is present when Chrome honored Accept-CH.
			// An empty value (`""`) is the broken case (CELL-150) vs `"64"` GREEN.
			if v, ok := h["sec-ch-ua-bitness"]; ok {
				bitnessHeader = strings.Trim(v, `"`)
				bitnessSeen = true
			}
			if v := h["user-agent"]; v != "" {
				uaHeader = v
			}
		}
		t.Logf("HTTP headers observed: Sec-CH-UA-Arch=%q  Sec-CH-UA-Platform=%q  Sec-CH-UA-Bitness=%q(seen=%v)  User-Agent=%q",
			archHeader, platformHeader, bitnessHeader, bitnessSeen, uaHeader)

		// ── Layer 0: sentinel ──────────────────────────────────────────────────
		// If __cellStealth is undefined, the wrapper's --init-script preamble
		// never reached the page context. This is the CELL-161 silent-abort
		// signature and the single most important assertion in this file.
		if results.Main.CellStealth == "undefined" || results.Main.CellStealth == "" {
			t.Errorf("FAIL Layer 0 (sentinel): typeof window.__cellStealth=%q, want %q — "+
				"stealth-init.js / __cellStealth preamble never reached the page (CELL-161 regression class)",
				results.Main.CellStealth, "object")
		} else {
			t.Logf("PASS Layer 0: __cellStealth=%s  value=%s", results.Main.CellStealth, results.Main.CellStealthValue)
		}

		// ── Layer 1: CDP HTTP headers ──────────────────────────────────────────
		if archHeader == "" {
			t.Errorf("FAIL Layer 1: no Sec-CH-UA-Arch header observed in any request — "+
				"either jq merge of userAgentMetadata failed (CELL-68), or Chrome didn't honor Accept-CH/Critical-CH. "+
				"recorded headers:\n%s", lastNLines(headersRaw, 20))
		} else if archHeader != archEnv {
			t.Errorf("FAIL Layer 1: Sec-CH-UA-Arch=%q, want %q (CELL-68 cross-layer arch mismatch)",
				archHeader, archEnv)
		} else {
			t.Logf("PASS Layer 1: Sec-CH-UA-Arch=%q matches DEVCELL_STEALTH_ARCH", archHeader)
		}
		if platformHeader != "" && platformHeader != platformEnv {
			t.Errorf("FAIL Layer 1: Sec-CH-UA-Platform=%q, want %q", platformHeader, platformEnv)
		}
		// CELL-150: Sec-CH-UA-Bitness must be "64" (real desktop Chrome). Empty
		// string was observed in the broken state, anomalous for any x86/arm64
		// desktop and a strong fingerprint.
		if !bitnessSeen {
			t.Errorf("FAIL Layer 1: Sec-CH-UA-Bitness header never sent — Accept-CH not honored, or wrapper not advertising it (CELL-150)")
		} else if bitnessHeader != "64" {
			t.Errorf("FAIL Layer 1: Sec-CH-UA-Bitness=%q, want %q (CELL-150: empty string is the leak signature)",
				bitnessHeader, "64")
		} else {
			t.Logf("PASS Layer 1: Sec-CH-UA-Bitness=%q", bitnessHeader)
		}

		// ── Layer 2: main-thread JS ────────────────────────────────────────────
		if results.Main.Platform != expectedPlatform {
			t.Errorf("FAIL Layer 2: navigator.platform=%q, want %q (CELL-68 main-thread spoof)",
				results.Main.Platform, expectedPlatform)
		} else {
			t.Logf("PASS Layer 2: navigator.platform=%q", results.Main.Platform)
		}
		// navigator.webdriver must be boolean false, the real non-automated
		// Chrome value. `undefined` (property deleted) reads as tampering:
		// BrowserScan's Navigator check flags a missing webdriver property
		// (seen live 2026-09-03).
		if results.Main.WebdriverType != "boolean" || results.Main.Webdriver == true {
			t.Errorf("FAIL Layer 2: navigator.webdriver=%v (type=%s), want boolean false — missing/undefined is itself a bot signal",
				results.Main.Webdriver, results.Main.WebdriverType)
		} else {
			t.Logf("PASS Layer 2: navigator.webdriver=%v (type=%s)", results.Main.Webdriver, results.Main.WebdriverType)
		}
		// Real Chrome has NO chrome.runtime on ordinary pages; a fabricated
		// runtime is CreepJS's hasBadChromeRuntime signal. window.chrome itself
		// (app/csi/loadTimes) must exist.
		if !results.Main.HasChrome {
			t.Errorf("FAIL Layer 2: window.chrome missing entirely")
		} else if results.Main.HasChromeRuntime {
			t.Errorf("FAIL Layer 2: window.chrome.runtime fabricated — real Chrome pages have no chrome.runtime (CreepJS hasBadChromeRuntime)")
		} else {
			t.Logf("PASS Layer 2: window.chrome present, no fabricated chrome.runtime")
		}
		if results.Main.HeaArch == "" {
			t.Errorf("FAIL Layer 2: getHighEntropyValues().architecture empty — main-thread spoof not running (CELL-68). hea_error=%q",
				results.Main.HeaError)
		} else if results.Main.HeaArch != archEnv {
			t.Errorf("FAIL Layer 2: getHighEntropyValues().architecture=%q, want %q (CELL-68)",
				results.Main.HeaArch, archEnv)
		} else {
			t.Logf("PASS Layer 2: hea.architecture=%q matches env", results.Main.HeaArch)
		}
		// CELL-150: main-thread bitness must be "64" (matches HTTP layer).
		if results.Main.HeaBitness != "64" {
			t.Errorf("FAIL Layer 2: getHighEntropyValues().bitness=%q, want %q (CELL-150)",
				results.Main.HeaBitness, "64")
		} else {
			t.Logf("PASS Layer 2: hea.bitness=%q", results.Main.HeaBitness)
		}

		// ── Layer 3: Worker JS ─────────────────────────────────────────────────
		if results.Worker.Error != "" {
			t.Errorf("FAIL Layer 3: Worker probe failed: %q (workers must be reachable for cross-context consistency)",
				results.Worker.Error)
		} else {
			if results.Worker.Platform != expectedPlatform {
				t.Errorf("FAIL Layer 3: Worker navigator.platform=%q, want %q (CELL-34)",
					results.Worker.Platform, expectedPlatform)
			} else {
				t.Logf("PASS Layer 3: Worker navigator.platform=%q matches main", results.Worker.Platform)
			}
			if results.Worker.HeaArch == "" {
				t.Errorf("FAIL Layer 3: Worker getHighEntropyValues().architecture empty (CELL-25). hea_error=%q",
					results.Worker.HeaError)
			} else if results.Worker.HeaArch != archEnv {
				t.Errorf("FAIL Layer 3: Worker hea.architecture=%q, want %q (CELL-25)",
					results.Worker.HeaArch, archEnv)
			} else {
				t.Logf("PASS Layer 3: Worker hea.architecture=%q matches env", results.Worker.HeaArch)
			}
			// CELL-150: Worker bitness must match main (consistency across contexts).
			if results.Worker.HeaBitness != "64" {
				t.Errorf("FAIL Layer 3: Worker getHighEntropyValues().bitness=%q, want %q (CELL-150 worker propagation)",
					results.Worker.HeaBitness, "64")
			} else {
				t.Logf("PASS Layer 3: Worker hea.bitness=%q matches main", results.Worker.HeaBitness)
			}
		}
	})

	// GoogleChromeBrand (DIMM-XXX): Sec-CH-UA must include the "Google
	// Chrome" brand. A detection-site sweep (creepjs, browserleaks, pixelscan)
	// showed our Sec-CH-UA reporting only
	// `"Chromium";v="141", "Not?A_Brand";v="8"`. Real Chrome adds
	// `"Google Chrome";v="141"`; its absence is one of the strongest signals
	// Google uses at the identifier step ("Couldn't sign you in: this browser
	// or app may not be secure"). patchright-core's `_calculateBrandsList`
	// omits it, the same class of patch as CELL-150 (nix postPatch on
	// coreBundle.js).
	//
	// Production-mode launch: NO --headless, NO --executable-path. Lets
	// patchright use its bundled chromium and headed Xvfb display, same as
	// 'cell claude' (CELL-17).
	t.Run("GoogleChromeBrand", func(t *testing.T) {
		var results struct {
			UA     string `json:"ua"`
			Brands []struct {
				Brand   string `json:"brand"`
				Version string `json:"version"`
			} `json:"brands"`
			FullVersionList []struct {
				Brand   string `json:"brand"`
				Version string `json:"version"`
			} `json:"fullVersionList"`
			Error string `json:"error"`
		}
		headersRaw := runStealthProbe(t, c, stealthProbe{name: "brand", headed: true}, &results)
		t.Logf("probe results: ua=%q brands=%+v fullVersionList=%+v",
			results.UA, results.Brands, results.FullVersionList)

		var secChUA string
		for _, h := range parseStealthHeaders(headersRaw) {
			if v := h["sec-ch-ua"]; v != "" {
				secChUA = v
			}
		}
		t.Logf("HTTP Sec-CH-UA=%q", secChUA)

		// ── Assertion 1: JS userAgentData.brands includes "Google Chrome" ──
		var jsBrands []string
		for _, b := range results.Brands {
			jsBrands = append(jsBrands, b.Brand)
		}
		hasGoogleChromeJS := false
		for _, b := range jsBrands {
			if b == "Google Chrome" {
				hasGoogleChromeJS = true
				break
			}
		}
		if !hasGoogleChromeJS {
			t.Errorf("FAIL: navigator.userAgentData.brands missing %q — got %v. "+
				"Real Chrome always advertises Google Chrome brand; absence is a Google sign-in rejection signal.",
				"Google Chrome", jsBrands)
		} else {
			t.Logf("PASS: brands include %q", "Google Chrome")
		}

		// ── Assertion 2: HTTP Sec-CH-UA header contains "Google Chrome" ──
		if secChUA == "" {
			t.Error("FAIL: no Sec-CH-UA header observed in any request — Accept-CH not honored?")
		} else if !strings.Contains(secChUA, "Google Chrome") {
			t.Errorf("FAIL: Sec-CH-UA=%q missing %q brand. Real Chrome sends "+
				`"Google Chrome";v="141", "Chromium";v="141", "Not?A_Brand";v="8". `+
				"This is the patchright-core _calculateBrandsList omission.",
				secChUA, "Google Chrome")
		} else {
			t.Logf("PASS: Sec-CH-UA contains Google Chrome")
		}

		// ── Assertion 3: fullVersionList populated with Google Chrome ──
		hasGoogleChromeFVL := false
		for _, b := range results.FullVersionList {
			if b.Brand == "Google Chrome" {
				hasGoogleChromeFVL = true
				break
			}
		}
		if len(results.FullVersionList) == 0 {
			t.Errorf("FAIL: fullVersionList is empty — getHighEntropyValues didn't return brand versions (real Chrome populates this).")
		} else if !hasGoogleChromeFVL {
			var fvlBrands []string
			for _, b := range results.FullVersionList {
				fvlBrands = append(fvlBrands, b.Brand)
			}
			t.Errorf("FAIL: fullVersionList missing %q brand; got %v", "Google Chrome", fvlBrands)
		} else {
			t.Logf("PASS: fullVersionList includes Google Chrome")
		}
	})

	// H264CodecSupport (CELL-20): real Chrome reports
	// `canPlayType("video/mp4; codecs=avc1.42E01E")` == "probably"; upstream
	// Chromium returns "". Sannysoft flags this as VIDEO_CODECS WARN. Broken
	// state: H.264 and AAC return "" (unsupported) on
	// Chromium-without-proprietary-codecs.
	t.Run("H264CodecSupport", func(t *testing.T) {
		var r struct {
			H264Baseline   string `json:"h264_baseline"`
			H264Main       string `json:"h264_main"`
			AACLC          string `json:"aac_lc"`
			MP3            string `json:"mp3"`
			VP9Webm        string `json:"vp9_webm"`
			OpusWebm       string `json:"opus_webm"`
			MSH264Baseline bool   `json:"ms_h264_baseline"`
			MSAACLC        bool   `json:"ms_aac_lc"`
			MCH264         *struct {
				Supported bool `json:"supported"`
			} `json:"mc_h264"`
		}
		runStealthProbe(t, c, stealthProbe{name: "codec", nixChromium: true}, &r)
		t.Logf("probe: h264=%q h264_main=%q aac=%q mp3=%q vp9=%q opus=%q ms_h264=%v ms_aac=%v",
			r.H264Baseline, r.H264Main, r.AACLC, r.MP3, r.VP9Webm, r.OpusWebm, r.MSH264Baseline, r.MSAACLC)

		if r.H264Baseline != "probably" {
			t.Errorf("FAIL: canPlayType(H.264 baseline avc1.42E01E)=%q, want %q. "+
				"Chromium-without-proprietary-codecs returns \"\" — claims Chrome UA but can't actually decode H.264. "+
				"Strong sannysoft / creepjs fingerprint (mimes 6/12).",
				r.H264Baseline, "probably")
		}
		if r.H264Main != "probably" {
			t.Errorf("FAIL: canPlayType(H.264 main avc1.4d4015)=%q, want %q",
				r.H264Main, "probably")
		}
		if r.AACLC != "probably" {
			t.Errorf("FAIL: canPlayType(AAC-LC mp4a.40.2)=%q, want %q (AAC ships with H.264)",
				r.AACLC, "probably")
		}
		if !r.MSH264Baseline {
			t.Errorf("FAIL: MediaSource.isTypeSupported(H.264 baseline)=false, want true")
		}
		if !r.MSAACLC {
			t.Errorf("FAIL: MediaSource.isTypeSupported(AAC-LC)=false, want true")
		}
		if r.MCH264 != nil && !r.MCH264.Supported {
			t.Errorf("FAIL: mediaCapabilities.decodingInfo(H.264).supported=false, want true")
		}
		// Sanity: open codecs MUST work (else something is very wrong).
		if r.VP9Webm != "probably" {
			t.Errorf("SANITY FAIL: VP9 canPlayType=%q (open codec — should always work)", r.VP9Webm)
		}
		if r.OpusWebm != "probably" {
			t.Errorf("SANITY FAIL: Opus canPlayType=%q", r.OpusWebm)
		}
	})

	// PermissionsAPISupported (CELL-19): real Chrome returns a
	// PermissionStatus for most queries (state "prompt" or "granted"). The
	// broken state returns "Not supported" for ~15 of them; amiunique scores
	// that at 0.03% similarity, essentially unique. Must be >=10 of 15
	// queryable (returning a state) to pass.
	//
	// NO --headless: production runs Xvfb-headed via DISPLAY=:99 (CELL-17).
	// Headless mode disables most permission backends, returning "Illegal
	// invocation" for queries that DO work in real production Chrome.
	t.Run("PermissionsAPISupported", func(t *testing.T) {
		var r struct {
			Error string `json:"error"`
			Perms map[string]struct {
				State string `json:"state"`
				OK    bool   `json:"ok"`
				Error string `json:"error"`
			} `json:"perms"`
		}
		runStealthProbe(t, c, stealthProbe{name: "permissions", headed: true, nixChromium: true}, &r)
		if r.Error != "" {
			t.Fatalf("FAIL: %s", r.Error)
		}

		queryable := 0
		failing := []string{}
		for name, p := range r.Perms {
			if p.OK && p.State != "" {
				queryable++
			} else {
				failing = append(failing, name+":"+p.Error)
			}
		}
		t.Logf("probe: %d/%d permissions queryable; failing: %v", queryable, len(r.Perms), failing)

		// Core 5: geo, notifications, camera, microphone, push: should ALWAYS query.
		core := []string{"geolocation", "notifications", "camera", "microphone", "push"}
		for _, name := range core {
			p, ok := r.Perms[name]
			if !ok {
				t.Errorf("FAIL: %q not in results — probe didn't query?", name)
				continue
			}
			if !p.OK || p.State == "" {
				t.Errorf("FAIL: navigator.permissions.query({name:%q}) failed/Not supported (state=%q, err=%q). "+
					"Real Chrome ALWAYS returns a PermissionStatus for this. amiunique 0.03%% similarity → uniquely identifying.",
					name, p.State, p.Error)
			}
		}

		// Bulk: at least 10 of 15 should succeed.
		if queryable < 10 {
			t.Errorf("FAIL: only %d/%d permissions queryable, want ≥10. "+
				"creepjs reports `permissions (0)` — zero queryable permissions is uniquely identifying.",
				queryable, len(r.Perms))
		}
	})

	// TimezoneMatchesContainer (CELL-21): timezone vs IP geolocation
	// mismatch. JS Intl.timeZone reported UTC ("Africa/Abidjan") while the
	// container's egress IP geolocated to Czechia (Europe/Prague); Pixelscan
	// flags this as the #1 inconsistency. Asserts JS Intl.timeZone matches
	// the TZ patchright-mcp-cell was launched with.
	t.Run("TimezoneMatchesContainer", func(t *testing.T) {
		const expectedTZ = "Europe/Prague"

		var r struct {
			IntlTz    string `json:"intlTz"`
			TzOffset  int    `json:"tzOffset"`
			DateStr   string `json:"dateStr"`
			JsEpochMs int64  `json:"jsEpochMs"`
			Worker    struct {
				WTz   string `json:"wTz"`
				WOff  int    `json:"wOff"`
				Error string `json:"error"`
			} `json:"worker"`
		}
		runStealthProbe(t, c, stealthProbe{
			name:        "timezone",
			nixChromium: true,
			env:         []string{"TZ=" + expectedTZ},
		}, &r)
		t.Logf("probe: intlTz=%q tzOffset=%d dateStr=%q worker=%+v",
			r.IntlTz, r.TzOffset, r.DateStr, r.Worker)

		if r.IntlTz != expectedTZ {
			t.Errorf("FAIL: Intl.DateTimeFormat().resolvedOptions().timeZone=%q, want %q "+
				"(container TZ=%q ignored — JS sees UTC, IP geolocates Czechia → pixelscan flags as spoofed)",
				r.IntlTz, expectedTZ, expectedTZ)
		}
		// Europe/Prague is UTC+1 (or +2 in DST). getTimezoneOffset is INVERTED
		// (returns -60 or -120 minutes). UTC returns 0.
		if r.TzOffset == 0 {
			t.Errorf("FAIL: Date().getTimezoneOffset()=0 (UTC); container TZ=%s should produce non-zero offset",
				expectedTZ)
		}
		if r.Worker.Error == "" && r.Worker.WTz != expectedTZ {
			t.Errorf("FAIL: Worker Intl.timeZone=%q, want %q (Worker context not honoring TZ)",
				r.Worker.WTz, expectedTZ)
		}
		if r.Worker.Error == "" && r.Worker.WTz != r.IntlTz {
			t.Errorf("FAIL: Worker TZ %q != main TZ %q (cross-context inconsistency)",
				r.Worker.WTz, r.IntlTz)
		}
		if strings.Contains(r.DateStr, "GMT+0000") && expectedTZ != "UTC" {
			t.Errorf("FAIL: Date.toString()=%q reports GMT+0000 despite TZ=%q", r.DateStr, expectedTZ)
		}
	})

	// MainThreadWebGLMatchesPlatform (CELL-70): main-thread WebGL strings.
	// The broken state reports "Intel Inc." / "Intel Iris OpenGL Engine",
	// macOS strings on Linux aarch64.
	//
	// Production-mode launch: NO --headless, NO --executable-path. Lets
	// patchright-mcp-cell use its bundled chromium (the same one Claude Code's
	// 'playwright' MCP uses). Forcing --executable-path to nix chromium yields
	// 'no WebGL' and masks the actual production fingerprint.
	t.Run("MainThreadWebGLMatchesPlatform", func(t *testing.T) {
		var r struct {
			UnmaskedVendor   string `json:"unmaskedVendor"`
			UnmaskedRenderer string `json:"unmaskedRenderer"`
			Vendor           string `json:"vendor"`
			Renderer         string `json:"renderer"`
			Version          string `json:"version"`
			ShadingLang      string `json:"shadingLang"`
			Platform         string `json:"platform"`
			Error            string `json:"error"`
		}
		runStealthProbe(t, c, stealthProbe{name: "webgl", headed: true}, &r)
		t.Logf("probe: platform=%q unmaskedVendor=%q unmaskedRenderer=%q vendor=%q renderer=%q",
			r.Platform, r.UnmaskedVendor, r.UnmaskedRenderer, r.Vendor, r.Renderer)

		if r.Error != "" {
			t.Fatalf("FAIL: WebGL unavailable: %q", r.Error)
		}

		// FAIL if Mac-style vendor on a non-Mac platform.
		isLinux := strings.Contains(r.Platform, "Linux")
		macStyleVendor := r.UnmaskedVendor == "Intel Inc." || strings.HasPrefix(r.UnmaskedVendor, "Apple")
		macStyleRenderer := strings.Contains(r.UnmaskedRenderer, "Intel Iris") ||
			strings.Contains(r.UnmaskedRenderer, "OpenGL Engine") ||
			strings.HasPrefix(r.UnmaskedRenderer, "Apple")

		if isLinux && macStyleVendor {
			t.Errorf("FAIL: UNMASKED_VENDOR_WEBGL=%q on platform=%q — Mac-style vendor on Linux. "+
				"Real Linux Chrome reports Mesa/Google Inc./Khronos. amiunique scores this at 1.85%% similarity.",
				r.UnmaskedVendor, r.Platform)
		}
		if isLinux && macStyleRenderer {
			t.Errorf("FAIL: UNMASKED_RENDERER_WEBGL=%q on platform=%q — macOS-style renderer string on Linux. "+
				"amiunique scores this at 1.22%% similarity. Strong cross-fingerprint Google signal.",
				r.UnmaskedRenderer, r.Platform)
		}
		if !strings.HasPrefix(r.Version, "WebGL ") {
			t.Errorf("FAIL: unexpected WebGL VERSION=%q", r.Version)
		}
	})
}
