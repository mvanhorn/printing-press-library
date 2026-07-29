// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/overpass/internal/subjects"

	"github.com/spf13/cobra"
)

// truncatedBody is what Overpass returns when the query hit its server-side
// timeout or memory ceiling: real elements PLUS a remark. The elements are
// usable; the answer is not complete.
const truncatedBody = `{
  "elements": [
    {"type": "node", "id": 1, "lat": 34.05, "lon": -118.24, "tags": {"name": "Angels Gate Light"}}
  ],
  "remark": "runtime error: Query timed out in \"query\" at line 3 after 25 seconds."
}`

// stubMirror points subjects.Mirrors at a single test server for the duration
// of one test and restores the real list afterwards.
func stubMirror(t *testing.T, h http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(h)
	prev := subjects.Mirrors
	subjects.Mirrors = []string{srv.URL}
	t.Cleanup(func() {
		subjects.Mirrors = prev
		srv.Close()
	})
}

// runNovel executes a novel command's RunE with the given flags, returning
// stdout and stderr separately. Separately is the point: the defect these
// tests exist for was prose landing on the stdout side.
func runNovel(t *testing.T, cmd *cobra.Command, ctx context.Context, args ...string) (string, string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetContext(ctx)
	for i := 0; i+1 < len(args); i += 2 {
		if err := cmd.Flags().Set(args[i], args[i+1]); err != nil {
			t.Fatalf("set --%s=%s: %v", args[i], args[i+1], err)
		}
	}
	err := cmd.RunE(cmd, nil)
	return out.String(), errOut.String(), err
}

// TestNearJSONParsesWhenOverpassTruncates is the planted failure for the
// stdout-corruption defect. `near --json` promises a JSON document on stdout;
// when Overpass truncates, a prose warning printed ahead of it makes every
// downstream parser fail on exactly the responses that most need reading.
func TestNearJSONParsesWhenOverpassTruncates(t *testing.T) {
	stubMirror(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(truncatedBody))
	})
	flags := rootFlags{asJSON: true, timeout: 5 * time.Second}
	stdout, stderr, err := runNovel(t, newNovelNearCmd(&flags), context.Background(),
		"latitude", "34.05", "longitude", "-118.24", "type", "lighthouse")
	if err != nil {
		t.Fatalf("near --json: %v (stderr %q)", err, stderr)
	}

	var doc map[string]any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("stdout is not parseable JSON: %v\n--- stdout ---\n%s", err, stdout)
	}
	if doc["partial"] != true {
		t.Errorf("partial = %v, want true — a truncated answer must say so inside the document", doc["partial"])
	}
	remark, _ := doc["partial_remark"].(string)
	if !strings.Contains(remark, "timed out") {
		t.Errorf("partial_remark = %q, want Overpass's own remark", remark)
	}
	// The human-facing warning still has to reach a human — on stderr.
	if !strings.Contains(stderr, "partial results") {
		t.Errorf("stderr lost the truncation warning; got %q", stderr)
	}
}

// TestGeojsonParsesWhenOverpassTruncates covers the same defect on the
// GeoJSON surface, which is the one piped into maps and GIS tools.
func TestGeojsonParsesWhenOverpassTruncates(t *testing.T) {
	stubMirror(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(truncatedBody))
	})
	flags := rootFlags{timeout: 5 * time.Second}
	stdout, stderr, err := runNovel(t, newNovelGeojsonCmd(&flags), context.Background(),
		"latitude", "34.05", "longitude", "-118.24", "type", "lighthouse")
	if err != nil {
		t.Fatalf("geojson: %v (stderr %q)", err, stderr)
	}

	var doc map[string]any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("stdout is not parseable GeoJSON: %v\n--- stdout ---\n%s", err, stdout)
	}
	if doc["type"] != "FeatureCollection" {
		t.Errorf("type = %v, want FeatureCollection", doc["type"])
	}
	if feats, ok := doc["features"].([]any); !ok || len(feats) != 1 {
		t.Errorf("features = %v, want the one element Overpass did return", doc["features"])
	}
	// The stderr warning is gone the moment the file is reopened; a truncated
	// export otherwise looks exactly like a complete one.
	if doc["partial"] != true {
		t.Errorf("partial = %v, want true carried in the document", doc["partial"])
	}
}

