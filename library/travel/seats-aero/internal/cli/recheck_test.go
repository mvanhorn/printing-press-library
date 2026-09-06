package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/seats-aero/internal/store"
)

type recheckServer struct {
	*httptest.Server
	mu             sync.Mutex
	refreshes      int
	ids            []string
	quotaStatus    int
	quotaRemaining string
	refreshStatus  int
	refreshBody    string
}

func newRecheckServer(t *testing.T) *recheckServer {
	t.Helper()
	rs := &recheckServer{quotaStatus: http.StatusOK, quotaRemaining: "997", refreshStatus: http.StatusOK, refreshBody: `{"items":[{"availability_id":"same-day-8h","status":"queued"},{"availability_id":"oldest-space","status":"queued"},{"availability_id":"fresh-1h","status":"queued"},{"availability_id":"middle-12h","status":"queued"},{"availability_id":"award-only","status":"queued"}]}`}
	rs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rs.mu.Lock()
		defer rs.mu.Unlock()
		switch r.URL.Path {
		case "/destinations":
			if rs.quotaRemaining != "" {
				w.Header().Set("X-RateLimit-Limit", "1000")
				w.Header().Set("X-RateLimit-Remaining", rs.quotaRemaining)
				w.Header().Set("X-RateLimit-Reset", "30")
			}
			w.WriteHeader(rs.quotaStatus)
			_, _ = w.Write([]byte(`{"destinations":[]}`))
		case "/refresh":
			rs.refreshes++
			var body struct {
				IDs []string `json:"availability_ids"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode refresh: %v", err)
			}
			rs.ids = body.IDs
			w.WriteHeader(rs.refreshStatus)
			_, _ = w.Write([]byte(rs.refreshBody))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(rs.Close)
	return rs
}

func seedRecheckStore(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	items := []struct {
		id    string
		age   time.Duration
		space bool
	}{
		{"same-day-8h", 8 * time.Hour, false},
		{"oldest-space", 15 * time.Hour, true},
		{"fresh-1h", time.Hour, false},
		{"middle-12h", 12 * time.Hour, false},
	}
	for i, item := range items {
		row := map[string]any{"ID": item.id, "RouteID": "jfk-nrt", "Route": map[string]any{"ID": "jfk-nrt", "OriginAirport": "JFK", "DestinationAirport": "NRT", "Source": "aeroplan"}, "Date": time.Date(2026, 10, 10+i, 0, 0, 0, 0, time.UTC).Format(time.RFC3339), "Source": "aeroplan", "JAvailable": true, "JMileageCostRaw": 29000}
		raw, _ := json.Marshal(row)
		if err := db.UpsertAvailability(raw); err != nil {
			t.Fatal(err)
		}
		stamp := now.Add(-item.age).Format(time.RFC3339)
		if item.space {
			stamp = now.Add(-item.age).Format("2006-01-02 15:04:05")
		}
		if _, err := db.DB().Exec(`UPDATE "availability" SET synced_at = ? WHERE id = ?`, stamp, item.id); err != nil {
			t.Fatal(err)
		}
	}
	award := map[string]any{"ID": "award-only", "RouteID": "jfk-nrt", "Route": map[string]any{"OriginAirport": "JFK", "DestinationAirport": "NRT"}, "Date": "2026-10-20T00:00:00Z", "Source": "aeroplan", "JAvailable": true, "JMileageCostRaw": 29000}
	raw, _ := json.Marshal(award)
	if err := db.UpsertAwards(raw); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().Exec(`UPDATE "awards" SET synced_at = ? WHERE id = ?`, now.Add(-10*time.Hour).Format(time.RFC3339), "award-only"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestNovelRecheckShortlistAndApply(t *testing.T) {
	isolateNovelTest(t)
	path := seedRecheckStore(t)
	server := newRecheckServer(t)
	t.Setenv("SEATS_AERO_BASE_URL", server.URL)
	out, _, err := executeRecheck([]string{"recheck", "--db", path, "--origin", "JFK", "--destination", "NRT", "--cabin", "business", "--older-than", "6h", "--json"})
	if err != nil {
		t.Fatal(err)
	}
	var plan recheckPlan
	if err := json.Unmarshal(out.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(plan.Shortlist))
	for _, row := range plan.Shortlist {
		got = append(got, row.ID)
		if len(row.Date) != 10 {
			t.Fatalf("date not truncated: %q", row.Date)
		}
		if row.ID == "oldest-space" && strings.Contains(row.SyncedAt, "T") {
			t.Fatalf("synced_at was normalized: %q", row.SyncedAt)
		}
	}
	want := []string{"oldest-space", "middle-12h", "award-only", "same-day-8h"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ids=%v want %v", got, want)
	}
	out, _, err = executeRecheck([]string{"recheck", "--db", path, "--older-than", "6h", "--apply", "--json"})
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.Mode != "applied" {
		t.Fatalf("mode=%q", plan.Mode)
	}
	server.mu.Lock()
	defer server.mu.Unlock()
	if server.refreshes != 1 || !reflect.DeepEqual(server.ids, want) {
		t.Fatalf("refreshes=%d ids=%v", server.refreshes, server.ids)
	}
}

func TestNovelRecheckQuotaGuards(t *testing.T) {
	isolateNovelTest(t)
	path := seedRecheckStore(t)
	server := newRecheckServer(t)
	t.Setenv("SEATS_AERO_BASE_URL", server.URL)
	server.quotaRemaining = "1"
	_, _, err := executeRecheck([]string{"recheck", "--db", path, "--apply", "--json"})
	if err == nil || !strings.Contains(err.Error(), "quota") {
		t.Fatalf("err=%v", err)
	}
	server.mu.Lock()
	if server.refreshes != 0 {
		t.Fatalf("refreshes=%d", server.refreshes)
	}
	server.mu.Unlock()
	server.quotaRemaining, server.quotaStatus = "", http.StatusInternalServerError
	_, _, err = executeRecheck([]string{"recheck", "--db", path, "--apply", "--json"})
	if err == nil || !strings.Contains(err.Error(), "quota unknown") {
		t.Fatalf("err=%v", err)
	}
	server.mu.Lock()
	if server.refreshes != 0 {
		t.Fatalf("refreshes=%d", server.refreshes)
	}
	server.mu.Unlock()
	_, _, err = executeRecheck([]string{"recheck", "--db", path, "--apply", "--ignore-quota", "--json"})
	if err != nil {
		t.Fatal(err)
	}
	server.mu.Lock()
	if server.refreshes != 1 {
		t.Fatalf("refreshes=%d", server.refreshes)
	}
	server.mu.Unlock()
}

func TestReadRecheckRowsOrdersEqualTimestampsByID(t *testing.T) {
	isolateNovelTest(t)
	path := filepath.Join(t.TempDir(), "equal.db")
	writable, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"b", "a"} {
		raw := json.RawMessage(`{"ID":"` + id + `","Route":{"OriginAirport":"JFK","DestinationAirport":"NRT"},"Date":"2099-01-01","Source":"united","JAvailable":true,"JMileageCostRaw":1}`)
		if err := writable.UpsertAvailability(raw); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := writable.DB().Exec(`UPDATE availability SET synced_at='2000-01-01T00:00:00Z'`); err != nil {
		t.Fatal(err)
	}
	if err := writable.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := store.OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := readRecheckRows(t.Context(), db, "j", "JFK", "NRT", "", 0, time.Now().UTC().Add(time.Hour), 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].ID != "a" || rows[1].ID != "b" {
		t.Fatalf("IDs=%v", []string{rows[0].ID, rows[1].ID})
	}
}

func TestNovelRecheckLocalAndProbeFailurePlans(t *testing.T) {
	isolateNovelTest(t)
	path := seedRecheckStore(t)
	server := newRecheckServer(t)
	t.Setenv("SEATS_AERO_BASE_URL", server.URL)
	server.quotaRemaining, server.quotaStatus = "", http.StatusInternalServerError
	for _, args := range [][]string{{"recheck", "--db", path, "--json"}, {"recheck", "--db", path, "--data-source", "local", "--json"}} {
		out, _, err := executeRecheck(args)
		if err != nil {
			t.Fatal(err)
		}
		var plan recheckPlan
		if err := json.Unmarshal(out.Bytes(), &plan); err != nil {
			t.Fatal(err)
		}
		if plan.Quota != nil || len(plan.Warnings) == 0 || len(plan.Shortlist) == 0 {
			t.Fatalf("plan=%+v", plan)
		}
	}
	_, _, err := executeRecheck([]string{"recheck", "--db", path, "--data-source", "local", "--apply", "--json"})
	ce, ok := err.(*cliError)
	if !ok || ce.code != 2 || !strings.Contains(err.Error(), "requires live") {
		t.Fatalf("err=%T %v", err, err)
	}
	server.mu.Lock()
	defer server.mu.Unlock()
	if server.refreshes != 0 {
		t.Fatalf("refreshes=%d", server.refreshes)
	}
}

func TestNovelRecheckRefreshErrorAndEmptyApply(t *testing.T) {
	isolateNovelTest(t)
	path := seedRecheckStore(t)
	server := newRecheckServer(t)
	t.Setenv("SEATS_AERO_BASE_URL", server.URL)
	server.refreshStatus = http.StatusTooManyRequests
	_, _, err := executeRecheck([]string{"recheck", "--db", path, "--apply", "--json", "--timeout", "1s"})
	if err == nil || (!strings.Contains(err.Error(), "429") && !strings.Contains(err.Error(), "rate limited")) {
		t.Fatalf("err=%v", err)
	}
	server.mu.Lock()
	server.refreshes = 0
	server.refreshStatus = http.StatusOK
	server.mu.Unlock()
	out, _, err := executeRecheck([]string{"recheck", "--db", path, "--older-than", "1000h", "--apply", "--json"})
	if err != nil {
		t.Fatal(err)
	}
	var plan recheckPlan
	if err := json.Unmarshal(out.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.WouldRefresh != 0 {
		t.Fatalf("plan=%+v", plan)
	}
	server.mu.Lock()
	defer server.mu.Unlock()
	if server.refreshes != 0 {
		t.Fatalf("refreshes=%d", server.refreshes)
	}
}

func TestNovelRecheckRefreshItemStatuses(t *testing.T) {
	isolateNovelTest(t)
	path := seedRecheckStore(t)
	server := newRecheckServer(t)
	t.Setenv("SEATS_AERO_BASE_URL", server.URL)
	tests := []struct {
		name         string
		body         string
		wantMode     string
		wantFailures int
		wantErr      bool
	}{
		{"mixed", `{"items":[{"availability_id":"oldest-space","status":"queued","updated_at":"2026-09-06T00:00:00Z"},{"availability_id":"middle-12h","status":"insufficient_quota"}],"queued":1,"refunded":1,"counts":{"queued":1,"insufficient_quota":1},"quota":{"remaining":0}}`, "partial", 1, false},
		{"all failed", `{"items":[{"availability_id":"oldest-space","status":"not_found"},{"availability_id":"middle-12h","status":"insufficient_quota"}],"queued":0,"refunded":2}`, "partial", 2, true},
		{"all queued", `{"items":[{"availability_id":"oldest-space","status":"queued"},{"availability_id":"middle-12h","status":"queued"}],"queued":2,"refunded":0}`, "applied", 0, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server.mu.Lock()
			server.refreshBody = tc.body
			server.mu.Unlock()
			out, _, err := executeRecheck([]string{"recheck", "--db", path, "--older-than", "6h", "--apply", "--json"})
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if tc.name == "all failed" && ExitCode(err) != 3 {
				t.Fatalf("all-failed exit=%d want 3", ExitCode(err))
			}
			var plan recheckPlan
			if decodeErr := json.Unmarshal(out.Bytes(), &plan); decodeErr != nil {
				t.Fatalf("decode %q: %v", out.String(), decodeErr)
			}
			if plan.Mode != tc.wantMode || len(plan.RefreshFailures) != tc.wantFailures {
				t.Fatalf("plan=%+v", plan)
			}
			if tc.wantFailures > 0 && (len(plan.Warnings) == 0 || !strings.Contains(plan.Warnings[len(plan.Warnings)-1], "rows were not refreshed")) {
				t.Fatalf("warnings=%v", plan.Warnings)
			}
			var gotResponse, wantResponse any
			if err := json.Unmarshal(plan.RefreshResponse, &gotResponse); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(tc.body), &wantResponse); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(gotResponse, wantResponse) {
				t.Fatalf("refresh_response=%s want %s", plan.RefreshResponse, tc.body)
			}
		})
	}
}

func executeRecheck(args []string) (*bytes.Buffer, *bytes.Buffer, error) {
	cmd := RootCmd()
	out, stderr := new(bytes.Buffer), new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetErr(stderr)
	cmd.SetArgs(args)
	return out, stderr, cmd.Execute()
}
