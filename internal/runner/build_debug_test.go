package runner

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

// TestLayerCounter_CountsDoneAndSkipped exercises the skopeo log parser.
// Pre-fix this test fails because LayerCounter doesn't exist yet.
//
// Skopeo's non-TTY output emits one line per blob in two shapes:
//
//	Copying blob <id> done   |
//	Copying blob <id> skipped: already exists
//
// "done" → blob was transferred (= new at destination).
// "skipped" → destination already had it (= cached / dedup hit).
// We use the push step's output for the count because that's where the
// ephemeral registry's blob cache surfaces the hit/miss decision.
func TestLayerCounter_CountsDoneAndSkipped(t *testing.T) {
	t.Parallel()
	input := strings.Join([]string{
		"Getting image source signatures",
		"Copying blob eecc815b2162 done   |",
		"Copying blob a6517f7fcac2 skipped: already exists  |",
		"Copying blob 2caf8f78bb77 done   |",
		"Copying blob 1cc48024c353 skipped: already exists  |",
		"Copying blob 07d0c45eb5c7 done   |",
		"Copying config 19c6e0712c done   |",
		"Writing manifest to image destination",
	}, "\n") + "\n"

	var passthrough bytes.Buffer
	lc := NewLayerCounter(&passthrough)
	if _, err := io.Copy(lc, strings.NewReader(input)); err != nil {
		t.Fatalf("copy: %v", err)
	}

	if got, want := passthrough.String(), input; got != want {
		t.Fatalf("LayerCounter must pass bytes through unchanged.\n got:\n%s\nwant:\n%s", got, want)
	}

	stats := lc.Stats()
	// Config blobs are not layers; the counter must ignore them so the
	// New/Cached numbers match what `docker image inspect` reports for layers.
	if stats.New != 3 {
		t.Errorf("New = %d, want 3 (three `done` blob lines, config blob excluded)", stats.New)
	}
	if stats.Cached != 2 {
		t.Errorf("Cached = %d, want 2 (two `skipped: already exists` blob lines)", stats.Cached)
	}
}

// TestLayerCounter_PartialLineBuffering — io.Copy may split bytes mid-line.
// The counter must buffer until newline and only count complete lines, else
// "...don" + "e |" produces zero matches and we under-count transfers.
func TestLayerCounter_PartialLineBuffering(t *testing.T) {
	t.Parallel()
	var sink bytes.Buffer
	lc := NewLayerCounter(&sink)

	for _, chunk := range []string{
		"Copying blob abc123 ", "do", "ne   |\nCopying blob def456 skip",
		"ped: already exists  |\n",
	} {
		if _, err := lc.Write([]byte(chunk)); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	s := lc.Stats()
	if s.New != 1 || s.Cached != 1 {
		t.Errorf("got New=%d Cached=%d, want New=1 Cached=1", s.New, s.Cached)
	}
}

// TestLayerCounter_DedupAcrossSkopeoRuns — BuildImagePure invokes skopeo
// twice (nix→registry, registry→daemon), so the same blob ID appears in
// both passes. The counter must dedupe by ID so the summary reflects the
// build's real cache state, not 2× the layer count.
func TestLayerCounter_DedupAcrossSkopeoRuns(t *testing.T) {
	t.Parallel()
	input := strings.Join([]string{
		// First skopeo pass: nix → registry. Registry cache decides done/skipped.
		"Copying blob aaaaaa done   |",
		"Copying blob bbbbbb skipped: already exists  |",
		"Copying blob cccccc done   |",
		// Second skopeo pass: registry → docker daemon. Same blob IDs reappear.
		"Copying blob aaaaaa done   |",
		"Copying blob bbbbbb done   |", // daemon doesn't have it yet
		"Copying blob cccccc done   |",
	}, "\n") + "\n"

	var sink bytes.Buffer
	lc := NewLayerCounter(&sink)
	if _, err := io.Copy(lc, strings.NewReader(input)); err != nil {
		t.Fatalf("copy: %v", err)
	}

	s := lc.Stats()
	if s.New != 2 || s.Cached != 1 {
		t.Errorf("got New=%d Cached=%d, want New=2 Cached=1 (dedup by blob ID, first-classification wins)", s.New, s.Cached)
	}
}

// TestInspectImageDebug_ParsesDockerOutput pins the parser for one
// `docker image inspect <tag> --format '{{json .}}'` response. The parser
// must accept both the JSON-array form (older docker) and single-object
// form (newer docker with --format). The function under test is the parser,
// not the docker invocation — call sites mock by passing the raw JSON.
func TestInspectImageDebug_ParsesDockerOutput(t *testing.T) {
	t.Parallel()

	// Single-object form — what `--format '{{json .}}'` produces today.
	single := `{
		"Id": "sha256:19c6e0712ca3f9c7fc8bb5ede60e4ba77b630a3c6cec78bd3f12a31d3b87e369",
		"Created": "2026-05-20T17:23:53Z",
		"Size": 29800000000,
		"RootFS": {"Layers": ["sha256:a", "sha256:b", "sha256:c", "sha256:d"]}
	}`

	info, err := parseImageInspectDebug([]byte(single))
	if err != nil {
		t.Fatalf("parse single: %v", err)
	}
	if info.ID != "sha256:19c6e0712ca3f9c7fc8bb5ede60e4ba77b630a3c6cec78bd3f12a31d3b87e369" {
		t.Errorf("ID = %q", info.ID)
	}
	if info.Created != "2026-05-20T17:23:53Z" {
		t.Errorf("Created = %q", info.Created)
	}
	if info.SizeBytes != 29800000000 {
		t.Errorf("SizeBytes = %d", info.SizeBytes)
	}
	if info.LayerCount != 4 {
		t.Errorf("LayerCount = %d, want 4", info.LayerCount)
	}

	// Array form fallback.
	arr := `[{
		"Id": "sha256:abc",
		"Created": "2026-05-20T17:23:53Z",
		"Size": 100,
		"RootFS": {"Layers": ["sha256:x"]}
	}]`
	info, err = parseImageInspectDebug([]byte(arr))
	if err != nil {
		t.Fatalf("parse array: %v", err)
	}
	if info.ID != "sha256:abc" || info.LayerCount != 1 {
		t.Errorf("array form: %+v", info)
	}
}
