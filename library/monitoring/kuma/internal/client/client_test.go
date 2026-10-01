package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// enc frames one engine.io record with its byte-length prefix.
func rec(s string) string { return itoa(len([]byte(s))) + ":" + s }

var emitIDRe = regexp.MustCompile(`^42(\d+)\[`)

// fakeKuma speaks the subset of engine.io v4 + Socket.IO v4 that Uptime
// Kuma v2 uses over HTTP long-polling.
type fakeKuma struct {
	srv *httptest.Server

	mu        sync.Mutex
	emits     []string // raw POST bodies ("421[...]" etc.)
	nsConnect bool
	queue     []string // framed records served on subsequent sid-polls
	failAuth  bool
	pollN     int
}

func newFakeKuma(t *testing.T) *fakeKuma {
	t.Helper()
	f := &fakeKuma{}
	mux := http.NewServeMux()
	mux.HandleFunc("/socket.io/", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("EIO") != "4" || q.Get("transport") != "polling" {
			http.Error(w, "bad transport", http.StatusBadRequest)
			return
		}
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
			if q.Get("sid") == "" {
				hs := []byte(`{"sid":"SID123","upgrades":["websocket"],"pingInterval":25000,"pingTimeout":20000,"maxPayload":1000000}`)
				open := append([]byte{'0'}, hs...)
				w.Write([]byte(rec(string(open))))
				return
			}
			f.mu.Lock()
			f.pollN++
			var out string
			if f.pollN == 1 {
				out = rec(`40{"sid":"NS"}`) // namespace connect includes sid
			} else {
				out = strings.Join(f.queue, "")
				f.queue = nil
			}
			f.mu.Unlock()
			io.WriteString(w, out)
		case http.MethodPost:
			body, _ := io.ReadAll(r.Body)
			f.mu.Lock()
			f.emits = append(f.emits, string(body))
			if string(body) == "40" {
				f.nsConnect = true
			}
			if f.failAuth && strings.Contains(string(body), `"login"`) {
				id := "1"
				if m := emitIDRe.FindSubmatch(body); m != nil {
					id = string(m[1])
				}
				f.queue = append(f.queue,
					rec(`43`+id+`[{"ok":false,"message":"Invalid credentials","token":null}]`))
			}
			f.mu.Unlock()
			io.WriteString(w, "ok")
		default:
			http.Error(w, "method", http.StatusMethodNotAllowed)
		}
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeKuma) enqueue(records ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range records {
		f.queue = append(f.queue, rec(r))
	}
}

func (f *fakeKuma) emitCount(event string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, e := range f.emits {
		if strings.Contains(e, `"`+event+`"`) {
			n++
		}
	}
	return n
}

func newClient(f *fakeKuma) *Client {
	return New(Config{BaseURL: f.srv.URL, Username: "u", Password: "p"})
}

func loginClient(t *testing.T, ctx context.Context, f *fakeKuma) *Client {
	t.Helper()
	c := newClient(f)
	if err := c.connect(ctx); err != nil {
		t.Fatalf("connect: %v", err)
	}
	f.enqueue(`431[{"ok":true,"token":"jwt-test"}]`)
	if err := c.login(ctx); err != nil {
		t.Fatalf("login: %v", err)
	}
	return c
}

func TestHandshakeExtractsSession(t *testing.T) {
	f := newFakeKuma(t)
	c := newClient(f)
	if err := c.connect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if c.SessionID() != "SID123" {
		t.Fatalf("sid = %q", c.SessionID())
	}
}

func TestNamespaceConnectPacket(t *testing.T) {
	f := newFakeKuma(t)
	c := newClient(f)
	if err := c.connect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	f.mu.Lock()
	connected := f.nsConnect
	f.mu.Unlock()
	if !connected {
		t.Fatal("client did not send Socket.IO namespace-connect packet")
	}
}

func TestLoginMarksAuthenticated(t *testing.T) {
	f := newFakeKuma(t)
	c := loginClient(t, context.Background(), f)
	if !c.Authenticated() {
		t.Fatal("client should be authenticated")
	}
	if f.emitCount("login") != 1 {
		t.Fatalf("expected exactly one login emit, got %d", f.emitCount("login"))
	}
}

