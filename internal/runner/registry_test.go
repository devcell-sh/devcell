package runner

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestEphemeralRegistry_StartsOnRandomPort(t *testing.T) {
	dir := t.TempDir()
	reg := &EphemeralRegistry{}
	if err := reg.Start(dir); err != nil {
		t.Fatal(err)
	}
	defer reg.Stop()

	if reg.Port == 0 {
		t.Fatal("expected non-zero port")
	}

	resp, err := http.Get(fmt.Sprintf("http://%s/v2/", reg.Addr()))
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestEphemeralRegistry_PersistsBlobsToDisk(t *testing.T) {
	dir := t.TempDir()

	reg := &EphemeralRegistry{}
	if err := reg.Start(dir); err != nil {
		t.Fatal(err)
	}

	blob := []byte("hello devcell layer")
	digest := "sha256:" + sha256Hex(blob)

	pushBlob(t, reg.Addr(), "test", blob, digest)
	reg.Stop()

	// Restart on same dir — blob must survive.
	reg2 := &EphemeralRegistry{}
	if err := reg2.Start(dir); err != nil {
		t.Fatal(err)
	}
	defer reg2.Stop()

	headURL := fmt.Sprintf("http://%s/v2/test/blobs/%s", reg2.Addr(), digest)
	req, _ := http.NewRequest("HEAD", headURL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("blob HEAD after restart: expected 200, got %d", resp.StatusCode)
	}
}

func TestEphemeralRegistry_ParallelSafe(t *testing.T) {
	dir := t.TempDir()

	reg := &EphemeralRegistry{}
	if err := reg.Start(dir); err != nil {
		t.Fatal(err)
	}
	defer reg.Stop()

	blob := []byte("parallel-test-blob-content")
	digest := "sha256:" + sha256Hex(blob)

	var wg sync.WaitGroup
	errs := make([]error, 10)
	for i := range 10 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			errs[idx] = pushBlobErr(reg.Addr(), fmt.Sprintf("repo%d", idx), blob, digest)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: %v", i, err)
		}
	}

	// Only one copy on disk (content-addressed).
	blobPath := fmt.Sprintf("%s/sha256/%s", dir, sha256Hex(blob))
	if _, err := os.Stat(blobPath); err != nil {
		t.Fatalf("expected blob on disk: %v", err)
	}
}

// --- helpers ---

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h[:])
}

func pushBlob(t *testing.T, addr, repo string, blob []byte, digest string) {
	t.Helper()
	if err := pushBlobErr(addr, repo, blob, digest); err != nil {
		t.Fatal(err)
	}
}

func pushBlobErr(addr, repo string, blob []byte, digest string) error {
	resp, err := http.Post(fmt.Sprintf("http://%s/v2/%s/blobs/uploads/", addr, repo), "", nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("upload initiate: %d", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")

	sep := "?"
	if strings.Contains(loc, "?") {
		sep = "&"
	}
	putURL := fmt.Sprintf("http://%s%s%sdigest=%s", addr, loc, sep, digest)
	req, _ := http.NewRequest("PUT", putURL, io.NopCloser(bytes.NewReader(blob)))
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("blob PUT: %d", resp.StatusCode)
	}
	return nil
}
