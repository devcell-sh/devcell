#!/usr/bin/env node
// stealth-fingerprint-test.js — table-driven stealth fingerprint test suite.
//
// Spawns patchright-mcp-cell (the PRODUCTION MCP wrapper) over stdio and
// drives the browser via MCP JSON-RPC. Each site is a table entry with a URL,
// wait time, JS extraction function, and assertion list.
//
// Browser profiles control WHICH chromium binary and stealth config is used.
// Each profile is an independent test dimension: results are tracked per
// (profile, site) pair.
//
// Usage:
//   node test/js/stealth-fingerprint-test.js [--only creepjs,sannysoft,...] [--wait-mult N] [--profile name,...]
//
// --wait-mult  Multiplier for per-site wait times (default: 1). Use 0.5 for
//              fast networks or 2 for slow CI.
// --profile    Comma-separated profile names (default: patchright).
//              Use "all" to run every non-skipped profile.
//
// Exits 0 on all-pass, 1 on any failure.

'use strict';

const { spawn, execSync } = require('child_process');
const readline = require('readline');
const fs = require('fs');
const os = require('os');
const path = require('path');

// ── Minimal MCP stdio client ────────────────────────────────────────────────

class McpClient {
  constructor(command, args, env) {
    this._nextId = 1;
    this._pending = new Map();
    this._proc = spawn(command, args, {
      stdio: ['pipe', 'pipe', 'inherit'],
      env: { ...process.env, ...(env || {}) },
    });
    this._rl = readline.createInterface({ input: this._proc.stdout });
    this._rl.on('line', (line) => {
      if (!line.trim()) return;
      let msg;
      try { msg = JSON.parse(line); } catch { return; }
      if (msg.id != null && this._pending.has(msg.id)) {
        const { resolve, reject, timer } = this._pending.get(msg.id);
        this._pending.delete(msg.id);
        clearTimeout(timer);
        if (msg.error) reject(new Error(JSON.stringify(msg.error)));
        else resolve(msg.result);
      }
    });
    this._proc.on('exit', () => {
      for (const { reject, timer } of this._pending.values()) {
        clearTimeout(timer);
        reject(new Error('MCP process exited'));
      }
      this._pending.clear();
    });
  }

  _send(obj) { this._proc.stdin.write(JSON.stringify(obj) + '\n'); }

  request(method, params, timeoutMs = 60000) {
    const id = this._nextId++;
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        this._pending.delete(id);
        reject(new Error(`MCP request ${method} timed out after ${timeoutMs}ms`));
      }, timeoutMs);
      this._pending.set(id, { resolve, reject, timer });
      this._send({ jsonrpc: '2.0', id, method, params });
    });
  }

  notify(method, params) { this._send({ jsonrpc: '2.0', method, params }); }

  async initialize() {
    await this.request('initialize', {
      protocolVersion: '2024-11-05',
      capabilities: {},
      clientInfo: { name: 'stealth-fingerprint-test', version: '0.1' },
    });
    this.notify('notifications/initialized');
  }

  async callTool(name, args, timeoutMs) {
    return this.request('tools/call', { name, arguments: args }, timeoutMs);
  }

  async close() {
    try { await this.callTool('browser_close', {}, 5000); } catch {}
    this._proc.stdin.end();
    await new Promise(r => this._proc.on('exit', r));
  }
}

// ── Assertion tracker ───────────────────────────────────────────────────────

function makeTracker() {
  const failures = [];
  const passes = [];

  function assert(name, actual, expected, cmp) {
    const op = cmp || '===';
    let pass;
    if (op === '===') pass = actual === expected;
    else if (op === '!==') pass = actual !== expected;
    else if (op === 'includes') pass = typeof actual === 'string' && actual.includes(expected);
    else if (op === '!includes') pass = typeof actual !== 'string' || !actual.includes(expected);
    else if (op === '<') pass = actual < expected;
    else if (op === '>=') pass = actual >= expected;
    else if (op === '>') pass = actual > expected;
    else pass = actual === expected;

    const status = pass ? 'PASS' : 'FAIL';
    const line = `  [${status}] ${name}: got ${JSON.stringify(actual)}` +
      (pass ? '' : `, want ${op} ${JSON.stringify(expected)}`);
    console.log(line);
    if (pass) passes.push(name);
    else failures.push(name);
  }

  return { failures, passes, assert };
}

