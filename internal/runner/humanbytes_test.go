package runner_test

import (
	"testing"

	"github.com/DimmKirr/devcell/internal/runner"
)

// HumanBytes is the format helper used by the pure-build spinner success line
// to surface image size when skopeo's per-blob progress is empty (non-TTY).
// 1024-based units with the conventional KB/MB/GB abbreviations (matches what
// `docker images` prints).
func TestHumanBytes(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{0, "0 B"},
		{42, "42 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1024 * 1024, "1.0 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
		{31568256204, "29.4 GB"}, // 29.4 GiB rounded — exercises GB-tier branch
	}
	for _, c := range cases {
		if got := runner.HumanBytes(c.n); got != c.want {
			t.Errorf("HumanBytes(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}
