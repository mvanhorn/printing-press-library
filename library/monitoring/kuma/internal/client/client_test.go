package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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
