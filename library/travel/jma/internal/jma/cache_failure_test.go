package jma

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Synthetic source and filesystem fixtures exercise cache failure deterministically.
func TestCacheFailurePreservesLiveResult(t *testing.T) {
	for _, mode := range []string{"write", "cleanup"} {
		t.Run(mode, func(t *testing.T) {
			c := testClient(t, fixtureTransport{"/bosai/x.json": `{"ok":true}`})
			c.NoCache = false
			httpDir := filepath.Join(c.CacheDir, "http")
			if mode == "write" {
				if err := os.WriteFile(httpDir, []byte("blocks the cache directory"), 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.Mkdir(httpDir, 0700); err != nil {
					t.Fatal(err)
				}
				for i := 0; i < 513; i++ {
					if err := os.WriteFile(filepath.Join(httpDir, fmt.Sprintf("fixture-%03d.json", i)), []byte("{}"), 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			var result map[string]any
			if err := c.Get(context.Background(), "/x.json", time.Minute, &result); err != nil {
				t.Fatal("valid live result discarded:", err)
			}
			envelope := c.Envelope(result, "complete", "source interpretation")
			if result["ok"] != true || c.requests != 1 || len(envelope.Meta.Sources) != 1 || envelope.Meta.Sources[0].Cache {
				t.Fatalf("live result or provenance lost: %+v", envelope)
			}
			if c.NoCache || !c.skipCacheWrites || len(envelope.Meta.Notes) != 2 || envelope.Meta.Notes[0] != "source interpretation" {
				t.Fatalf("cache failure not disclosed or disabled: %+v", envelope.Meta)
			}
			if mode == "cleanup" {
				entries, err := os.ReadDir(httpDir)
				if err != nil || len(entries) != 513 {
					t.Fatalf("failed cleanup grew the cache: %d entries, %v", len(entries), err)
				}
			}
			c.Offline = true
			if err := c.Get(context.Background(), "/x.json", time.Minute, &result); err == nil || c.requests != 1 {
				t.Fatal("unpersisted result became an offline cache hit")
			}
		})
	}
}

func TestCacheFailurePreservesFreshCacheReads(t *testing.T) {
	fixtures := fixtureTransport{"/bosai/x.json": `{"live":true}`, "/bosai/y.json": `{"cached":true}`}
	c := testClient(t, fixtures)
	c.NoCache = false
	var result map[string]any
	if err := c.Get(context.Background(), "/y.json", time.Minute, &result); err != nil {
		t.Fatal(err)
	}
	delete(fixtures, "/bosai/y.json") // The cached document is unavailable on the network.
	httpDir := filepath.Join(c.CacheDir, "http")
	for i := 0; i < 513; i++ {
		if err := os.WriteFile(filepath.Join(httpDir, fmt.Sprintf("fixture-%03d.json", i)), []byte("{}"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.Get(context.Background(), "/x.json", time.Minute, &result); err != nil {
		t.Fatal(err)
	}
	if !c.skipCacheWrites || c.requests != 2 {
		t.Fatal("fixture did not cause a cache cleanup failure")
	}
	for _, offline := range []bool{false, true} {
		c.Offline = offline
		result = nil
		if err := c.Get(context.Background(), "/y.json", time.Minute, &result); err != nil {
			t.Fatal("fresh cached document lost after write failure:", err)
		}
		if result["cached"] != true || c.requests != 2 || !c.sources[len(c.sources)-1].Cache {
			t.Fatalf("fresh cache was bypassed: %+v", c.Envelope(result, "complete"))
		}
	}
}
