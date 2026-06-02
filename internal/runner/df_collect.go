package runner

import (
	"context"
	"fmt"
	"io"
	"os/exec"
)

// DFCollector is the test seam for `cell build df` — production wires
// ExecCollector, which shells to `docker system df -v --format json`.
// Tests inject a fake that returns a canned JSON fixture.
type DFCollector interface {
	CollectSystemDF(ctx context.Context) ([]byte, error)
}

// ExecCollector is the real collector. It runs docker once and returns
// the raw JSON for ParseSystemDF to handle.
type ExecCollector struct{}

func (ExecCollector) CollectSystemDF(ctx context.Context) ([]byte, error) {
	out, err := exec.CommandContext(ctx, "docker", "system", "df", "-v", "--format", "json").Output()
	if err != nil {
		return nil, fmt.Errorf("docker system df: %w", err)
	}
	return out, nil
}

// DFOpts controls a single `cell build df` invocation.
type DFOpts struct {
	TopN  int         // 0 = all rows
	Kinds []EntryKind // empty = all kinds
	JSON  bool        // true → FormatJSON, false → FormatTable
}

// RunDFArgs bundles inputs to RunDF — keeps the call site readable
// (matches the pattern of RunPruneArgs).
type RunDFArgs struct {
	Ctx       context.Context
	Collector DFCollector
	Opts      DFOpts
	Out       io.Writer
}

// RunDF orchestrates: collect → parse → format. The pure-data steps live
// in df.go; this function just wires them.
func RunDF(a RunDFArgs) error {
	raw, err := a.Collector.CollectSystemDF(a.Ctx)
	if err != nil {
		return err
	}
	snap, err := ParseSystemDF(raw)
	if err != nil {
		return err
	}
	fopts := FormatOpts{TopN: a.Opts.TopN, Kinds: a.Opts.Kinds}
	if a.Opts.JSON {
		return FormatJSON(snap, fopts, a.Out)
	}
	return FormatTable(snap, fopts, a.Out)
}
