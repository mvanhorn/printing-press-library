// Copyright 2026 gregce. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// PATCH(library): tests for parseMistakesSSE + analyzeMistakes. Pins the
// real wire shape captured 2026-05-16 (data: ["Mistakes", [{...}, ...]]
// repeated per event, terminated by EOF on the response body). Cataloged
// in .printing-press-patches.json#add-cut-panel-parity.

func TestParseMistakesSSE_LiveWireShape(t *testing.T) {
	// Sample drawn verbatim from the captured HAR. Each event arrives on
	// its own `data:` line, separated by blank lines.
	stream := strings.NewReader(`data: ["Mistakes",[{"trim":{"startTime":93820.0,"duration":280.0},"reasoning":"Early terminated sentence","wordsToCut":"So I'm basically.","confidence":1.0,"rawStart":"01:33.820","rawEnd":"01:34.100"}]]

data: ["Mistakes",[{"trim":{"startTime":123420.0,"duration":400.0},"reasoning":"Early terminated sentence","wordsToCut":"So,","confidence":1.0,"rawStart":"02:03.420","rawEnd":"02:03.820"}]]

data: ["Mistakes",[{"trim":{"startTime":132870.0,"duration":880.0},"reasoning":"False start","wordsToCut":"So the cli,","confidence":1.0,"rawStart":"02:12.870","rawEnd":"02:13.750"}]]
`)
	mistakes, unknown, err := parseMistakesSSE(stream)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if unknown != 0 {
		t.Errorf("expected 0 unknown events, got %d", unknown)
	}
	if len(mistakes) != 3 {
		t.Fatalf("got %d mistakes, want 3", len(mistakes))
	}
	if mistakes[0].WordsToCut != "So I'm basically." {
		t.Errorf("mistakes[0].WordsToCut = %q, want %q", mistakes[0].WordsToCut, "So I'm basically.")
	}
	if mistakes[2].Trim.StartTime != 132870.0 || mistakes[2].Trim.Duration != 880.0 {
		t.Errorf("mistakes[2].Trim = %+v, want {132870, 880}", mistakes[2].Trim)
	}
}

func TestParseMistakesSSE_MultipleMistakesPerEvent(t *testing.T) {
	// Some events carry multiple mistakes in a single batch.
	stream := strings.NewReader(`data: ["Mistakes",[{"trim":{"startTime":1000,"duration":100},"wordsToCut":"a","confidence":1.0},{"trim":{"startTime":2000,"duration":200},"wordsToCut":"b","confidence":0.95}]]
`)
	got, unknown, err := parseMistakesSSE(stream)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if unknown != 0 || len(got) != 2 {
		t.Fatalf("got %d mistakes, %d unknown; want 2 mistakes, 0 unknown", len(got), unknown)
	}
}

func TestParseMistakesSSE_BareMistakesArray(t *testing.T) {
	stream := strings.NewReader(`event: Mistakes
data: [{"trim":{"startTime":1000,"duration":100},"wordsToCut":"a","confidence":1.0},{"trim":{"startTime":2000,"duration":200},"wordsToCut":"b","confidence":0.95}]
`)
	got, unknown, err := parseMistakesSSE(stream)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if unknown != 0 || len(got) != 2 {
		t.Fatalf("got %d mistakes, %d unknown; want 2 mistakes, 0 unknown", len(got), unknown)
	}
	if got[1].WordsToCut != "b" {
		t.Fatalf("got[1].WordsToCut = %q, want b", got[1].WordsToCut)
	}
}

func TestParseMistakesSSE_IgnoresNonMistakesEventTypes(t *testing.T) {
	stream := strings.NewReader(`data: ["KeepAlive", {}]

data: ["Mistakes",[{"trim":{"startTime":500,"duration":50},"confidence":0.9}]]

data: ["Done", {}]
`)
	mistakes, unknown, err := parseMistakesSSE(stream)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mistakes) != 1 {
		t.Fatalf("got %d mistakes, want 1", len(mistakes))
	}
	if unknown != 2 {
		t.Errorf("expected 2 unknown events (KeepAlive + Done), got %d", unknown)
	}
}

