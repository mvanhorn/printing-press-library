// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package uberjobs

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/store"
)

// Shared helpers for the uberjobs tests. Every HTTP case runs against a
// loopback httptest server; zz_netguard_test.go refuses any other dial.

// ujT0 is a fixed reference instant so stored stamps are exact strings.
var ujT0 = time.Date(2026, 10, 5, 8, 0, 0, 0, time.UTC)

// ujHit is one request as the fake server saw it.
type ujHit struct {
	Method   string
	Path     string
	RawQuery string
	Header   http.Header
	Body     []byte
}

// ujFake is a loopback server that records every request it receives.
type ujFake struct {
	*httptest.Server
	mu   sync.Mutex
	hits []ujHit
}

func ujNewFake(t *testing.T, h http.HandlerFunc) *ujFake {
	t.Helper()
	f := &ujFake{}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.hits = append(f.hits, ujHit{Method: r.Method, Path: r.URL.Path, RawQuery: r.URL.RawQuery, Header: r.Header.Clone(), Body: body})
		f.mu.Unlock()
		r.Body = io.NopCloser(bytes.NewReader(body))
		h(w, r)
	}))
	t.Cleanup(f.Close)
	return f
}

func (f *ujFake) Hits() []ujHit {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]ujHit(nil), f.hits...)
}

func (f *ujFake) Count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.hits)
}

// Host is the host:port the client keys refusals and latches by.
func (f *ujFake) Host() string {
	u, err := url.Parse(f.URL)
	if err != nil {
		panic(err)
	}
	return u.Host
}

// ujClient builds a client with no gate file, no limiter, and no request
// log, so tests run fast and never write to a ledger from the environment.
func ujClient(t *testing.T, base string) *Client {
	t.Helper()
	c := NewClient(base, 5*time.Second, "", "")
	c.Limiter = nil
	c.RequestLog = ""
	return c
}

// ujStateClient is ujClient with a state dir, so refusal latches are written
// and read, but with the gate disabled so a test never waits 3 s.
func ujStateClient(t *testing.T, base, stateDir string) *Client {
	t.Helper()
	c := NewClient(base, 5*time.Second, stateDir, "")
	c.Limiter = nil
	c.RequestLog = ""
	c.Gate = NewGate("")
	return c
}

func ujReadTestdata(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading testdata/%s: %v", name, err)
	}
	return b
}

// ujCorpusRows returns the 56 captured search rows as raw JSON.
func ujCorpusRows(t *testing.T) []json.RawMessage {
	t.Helper()
	var env struct {
		Jobs []json.RawMessage `json:"jobs"`
	}
	if err := json.Unmarshal(ujReadTestdata(t, "search_corpus.json"), &env); err != nil {
		t.Fatalf("decoding search_corpus.json: %v", err)
	}
	if len(env.Jobs) != 56 {
		t.Fatalf("search_corpus.json has %d rows, want 56", len(env.Jobs))
	}
	return env.Jobs
}

func ujCorpusRaw(t *testing.T) []RawPosting {
	t.Helper()
	rows := ujCorpusRows(t)
	out := make([]RawPosting, 0, len(rows))
	for i, r := range rows {
		var p RawPosting
		if err := json.Unmarshal(r, &p); err != nil {
			t.Fatalf("row %d does not decode: %v", i, err)
		}
		out = append(out, p)
	}
	return out
}

func ujCorpusByID(t *testing.T) map[string]Posting {
	t.Helper()
	out := map[string]Posting{}
	for _, r := range ujCorpusRaw(t) {
		p := Normalize(r, "")
		out[p.ID] = p
	}
	return out
}

func ujSearchJSON(rows []json.RawMessage, total int) []byte {
	if rows == nil {
		rows = []json.RawMessage{}
	}
	b, _ := json.Marshal(map[string]any{"jobs": rows, "totalJobs": total, "totalPages": 1, "page": 1, "pageSize": len(rows)})
	return b
}

// ujSearchServer answers /api/jobs/search/ like the site: the probe
// (pagesize=1) gets the first row, any other page size gets up to that many
// rows. total is reported as totalJobs on every reply.
func ujSearchServer(t *testing.T, rows []json.RawMessage, total int) *ujFake {
	t.Helper()
	return ujNewFake(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != searchPath {
			http.NotFound(w, r)
			return
		}
		size, _ := strconv.Atoi(r.URL.Query().Get("pagesize"))
		if size < 1 {
			size = 10
		}
		page := rows
		if len(page) > size {
			page = page[:size]
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(ujSearchJSON(page, total))
	})
}

// ujOpenDB opens a temp-file SQLite store through the module's store
// package and adds the uj_ tables.
func ujOpenDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()
	st, err := store.OpenWithContext(ctx, filepath.Join(t.TempDir(), "uj.db"))
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := EnsureSchema(ctx, st.DB()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	return st.DB()
}

// ujS renders a nullable string for messages and comparisons.
func ujS(p *string) string {
	if p == nil {
		return "<nil>"
	}
	return *p
}

func ujStrp(s string) *string { return &s }

func ujStamp(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// ujQueryValues parses a raw query that may hold literal ';' (Oracle
// finders), which url.ParseQuery would drop.
func ujQueryValues(raw string) map[string][]string {
	out := map[string][]string{}
	for _, part := range strings.Split(raw, "&") {
		if part == "" {
			continue
		}
		k, v, _ := strings.Cut(part, "=")
		out[k] = append(out[k], v)
	}
	return out
}
