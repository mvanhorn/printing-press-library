// Copyright 2026 Ryan Gravette and contributors. Licensed under Apache-2.0. See LICENSE.
// Behavioral tests for Dawn concept parsing + aggregation (powers course stats/tree).

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/agilix-dawn/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/agilix-dawn/internal/config"
)

// synthetic concept fixture — no real data.
const sampleConcept = `{
  "id": "c_test",
  "title": "Test Course",
  "status": "enabled",
  "section": [
    {"id":"s1","title":"Sec One","status":"enabled","instruction":[
      {"id":"i1","title":"Intro","type":"video","duration":100,"points":5,"interaction":[{"id":"x1"},{"id":"x2"}]},
      {"id":"i2","title":"Quiz","type":"survey","duration":50,"points":0,"interaction":[{"id":"x3"}]}
    ]},
    {"id":"s2","title":"Sec Two","status":"enabled","instruction":[
      {"id":"i3","title":"Lesson","type":"activity","duration":200,"points":10,"interaction":[]}
    ]}
  ]
}`

func TestConceptParseAndAggregate(t *testing.T) {
	var c dawnConcept
	if err := json.Unmarshal([]byte(sampleConcept), &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if c.ID != "c_test" || c.Title != "Test Course" {
		t.Fatalf("bad top-level parse: %+v", c)
	}
	if len(c.Section) != 2 {
		t.Fatalf("sections = %d, want 2", len(c.Section))
	}
	var instr, inter, points int
	var dur float64
	for _, s := range c.Section {
		instr += len(s.Instruction)
		for _, in := range s.Instruction {
			inter += in.interactionCount()
			points += in.Points
			dur += in.Duration
		}
	}
	if instr != 3 {
		t.Errorf("instructions = %d, want 3", instr)
	}
	if inter != 3 {
		t.Errorf("interactions = %d, want 3", inter)
	}
	if points != 15 {
		t.Errorf("points = %d, want 15", points)
	}
	if dur != 350 {
		t.Errorf("duration = %v, want 350", dur)
	}
}

func TestInteractionCount(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want int
	}{
		{"none", `{"id":"i","interaction":[]}`, 0},
		{"two", `{"id":"i","interaction":[{"id":"a"},{"id":"b"}]}`, 2},
		{"missing", `{"id":"i"}`, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var in dawnInstruction
			if err := json.Unmarshal([]byte(tc.raw), &in); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got := in.interactionCount(); got != tc.want {
				t.Errorf("interactionCount() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestEnvelopeParse(t *testing.T) {
	raw := `{"totalMatches":2,"matches":[{"id":"a"},{"id":"b"}]}`
	var env dawnEnvelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.TotalMatches != 2 {
		t.Errorf("totalMatches = %d, want 2", env.TotalMatches)
	}
	if len(env.Matches) != 2 {
		t.Errorf("matches = %d, want 2", len(env.Matches))
	}
}

// pagingServer returns a fake Dawn search endpoint holding `total` synthetic
// records, honoring the start/limit window in the search DSL. It lets tests
// prove fetchAllSearch walks past a single page instead of truncating.
func pagingServer(t *testing.T, total int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var q struct {
			Start int `json:"start"`
			Limit int `json:"limit"`
		}
		if err := json.Unmarshal([]byte(r.URL.Query().Get("search")), &q); err != nil {
			t.Errorf("server: bad search param %q: %v", r.URL.Query().Get("search"), err)
		}
		matches := []json.RawMessage{}
		for i := q.Start; i < q.Start+q.Limit && i < total; i++ {
			matches = append(matches, json.RawMessage(fmt.Sprintf(`{"id":"r_%d"}`, i)))
		}
		body, _ := json.Marshal(map[string]any{"totalMatches": total, "matches": matches})
		_, _ = w.Write(body)
	}))
}

// TestFetchAllSearchPagesPastOnePage is the core anti-truncation guarantee:
// a resource larger than one page must come back complete, with the server
// total reported so callers can detect a cap hit. A regression here reinstates
// the silent-truncation bug the helper exists to kill.
func TestFetchAllSearchPagesPastOnePage(t *testing.T) {
	const total = searchPageSize*2 + 37
	srv := pagingServer(t, total)
	defer srv.Close()
	c := client.New(&config.Config{BaseURL: srv.URL}, time.Second, 0)

	got, reported, err := fetchAllSearch(context.Background(), c, "concept", nil)
	if err != nil {
		t.Fatalf("fetchAllSearch error = %v", err)
	}
	if reported != total {
		t.Errorf("reported total = %d, want %d", reported, total)
	}
	if len(got) != total {
		t.Fatalf("collected %d records, want %d (truncated?)", len(got), total)
	}
	// Records must be distinct across pages (no window overlap or gap).
	seen := map[string]bool{}
	for _, m := range got {
		var e struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(m, &e)
		if seen[e.ID] {
			t.Fatalf("duplicate record across pages: %s", e.ID)
		}
		seen[e.ID] = true
	}
}

// TestWarnTruncatedOnlyWhenShort pins the operator-facing signal: a warning
// fires when fewer records came back than the server reported, and stays
// silent when the fetch is complete.
func TestWarnTruncatedOnlyWhenShort(t *testing.T) {
	var short bytes.Buffer
	warnTruncated(&short, "user", 1000, 1200)
	if !bytes.Contains(short.Bytes(), []byte("truncated")) {
		t.Errorf("expected truncation warning, got %q", short.String())
	}
	var complete bytes.Buffer
	warnTruncated(&complete, "user", 1200, 1200)
	if complete.Len() != 0 {
		t.Errorf("expected no warning when complete, got %q", complete.String())
	}
}