func TestLoginBadCredentialsIsAuthError(t *testing.T) {
	f := newFakeKuma(t)
	f.failAuth = true
	c := newClient(f)
	ctx := context.Background()
	if err := c.connect(ctx); err != nil {
		t.Fatalf("connect: %v", err)
	}
	err := c.login(ctx)
	if err == nil {
		t.Fatal("expected auth error")
	}
	if _, isAuth := err.(*AuthError); !isAuth {
		t.Fatalf("expected *AuthError, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "Invalid credentials") {
		t.Fatalf("error should carry server message, got: %v", err)
	}
}

func TestEmitSendsSocketIOFrameAndParsesAck(t *testing.T) {
	f := newFakeKuma(t)
	c := loginClient(t, context.Background(), f)
	f.enqueue(`432{"ok":true}`)
	ack, err := c.CallRaw(context.Background(), "health", nil)
	if err != nil {
		t.Fatalf("CallRaw: %v", err)
	}
	if !strings.Contains(string(ack), `"ok":true`) {
		t.Fatalf("unexpected ack: %s", ack)
	}
	if f.emitCount("health") != 1 {
		t.Fatal("expected health emit recorded")
	}
}

func TestPushFallbackAfterBareAck(t *testing.T) {
	f := newFakeKuma(t)
	c := loginClient(t, context.Background(), f)
	// getMonitorList acks bare ok:true; payload rides the monitorList push.
	f.enqueue(
		`432{"ok":true}`,
		`42["monitorList",{"25":{"id":25,"name":"web","type":"http"}}]`,
	)
	raw, err := c.CallWithPushFallback(context.Background(), "getMonitorList", nil, "monitorList", 3*time.Second)
	if err != nil {
		t.Fatalf("CallWithPushFallback: %v", err)
	}
	if !strings.Contains(string(raw), `"web"`) {
		t.Fatalf("expected monitorList payload, got: %s", raw)
	}
}

func TestFullPayloadAckAcceptedWithoutPush(t *testing.T) {
	f := newFakeKuma(t)
	c := loginClient(t, context.Background(), f)
	// Ack carries ok:true AND the payload -> returned directly, no push needed.
	f.mu.Lock()
	f.queue = nil
	f.queue = append(f.queue, rec(`432{"ok":true,"monitorList":{"7":{"id":7}}}`))
	f.mu.Unlock()
	raw, err := c.CallWithPushFallback(context.Background(), "getMonitorList", nil, "monitorList", time.Second)
	if err != nil {
		t.Fatalf("CallWithPushFallback: %v", err)
	}
	if !strings.Contains(string(raw), `"id":7`) {
		t.Fatalf("expected full-payload ack, got: %s", raw)
	}
}

func TestFramedPingProducesPong(t *testing.T) {
	f := newFakeKuma(t)
	loginClient(t, context.Background(), f)
	f.enqueue(`2`)
	time.Sleep(100 * time.Millisecond)
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, body := range f.emits {
		if body == "3" {
			return
		}
	}
	t.Fatalf("reader did not answer framed PING: %#v", f.emits)
}

func TestRepeatedNamespaceConnectDoesNotPanic(t *testing.T) {
	f := newFakeKuma(t)
	c := newClient(f)
	if err := c.connect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	f.enqueue(`40`, `40`)
	time.Sleep(100 * time.Millisecond)
}

func TestHeartbeatCollectionKeepsMultiplePushes(t *testing.T) {
	f := newFakeKuma(t)
	c := loginClient(t, context.Background(), f)
	f.enqueue(
		`42["heartbeatList",{"25":[{"monitorID":25,"status":1}]}]`,
		`42["heartbeatList",{"43":[{"monitorID":43,"status":0}]}]`,
	)
	raw, err := c.CallWithPushFallback(context.Background(), "getHeartbeats", nil, "heartbeatList", time.Second)
	if err != nil {
		t.Fatalf("heartbeat collection: %v", err)
	}
	if !strings.Contains(string(raw), `"25"`) || !strings.Contains(string(raw), `"43"`) {
		t.Fatalf("collection lost a streamed payload: %s", raw)
	}
}

func TestHTTPClientTimeoutBoundsHandshake(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()
	c := New(Config{BaseURL: srv.URL, HTTPClient: &http.Client{Timeout: 20 * time.Millisecond}})
	started := time.Now()
	err := c.connect(context.Background())
	if err == nil {
		t.Fatal("expected handshake timeout")
	}
	if time.Since(started) > time.Second {
		t.Fatalf("timeout took too long: %v", time.Since(started))
	}
}

func TestDoRejectsOversizedResponseBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, strings.Repeat("x", 10<<20+1))
	}))
	defer srv.Close()
	c := New(Config{BaseURL: srv.URL})
	u, err := socketIOParse(srv.URL)
	if err != nil {
		t.Fatalf("socketIOParse: %v", err)
	}
	if _, err := c.do(context.Background(), u, http.MethodGet, nil); err == nil {
		t.Fatal("expected oversized response body to be rejected")
	}
}

