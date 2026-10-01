// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestConcurrentCookieJarWritesPreserveEveryUpdate(t *testing.T) {
	jarPath := filepath.Join(t.TempDir(), "cookies.json")
	const writers = 32
	start := make(chan struct{})
	errCh := make(chan error, writers)
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			errCh <- mergeAndWriteCookieRows(jarPath, []persistedCookie{{
				Name:   fmt.Sprintf("cookie_%02d", i),
				Value:  fmt.Sprintf("value_%02d", i),
				Domain: ".taskrabbit.com",
				Path:   "/",
			}})
		}(i)
	}
	close(start)
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("mergeAndWriteCookieRows() error: %v", err)
		}
	}

	data, err := os.ReadFile(jarPath)
	if err != nil {
		t.Fatalf("read jar: %v", err)
	}
	var rows []persistedCookie
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatalf("decode jar: %v", err)
	}
	if len(rows) != writers {
		t.Fatalf("persisted cookies = %d, want %d", len(rows), writers)
	}
	values := make(map[string]string, len(rows))
	for _, row := range rows {
		values[row.Name] = row.Value
	}
	for i := 0; i < writers; i++ {
		name := fmt.Sprintf("cookie_%02d", i)
		want := fmt.Sprintf("value_%02d", i)
		if got := values[name]; got != want {
			t.Fatalf("persisted cookie %s = %q, want %q", name, got, want)
		}
	}
}
