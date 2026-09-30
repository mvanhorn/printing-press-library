// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/cliutil/testenv"
)

// postmarkFakeRequest is one request the fake Postmark server received.
type postmarkFakeRequest struct {
	Method      string
	Path        string
	Query       string
	ServerToken string
	AccountTok  string
	Body        string
}

// postmarkFake is an httptest Postmark stand-in. Handlers are keyed by
// "METHOD /path"; the handler receives the request and returns status + body.
type postmarkFake struct {
	t        *testing.T
	srv      *httptest.Server
	mu       sync.Mutex
	requests []postmarkFakeRequest
	handlers map[string]func(r *http.Request, body string) (int, any)
}

func newPostmarkFake(t *testing.T) *postmarkFake {
	t.Helper()
	f := &postmarkFake{t: t, handlers: map[string]func(*http.Request, string) (int, any){}}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.requests = append(f.requests, postmarkFakeRequest{
			Method:      r.Method,
			Path:        r.URL.Path,
			Query:       r.URL.RawQuery,
			ServerToken: r.Header.Get(postmarkServerTokenHeader),
			AccountTok:  r.Header.Get(postmarkAccountTokenHeader),
			Body:        string(raw),
		})
		h, ok := f.handlers[r.Method+" "+r.URL.Path]
		f.mu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"ErrorCode":404,"Message":"no fake handler for ` + r.Method + " " + r.URL.Path + `"}`))
			return
		}
		status, payload := h(r, string(raw))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		switch p := payload.(type) {
		case string:
			_, _ = w.Write([]byte(p))
		default:
			_ = json.NewEncoder(w).Encode(p)
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *postmarkFake) handle(key string, h func(r *http.Request, body string) (int, any)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.handlers[key] = h
}

func (f *postmarkFake) reply(key string, status int, payload any) {
	f.handle(key, func(*http.Request, string) (int, any) { return status, payload })
}

func (f *postmarkFake) log() []postmarkFakeRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]postmarkFakeRequest(nil), f.requests...)
}

// postmarkTestEnv isolates HOME, points the client at the fake, and clears
// cached server tokens between tests.
func postmarkTestEnv(t *testing.T, f *postmarkFake) string {
	t.Helper()
	home := testenv.Isolate(t)
	t.Setenv("POSTMARK_BASE_URL", f.srv.URL)
	t.Setenv("POSTMARK_ACCOUNT_TOKEN", "acct-token")
	t.Setenv("POSTMARK_SERVER_TOKEN", "")
	t.Setenv("POSTMARK_SERVER", "")
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	t.Setenv("PRINTING_PRESS_DOGFOOD", "")
	postmarkTokenCacheMu.Lock()
	postmarkTokenCache = map[string]postmarkServerRef{}
	postmarkTokenCacheMu.Unlock()
	return home
}

// postmarkRun executes the root command and returns stdout, stderr, and err.
func postmarkRun(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	cmd := RootCmd()
	cmd.SetArgs(append(args, "--no-cache", "--rate-limit", "0"))
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	err := cmd.Execute()
	return out.String(), errb.String(), err
}

// postmarkResults unwraps the {meta, results} envelope when present.
func postmarkResults(t *testing.T, stdout string, v any) {
	t.Helper()
	var env map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &env); err == nil {
		if inner, ok := env["results"]; ok {
			if err := json.Unmarshal(inner, v); err != nil {
				t.Fatalf("decode results: %v\n%s", err, stdout)
			}
			return
		}
	}
	if err := json.Unmarshal([]byte(stdout), v); err != nil {
		t.Fatalf("decode stdout: %v\n%s", err, stdout)
	}
}

func postmarkServersPayload(servers ...[3]any) map[string]any {
	list := make([]map[string]any, 0, len(servers))
	for _, s := range servers {
		list = append(list, map[string]any{"ID": s[0], "Name": s[1], "ApiTokens": []string{s[2].(string)}, "DeliveryType": "Live"})
	}
	return map[string]any{"TotalCount": len(list), "Servers": list}
}

func TestPostmarkParseDays(t *testing.T) {
	cases := []struct {
		in      string
		want    int
		wantErr bool
	}{
		{"7d", 7, false},
		{"4w", 28, false},
		{"36h", 2, false},
		{"1h", 1, false},
		{"0d", 0, true},
		{"soon", 0, true},
	}
	for _, tc := range cases {
		got, err := postmarkParseDays(tc.in, "window")
		if (err != nil) != tc.wantErr || got != tc.want {
			t.Errorf("postmarkParseDays(%q) = %d, %v; want %d, err=%v", tc.in, got, err, tc.want, tc.wantErr)
		}
	}
}

func TestPostmarkStatsDays(t *testing.T) {
	raw := []byte(`{"Days":[{"Date":"2026-09-01","HardBounce":2,"Transient":1},{"Date":"2026-09-03","SMTPApiError":4}],"HardBounce":2}`)
	got, err := postmarkStatsDays(raw, "", true)
	if err != nil {
		t.Fatal(err)
	}
	if got["2026-09-01"] != 3 || got["2026-09-03"] != 4 || len(got) != 2 {
		t.Fatalf("sum-all days = %v", got)
	}
	sends, err := postmarkStatsDays([]byte(`{"Days":[{"Date":"2026-09-02","Sent":"7"}],"Sent":7}`), "Sent", false)
	if err != nil || sends["2026-09-02"] != 7 {
		t.Fatalf("sends days = %v, %v", sends, err)
	}
}

func TestZeroFillDaysCountsEveryDay(t *testing.T) {
	loc := postmarkEastern()
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, loc)
	days := map[string]*postmarkDay{
		"2026-09-02": {Sent: 5},
		"2026-09-05": {Sent: 1, Bounced: 1},
		"2026-08-31": {Sent: 99}, // before the range: ignored
	}
	got := zeroFillDays(days, from, 7)
	if len(got) != 7 {
		t.Fatalf("len = %d, want 7", len(got))
	}
	if got[0].Date != "2026-09-01" || got[6].Date != "2026-09-07" {
		t.Fatalf("range = %s..%s", got[0].Date, got[6].Date)
	}
	total := 0
	for _, d := range got {
		total += d.Sent
	}
	if total != 6 || got[1].Sent != 5 || got[4].Bounced != 1 || got[2].Sent != 0 {
		t.Fatalf("zero-filled series wrong: %+v", got)
	}
}

func TestPostmarkParseTime(t *testing.T) {
	for _, s := range []string{"2026-09-30T13:35:39.0000000-04:00", "2026-09-27T14:41:41Z", "2026-09-27T10:40:19", "2026-09-27"} {
		if _, ok := postmarkParseTime(s); !ok {
			t.Errorf("postmarkParseTime(%q) failed", s)
		}
	}
	if _, ok := postmarkParseTime("not a time"); ok {
		t.Error("garbage parsed")
	}
}

func TestPostmarkServerArg(t *testing.T) {
	if got := postmarkServerArg("Main App"); got != ` --server 'Main App'` {
		t.Errorf("server arg = %q", got)
	}
	if got := postmarkServerArg("Staging"); got != " --server Staging" {
		t.Errorf("server arg = %q", got)
	}
	if postmarkServerArg("") != "" {
		t.Error("empty server should render nothing")
	}
	if got := shellQuoteWord(`a"b`); got != `'a"b'` {
		t.Errorf("quote = %s, want a single-quoted word", got)
	}
}