// ── MCP result parsing ──────────────────────────────────────────────────────

function extractJson(toolResult) {
  if (!toolResult || !toolResult.content) return null;
  const text = toolResult.content
    .filter(c => c.type === 'text')
    .map(c => c.text)
    .join('\n');
  const match = text.match(/### Result\n([\s\S]*?)(?:\n### |$)/);
  if (!match) return null;
  const raw = match[1].trim();
  try {
    const outer = JSON.parse(raw);
    if (typeof outer === 'string') return JSON.parse(outer);
    return outer;
  } catch { return null; }
}

async function evaluate(mcp, fn, timeoutMs = 30000) {
  const result = await mcp.callTool('browser_evaluate', { 'function': fn }, timeoutMs);
  return extractJson(result);
}

// ── Browser profiles ───────────────────────────────────────────────────────
//
// Each profile defines HOW chromium is launched. The test matrix is
// profiles x sites. Profiles that can't run on the current arch are skipped.

const ARCH = os.arch();      // 'arm64' or 'x64'
const UNAME_M = execSync('uname -m').toString().trim(); // 'aarch64' or 'x86_64'

function hasRosetta() {
  if (UNAME_M !== 'aarch64') return false;
  try {
    const x86Chrome = execSync(
      "grep '_CHROMIUM_X86=' $(readlink -f $(which patchright-mcp-cell)) | head -1",
      { encoding: 'utf8' }
    ).match(/"([^"]+)"/);
    if (!x86Chrome) return false;
    execSync(`timeout 10 "${x86Chrome[1]}" --version`, { stdio: 'pipe' });
    return true;
  } catch { return false; }
}