func TestSocketIOParseRequiresHTTPSOffLoopback(t *testing.T) {
	for _, tc := range []struct {
		url     string
		wantErr bool
	}{
		{"https://kuma.example.com", false},
		{"http://localhost:3001", false},
		{"http://127.0.0.1:3001", false},
		{"http://[::1]:3001", false},
		{"http://kuma.example.com", true},
		{"ftp://kuma.example.com", true},
	} {
		if _, err := socketIOParse(tc.url); (err != nil) != tc.wantErr {
			t.Errorf("socketIOParse(%q) error = %v, wantErr %v", tc.url, err, tc.wantErr)
		}
	}
}

func TestHTTPClientRejectsCredentialRedirectsOffOrigin(t *testing.T) {
	safe := withSafeRedirects(&http.Client{})
	from := &http.Request{URL: &url.URL{Scheme: "https", Host: "kuma.example.com"}}

	for _, target := range []string{
		"http://attacker.example.com/socket.io/",
		"http://192.0.2.10/socket.io/",
		"https://kuma-backup.example.com/socket.io/",
		"http://127.0.0.1:3001/socket.io/",
		"https://kuma.example.com:8443/socket.io/",
	} {
		to, err := http.NewRequest(http.MethodPost, target, strings.NewReader(`credential-frame`))
		if err != nil {
			t.Fatal(err)
		}
		if err := safe.CheckRedirect(to, []*http.Request{from}); err == nil {
			t.Fatalf("redirect to %s was allowed, want cross-origin or TLS downgrade rejection", target)
		}
	}

	for _, target := range []string{
		"https://kuma.example.com/socket.io/",
		"https://kuma.example.com:443/socket.io/",
		"https://kuma.example.com/other",
	} {
		to, err := http.NewRequest(http.MethodPost, target, strings.NewReader(`credential-frame`))
		if err != nil {
			t.Fatal(err)
		}
		if err := safe.CheckRedirect(to, []*http.Request{from}); err != nil {
			t.Fatalf("redirect to %s rejected: %v", target, err)
		}
	}
}

func TestHTTPClientPreservesCallerRedirectPolicy(t *testing.T) {
	want := fmt.Errorf("caller policy")
	safe := withSafeRedirects(&http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return want
	}})
	to, err := http.NewRequest(http.MethodGet, "https://kuma.example.com/other", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := safe.CheckRedirect(to, []*http.Request{{URL: &url.URL{Scheme: "https", Host: "kuma.example.com"}}}); got != want {
		t.Fatalf("redirect policy error = %v, want caller policy error", got)
	}
}

func TestParseEngineIORejectsMalformedLengthWithoutResync(t *testing.T) {
	if got := parseEngineIO([]byte("3:abc2:40")); len(got) != 2 || got[0] != "abc" || got[1] != "40" {
		t.Fatalf("valid frames parsed as %#v", got)
	}
	if got := parseEngineIO([]byte("99:40")); len(got) != 0 {
		t.Fatalf("malformed length resynchronized into attacker suffix: %#v", got)
	}
	if got := parseEngineIO([]byte(strings.Repeat("9", 4096) + ":40")); len(got) != 0 {
		t.Fatalf("overflowing length accepted: %#v", got)
	}
}

func TestStashedEventsRemainBounded(t *testing.T) {
	c := New(Config{})
	for i := 0; i < maxStashedEvents+50; i++ {
		c.stashEvents([]string{`42["event",` + itoa(i) + `]`})
	}
	if len(c.stash) != maxStashedEvents {
		t.Fatalf("stash length = %d, want %d", len(c.stash), maxStashedEvents)
	}
	if strings.Contains(string(c.stash[0]), `,0]`) {
		t.Fatal("bounded stash retained the oldest event instead of the newest tail")
	}
}

func TestHeartbeatCollectionKeepsBurstLargerThanIdleStash(t *testing.T) {
	f := newFakeKuma(t)
	c := loginClient(t, context.Background(), f)
	count := maxStashedEvents + 44
	records := make([]string, count)
	for i := range records {
		records[i] = fmt.Sprintf(`42["heartbeatList",{"%d":[{"monitorID":%d,"status":1}]}]`, i+1, i+1)
	}
	f.enqueue(records...)
	raw, err := c.CallWithPushFallback(context.Background(), "getHeartbeats", nil, "heartbeatList", 5*time.Second)
	if err != nil {
		t.Fatalf("collecting %d heartbeats: %v", count, err)
	}
	var payloads []json.RawMessage
	if err := json.Unmarshal(raw, &payloads); err != nil {
		t.Fatalf("decoding heartbeat burst: %v", err)
	}
	if len(payloads) != count {
		t.Fatalf("collected %d heartbeats, want %d", len(payloads), count)
	}
}

