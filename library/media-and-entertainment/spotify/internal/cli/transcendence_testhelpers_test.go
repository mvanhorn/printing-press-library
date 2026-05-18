// Copyright 2026 rob-coco. Licensed under Apache-2.0. See LICENSE.

// Internal-only test helpers for transcendence_test.go. Keeps the actual
// test file focused on the per-feature assertions.

package cli

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/spotify/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/spotify/internal/config"
)

// newTestClient builds a *client.Client pointed at a test server URL with
// no auth, no cache, and a short timeout.
func newTestClient(t *testing.T, baseURL string) *client.Client {
	t.Helper()
	cfg := &config.Config{BaseURL: baseURL}
	c := client.New(cfg, 5*time.Second, 0)
	c.NoCache = true
	return c
}

// jsonUnmarshal wraps json.Unmarshal so transcendence_test.go can call it
// without importing encoding/json itself (kept tidy by isolating the dep).
func jsonUnmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
