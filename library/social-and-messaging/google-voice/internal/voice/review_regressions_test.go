package voice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/google-voice/internal/cliutil"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRejectMalformedProtocolBeforeCommit(t *testing.T) {
	for _, tc := range []struct {
		raw    string
		detail bool
	}{{`[["t",0,{},null]]`, true}, {`[["t",0,[],["bad"]]]`, true}, {`[[],123]`, false}, {`[[],""] {}`, false}} {
		if _, e := DecodePage([]byte(tc.raw), tc.detail); e == nil {
			t.Fatalf("malformed page accepted: %s", tc.raw)
		}
	}
	account := make([]any, 13)
	account[0] = "account-id"
	account[12] = []any{"version-token"}
	b, _ := json.Marshal([]any{account})
	a, e := DecodeAccount(b)
	if e != nil || a.Version != "version-token" {
		t.Fatalf("nested account version %+v %v", a, e)
	}
	v := threadFixture("t", "")
	m := v[2].([]any)[0].([]any)
	m[4] = 2
	m[6] = []any{nil, []any{[]any{"Hello"}, []any{"there"}}}
	m[8] = 1.5
	b, _ = json.Marshal([]any{v})
	p, e := DecodePage(b, true)
	if e != nil || p.Threads[0].Messages[0].Transcript != "Hello there" || p.Threads[0].Messages[0].Duration != 1.5 {
		t.Fatalf("transcript/duration %+v %v", p, e)
	}
}
func TestHalfOpenSubmillisecondBounds(t *testing.T) {
	m := historyFixture(t)
	ctx := context.Background()
	for _, tc := range []struct {
		q    Query
		want int
	}{{Query{After: "2023-11-14T22:13:20.000500Z", Before: "2023-11-14T22:13:20.001Z", Limit: 10}, 0}, {Query{Before: "2023-11-14T22:13:20.000500Z", Limit: 10}, 1}} {
		got, e := m.Messages(ctx, tc.q)
		if e != nil || len(got) != tc.want {
			t.Fatalf("bounds %+v rows=%d %v", tc.q, len(got), e)
		}
	}
	end, _ := time.Parse(time.RFC3339Nano, "2023-11-14T22:13:20.000500Z")
	rows, e := m.Activity(ctx, end.Add(-time.Second), end, 10)
	if e != nil {
		t.Fatal(e)
	}
	for _, r := range rows {
		if r.ThreadID == "t-1" && r.Inbound != 1 {
			t.Fatalf("exclusive upper bound rounded down: %+v", r)
		}
	}
	m.DB.Exec("UPDATE voice_threads SET history_complete=1,history_observed_at='2020-01-01T00:00:00Z',observed_at='2026-01-01T00:00:00Z',last_refresh='2020-01-01T00:00:00Z',needs_refresh=1,refresh_cursor='gap' WHERE id='t-1'")
	packet, e := m.Context(ctx, "m-1", 1, 1)
	if e != nil {
		t.Fatal(e)
	}
	state := packet["collection"].(map[string]any)
	if state["history_observed_at"] != "2020-01-01T00:00:00Z" || state["needs_refresh"] != true || state["pending_refresh_gap"] != true {
		t.Fatalf("observation axes lost %+v", state)
	}
}
func makeReply(status int, v any) *http.Response {
	b, _ := json.Marshal(v)
	return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(string(b)))}
}
func fakeSession() Session {
	return Session{AccountID: "account-id", Cookies: map[string]string{"SID": "fake", "SAPISID": "fake"}, Headers: map[string]string{}, PublicKey: "public-key"}
}
func TestFairBudgetAndRefreshGap(t *testing.T) {
	m, e := OpenMirrorContext(context.Background(), filepath.Join(privateTestDir(t), "data.sqlite3"), "account-id", true)
	if e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	s := fakeSession()
	r := NewRemote(&s, 0)
	texts, archived, gets, older := 0, 0, 0, 0
	r.HTTP.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var args []any
		json.NewDecoder(req.Body).Decode(&args)
		switch req.URL.Path {
		case "/voice/v1/voiceclient/account/get":
			return makeReply(200, []any{[]any{"account-id"}}), nil
		case "/voice/v1/voiceclient/api2thread/list":
			if int(args[0].(float64)) == 6 {
				archived++
				return makeReply(200, []any{[]any{}, ""}), nil
			}
			texts++
			return makeReply(200, []any{[]any{threadFixture("t", "more")}, ""}), nil
		default:
			gets++
			cursor := "more"
			if str(args[2]) != "" {
				older++
				cursor = ""
			}
			return makeReply(200, []any{threadFixture("t", cursor)}), nil
		}
	})
	first, e := m.Sync(context.Background(), r, []string{"texts", "archived"}, 1)
	if e != nil || first.InventoryComplete {
		t.Fatalf("unvisited archive claimed exhausted %+v %v", first, e)
	}
	for i := 0; i < 9; i++ {
		if _, e = m.Sync(context.Background(), r, []string{"texts", "archived"}, 1); e != nil {
			t.Fatal(e)
		}
	}
	if texts == 0 || archived == 0 || gets == 0 || older == 0 {
		t.Fatalf("work starvation texts=%d archived=%d get=%d older=%d", texts, archived, gets, older)
	}
	// A fresh page must bridge back to the old anchor without discarding deeper history work.
	m.DB.Exec("DELETE FROM voice_threads")
	m.DB.Exec("DELETE FROM voice_messages")
	old := Thread{ID: "gap-thread", Messages: []Message{{ID: "old", ThreadID: "gap-thread", TimestampMS: 1700000000000, Timestamp: "2023-11-14T22:13:20Z", Direction: "inbound", Kind: "text", Attachments: []Attachment{}}}, Contacts: []Contact{}, OlderCursor: "deep"}
	if _, e = m.commit(context.Background(), 1, "refresh", old.ID, "", "deep", []Thread{old}); e != nil {
		t.Fatal(e)
	}
	fresh := old
	fresh.Messages = append([]Message{}, old.Messages...)
	fresh.Messages[0].ID = "new"
	fresh.Messages[0].TimestampMS += 100000
	fresh.OlderCursor = "gap"
	if _, e = m.commit(context.Background(), 2, "refresh", old.ID, "", "gap", []Thread{fresh}); e != nil {
		t.Fatal(e)
	}
	var gap, deep string
	m.DB.QueryRow("SELECT refresh_cursor,older_cursor FROM voice_threads WHERE id=?", old.ID).Scan(&gap, &deep)
	if gap != "gap" || deep != "deep" {
		t.Fatalf("two continuations lost: gap=%s deep=%s", gap, deep)
	}
	bridge := old
	bridge.OlderCursor = "bridge-continues"
	mid := old.Messages[0]
	mid.ID = "middle"
	mid.TimestampMS += 50000
	bridge.Messages = append([]Message{mid}, old.Messages...)
	if _, e = m.commit(context.Background(), 3, "refresh_gap", old.ID, "gap", bridge.OlderCursor, []Thread{bridge}); e != nil {
		t.Fatal(e)
	}
	m.DB.QueryRow("SELECT refresh_cursor,older_cursor FROM voice_threads WHERE id=?", old.ID).Scan(&gap, &deep)
	if gap != "" || deep != "deep" {
		t.Fatalf("bridging reset historical checkpoint gap=%s deep=%s", gap, deep)
	}
	rows, e := m.Messages(context.Background(), Query{Limit: 10})
	if e != nil || len(rows) != 3 {
		t.Fatalf("new middle history missing rows=%d %v", len(rows), e)
	}
}
func TestSyncLockCancellationAndAccountSingleton(t *testing.T) {
	path := filepath.Join(privateTestDir(t), "data.sqlite3")
	m, e := OpenMirror(path, "account-id", true)
	if e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	peer, e := OpenMirror(path, "account-id", true)
	if e != nil {
		t.Fatal(e)
	}
	defer peer.Close()
	entered, release := make(chan struct{}), make(chan struct{})
	a := fakeSession()
	r := NewRemote(&a, 0)
	r.HTTP.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if strings.HasSuffix(req.URL.Path, "account/get") {
			return makeReply(200, []any{[]any{"account-id"}}), nil
		}
		close(entered)
		<-release
		return makeReply(200, []any{[]any{}, ""}), nil
	})
	done := make(chan error, 1)
	go func() { _, e := m.Sync(context.Background(), r, []string{"texts"}, 1); done <- e }()
	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	s := fakeSession()
	other := NewRemote(&s, 0)
	called := false
	other.HTTP.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) { called = true; return makeReply(200, []any{}), nil })
	if _, e = peer.Sync(ctx, other, []string{"texts"}, 1); !errors.Is(e, context.DeadlineExceeded) || called {
		t.Fatalf("overlapping run passed lock: called=%v err=%v", called, e)
	}
	close(release)
	if e = <-done; e != nil {
		t.Fatal(e)
	}
	newPath := filepath.Join(privateTestDir(t), "singleton.sqlite3")
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, account := range []string{"one", "two"} {
		wg.Add(1)
		go func(a string) {
			defer wg.Done()
			db, e := OpenMirror(newPath, a, true)
			if e == nil {
				db.Close()
			}
			results <- e
		}(account)
	}
	wg.Wait()
	close(results)
	success := 0
	for e := range results {
		if e == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("singleton account binding allowed %d owners", success)
	}
}
func TestGatewayIdentityUnknownReadAndRateStatus(t *testing.T) {
	restore, e := cliutil.SetHomeOverride(privateTestDir(t))
	if e != nil {
		t.Fatal(e)
	}
	defer restore()
	t.Setenv("GOOGLE_VOICE_PROFILE", "review")
	if e = SaveSession(fakeSession()); e != nil {
		t.Fatal(e)
	}
	for _, test := range []struct {
		name       string
		status     int
		account    string
		wantStatus int
		unknown    bool
	}{{"throttle", 429, "account-id", 429, false}, {"unknown-unread", 200, "account-id", 200, true}} {
		t.Run(test.name, func(t *testing.T) {
			g := Gateway{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if test.status == 429 {
					resp := makeReply(429, map[string]string{})
					resp.Header.Set("Retry-After", "30")
					return resp, nil
				}
				if strings.HasSuffix(req.URL.Path, "account/get") {
					return makeReply(200, []any{[]any{test.account}}), nil
				}
				v := threadFixture("t", "")
				v[1] = nil
				return makeReply(200, []any{v}), nil
			})}
			req, _ := http.NewRequest("POST", Base+"/api2thread/get", strings.NewReader(`{"thread_id":"t","unread":true}`))
			resp, e := g.RoundTrip(req)
			if e != nil || resp.StatusCode != test.wantStatus {
				t.Fatalf("status=%v err=%v", resp, e)
			}
			defer resp.Body.Close()
			if test.status == 429 {
				if resp.Header.Get("Retry-After") != "30" {
					t.Fatal("retry deadline lost")
				}
			} else {
				var p Page
				if e = json.NewDecoder(resp.Body).Decode(&p); e != nil || len(p.Threads) != 0 || !p.UnknownReadState {
					t.Fatalf("unknown unread admitted %+v %v", p, e)
				}
			}
		})
	}
	s := fakeSession()
	r := NewRemote(&s, 0)
	dataCalls := 0
	r.HTTP.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if strings.HasSuffix(req.URL.Path, "account/get") {
			return makeReply(200, []any{[]any{"other-account"}}), nil
		}
		dataCalls++
		return makeReply(200, []any{}), nil
	})
	if _, e = r.List(context.Background(), "texts", ""); e == nil || dataCalls != 0 {
		t.Fatal("unpinned account data was read")
	}
}

