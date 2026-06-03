package cli

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestParseDeliverSink(t *testing.T) {
	cases := []struct {
		in     string
		scheme string
		target string
		err    bool
	}{
		{"", "stdout", "", false},
		{"stdout", "stdout", "", false},
		{"file:/tmp/foo.json", "file", "/tmp/foo.json", false},
		{"webhook:https://example.com/hook", "webhook", "https://example.com/hook", false},
		{"webhook:http://localhost:8080/x", "webhook", "http://localhost:8080/x", false},
		// errors
		{"unknown-scheme:foo", "", "", true},
		{"file:", "", "", true},                     // empty target
		{"webhook:ftp://example.com", "", "", true}, // not http/https
		{"webhook:not-a-url", "", "", true},         // missing scheme
		{"noseparator", "", "", true},               // no `:`
	}
	for _, c := range cases {
		s, err := ParseDeliverSink(c.in)
		if (err != nil) != c.err {
			t.Errorf("ParseDeliverSink(%q): err=%v, want err=%v", c.in, err, c.err)
			continue
		}
		if err != nil {
			continue
		}
		if s.Scheme != c.scheme || s.Target != c.target {
			t.Errorf("ParseDeliverSink(%q): {%s, %s}, want {%s, %s}", c.in, s.Scheme, s.Target, c.scheme, c.target)
		}
	}
}

func TestDeliverFile_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")
	body := []byte(`{"hello":"world"}`)
	sink := DeliverSink{Scheme: "file", Target: path}
	if err := Deliver(sink, body, false); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(body) {
		t.Errorf("file body: want %q got %q", body, got)
	}
	// .tmp must not linger
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("temp file %s should be renamed away, got err=%v", path+".tmp", err)
	}
}

func TestDeliverFile_CreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested/path/out.json")
	if err := Deliver(DeliverSink{Scheme: "file", Target: path}, []byte("x"), false); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file at nested path not created: %v", err)
	}
}

func TestDeliverWebhook_Success(t *testing.T) {
	var gotBody []byte
	var gotCT, gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		gotUA = r.Header.Get("User-Agent")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	body := []byte(`{"x":1}`)
	if err := Deliver(DeliverSink{Scheme: "webhook", Target: srv.URL}, body, false); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if string(gotBody) != string(body) {
		t.Errorf("body: want %q got %q", body, gotBody)
	}
	if gotCT != "application/json" {
		t.Errorf("content-type: want application/json, got %q", gotCT)
	}
	if gotUA == "" {
		t.Error("user-agent header missing")
	}
}

func TestDeliverWebhook_CompactSwitchesContentType(t *testing.T) {
	var gotCT string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	if err := Deliver(DeliverSink{Scheme: "webhook", Target: srv.URL}, []byte("x"), true); err != nil {
		t.Fatal(err)
	}
	if gotCT != "application/x-ndjson" {
		t.Errorf("compact mode content-type: want application/x-ndjson, got %q", gotCT)
	}
}

func TestDeliverWebhook_4xxIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusBadRequest)
	}))
	defer srv.Close()
	err := Deliver(DeliverSink{Scheme: "webhook", Target: srv.URL}, []byte("x"), false)
	if err == nil {
		t.Fatal("expected error for 4xx response")
	}
}

func TestDeliver_StdoutIsNoop(t *testing.T) {
	// stdout is a no-op (the buffer has already been written by MultiWriter).
	if err := Deliver(DeliverSink{Scheme: "stdout"}, []byte("x"), false); err != nil {
		t.Errorf("stdout Deliver: %v", err)
	}
}