func TestHeartbeatCollectionEnforcesDeadlineDuringSustainedBurst(t *testing.T) {
	c := New(Config{})
	stream, err := c.beginEventStream("heartbeatList")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; ctx.Err() == nil; i++ {
			c.routeEvent(ctx, []byte(fmt.Sprintf(`["heartbeatList",{"%d":[{"status":1}]}]`, i)))
		}
	}()
	started := time.Now()
	_, err = c.collectStashedEvents(ctx, "heartbeatList", 40*time.Millisecond, stream)
	c.endEventStream(stream)
	cancel()
	<-done
	if err == nil || !strings.Contains(err.Error(), "timed out collecting") {
		t.Fatalf("sustained burst error = %v, want deadline error", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("collection exceeded deadline by too much: %v", elapsed)
	}
}

func TestEventStreamFinishDrainsPendingEventBeforeClosing(t *testing.T) {
	c := New(Config{})
	stream, err := c.beginEventStream("heartbeatList")
	if err != nil {
		t.Fatal(err)
	}
	c.routeEvent(context.Background(), []byte(`["heartbeatList",{"25":[{"status":1}]}]`))
	late, finished := c.finishEventStream(stream)
	if finished || len(late) != 1 {
		t.Fatalf("finished=%v, pending events=%d; want one event before teardown", finished, len(late))
	}
	late, finished = c.finishEventStream(stream)
	if !finished || len(late) != 0 {
		t.Fatalf("finished=%v, pending events=%d; want closed empty stream", finished, len(late))
	}
	// A reader arriving after teardown goes to the ordinary stash, not an
	// abandoned stream queue.
	c.routeEvent(context.Background(), []byte(`["heartbeatList",{"43":[{"status":0}]}]`))
	if got := c.takeStashedEvent("heartbeatList"); !bytes.Contains(got, []byte(`"43"`)) {
		t.Fatalf("event after teardown was lost: %s", got)
	}
}

func TestDebugURLRedactionDropsQueryAndFragment(t *testing.T) {
	u, err := url.Parse("https://alice:secret@example.test/socket.io/?sid=session-secret&token=query-secret#fragment-secret")
	if err != nil {
		t.Fatal(err)
	}
	got := redactURL(u)
	if strings.Contains(got, "alice") || strings.Contains(got, "secret") || strings.ContainsAny(got, "?#") {
		t.Fatalf("redactURL leaked URL secrets: %q", got)
	}
}

func TestHeartbeatCollectionKeepsMultipleObjectPushes(t *testing.T) {
	f := newFakeKuma(t)
	c := loginClient(t, context.Background(), f)
	f.enqueue(
		`42["heartbeatList",{"25":[{"monitorID":25,"status":1}]}]`,
		`42["heartbeatList",{"43":[{"monitorID":43,"status":0}]}]`,
	)
	raw, err := c.CallWithPushFallback(context.Background(), "getHeartbeats", nil, "heartbeatList", time.Second)
	if err != nil {
		t.Fatalf("heartbeat collection: %v", err)
	}
	var payloads []json.RawMessage
	if err := json.Unmarshal(raw, &payloads); err != nil || len(payloads) != 2 {
		t.Fatalf("expected two object payloads, got %s", raw)
	}
}

// TestHeartbeatCollectionReturnsBeforeDeadline pins the burst-collection
// timing contract: once the stream goes quiet the call must return promptly
// rather than always paying the caller's full timeout.
func TestHeartbeatCollectionReturnsBeforeDeadline(t *testing.T) {
	f := newFakeKuma(t)
	c := loginClient(t, context.Background(), f)
	f.enqueue(`42["heartbeatList",{"25":[{"monitorID":25,"status":1}]}]`)
	started := time.Now()
	if _, err := c.CallWithPushFallback(context.Background(), "getHeartbeats", nil, "heartbeatList", 10*time.Second); err != nil {
		t.Fatalf("heartbeat collection: %v", err)
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("collection waited for the full deadline instead of the idle gap: %v", elapsed)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	d := ""
	for n > 0 {
		d = string(rune('0'+n%10)) + d
		n /= 10
	}
	return d
}
