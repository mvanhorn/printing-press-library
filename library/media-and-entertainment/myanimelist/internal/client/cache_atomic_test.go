package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestWriteCacheIsAtomicUnderConcurrentReaders is the regression guard for the
// remove-then-write cache refresh: a reader must never observe a truncated or
// half-written body, because a partial body is returned as a successful cache
// hit and downstream JSON decoding then fails on data the CLI claims it cached
// cleanly.
func TestWriteCacheIsAtomicUnderConcurrentReaders(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	c := &Client{BaseURL: "https://example.test", cacheDir: dir}
	path := "/anime/1"

	small := json.RawMessage(`{"id":1,"title":"small"}`)
	large := json.RawMessage(`{"id":1,"title":"large","blob":"` + strings.Repeat("x", 1<<16) + `"}`)

	const writes = 300
	readers := 4

	stop := make(chan struct{})
	var wg sync.WaitGroup
	var mu sync.Mutex
	var readErr error
	var hits int
	recordErr := func(err error) {
		mu.Lock()
		defer mu.Unlock()
		if readErr == nil {
			readErr = err
		}
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(stop)
		for i := 0; i < writes; i++ {
			if i%2 == 0 {
				c.writeCache(path, nil, small, "application/json")
			} else {
				c.writeCache(path, nil, large, "application/json")
			}
		}
	}()

	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				data, _, ok := c.readCache(path, nil)
				if !ok {
					continue
				}
				mu.Lock()
				hits++
				mu.Unlock()
				if !json.Valid(data) {
					recordErr(fmt.Errorf("cache hit returned an incomplete body (%d bytes): %q", len(data), truncateRunes(string(data), 120)))
					return
				}
				var decoded map[string]any
				if err := json.Unmarshal(data, &decoded); err != nil {
					recordErr(fmt.Errorf("cache hit returned undecodable JSON: %w", err))
					return
				}
			}
		}()
	}

	wg.Wait()
	if readErr != nil {
		t.Fatal(readErr)
	}
	mu.Lock()
	observed := hits
	mu.Unlock()
	if observed == 0 {
		t.Fatal("no reader ever observed a cache hit; the write path produced no readable body")
	}

	// The last write wins: 300 alternating writes end on an odd index, i.e. the
	// large body. A reader after the writer stops must see exactly that.
	final, _, ok := c.readCache(path, nil)
	if !ok {
		t.Fatal("cache body missing after the writer finished")
	}
	if !strings.Contains(string(final), `"title":"large"`) {
		t.Fatalf("final cache body = %q, want the last written body", truncateRunes(string(final), 120))
	}

	var leftovers []string
	err := filepath.Walk(dir, func(p string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), ".tmp") {
			leftovers = append(leftovers, p)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking cache dir: %v", err)
	}
	if len(leftovers) > 0 {
		t.Fatalf("temporary cache files left behind: %v", leftovers)
	}
}

// TestWriteCacheLeavesFinalFilePrivate pins the permission behaviour the
// atomic rewrite has to preserve: the cached body stays 0600 and the resource
// directory 0700. The body path is computed from the same cache key the writer
// uses, so this cannot accidentally assert on the .meta.json sidecar that the
// contentType bookkeeping writes alongside it.
func TestWriteCacheLeavesFinalFilePrivate(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	c := &Client{BaseURL: "https://example.test", cacheDir: dir}
	path := "/anime/1"
	c.writeCache(path, nil, json.RawMessage(`{"id":1}`), "application/json")

	resourceDir := c.cacheResourceDir(path)
	cacheFile := filepath.Join(resourceDir, c.cacheKeyFor(http.MethodGet, path, nil, nil, nil)+".json")

	if _, err := os.Stat(cacheFile); err != nil {
		t.Fatalf("cache body %s was not written: %v", cacheFile, err)
	}
	body, err := os.ReadFile(cacheFile) // #nosec G304 -- test-owned temp path.
	if err != nil {
		t.Fatalf("read cache body: %v", err)
	}
	if !json.Valid(body) {
		t.Fatalf("cache body is not valid JSON: %q", body)
	}

	info, err := os.Stat(cacheFile)
	if err != nil {
		t.Fatalf("stat cache file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("cache file mode = %o, want 600", perm)
	}
	dirInfo, err := os.Stat(resourceDir)
	if err != nil {
		t.Fatalf("stat cache resource dir: %v", err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0o700 {
		t.Fatalf("cache resource dir mode = %o, want 700", perm)
	}
}