func TestParseMistakesSSE_SkipsMalformedMistakesBatch(t *testing.T) {
	stream := strings.NewReader(`data: ["Mistakes",[{"trim":{"startTime":100,"duration":50},"confidence":1.0}]]

data: ["Mistakes",{"not":"an array"}]

data: ["Mistakes",[{"trim":{"startTime":200,"duration":75},"confidence":1.0}]]
`)
	mistakes, unknown, err := parseMistakesSSE(stream)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mistakes) != 2 {
		t.Fatalf("got %d mistakes, want 2 valid mistakes despite malformed middle batch", len(mistakes))
	}
	if unknown != 1 {
		t.Fatalf("unknown = %d, want 1 malformed batch", unknown)
	}
}

func TestParseMistakesSSE_SkipsBlankAndCommentLines(t *testing.T) {
	stream := strings.NewReader(`:heartbeat

data: ["Mistakes",[{"trim":{"startTime":100,"duration":50},"confidence":1.0}]]

`)
	got, _, err := parseMistakesSSE(stream)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d mistakes, want 1", len(got))
	}
}

func TestPostSSEAndParseMistakesSSE_HappyPath(t *testing.T) {
	// Stand up a fake unofficial AI service that returns a deterministic
	// SSE stream. Verifies the lower-level postSSE + parseMistakesSSE
	// wiring before analyzeMistakes adds its URL/body construction.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ai-mistakes/analyze-scene" {
			http.Error(w, "wrong path", http.StatusNotFound)
			return
		}
		if r.Header.Get("Cookie") == "" {
			http.Error(w, "missing cookie", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		// Two mistakes, deterministic.
		_, _ = w.Write([]byte(`data: ["Mistakes",[{"trim":{"startTime":1000,"duration":100},"wordsToCut":"um","confidence":0.99}]]

data: ["Mistakes",[{"trim":{"startTime":5000,"duration":250},"wordsToCut":"like","confidence":0.97}]]
`))
	}))
	defer srv.Close()

	uc := &unofficialClient{
		http:   srv.Client(),
		cookie: "fake-session=1",
	}
	stream, status, err := uc.postSSE(srv.URL+"/ai-mistakes/analyze-scene", map[string]any{
		"storyID": "vid_abc",
		"sceneID": "cl_xyz",
	})
	if err != nil {
		t.Fatalf("postSSE: %v (status=%d)", err, status)
	}
	defer stream.Close()
	mistakes, _, perr := parseMistakesSSE(stream)
	if perr != nil {
		t.Fatalf("parseMistakesSSE: %v", perr)
	}
	if status != http.StatusOK {
		t.Errorf("status = %d, want 200", status)
	}
	if len(mistakes) != 2 {
		t.Fatalf("got %d mistakes, want 2", len(mistakes))
	}
}

func TestAnalyzeMistakes_HappyPath(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ai-mistakes/analyze-scene" {
			http.Error(w, "wrong path", http.StatusNotFound)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`data: ["Mistakes",[{"trim":{"startTime":5000,"duration":250},"wordsToCut":"like","confidence":0.97}]]
`))
	}))
	defer srv.Close()

	uc := &unofficialClient{
		http:      srv.Client(),
		cookie:    "fake-session=1",
		aiBaseURL: srv.URL,
	}
	mistakes, unknown, status, err := analyzeMistakes(uc, "vid_abc", "cl_xyz")
	if err != nil {
		t.Fatalf("analyzeMistakes: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if unknown != 0 {
		t.Fatalf("unknown = %d, want 0", unknown)
	}
	if gotBody["storyID"] != "vid_abc" || gotBody["sceneID"] != "cl_xyz" {
		t.Fatalf("request body = %+v, want storyID=vid_abc sceneID=cl_xyz", gotBody)
	}
	if len(mistakes) != 1 || mistakes[0].WordsToCut != "like" {
		t.Fatalf("mistakes = %+v, want one parsed mistake for like", mistakes)
	}
}

