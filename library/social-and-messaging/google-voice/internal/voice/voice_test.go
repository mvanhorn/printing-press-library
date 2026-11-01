package voice

import (
	"context"
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/google-voice/internal/cliutil"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func threadFixture(id, cursor string) []any {
	m := make([]any, 15)
	m[0] = "m-1"
	m[1] = int64(1700000000000)
	m[4] = 10
	m[9] = "Appointment tomorrow"
	v := make([]any, 9)
	v[0] = id
	v[1] = 0
	v[2] = []any{m}
	v[3] = cursor
	v[4] = []any{[]any{"Example", "+15555550100"}}
	v[5] = []any{2}
	v[7] = true
	return v
}
func TestDecodeProtocol(t *testing.T) {
	for _, tc := range []struct {
		name   string
		raw    any
		detail bool
		bad    bool
	}{{"list", []any{[]any{threadFixture("t-1", "older")}, "next", "v1"}, false, false}, {"detail", []any{threadFixture("t-1", "older")}, true, false}, {"missing-id", []any{[]any{nil}}, true, true}, {"bad-inventory", []any{7}, false, true}} {
		t.Run(tc.name, func(t *testing.T) {
			b, _ := json.Marshal(tc.raw)
			p, e := DecodePage(b, tc.detail)
			if (e != nil) != tc.bad {
				t.Fatalf("error=%v", e)
			}
			if !tc.bad {
				m := p.Threads[0].Messages[0]
				if m.Direction != "inbound" || m.Text != "Appointment tomorrow" || m.TimestampMS != 1700000000000 || len(m.Attachments) != 0 {
					t.Fatalf("bad message %+v", m)
				}
				if p.Threads[0].Read == nil || *p.Threads[0].Read {
					t.Fatal("unread state lost")
				}
			}
		})
	}
	for _, tc := range []struct {
		raw string
		bad bool
	}{{`[["account-id"]]`, false}, {`[[]]`, true}, {`{}`, true}} {
		a, e := DecodeAccount([]byte(tc.raw))
		if (e != nil) != tc.bad {
			t.Fatal(e)
		}
		if !tc.bad && a.ID != "account-id" {
			t.Fatal(a)
		}
	}
	v := threadFixture("t-1", "")
	msgs := v[2].([]any)
	m := msgs[0].([]any)
	m[4] = 11
	m[14] = []any{"photo text", nil, []any{[]any{"image/jpeg", "attachment-id", nil, []any{[]any{1, 640, 480}, []any{2, 128, 96}}}}}
	b, _ := json.Marshal([]any{v})
	p, e := DecodePage(b, true)
	if e != nil {
		t.Fatal(e)
	}
	got := p.Threads[0].Messages[0]
	if got.Kind != "mms" || got.Direction != "outbound" || got.Text != "photo text" || got.Attachments[0].Width != 640 {
		t.Fatalf("bad MMS %+v", got)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestRemoteAndGateway(t *testing.T) {
	s := Session{Cookies: map[string]string{"SAPISID": "fake", "SID": "fake"}, PublicKey: "public-key", Headers: map[string]string{}}
	r := NewRemote(&s)
	r.HTTP.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "clients6.google.com" || !strings.HasPrefix(req.Header.Get("Authorization"), "SAPISIDHASH ") {
			t.Fatal("missing fixed-host signing")
		}
		b, _ := io.ReadAll(req.Body)
		if string(b) != "[null,1]" {
			t.Fatalf("body=%s", b)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`[["account-id"]]`))}, nil
	})
	a, e := r.Account(context.Background())
	if e != nil || a.ID != "account-id" {
		t.Fatalf("account=%+v err=%v", a, e)
	}
	s.AccountID = "other"
	if _, e = r.Account(context.Background()); e == nil {
		t.Fatal("account mismatch accepted")
	}
	for _, u := range []string{"https://evil.example/voice/v1/voiceclient/account/get", "https://clients6.google.com/voice/v1/voiceclient/sms/send", "http://clients6.google.com/voice/v1/voiceclient/account/get"} {
		req, _ := http.NewRequest("POST", u, strings.NewReader(`{}`))
		if _, e := (&Gateway{}).RoundTrip(req); e == nil {
			t.Fatal("unsafe route accepted")
		}
	}
	if got := signature("fake", time.Unix(1700000000, 0)); got != "SAPISIDHASH 1700000000_8b3a6621f4752f59e376b3b238f08213786d5783" {
		t.Fatalf("unexpected signature %s", got)
	}

	r.limiter = cliutil.NewAdaptiveLimiter(1000)
	r.HTTP.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": {"2"}}, Body: io.NopCloser(strings.NewReader(""))}, nil
	})
	if _, e = r.Account(context.Background()); e == nil {
		t.Fatal("429 suppressed")
	}
}
func TestPrivateImport(t *testing.T) {
	restore, e := cliutil.SetHomeOverride(privateTestDir(t))
	if e != nil {
		t.Fatal(e)
	}
	defer restore()
	t.Setenv("GOOGLE_VOICE_PROFILE", "test")
	for _, tc := range []struct {
		url string
		bad bool
	}{{"https://clients6.google.com/voice/v1/voiceclient/account/get?key=public-key", false}, {"https://evil.example/voice/v1/voiceclient/account/get?key=public-key", true}, {"https://clients6.google.com/voice/v1/voiceclient/sms/send?key=public-key", true}} {
		v := map[string]any{"log": map[string]any{"entries": []any{map[string]any{"request": map[string]any{"url": tc.url, "headers": []any{map[string]string{"name": "Cookie", "value": "SID=fake; SAPISID=fake; unwanted=drop"}}}, "response": map[string]int{"status": 200}}}}}
		b, _ := json.Marshal(v)
		s, e := ImportHAR(b)
		if (e != nil) != tc.bad {
			t.Fatal(e)
		}
		if !tc.bad {
			s.AccountID = "account-id"
			if e = SaveSession(s); e != nil {
				t.Fatal(e)
			}
			got, e := LoadSession()
			if e != nil || got.AccountID != "account-id" || got.Cookies["unwanted"] != "" {
				t.Fatal("import did not isolate session")
			}
		}
	}
	p, _, e := Paths()
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Chmod(p, 0644); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadPrivate(p); e == nil {
		t.Fatal("public credential file accepted")
	}
	t.Setenv("GOOGLE_VOICE_PROFILE", "../escape")
	if _, e = Profile(); e == nil {
		t.Fatal("profile traversal accepted")
	}
}
func TestMirrorTransactionsAndQueries(t *testing.T) {
	path := filepath.Join(privateTestDir(t), "messages.sqlite3")
	m, e := OpenMirror(path, "account-id", true)
	if e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	ctx := context.Background()
	m.DB.Exec("INSERT INTO voice_runs(id,started_at,status) VALUES(1,'now','running')")
	b, _ := json.Marshal([]any{threadFixture("t-1", "older")})
	p, e := DecodePage(b, true)
	if e != nil {
		t.Fatal(e)
	}
	n, e := m.commit(ctx, 1, "refresh", "t-1", "", "older", p.Threads)
	if e != nil || n != 1 {
		t.Fatalf("added=%d err=%v", n, e)
	}
	n, e = m.commit(ctx, 1, "refresh", "t-1", "", "older", p.Threads)
	if e != nil || n != 0 {
		t.Fatalf("dedup added=%d err=%v", n, e)
	}
	for _, tc := range []struct {
		q    Query
		want int
	}{{Query{Text: "appointment", Limit: 10}, 1}, {Query{Text: "appointment", FTS: true, Limit: 10}, 1}, {Query{Text: "not-present", Limit: 10}, 0}, {Query{Participant: "+15555550100", Direction: "inbound", Limit: 10}, 1}, {Query{Participant: "other", Limit: 10}, 0}, {Query{HasAttachment: true, Limit: 10}, 0}, {Query{After: "2024-01-01T00:00:00Z", Limit: 10}, 0}} {
		got, e := m.Messages(ctx, tc.q)
		if e != nil || len(got) != tc.want {
			t.Fatalf("query=%+v got=%d err=%v", tc.q, len(got), e)
		}
	}
	got, _ := m.Messages(ctx, Query{Limit: 10})
	if got[0].FirstRun != 1 || got[0].Sequence != 1 {
		t.Fatalf("immutable arrival changed: %+v", got)
	}
	bad := p.Threads[0]
	bad.ID = "different-thread"
	if _, e = m.commit(ctx, 1, "history", bad.ID, "cursor", "", []Thread{bad}); e == nil {
		t.Fatal("cross-thread ID accepted")
	}
	var count int
	m.DB.QueryRow("SELECT count(*) FROM voice_threads").Scan(&count)
	if count != 1 {
		t.Fatal("failed page was not rolled back")
	}
	c, e := m.Coverage(ctx, "+15555550100", 10)
	if e != nil || len(c.Threads) != 1 || c.Threads[0]["pending_older_cursor"] != int64(1) {
		t.Fatalf("coverage=%+v err=%v", c, e)
	}
	m.Close()
	if _, e = OpenMirror(path, "different-account", false); e == nil {
		t.Fatal("different account opened mirror")
	}
}