// TestNearHumanShowsPartialNote guards the other direction: dropping the
// stdout write must not make truncation invisible to someone reading a table.
func TestNearHumanShowsPartialNote(t *testing.T) {
	stubMirror(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(truncatedBody))
	})
	flags := rootFlags{timeout: 5 * time.Second}
	stdout, stderr, err := runNovel(t, newNovelNearCmd(&flags), context.Background(),
		"latitude", "34.05", "longitude", "-118.24", "type", "lighthouse")
	if err != nil {
		t.Fatalf("near: %v (stderr %q)", err, stderr)
	}
	if !strings.Contains(stdout, "INCOMPLETE") {
		t.Errorf("human output hid the truncation; got %q", stdout)
	}
}

// TestCompleteAnswerCarriesNoPartialKeys pins the other edge of the
// threshold: an untruncated response must not start claiming partiality, and
// must not grow a remark key that downstream readers would treat as a flag.
func TestCompleteAnswerCarriesNoPartialKeys(t *testing.T) {
	stubMirror(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"elements":[{"type":"node","id":1,"lat":34.05,"lon":-118.24,"tags":{"name":"Angels Gate Light"}}]}`))
	})
	flags := rootFlags{asJSON: true, timeout: 5 * time.Second}
	stdout, stderr, err := runNovel(t, newNovelNearCmd(&flags), context.Background(),
		"latitude", "34.05", "longitude", "-118.24", "type", "lighthouse")
	if err != nil {
		t.Fatalf("near --json: %v (stderr %q)", err, stderr)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("stdout is not parseable JSON: %v", err)
	}
	if doc["partial"] != false {
		t.Errorf("partial = %v, want false on a complete answer", doc["partial"])
	}
	if _, ok := doc["partial_remark"]; ok {
		t.Errorf("partial_remark present on a complete answer: %v", doc["partial_remark"])
	}
	if strings.Contains(stderr, "partial results") {
		t.Errorf("warned about truncation that did not happen: %q", stderr)
	}
}

// TestRunQueryStopsOnCallerCancellation is the planted failure for the
// dropped-cancellation defect. The failover loop deliberately escapes the
// caller's per-request DEADLINE so one slow mirror cannot eat the whole
// budget — but it must still stop when the caller actually hangs up, rather
// than working through every mirror for its full independent budget.
func TestRunQueryStopsOnCallerCancellation(t *testing.T) {
	const perMirror = 5 * time.Second
	// The handler never answers. It releases on `release` rather than on the
	// request context alone: a client hang-up does not always reach the
	// server side promptly, and httptest.Server.Close blocks on live handlers.
	release := make(chan struct{})
	stubMirror(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	})
	t.Cleanup(func() { close(release) }) // LIFO: runs before the server Close

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	time.AfterFunc(150*time.Millisecond, cancel)

	flags := rootFlags{asJSON: true, timeout: perMirror}
	start := time.Now()
	_, _, _, err := func() ([]subjects.Subject, []subjects.Attempt, string, error) {
		cmd := newNovelNearCmd(&flags)
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))
		cmd.SetContext(ctx)
		area := subjects.Area{Lat: 34.05, Lon: -118.24, RadiusM: 1000}
		ty, lerr := subjects.Lookup("lighthouse")
		if lerr != nil {
			t.Fatalf("Lookup: %v", lerr)
		}
		q, qerr := subjects.BuildQuery(ty, area, 25, 10)
		if qerr != nil {
			t.Fatalf("BuildQuery: %v", qerr)
		}
		return runQuery(boundCtxFor(ctx, &flags), cmd, &flags, q, &area)
	}()
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("cancelled query returned success")
	}
	// Without cancellation propagation this runs the full independent budget
	// (len(Mirrors) * perMirror), so the margin is wide on purpose.
	if elapsed > 2*time.Second {
		t.Errorf("cancellation took %v to stop the failover loop; caller hang-up is not reaching the request", elapsed)
	}
}

// boundCtxFor mirrors what every novel command does before calling runQuery.
func boundCtxFor(parent context.Context, flags *rootFlags) context.Context {
	ctx, cancel := boundCtx(parent, flags)
	_ = cancel
	return ctx
}
