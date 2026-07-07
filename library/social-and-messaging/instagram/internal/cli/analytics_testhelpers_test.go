package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/instagram/internal/store"
)

// newTestDB creates a temp store with the analytics schema ensured and
// returns its path. The store is closed before return so the command under
// test opens it independently (single-process WAL).
func newTestDB(t *testing.T) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "analytics.db")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("OpenWithContext: %v", err)
	}
	if err := db.EnsureAnalyticsSchema(context.Background()); err != nil {
		t.Fatalf("EnsureAnalyticsSchema: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return dbPath
}

// execTest runs a freshly-built command with the given args (always JSON +
// db) and returns parsed stdout.
func execTestJSON(t *testing.T, build func(*rootFlags) *cobra.Command, dbPath string, args ...string) map[string]any {
	t.Helper()
	flags := &rootFlags{asJSON: true}
	cmd := build(flags)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	// --json is a root persistent flag; when the subcommand is built in
	// isolation it isn't registered, so we drive JSON via flags.asJSON
	// (set above) and pass only the subcommand's own flags here.
	full := append([]string{"--db", dbPath}, args...)
	cmd.SetArgs(full)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute %v: %v\noutput: %s", full, err, buf.String())
	}
	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal output %q: %v", buf.String(), err)
	}
	return out
}

const (
	hour = time.Hour
	day  = 24 * time.Hour
	week = 7 * day
)

func rfcAgo(d time.Duration) string {
	return time.Now().UTC().Add(-d).Format(time.RFC3339)
}

// asSlice returns a top-level array field as []map[string]any.
func asSlice(t *testing.T, m map[string]any, key string) []map[string]any {
	t.Helper()
	raw, ok := m[key]
	if !ok {
		t.Fatalf("missing key %q in %v", key, m)
	}
	arr, ok := raw.([]any)
	if !ok {
		t.Fatalf("key %q is not an array: %T", key, raw)
	}
	out := make([]map[string]any, 0, len(arr))
	for _, el := range arr {
		mm, ok := el.(map[string]any)
		if !ok {
			t.Fatalf("element of %q is not an object: %T", key, el)
		}
		out = append(out, mm)
	}
	return out
}

func num(t *testing.T, m map[string]any, key string) float64 {
	t.Helper()
	v, ok := m[key]
	if !ok {
		t.Fatalf("missing numeric key %q in %v", key, m)
	}
	f, ok := v.(float64)
	if !ok {
		t.Fatalf("key %q is not numeric: %T", key, v)
	}
	return f
}

func str(t *testing.T, m map[string]any, key string) string {
	t.Helper()
	v, ok := m[key]
	if !ok {
		t.Fatalf("missing string key %q in %v", key, m)
	}
	s, ok := v.(string)
	if !ok {
		t.Fatalf("key %q is not a string: %T", key, v)
	}
	return s
}
