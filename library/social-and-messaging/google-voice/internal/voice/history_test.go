package voice

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func historyFixture(t *testing.T) *Mirror {
	t.Helper()
	m, e := OpenMirror(filepath.Join(privateTestDir(t), "messages.sqlite3"), "account-id", true)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { m.Close() })
	for i := 1; i <= 2; i++ {
		if _, e = m.DB.Exec("INSERT INTO voice_runs(id,started_at,finished_at,status) VALUES(?,'start','end','bounded')", i); e != nil {
			t.Fatal(e)
		}
	}
	b, _ := json.Marshal([]any{threadFixture("t-1", "older")})
	p, e := DecodePage(b, true)
	if e != nil {
		t.Fatal(e)
	}
	base := p.Threads[0].Messages[0]
	p.Threads[0].Messages = nil
	for i, id := range []string{"m-1", "m-2", "m-3", "m-4"} {
		v := base
		v.ID = id
		v.TimestampMS += int64(i) * 1000
		v.Timestamp = time.UnixMilli(v.TimestampMS).UTC().Format(time.RFC3339Nano)
		if i == 1 {
			v.Direction = "outbound"
		}
		p.Threads[0].Messages = append(p.Threads[0].Messages, v)
	}
	if _, e = m.commit(context.Background(), 1, "refresh", "t-1", "", "older", p.Threads); e != nil {
		t.Fatal(e)
	}
	empty := Thread{ID: "empty-thread", Messages: []Message{}, Contacts: []Contact{}}
	if _, e = m.commit(context.Background(), 1, "refresh", empty.ID, "", "", []Thread{empty}); e != nil {
		t.Fatal(e)
	}
	return m
}
func TestContextAndActivity(t *testing.T) {
	m := historyFixture(t)
	ctx := context.Background()
	for _, tc := range []struct {
		id                  string
		before, after, want int
		bad                 bool
	}{{"m-2", 1, 1, 3, false}, {"m-1", 0, 0, 1, false}, {"missing", 1, 1, 0, true}, {"m-1", -1, 1, 0, true}} {
		v, e := m.Context(ctx, tc.id, tc.before, tc.after)
		if (e != nil) != tc.bad {
			t.Fatalf("context error %v", e)
		}
		if !tc.bad {
			messages := v["messages"].([]Record)
			if len(messages) != tc.want {
				t.Fatal(v)
			}
			for i := 1; i < len(messages); i++ {
				if messages[i-1].TimestampMS > messages[i].TimestampMS {
					t.Fatal("context is not chronological")
				}
			}
		}
	}
	for _, tc := range []struct {
		start, end int64
		in, out    int64
	}{{1699999999000, 1700000005000, 3, 1}, {1700000001000, 1700000002000, 0, 1}, {1800000000000, 1800000005000, 0, 0}} {
		rows, e := m.Activity(ctx, time.UnixMilli(tc.start), time.UnixMilli(tc.end), 10)
		if e != nil || len(rows) != 2 {
			t.Fatalf("activity rows=%+v err=%v", rows, e)
		}
		for _, v := range rows {
			if v.ThreadID == "t-1" && (v.Inbound != tc.in || v.Outbound != tc.out) {
				t.Fatalf("wrong counts %+v", v)
			}
			if v.ThreadID == "empty-thread" && (v.Inbound != 0 || v.LatestInbound != nil) {
				t.Fatalf("empty-window evidence invented %+v", v)
			}
		}
	}
	c, e := m.Coverage(ctx, "missing-participant", 10)
	if e != nil || len(c.Threads) != 0 || len(c.Inventory) != 2 {
		t.Fatalf("unknown coverage discarded %+v %v", c, e)
	}
}
func TestArrivalsAndLiteralFilters(t *testing.T) {
	m := historyFixture(t)
	ctx := context.Background()
	v, e := m.Arrivals(ctx, 1, "", 10)
	if e != nil || len(v["messages"].([]Record)) != 0 {
		t.Fatal("re-import became an arrival")
	}
	b, _ := json.Marshal([]any{threadFixture("t-1", "older")})
	p, _ := DecodePage(b, true)
	p.Threads[0].Messages[0].ID = "old-backfill"
	p.Threads[0].Messages[0].TimestampMS = 1600000000000
	p.Threads[0].Messages[0].Timestamp = time.UnixMilli(1600000000000).UTC().Format(time.RFC3339Nano)
	if _, e = m.commit(ctx, 2, "history", "t-1", "older", "", p.Threads); e != nil {
		t.Fatal(e)
	}
	v, e = m.Arrivals(ctx, 1, "", 10)
	if e != nil || len(v["messages"].([]Record)) != 1 || v["messages"].([]Record)[0].TimestampMS != 1600000000000 {
		t.Fatalf("backfill chronology confused: %+v %v", v, e)
	}
	first, e := m.Arrivals(ctx, 0, "", 1)
	if e != nil || first["next_cursor"] == "" {
		t.Fatal("missing seek cursor")
	}
	second, e := m.Arrivals(ctx, 0, first["next_cursor"].(string), 10)
	if e != nil || len(second["messages"].([]Record)) != 4 {
		t.Fatalf("cursor page %+v %v", second, e)
	}
	if _, e = m.Arrivals(ctx, 0, "malformed", 1); e == nil {
		t.Fatal("bad cursor accepted")
	}
	if _, e = m.Arrivals(ctx, 99, "", 1); e == nil {
		t.Fatal("unknown run accepted")
	}
	for _, text := range []string{"' OR 1=1 --", "%", "a-does-not-match"} {
		rows, e := m.Messages(ctx, Query{Text: text, Limit: 10})
		if e != nil || len(rows) != 0 {
			t.Fatalf("literal filter leaked records %q", text)
		}
	}
	if _, e = m.Context(ctx, "absent", 1, 1); !errors.Is(e, sql.ErrNoRows) {
		t.Fatal("missing anchor does not preserve not-found")
	}
}
func TestSyncResumeAndFailure(t *testing.T) {
	m, e := OpenMirror(filepath.Join(privateTestDir(t), "messages.sqlite3"), "account-id", true)
	if e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	s := Session{AccountID: "account-id", Cookies: map[string]string{"SAPISID": "fake"}, Headers: map[string]string{}, PublicKey: "public-key"}
	r := NewRemote(&s)
	fail := false
	r.HTTP.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		status := 200
		var v any
		raw, _ := io.ReadAll(req.Body)
		var args []any
		json.Unmarshal(raw, &args)
		switch req.URL.Path {
		case "/voice/v1/voiceclient/account/get":
			v = []any{[]any{"account-id"}}
		case "/voice/v1/voiceclient/api2thread/list":
			if fail {
				status = 503
				v = map[string]string{"error": "unavailable"}
			} else if str(args[3]) == "" {
				v = []any{[]any{threadFixture("t-1", "older")}, "inventory-next"}
			} else {
				v = []any{[]any{}, ""}
			}
		case "/voice/v1/voiceclient/api2thread/get":
			v = []any{threadFixture("t-1", "")}
		}
		b, _ := json.Marshal(v)
		return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(string(b)))}, nil
	})
	first, e := m.Sync(context.Background(), r, []string{"texts"}, 1)
	if e != nil || first.Pages != 1 || first.NewMessages != 1 || first.InventoryComplete {
		t.Fatalf("first sync %+v %v", first, e)
	}
	second, e := m.Sync(context.Background(), r, []string{"texts"}, 2)
	if e != nil || second.NewMessages != 0 || !second.InventoryComplete || second.PendingThreads != 0 {
		t.Fatalf("resumed sync %+v %v", second, e)
	}
	fail = true
	if _, e = m.Sync(context.Background(), r, []string{"texts"}, 1); e == nil {
		t.Fatal("upstream failure became empty success")
	}
	var status string
	m.DB.QueryRow("SELECT status FROM voice_runs ORDER BY id DESC LIMIT 1").Scan(&status)
	if status != "failed" {
		t.Fatal("failed run not recorded")
	}
	var cursor string
	var complete bool
	m.DB.QueryRow("SELECT cursor,complete FROM voice_inventory WHERE folder='texts'").Scan(&cursor, &complete)
	if !complete || cursor != "" {
		t.Fatal("failed page advanced checkpoint")
	}
}