func TestUnofficialClient_RefusesEmptyCookie(t *testing.T) {
	_, err := newUnofficialClient("", 5)
	if err == nil {
		t.Fatal("expected error for empty cookie, got nil")
	}
	if !strings.Contains(err.Error(), "TELLA_SESSION_COOKIE") {
		t.Errorf("error should mention TELLA_SESSION_COOKIE; got: %v", err)
	}
}

func TestUnofficialClient_EnforcesMinimumSSETimeout(t *testing.T) {
	uc, err := newUnofficialClient("session=abc", 30*time.Second)
	if err != nil {
		t.Fatalf("newUnofficialClient: %v", err)
	}
	if uc.http.Timeout != 60*time.Second {
		t.Fatalf("timeout = %s, want 1m0s minimum for SSE streams", uc.http.Timeout)
	}

	uc, err = newUnofficialClient("session=abc", 90*time.Second)
	if err != nil {
		t.Fatalf("newUnofficialClient: %v", err)
	}
	if uc.http.Timeout != 90*time.Second {
		t.Fatalf("timeout = %s, want caller-provided timeout above minimum", uc.http.Timeout)
	}
}

func TestUnofficialClient_PostSSESendsCookieAndOrigin(t *testing.T) {
	var gotCookie, gotOrigin, gotReferer string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCookie = r.Header.Get("Cookie")
		gotOrigin = r.Header.Get("Origin")
		gotReferer = r.Header.Get("Referer")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: [\"KeepAlive\", {}]\n"))
	}))
	defer srv.Close()

	uc, err := newUnofficialClient("session=abc; XSRF-TOKEN=xyz", 5)
	if err != nil {
		t.Fatalf("newUnofficialClient: %v", err)
	}
	uc.http = srv.Client()

	stream, _, err := uc.postSSE(srv.URL+"/", map[string]any{})
	if err != nil {
		t.Fatalf("postSSE: %v", err)
	}
	stream.Close()

	if gotCookie != "session=abc; XSRF-TOKEN=xyz" {
		t.Errorf("Cookie = %q, want %q", gotCookie, "session=abc; XSRF-TOKEN=xyz")
	}
	if gotOrigin != unofficialFrontendHost {
		t.Errorf("Origin = %q, want %q", gotOrigin, unofficialFrontendHost)
	}
	if !strings.HasPrefix(gotReferer, unofficialFrontendHost) {
		t.Errorf("Referer = %q, want prefix %q", gotReferer, unofficialFrontendHost)
	}
}

func TestUnofficialClient_PostSSESurfaces401WithHelpfulMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"not_authenticated","description":"..."}`, http.StatusUnauthorized)
	}))
	defer srv.Close()

	uc, _ := newUnofficialClient("fake=1", 5)
	uc.http = srv.Client()

	_, status, err := uc.postSSE(srv.URL+"/", map[string]any{})
	if status != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", status)
	}
	if err == nil || !strings.Contains(err.Error(), "session cookie expired") {
		t.Errorf("error should mention 'session cookie expired'; got: %v", err)
	}
}

func TestUnofficialClient_PostSSEClosesNon2xxBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	uc, _ := newUnofficialClient("fake=1", 5)
	uc.http = srv.Client()

	stream, status, err := uc.postSSE(srv.URL+"/", map[string]any{})
	if stream != nil {
		t.Fatal("expected nil stream for non-2xx response")
	}
	if status != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", status)
	}
	if err == nil || !strings.Contains(err.Error(), "service unavailable") {
		t.Errorf("error should include drained response body; got: %v", err)
	}
}