function findNixpkgsArm64Chrome() {
  if (UNAME_M !== 'aarch64') return null;
  try {
    // The user nix profile has a chromium wrapper; trace it to the
    // unwrapped binary (the actual ELF we can pass as executablePath).
    const profileBin = fs.realpathSync(
      `${process.env.HOME}/.local/state/nix/profiles/profile/bin/chromium`
    );
    // profileBin is /nix/store/<hash>-chromium-<ver>/bin/chromium (a wrapper script).
    // The unwrapped binary lives in a sibling store path named chromium-unwrapped-<ver>.
    const wrapperSrc = fs.readFileSync(profileBin, 'utf8');
    const unwrapMatch = wrapperSrc.match(/(\/nix\/store\/[^\s"']+chromium-unwrapped[^\s"']*\/libexec\/chromium\/chromium)/);
    if (unwrapMatch && fs.existsSync(unwrapMatch[1])) return unwrapMatch[1];
    // Fallback: scan nix store for aarch64 chromium-unwrapped binaries
    const found = execSync(
      `find /nix/store -maxdepth 5 -name 'chromium' -path '*/chromium-unwrapped*/libexec/*' 2>/dev/null | ` +
      `xargs -I{} sh -c 'readelf -h "{}" 2>/dev/null | grep -q AArch64 && echo "{}"' | head -1`,
      { encoding: 'utf8', timeout: 15000 }
    ).trim();
    return found || null;
  } catch { return null; }
}

function findSpeechdLib() {
  try {
    // Check nix-ld libs first (fastest), then search nix store
    const nldPath = '/opt/devcell/.nix-ld-libs/libspeechd.so.2';
    if (fs.existsSync(nldPath)) return path.dirname(nldPath);
    const found = execSync(
      "find /nix/store -maxdepth 4 -name 'libspeechd.so.2' 2>/dev/null | head -1",
      { encoding: 'utf8', timeout: 10000 }
    ).trim();
    return found ? path.dirname(found) : null;
  } catch { return null; }
}

const _rosetta = hasRosetta();
const _arm64Chrome = UNAME_M === 'aarch64' ? findNixpkgsArm64Chrome() : null;
const _speechdLib = findSpeechdLib();
const _tmpWrappers = [];

function generatePatchedWrapper(opts) {
  const originalPath = execSync('readlink -f $(which patchright-mcp-cell)', { encoding: 'utf8' }).trim();
  let src = fs.readFileSync(originalPath, 'utf8');

  // Fix _SHARE: the original derives it from $0's location, which breaks
  // when the wrapper lives in /tmp. Hardcode the original share path.
  const originalShare = path.join(path.dirname(path.dirname(originalPath)), 'share', 'patchright');
  src = src.replace(
    '_SELF="$(readlink -f "$0")"',
    `_SELF="${originalPath}"`
  );
  src = src.replace(
    '_SHARE="$(dirname "$(dirname "$_SELF")")/share/patchright"',
    `_SHARE="${originalShare}"`
  );

  if (opts.execPath) {
    src = src.replace(
      /^_EXEC_PATH_OVERRIDE=""$/m,
      `_EXEC_PATH_OVERRIDE="${opts.execPath}"`
    );
  }

  if (opts.speechFlag) {
    src = src.replace(
      /\["--user-agent=" \+ \$ua\]/,
      '["--user-agent=" + $ua, "--enable-speech-dispatcher"]'
    );
  }

  if (opts.speechdLib) {
    src = src.replace(
      /^export LD_LIBRARY_PATH="/m,
      `export LD_LIBRARY_PATH="${opts.speechdLib}:`
    );
  }


  if (opts.noShims) {
    // Replace init-script content with empty file (keeps the flag but no JS)
    src = src.replace(
      /printf '%s\\n' "\$_STEALTH_PREAMBLE" > "\$_STEALTH_INIT"/,
      'printf "" > "$_STEALTH_INIT"'
    );
    src = src.replace(
      /if \[ -f "\$_SHARE\/stealth-init\.js" \]; then\n\s*cat "\$_SHARE\/stealth-init\.js" >> "\$_STEALTH_INIT"\nfi/,
      '# stealth-init.js skipped (bare profile)'
    );
  }

  const tmpPath = path.join(os.tmpdir(), `patchright-mcp-cell-${opts.name}-${process.pid}`);
  fs.writeFileSync(tmpPath, src, { mode: 0o755 });
  _tmpWrappers.push(tmpPath);
  return tmpPath;
}

function cleanupWrappers() {
  for (const p of _tmpWrappers) {
    try { fs.unlinkSync(p); } catch {}
  }
}

const PROFILES = {
  'x86-rosetta': {
    desc: 'nixpkgs x86_64 chromium (Rosetta)',
    skip: () => {
      if (UNAME_M !== 'aarch64') return false;
      if (!_rosetta) return 'no Rosetta available on aarch64';
      return false;
    },
    spawn: () => ({
      command: 'patchright-mcp-cell',
      args: ['--browser', 'chromium'],
      env: { DEVCELL_STEALTH_ARCH: 'x86' },
    }),
  },

  'patchright': {
    desc: 'patchright builtin + stealth shims (production)',
    skip: () => false,
    spawn: () => ({
      command: 'patchright-mcp-cell',
      args: ['--browser', 'chromium'],
      env: {},
    }),
  },

  'arm64-bare': {
    desc: 'nixpkgs arm64 chromium, no stealth shims',
    skip: () => {
      if (UNAME_M !== 'aarch64') return 'not on aarch64';
      if (!_arm64Chrome) return 'no native aarch64 nixpkgs chromium found';
      return false;
    },
    spawn: () => {
      const wrapper = generatePatchedWrapper({
        name: 'arm64-bare',
        execPath: _arm64Chrome,
        speechFlag: true,
        speechdLib: _speechdLib,
        noShims: true,
      });
      return { command: wrapper, args: ['--browser', 'chromium'], env: {} };
    },
  },

  'arm64-shims': {
    desc: 'nixpkgs arm64 chromium + stealth shims',
    skip: () => {
      if (UNAME_M !== 'aarch64') return 'not on aarch64';
      if (!_arm64Chrome) return 'no native aarch64 nixpkgs chromium found';
      return false;
    },
    spawn: () => {
      const wrapper = generatePatchedWrapper({
        name: 'arm64-shims',
        execPath: _arm64Chrome,
        speechFlag: true,
        speechdLib: _speechdLib,
        noShims: false,
      });
      return { command: wrapper, args: ['--browser', 'chromium'], env: {} };
    },
  },
};

// ── Site test table ─────────────────────────────────────────────────────────
//
// Each entry: { url, waitSec, extract (JS string), assertions (fn) }
// extract: arrow-function string returning JSON.stringify({...})
// assertions: receives the parsed object, calls assert()
//
// "protector" sites assert we aren't blocked.
// "scanner" sites extract detailed signals and assert correctness.

const SITES = {

  // ── Protectors (pass = page loads without challenge/block) ──────────────

  cloudflare: {
    url: 'https://nowsecure.nl/',
    waitSec: 15,
    extract: `() => {
      const text = document.body.innerText;
      return JSON.stringify({
        challenged: text.includes('Checking your browser') || text.includes('Just a moment'),
        title: document.title,
        text: text.substring(0, 300)
      });
    }`,
    assertions: (r, assert) => {
      assert('cloudflare-not-challenged', r.challenged, false);
    },
  },

  akamai: {
    url: 'https://www.akamai.com/',
    waitSec: 10,
    extract: `() => {
      const text = document.body.innerText;
      return JSON.stringify({
        blocked: text.includes('Access Denied') || text.includes('403 Forbidden'),
        loaded: text.length > 200,
      });
    }`,
    assertions: (r, assert) => {
      assert('akamai-not-blocked', r.blocked, false);
      assert('akamai-loaded', r.loaded, true);
    },
  },

  bet365: {
    url: 'https://www.bet365.com/',
    waitSec: 15,
    extract: `() => {
      const text = document.body.innerText;
      return JSON.stringify({
        blocked: text.includes('Access Denied') || text.includes('are unable to') || document.title.includes('Error'),
        loaded: text.length > 100,
        title: document.title,
      });
    }`,
    assertions: (r, assert) => {
      assert('bet365-not-blocked', r.blocked, false);
      assert('bet365-loaded', r.loaded, true);
    },
  },

  datadome: {
    url: 'https://antoinevastel.com/bots/datadome',
    waitSec: 10,
    extract: `() => {
      const text = document.body.innerText;
      return JSON.stringify({
        blocked: text.includes('blocked') || text.includes('Device Check') || text.includes('captcha'),
        loaded: text.length > 50,
      });
    }`,
    assertions: (r, assert) => {
      assert('datadome-not-blocked', r.blocked, false);
    },
  },

  fingerprint: {
    url: 'https://demo.fingerprint.com/',
    waitSec: 10,
    extract: `() => {
      const text = document.body.innerText;
      return JSON.stringify({
        botDetected: text.toLowerCase().includes('bot detected') || text.toLowerCase().includes('automation detected'),
        loaded: text.length > 100,
      });
    }`,
    assertions: (r, assert) => {
      assert('fingerprint-no-bot', r.botDetected, false);
      assert('fingerprint-loaded', r.loaded, true);
    },
  },

  // ── Scanners (extract signals, assert correctness) ─────────────────────

  sannysoft: {
    url: 'https://bot.sannysoft.com/',
    waitSec: 8,
    extract: `() => {
      const rows = [...document.querySelectorAll('table tr')];
      const results = {};
      for (const row of rows) {
        const cells = row.querySelectorAll('td');
        if (cells.length >= 2) {
          const key = cells[0].textContent.trim();
          const val = cells[cells.length - 1].textContent.trim();
          const cls = cells[cells.length - 1].className || '';
          results[key] = { value: val, pass: cls.includes('passed') || !cls.includes('failed') };
        }
      }
      const entries = Object.values(results);
      return JSON.stringify({
        passed: entries.filter(e => e.pass).length,
        failed: entries.filter(e => !e.pass).length,
        total: entries.length,
        details: results,
      });
    }`,
    assertions: (r, assert) => {
      assert('sannysoft-no-failures', r.failed, 0);
      assert('sannysoft-webgl-vendor-pass', r.details?.['WebGL Vendor']?.pass, true);
      assert('sannysoft-webdriver', r.details?.['WebDriver (New)']?.value, 'missing (passed)');
      assert('sannysoft-chrome', r.details?.['Chrome (New)']?.value, 'present (passed)');
    },
  },

  creepjs: {
    url: 'https://abrahamjuliot.github.io/creepjs/',
    waitSec: 25,
    extract: `() => {
      const text = document.body.innerText;
      const r = {};
      const likeMatch = text.match(/(\\d+)% like headless/);
      const headlessMatch = text.match(/(\\d+)% headless:/);
      const stealthMatch = text.match(/(\\d+)% stealth:/);
      r.likeHeadless = likeMatch ? parseInt(likeMatch[1]) : null;
      r.headless = headlessMatch ? parseInt(headlessMatch[1]) : null;
      r.stealth = stealthMatch ? parseInt(stealthMatch[1]) : null;
      const featMatch = text.match(/Features:\\s*(\\S+)/);
      r.featureVersion = featMatch ? featMatch[1] : 'unknown';
      const speechMatch = text.match(/Speech[\\s\\S]*?local \\((\\d+)\\):\\s*(\\S+)/);
      r.speechLocalCount = speechMatch ? parseInt(speechMatch[1]) : 0;
      r.speechLocalStatus = speechMatch ? speechMatch[2] : 'unknown';
      const fontMatch = text.match(/Fonts[\\s\\S]*?load \\((\\d+)\\/(\\d+)\\)/);
      r.fontsLoaded = fontMatch ? parseInt(fontMatch[1]) : -1;
      const gpuMatch = text.match(/webgpu:\\s*(\\S+)/);
      r.webgpu = gpuMatch ? gpuMatch[1] : 'unknown';
      const workerSection = text.match(/Worker[\\s\\S]*?userAgentData:\\s*\\n\\s*((?:Chromium|Chrome|Google Chrome)\\s+\\d+)/);
      r.workerUADBrand = workerSection ? workerSection[1].trim() : 'unknown';
      const navSection = text.match(/Navigator[\\s\\S]*?userAgentData:\\s*\\n\\s*((?:Google Chrome|Chrome|Chromium)\\s+\\d+)/);
      r.navUADBrand = navSection ? navSection[1].trim() : 'unknown';
      r.workerArch = (text.match(/Worker[\\s\\S]*?device:[\\s\\S]*?Linux\\s+(?:\\(Linux\\s+)?(x86_64|aarch64|arm_64)/m) || [])[1] || 'unknown';
      r.batteryLevelPct = parseInt((text.match(/level:\\s*(\\d+)%/) || [])[1]) || -1;
      r.netRtt = parseInt((text.match(/rtt:\\s*(\\d+)/) || [])[1]) || -1;
      r.netDownlink = parseFloat((text.match(/downlink:\\s*([\\d.]+)/) || [])[1]) || -1;
      const wgl = text.match(/WebGL[\\s\\S]*?gpu:[\\s\\S]*?confidence:[^\\n]*\\n([^\\n]+)\\n([^\\n]+)/);
      r.webglRenderer = wgl ? wgl[2].trim().substring(0, 120) : 'unknown';
      return JSON.stringify(r);
    }`,
    assertions: (r, assert) => {
      assert('creepjs-stealth-pct', r.stealth || 0, 10, '<');
      assert('creepjs-like-headless-pct', r.likeHeadless || 0, 50, '<');
      assert('creepjs-feature-version-not-113', r.featureVersion, '113-115+', '!==');
      assert('creepjs-speech-not-blocked', r.speechLocalStatus, 'blocked', '!==');
      assert('creepjs-speech-local-voices', r.speechLocalCount, 0, '>');
      assert('creepjs-fonts-loaded', r.fontsLoaded, 15, '>=');
      assert('creepjs-webgpu-supported', r.webgpu, 'unsupported', '!==');
      const wv = (r.workerUADBrand.match(/(\d+)/) || [])[1];
      const nv = (r.navUADBrand.match(/(\d+)/) || [])[1];
      if (wv && nv) assert('creepjs-worker-nav-version-match', wv, nv);
      else assert('creepjs-worker-nav-version-parseable', false, true);
      assert('creepjs-worker-arch-no-arm', r.workerArch, 'arm_64', '!==');
      assert('creepjs-webgl-not-llvmpipe', r.webglRenderer, 'llvmpipe', '!includes');
      assert('creepjs-webgl-not-swiftshader', r.webglRenderer, 'SwiftShader', '!includes');
      assert('creepjs-battery-level-valid', r.batteryLevelPct, 0, '>');
      assert('creepjs-net-rtt', r.netRtt, 50);
      assert('creepjs-net-downlink', r.netDownlink, 10.5);
    },
  },

  browserscan: {
    url: 'https://www.browserscan.net/',
    waitSec: 10,
    extract: `() => {
      const text = document.body.innerText;
      const authMatch = text.match(/(\\d+)%/);
      return JSON.stringify({
        authenticityPct: authMatch ? parseInt(authMatch[1]) : -1,
        loaded: text.length > 100,
        text: text.substring(0, 500),
      });
    }`,
    assertions: (r, assert) => {
      assert('browserscan-loaded', r.loaded, true);
      if (r.authenticityPct >= 0) {
        assert('browserscan-authenticity', r.authenticityPct, 80, '>=');
      }
    },
  },

  pixelscan: {
    url: 'https://pixelscan.net/fingerprint-check',
    waitSec: 25,
    extract: `() => {
      const text = document.body.innerText;
      return JSON.stringify({
        consistent: text.includes('Your Browser Fingerprint is consistent'),
        inconsistent: text.includes('Your Browser Fingerprint is inconsistent'),
        timezoneSpoofed: text.includes('Timezone spoofed'),
        maskingDetected: text.includes('Masking detected'),
        noBotDetected: text.includes('No automated behavior detected'),
        noProxy: text.includes('No proxy detected'),
      });
    }`,
    assertions: (r, assert) => {
      assert('pixelscan-consistent', r.consistent, true);
      assert('pixelscan-not-inconsistent', r.inconsistent, false);
      assert('pixelscan-timezone-not-spoofed', r.timezoneSpoofed, false);
      assert('pixelscan-no-masking', r.maskingDetected, false);
      assert('pixelscan-no-bot', r.noBotDetected, true);
      assert('pixelscan-no-proxy', r.noProxy, true);
    },
  },

  // ── Commented out: unreachable or need special handling ────────────────
  //
  // brotector: GitHub Pages 404 (site moved or removed).
  //   url: 'https://kaliiiiiiiiii.github.io/brotector/?crash=false'
  //
  // kasada: jd.com navigation hangs (geo-blocked from Czech IP).
  //   url: 'https://www.jd.com/'
  //
  // iphey: navigation hangs (aggressive bot blocking at TCP level).
  //   url: 'https://iphey.com/'
  //
  // incolumitas: requires behavioral interaction (mouse, scroll) to compute
  //   score. No automated score without simulation.
  //   url: 'https://bot.incolumitas.com/'
};

// ── CLI parsing ─────────────────────────────────────────────────────────────

const ALL_SITES = Object.keys(SITES);
const onlyIdx = process.argv.indexOf('--only');
const onlyTests = onlyIdx !== -1
  ? process.argv[onlyIdx + 1].split(',')
  : ALL_SITES;

const waitMultIdx = process.argv.indexOf('--wait-mult');
const WAIT_MULT = waitMultIdx !== -1 ? parseFloat(process.argv[waitMultIdx + 1]) : 1;

const profileIdx = process.argv.indexOf('--profile');
const profileArg = profileIdx !== -1 ? process.argv[profileIdx + 1] : 'patchright';
const selectedProfiles = profileArg === 'all'
  ? Object.keys(PROFILES)
  : profileArg.split(',');

// ── Run sites against one profile ──────────────────────────────────────────

async function runProfile(profileName) {
  const profile = PROFILES[profileName];
  if (!profile) {
    console.log(`Unknown profile: ${profileName}`);
    return { passes: [], failures: [`${profileName}-unknown`], skipped: false };
  }

  const skipReason = profile.skip();
  if (skipReason) {
    console.log(`  SKIP: ${skipReason}`);
    return { passes: [], failures: [], skipped: true, skipReason };
  }

  const tracker = makeTracker();
  const { command, args, env } = profile.spawn();

  console.log(`  Spawning: ${path.basename(command)} ${args.join(' ')}`);
  const mcp = new McpClient(command, args, env);
  await mcp.initialize();
  console.log('  MCP initialized.\n');

  try {
    for (const name of onlyTests) {
      const site = SITES[name];
      if (!site) { console.log(`  Unknown site: ${name}`); continue; }

      console.log(`  --- ${name.toUpperCase()} ---`);
      const waitMs = Math.round(site.waitSec * WAIT_MULT * 1000);

      try {
        await mcp.callTool('browser_navigate', { url: site.url }, 60000);
        await mcp.callTool('browser_wait_for', { time: waitMs }, waitMs + 30000);

        const r = await evaluate(mcp, site.extract);
        if (!r) {
          console.log(`  Failed to extract data from ${name}`);
          tracker.failures.push(`${name}-extract`);
          continue;
        }

        console.log(JSON.stringify(r, null, 2));
        console.log('  Assertions:');
        site.assertions(r, tracker.assert);
      } catch (err) {
        console.log(`  ERROR: ${err.message}`);
        tracker.failures.push(`${name}-error`);
      }
      console.log('');
    }
  } finally {
    console.log('  Closing browser...');
    await mcp.close();
  }

  return { passes: tracker.passes, failures: tracker.failures, skipped: false };
}

// ── Main ────────────────────────────────────────────────────────────────────

async function main() {
  console.log(`Sites: ${onlyTests.join(', ')}`);
  console.log(`Profiles: ${selectedProfiles.join(', ')}`);
  console.log(`Wait multiplier: ${WAIT_MULT}x`);
  if (_arm64Chrome) console.log(`nixpkgs arm64 chromium: ${_arm64Chrome}`);
  if (_speechdLib) console.log(`speechd lib: ${_speechdLib}`);
  console.log('');

  const results = {};
  let totalPasses = 0;
  let totalFailures = 0;

  for (const profileName of selectedProfiles) {
    console.log(`\n${'='.repeat(70)}`);
    console.log(`PROFILE: ${profileName} — ${(PROFILES[profileName] || {}).desc || '?'}`);
    console.log('='.repeat(70));

    try {
      results[profileName] = await runProfile(profileName);
    } catch (err) {
      console.log(`  PROFILE ERROR: ${err.message}`);
      results[profileName] = { passes: [], failures: [`${profileName}-launch-error`], skipped: false };
    }

    const r = results[profileName];
    if (!r.skipped) {
      totalPasses += r.passes.length;
      totalFailures += r.failures.length;
      console.log(`\n  Profile result: ${r.passes.length} PASSED, ${r.failures.length} FAILED`);
    }
  }

  // Summary
  console.log(`\n${'='.repeat(70)}`);
  console.log('SUMMARY');
  console.log('='.repeat(70));
  for (const [name, r] of Object.entries(results)) {
    if (r.skipped) {
      console.log(`  ${name}: SKIPPED (${r.skipReason})`);
    } else {
      const status = r.failures.length === 0 ? 'PASS' : 'FAIL';
      console.log(`  ${name}: ${status} (${r.passes.length} passed, ${r.failures.length} failed)`);
      if (r.failures.length > 0) {
        for (const f of r.failures) console.log(`    - ${f}`);
      }
    }
  }
  console.log(`\nTOTAL: ${totalPasses} PASSED, ${totalFailures} FAILED`);

  cleanupWrappers();

  if (totalFailures > 0) process.exit(1);
}

main().catch(err => { console.error(err); cleanupWrappers(); process.exit(1); });