func TestExhaustedInventoryStopsWithinRun(t *testing.T) {
	m, err := OpenMirror(filepath.Join(privateTestDir(t), "messages.sqlite3"), "account-id", true)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	var lists int
	s := fakeSession()
	r := NewRemote(&s, 0)
	r.HTTP.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if strings.HasSuffix(req.URL.Path, "/account/get") {
			return makeReply(200, []any{[]any{"account-id"}}), nil
		}
		lists++
		if lists > 1 {
			return nil, fmt.Errorf("unnecessary repeated inventory")
		}
		return makeReply(200, []any{[]any{}, ""}), nil
	})
	got, err := m.Sync(context.Background(), r, []string{"texts"}, 5)
	if err != nil || lists != 1 || got.Pages != 1 || got.Status != "observed_exhausted" {
		t.Fatalf("sync=%+v lists=%d error=%v", got, lists, err)
	}
}

func TestGatewayRejectsInvalidCountBeforeCredentials(t *testing.T) {
	g := &Gateway{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("network requested for invalid count")
		return nil, nil
	})}
	for _, count := range []string{"0", "1.5", "101", "null", "\"5\""} {
		req, _ := http.NewRequest("POST", Base+"/api2thread/get", strings.NewReader(`{"thread_id":"synthetic","count":`+count+`}`))
		if _, err := g.RoundTrip(req); err == nil || !strings.Contains(err.Error(), "count must be an integer") {
			t.Fatalf("count=%s error=%v", count, err)
		}
	}
}

func TestPBLiteTranscriptPreservesLiteralUTF8(t *testing.T) {
	v := threadFixture("synthetic-thread", "")
	m := v[2].([]any)[0].([]any)
	m[4] = 2
	m[6] = []any{nil, []any{[]any{"Hi"}, []any{"café"}, []any{"SGVsbG8="}}}
	b, _ := json.Marshal([]any{v})
	p, e := DecodePage(b, true)
	if e != nil || p.Threads[0].Messages[0].Transcript != "Hi café SGVsbG8=" {
		t.Fatalf("literal PBLite transcript not preserved: %v", e)
	}
}
