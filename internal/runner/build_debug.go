package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// LayerStats counts skopeo's per-blob outcomes from a single image copy.
// "New" = blobs transferred to the destination; "Cached" = blobs the
// destination already had (cache hit / dedup). Sum should equal the image's
// layer count for a clean push (the config blob is filtered out).
type LayerStats struct {
	New    int
	Cached int
}

// LayerCounter wraps an io.Writer, mirrors all bytes through unchanged,
// and tallies skopeo's "Copying blob ... done" / "Copying blob ... skipped"
// lines. Use it during the registry-push step of BuildImagePure to surface
// cache hit-rate to `cell build --debug`.
//
// Lines are buffered until '\n' so io.Copy chunking can't split a match.
// "Copying config" lines are intentionally ignored — that's the image
// config descriptor, not a layer.
//
// BuildImagePure invokes skopeo twice (nix→registry, registry→daemon), so
// the same blob ID appears in both passes. We dedupe by ID and only count
// each blob's FIRST classification — that line reflects the build-side
// cache state (was this layer in the local nix store / registry cache?),
// which is what the user means by "cached vs new".
type LayerCounter struct {
	w     io.Writer
	buf   bytes.Buffer
	stats LayerStats
	seen  map[string]struct{}
}

func NewLayerCounter(w io.Writer) *LayerCounter {
	return &LayerCounter{w: w, seen: map[string]struct{}{}}
}

func (lc *LayerCounter) Write(p []byte) (int, error) {
	n, err := lc.w.Write(p)
	if err != nil {
		return n, err
	}
	lc.buf.Write(p[:n])
	for {
		idx := bytes.IndexByte(lc.buf.Bytes(), '\n')
		if idx < 0 {
			break
		}
		line := lc.buf.Next(idx + 1)
		lc.scan(line)
	}
	return n, nil
}

func (lc *LayerCounter) scan(line []byte) {
	s := string(line)
	if !strings.HasPrefix(s, "Copying blob ") {
		return
	}
	// "Copying blob <id> done|skipped..."
	rest := strings.TrimPrefix(s, "Copying blob ")
	idEnd := strings.IndexByte(rest, ' ')
	if idEnd <= 0 {
		return
	}
	id := rest[:idEnd]
	if _, dup := lc.seen[id]; dup {
		return
	}
	lc.seen[id] = struct{}{}
	switch {
	case strings.Contains(s, " skipped"):
		lc.stats.Cached++
	case strings.Contains(s, " done"):
		lc.stats.New++
	}
}

func (lc *LayerCounter) Stats() LayerStats { return lc.stats }

// BuildDebugInfo bundles the post-build inspect data shown by `cell build --debug`.
type BuildDebugInfo struct {
	Tag        string
	ID         string
	Created    string
	SizeBytes  int64
	LayerCount int
}

// InspectImageDebug calls `docker image inspect <tag> --format '{{json .}}'`
// once and extracts the fields shown in the --debug summary.
func InspectImageDebug(ctx context.Context, tag string) (BuildDebugInfo, error) {
	out, err := exec.CommandContext(ctx,
		"docker", "image", "inspect", tag, "--format", "{{json .}}",
	).Output()
	if err != nil {
		return BuildDebugInfo{Tag: tag}, fmt.Errorf("inspect %s: %w", tag, err)
	}
	info, err := parseImageInspectDebug(out)
	if err != nil {
		return BuildDebugInfo{Tag: tag}, err
	}
	info.Tag = tag
	return info, nil
}

// inspectEntry is the on-the-wire shape of `docker image inspect` output
// needed by the --debug summary. Top-level type so both unmarshal paths
// share it (anonymous structs aren't assignable across declaration sites).
type inspectEntry struct {
	ID      string `json:"Id"`
	Created string `json:"Created"`
	Size    int64  `json:"Size"`
	RootFS  struct {
		Layers []string `json:"Layers"`
	} `json:"RootFS"`
}

// parseImageInspectDebug is the pure-data half of InspectImageDebug.
// Accepts either the array form (`docker image inspect` default) or the
// single-object form produced by `--format '{{json .}}'`.
func parseImageInspectDebug(raw []byte) (BuildDebugInfo, error) {
	var arr []inspectEntry
	if err := json.Unmarshal(raw, &arr); err == nil && len(arr) > 0 {
		return fromInspectEntry(arr[0]), nil
	}
	var single inspectEntry
	if err := json.Unmarshal(raw, &single); err != nil {
		return BuildDebugInfo{}, fmt.Errorf("parse docker inspect: %w", err)
	}
	return fromInspectEntry(single), nil
}

func fromInspectEntry(e inspectEntry) BuildDebugInfo {
	return BuildDebugInfo{
		ID:         e.ID,
		Created:    e.Created,
		SizeBytes:  e.Size,
		LayerCount: len(e.RootFS.Layers),
	}
}
