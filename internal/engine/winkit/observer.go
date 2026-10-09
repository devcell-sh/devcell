package winkit

import (
	"fmt"
	"sync"
	"time"
)

// Observer receives progress events from long-running QEMU operations.
type Observer interface {
	Logf(format string, args ...any)
	Progress(fraction float64, message string)
}

// debugObserver logs events via ux.Debugf and prints progress to the
// terminal. Used by Build's auto-download path.
type debugObserver struct{}

func (debugObserver) Logf(format string, args ...any) { fmt.Printf(format+"\n", args...) }
func (debugObserver) Progress(_ float64, msg string)  { fmt.Printf("\r  %s", msg) }

// NopObserver silently discards all events.
type NopObserver struct{}

func (NopObserver) Logf(string, ...any)      {}
func (NopObserver) Progress(float64, string) {}

// progressFunc adapts obs to winkit's OnProgress download hook: every update
// moves the spinner, and the log gets a line with speed and ETA every 5%.
// winkit calls it from concurrent UUP dump downloads, hence the lock.
func progressFunc(obs Observer) func(filename string, downloaded, total int64) {
	var (
		mu         sync.Mutex
		start      time.Time
		lastLogPct float64
	)
	return func(_ string, downloaded, total int64) {
		if total <= 0 {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if start.IsZero() {
			start = time.Now()
		}
		pct := float64(downloaded) / float64(total) * 100
		dlMB := float64(downloaded) / (1024 * 1024)
		totalMB := float64(total) / (1024 * 1024)

		msg := fmt.Sprintf("%.0f MB / %.0f MB (%.1f%%)", dlMB, totalMB, pct)
		obs.Progress(float64(downloaded)/float64(total), msg)

		if pct-lastLogPct < 5 && pct < 100 {
			return
		}
		lastLogPct = pct
		if elapsed := time.Since(start); downloaded > 0 && elapsed > time.Second {
			speed := float64(downloaded) / elapsed.Seconds() / (1024 * 1024)
			remaining := time.Duration(float64(total-downloaded) / float64(downloaded) * float64(elapsed))
			msg = fmt.Sprintf("%s, %s left @ %.0f MB/s", msg, remaining.Round(time.Second), speed)
		}
		obs.Logf("download: %s", msg)
	}
}
