//go:build stealth_remote

package container_test

// stealth_detection_test.go: detection-site signal verification and results
// publishing. Reuses the probe infrastructure from stealth_test.go to test
// the same signals CELL-667 found on 5 external sites, but against our own
// in-container probe server (deterministic, no network dependency).
//
// Publishes structured results to test/results/<datetime>/stealth-results.json
// matching the CELL-667 audit format.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/devcell-sh/devcell/internal/testutil"
)

type siteSignal struct {
	Site       string `json:"site"`
	Signal     string `json:"signal"`
	Pass       bool   `json:"pass"`
	Details    string `json:"details"`
	Severity   string `json:"severity"`
	CellTicket string `json:"cell_ticket"`
}

type detectionReport struct {
	Timestamp string       `json:"timestamp"`
	Arch      string       `json:"arch"`
	Platform  string       `json:"platform"`
	Signals   []siteSignal `json:"signals"`
	Summary   struct {
		Total  int `json:"total"`
		Passed int `json:"passed"`
		Failed int `json:"failed"`
	} `json:"summary"`
}

func TestStealthDetection(t *testing.T) {
	c := startStealthCell(t)

	const (
		archEnv     = "arm"
		platformEnv = "Linux"
		tzEnv       = "America/New_York"
	)

	var signals []siteSignal

	// 1. bot.sannysoft.com signals: WebDriver, Chrome object, plugins, codecs
	t.Run("sannysoft", func(t *testing.T) {
		var results struct {
			Main struct {
				CellStealth      string      `json:"cellStealth"`
				Platform         string      `json:"platform"`
				WebdriverType    string      `json:"webdriverType"`
				Webdriver        interface{} `json:"webdriver"`
				HasChrome        bool        `json:"hasChrome"`
				HasChromeRuntime bool        `json:"hasChromeRuntime"`
				UA               string      `json:"ua"`
				HeaArch          string      `json:"hea_arch"`
				HeaPlatform      string      `json:"hea_platform"`
				HeaBitness       string      `json:"hea_bitness"`
			} `json:"main"`
			Worker struct {
				Platform string `json:"platform"`
				HeaArch  string `json:"hea_arch"`
				Error    string `json:"error"`
			} `json:"worker"`
		}
		runStealthProbe(t, c, stealthProbe{
			name:        "all-layers",
			nixChromium: true,
			env: []string{
				"DEVCELL_STEALTH_ARCH=" + archEnv,
				"DEVCELL_STEALTH_PLATFORM=" + platformEnv,
			},
		}, &results)

		// WebDriver
		wdPass := results.Main.WebdriverType == "boolean" && results.Main.Webdriver == false
		signals = append(signals, siteSignal{
			Site: "bot.sannysoft.com", Signal: "WebDriver flag",
			Pass: wdPass, Severity: "HIGH", CellTicket: "CELL-153",
			Details: fmt.Sprintf("typeof=%s value=%v", results.Main.WebdriverType, results.Main.Webdriver),
		})

		// Chrome object
		chromePass := results.Main.HasChrome && !results.Main.HasChromeRuntime
		signals = append(signals, siteSignal{
			Site: "bot.sannysoft.com", Signal: "Chrome object",
			Pass: chromePass, Severity: "HIGH", CellTicket: "CELL-153",
			Details: fmt.Sprintf("chrome=%v runtime=%v", results.Main.HasChrome, results.Main.HasChromeRuntime),
		})

		// Stealth init sentinel
		sentinelPass := results.Main.CellStealth == "object"
		signals = append(signals, siteSignal{
			Site: "bot.sannysoft.com", Signal: "stealth-init.js active",
			Pass: sentinelPass, Severity: "CRITICAL", CellTicket: "CELL-161",
			Details: fmt.Sprintf("typeof __cellStealth=%s", results.Main.CellStealth),
		})
	})

	// 2. pixelscan.net signals: platform consistency, TZ match
	t.Run("pixelscan", func(t *testing.T) {
		var results struct {
			Main struct {
				Platform    string `json:"platform"`
				UA          string `json:"ua"`
				HeaPlatform string `json:"hea_platform"`
				HeaArch     string `json:"hea_arch"`
			} `json:"main"`
			Worker struct {
				Platform string `json:"platform"`
			} `json:"worker"`
		}
		runStealthProbe(t, c, stealthProbe{
			name:        "all-layers",
			nixChromium: true,
			env: []string{
				"DEVCELL_STEALTH_ARCH=" + archEnv,
				"DEVCELL_STEALTH_PLATFORM=" + platformEnv,
			},
		}, &results)

		// navigator.platform matches UA
		expectedPlatform := "Linux aarch64"
		platformPass := results.Main.Platform == expectedPlatform
		signals = append(signals, siteSignal{
			Site: "pixelscan.net", Signal: "navigator.platform consistency",
			Pass: platformPass, Severity: "HIGH", CellTicket: "CELL-667",
			Details: fmt.Sprintf("platform=%q expected=%q ua_contains_CrOS=%v",
				results.Main.Platform, expectedPlatform, strings.Contains(results.Main.UA, "CrOS")),
		})

		// Cross-layer arch consistency
		archPass := results.Main.HeaArch == archEnv
		signals = append(signals, siteSignal{
			Site: "pixelscan.net", Signal: "cross-layer arch consistency",
			Pass: archPass, Severity: "HIGH", CellTicket: "CELL-68",
			Details: fmt.Sprintf("main.hea_arch=%q worker.platform=%q", results.Main.HeaArch, results.Worker.Platform),
		})

		// Worker consistency
		workerPass := results.Worker.Platform == expectedPlatform
		signals = append(signals, siteSignal{
			Site: "pixelscan.net", Signal: "worker platform consistency",
			Pass: workerPass, Severity: "MEDIUM", CellTicket: "CELL-34",
			Details: fmt.Sprintf("worker.platform=%q", results.Worker.Platform),
		})
	})

	// 3. browserleaks.com signals: navigator leaks
	t.Run("browserleaks", func(t *testing.T) {
		headersRaw := runStealthProbe(t, c, stealthProbe{
			name:        "all-layers",
			nixChromium: true,
			env: []string{
				"DEVCELL_STEALTH_ARCH=" + archEnv,
				"DEVCELL_STEALTH_PLATFORM=" + platformEnv,
			},
		}, &struct{}{})

		// Sec-CH-UA-Arch header
		var archHeader, bitnessHeader string
		for _, h := range parseStealthHeaders(headersRaw) {
			if v := h["sec-ch-ua-arch"]; v != "" {
				archHeader = strings.Trim(v, `"`)
			}
			if v := h["sec-ch-ua-bitness"]; v != "" {
				bitnessHeader = strings.Trim(v, `"`)
			}
		}
		archHdrPass := archHeader == archEnv
		signals = append(signals, siteSignal{
			Site: "browserleaks.com", Signal: "Sec-CH-UA-Arch header",
			Pass: archHdrPass, Severity: "HIGH", CellTicket: "CELL-68",
			Details: fmt.Sprintf("header=%q expected=%q", archHeader, archEnv),
		})

		bitnessPass := bitnessHeader == "64"
		signals = append(signals, siteSignal{
			Site: "browserleaks.com", Signal: "Sec-CH-UA-Bitness header",
			Pass: bitnessPass, Severity: "MEDIUM", CellTicket: "CELL-150",
			Details: fmt.Sprintf("header=%q expected=64", bitnessHeader),
		})
	})

	// 4. creepjs signals: brands, permissions
	t.Run("creepjs", func(t *testing.T) {
		var brandResults struct {
			Brands []struct {
				Brand   string `json:"brand"`
				Version string `json:"version"`
			} `json:"brands"`
			FullVersionList []struct {
				Brand   string `json:"brand"`
				Version string `json:"version"`
			} `json:"fullVersionList"`
		}
		runStealthProbe(t, c, stealthProbe{name: "brand", headed: true}, &brandResults)

		hasGoogleChrome := false
		for _, b := range brandResults.Brands {
			if b.Brand == "Google Chrome" {
				hasGoogleChrome = true
			}
		}
		signals = append(signals, siteSignal{
			Site: "creepjs", Signal: "Google Chrome brand in userAgentData",
			Pass: hasGoogleChrome, Severity: "HIGH", CellTicket: "CELL-667",
			Details: fmt.Sprintf("brands=%v", brandResults.Brands),
		})

		hasFVL := false
		for _, b := range brandResults.FullVersionList {
			if b.Brand == "Google Chrome" {
				hasFVL = true
			}
		}
		signals = append(signals, siteSignal{
			Site: "creepjs", Signal: "Google Chrome in fullVersionList",
			Pass: hasFVL, Severity: "MEDIUM", CellTicket: "CELL-667",
			Details: fmt.Sprintf("fvl_count=%d", len(brandResults.FullVersionList)),
		})

		// Permissions
		var permResults struct {
			Perms map[string]struct {
				State string `json:"state"`
				OK    bool   `json:"ok"`
			} `json:"perms"`
		}
		runStealthProbe(t, c, stealthProbe{name: "permissions", headed: true, nixChromium: true}, &permResults)
		queryable := 0
		for _, p := range permResults.Perms {
			if p.OK && p.State != "" {
				queryable++
			}
		}
		permPass := queryable >= 10
		signals = append(signals, siteSignal{
			Site: "creepjs", Signal: "permissions API queryable",
			Pass: permPass, Severity: "MEDIUM", CellTicket: "CELL-19",
			Details: fmt.Sprintf("%d/%d queryable (need >=10)", queryable, len(permResults.Perms)),
		})
	})

	// 5. Timezone (pixelscan flags this)
	t.Run("timezone", func(t *testing.T) {
		var tzResults struct {
			IntlTz   string `json:"intlTz"`
			TzOffset int    `json:"tzOffset"`
			Worker   struct {
				WTz   string `json:"wTz"`
				Error string `json:"error"`
			} `json:"worker"`
		}
		runStealthProbe(t, c, stealthProbe{
			name:        "timezone",
			nixChromium: true,
			env:         []string{"TZ=" + tzEnv},
		}, &tzResults)

		tzPass := tzResults.IntlTz == tzEnv
		signals = append(signals, siteSignal{
			Site: "pixelscan.net", Signal: "timezone matches TZ env",
			Pass: tzPass, Severity: "HIGH", CellTicket: "CELL-21",
			Details: fmt.Sprintf("intlTz=%q expected=%q offset=%d", tzResults.IntlTz, tzEnv, tzResults.TzOffset),
		})

		workerTzPass := tzResults.Worker.Error != "" || tzResults.Worker.WTz == tzEnv
		signals = append(signals, siteSignal{
			Site: "pixelscan.net", Signal: "worker timezone consistency",
			Pass: workerTzPass, Severity: "MEDIUM", CellTicket: "CELL-21",
			Details: fmt.Sprintf("worker.tz=%q", tzResults.Worker.WTz),
		})
	})

	// 6. H264 codecs (sannysoft VIDEO_CODECS)
	t.Run("codecs", func(t *testing.T) {
		var codecResults struct {
			H264Baseline string `json:"h264_baseline"`
			H264Main     string `json:"h264_main"`
			AACLC        string `json:"aac_lc"`
			VP9Webm      string `json:"vp9_webm"`
		}
		runStealthProbe(t, c, stealthProbe{name: "codec", nixChromium: true}, &codecResults)

		h264Pass := codecResults.H264Baseline == "probably"
		signals = append(signals, siteSignal{
			Site: "bot.sannysoft.com", Signal: "H264 codec support",
			Pass: h264Pass, Severity: "LOW", CellTicket: "CELL-20",
			Details: fmt.Sprintf("h264_baseline=%q h264_main=%q aac=%q",
				codecResults.H264Baseline, codecResults.H264Main, codecResults.AACLC),
		})
	})

	// 7. WebGL (platform mismatch)
	t.Run("webgl", func(t *testing.T) {
		var webglResults struct {
			UnmaskedVendor   string `json:"unmaskedVendor"`
			UnmaskedRenderer string `json:"unmaskedRenderer"`
			Platform         string `json:"platform"`
			Error            string `json:"error"`
		}
		runStealthProbe(t, c, stealthProbe{name: "webgl", headed: true}, &webglResults)

		webglPass := true
		if webglResults.Error != "" {
			webglPass = false
		} else {
			isLinux := strings.Contains(webglResults.Platform, "Linux")
			macVendor := webglResults.UnmaskedVendor == "Intel Inc." || strings.HasPrefix(webglResults.UnmaskedVendor, "Apple")
			if isLinux && macVendor {
				webglPass = false
			}
		}
		signals = append(signals, siteSignal{
			Site: "browserleaks.com", Signal: "WebGL vendor matches platform",
			Pass: webglPass, Severity: "HIGH", CellTicket: "CELL-70",
			Details: fmt.Sprintf("vendor=%q renderer=%q platform=%q",
				webglResults.UnmaskedVendor, webglResults.UnmaskedRenderer, webglResults.Platform),
		})
	})

	// Publish results
	report := detectionReport{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Arch:      archEnv,
		Platform:  platformEnv,
	}
	report.Signals = signals
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
		t.Errorf("marshal: %v", err)
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
	t.Logf("Results published to: %s", outPath)
	t.Logf("Summary: %d total, %d passed, %d failed", report.Summary.Total, report.Summary.Passed, report.Summary.Failed)

	// Print to stderr for easy grep
	fmt.Fprintf(os.Stderr, "\n=== STEALTH DETECTION RESULTS ===\n")
	for _, s := range signals {
		status := "PASS"
		if !s.Pass {
			status = "FAIL"
		}
		fmt.Fprintf(os.Stderr, "  [%s] %s / %s: %s\n", status, s.Site, s.Signal, s.Details)
	}
	fmt.Fprintf(os.Stderr, "Results: %s\n", outPath)
}
