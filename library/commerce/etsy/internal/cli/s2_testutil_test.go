package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

// s2Run executes the real command tree and returns stdout, stderr and the error.
func s2Run(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	var flags rootFlags
	root := newRootCmd(&flags)
	var out, errb bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errb)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), errb.String(), err
}

type s2Req struct {
	Method, Path, Body string
	At                 time.Time
}

// s2Server is a recording fake Etsy API.
type s2Server struct {
	*httptest.Server
	mu   sync.Mutex
	reqs []s2Req
}

func (s *s2Server) all() []s2Req {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]s2Req(nil), s.reqs...)
}

func (s *s2Server) count(method, path string) int {
	n := 0
	for _, r := range s.all() {
		if r.Method == method && r.Path == path {
			n++
		}
	}
	return n
}

// s2NewServer starts a fake Etsy API and points the CLI at it. handler may be
// nil or return false to fall through to a 200 {} reply.
func s2NewServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request, body string) bool) *s2Server {
	t.Helper()
	s := &s2Server{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r.Body)
		s.mu.Lock()
		s.reqs = append(s.reqs, s2Req{r.Method, r.URL.Path, buf.String(), time.Now()})
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if handler != nil && handler(w, r, buf.String()) {
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(s.Close)
	t.Setenv("ETSY_BASE_URL", s.URL)
	t.Setenv("ETSY_API_KEY", "testkey:testsecret")
	t.Setenv("ETSY_SHOP_ID", "9000001")
	return s
}

// s2FixtureDB creates a store file in a temp dir and returns its path.
func s2FixtureDB(t *testing.T, fill func(db *store.Store)) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if fill != nil {
		fill(db)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func s2Must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func s2Upsert(t *testing.T, fn func(json.RawMessage) error, js string) {
	t.Helper()
	s2Must(t, fn(json.RawMessage(js)))
}

func s2Decode(t *testing.T, s string, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(s), v); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, s)
	}
}
