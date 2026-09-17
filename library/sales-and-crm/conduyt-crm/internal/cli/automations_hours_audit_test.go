// Sweep findings: missing/unsynced mirrors and local read/decode failures cannot yield a verifiable audit and must be partial API-class failures.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/store"
)

type failingHoursAuditRows struct {
	index int
	err   error
}

type paginationClientFunc func(context.Context, string, map[string]string, map[string]string) (json.RawMessage, error)

func (f paginationClientFunc) GetWithHeaders(ctx context.Context, path string, params map[string]string, headers map[string]string) (json.RawMessage, error) {
	return f(ctx, path, params, headers)
}

func TestNovelAutomationsHoursAuditStaleMirrorIsPartial(t *testing.T) {
	testenv.Isolate(t)
	path := syncedAutomationDB(t, map[string]string{"a1": `{"id":"a1","name":"kept","actions":{"nodes":[{"id":"sms","type":"action","action":{"type":"send_sms","config":{}}}]}}`})
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().Exec(`UPDATE sync_state SET last_synced_at = ? WHERE resource_type = 'automations'`, time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	flags := &rootFlags{asJSON: true, maxAge: time.Minute, dataSource: "local"}
	cmd := newNovelAutomationsHoursAuditCmd(flags)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"--db", path})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	err = cmd.Execute()
	var got hoursAuditView
	if decodeErr := json.Unmarshal(out.Bytes(), &got); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if ExitCode(err) != 5 || !got.Partial || got.Checked != 1 || got.Total == nil || *got.Total != 1 || len(got.Failures) != 1 || !strings.Contains(got.Failures[0], "older than --max-age") {
		t.Fatalf("err=%v view=%+v", err, got)
	}
}

func (r *failingHoursAuditRows) Next() bool { r.index++; return r.index == 1 }
func (r *failingHoursAuditRows) Scan(dest ...any) error {
	*(dest[0].(*string)) = "a1"
	*(dest[1].(*string)) = `{"id":"a1","name":"kept","actions":{"nodes":[{"id":"sms","type":"action","action":{"type":"send_sms","config":{}}}]}}`
	return nil
}
func (r *failingHoursAuditRows) Err() error   { return r.err }
func (r *failingHoursAuditRows) Close() error { return nil }

func TestNovelAutomationsHoursAuditHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"automations", "hours-audit", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Usage:") || !strings.Contains(out.String(), "--published-only") {
		t.Fatalf("help: %s", out.String())
	}
}

func TestNovelAutomationsHoursAuditLivePaginated(t *testing.T) {
	testenv.Isolate(t)
	out, _, err := runNovel(t, []string{"automations", "hours-audit", "--data-source", "live", "--json"}, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/automations" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":{"data":[{"id":"a1","name":"Live","actions":{"nodes":[{"id":"sms","type":"action","action":{"type":"send_sms","config":{}}}]}}],"meta":{"page":1,"per_page":50,"total":1}}}`))
	})
	if err != nil {
		t.Fatal(err)
	}
	var got hoursAuditView
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if got.Checked != 1 || got.Summary.Automations != 1 || got.Summary.Steps != 1 || got.Steps[0].AutomationID != "a1" {
		t.Fatalf("got=%+v", got)
	}
}

func TestNovelAutomationsHoursAuditLiveTruncationIsPartial(t *testing.T) {
	for _, tc := range []struct {
		name    string
		handler func(http.ResponseWriter, *http.Request)
	}{
		{"repeated page", func(w http.ResponseWriter, r *http.Request) {
			rows := make([]map[string]any, 50)
			for i := range rows {
				rows[i] = map[string]any{"id": "same", "name": "A"}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": rows, "meta": map[string]any{"total": 100}})
		}},
		{"later page failure", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("page") == "2" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"bad page"}`))
				return
			}
			rows := make([]map[string]any, 50)
			for i := range rows {
				rows[i] = map[string]any{"id": string(rune('a' + i)), "name": "A"}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": rows, "meta": map[string]any{"total": 100}})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, _, err := runNovel(t, []string{"automations", "hours-audit", "--data-source", "live", "--json"}, tc.handler)
			var got hoursAuditView
			if decodeErr := json.Unmarshal([]byte(out), &got); decodeErr != nil {
				t.Fatal(decodeErr)
			}
			if ExitCode(err) != 5 || !got.Partial || got.Total == nil || *got.Total != 100 || got.Checked == *got.Total {
				t.Fatalf("err=%v got=%+v", err, got)
			}
		})
	}
}

func TestPaginatedGetMissingCursorAndPageCapAreTyped(t *testing.T) {
	t.Run("missing cursor", func(t *testing.T) {
		_, err := paginatedGet(context.Background(), paginationClientFunc(func(context.Context, string, map[string]string, map[string]string) (json.RawMessage, error) {
			return json.RawMessage(`{"data":[{"id":"a"}],"has_more":true}`), nil
		}), "/things", nil, nil, true, "cursor", "cursor", "limit", 0, "next_cursor", "has_more")
		var typed *paginationTruncationError
		if !errors.As(err, &typed) || !strings.Contains(err.Error(), "omits the next cursor") {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("page cap", func(t *testing.T) {
		calls := 0
		_, err := paginatedGet(context.Background(), paginationClientFunc(func(context.Context, string, map[string]string, map[string]string) (json.RawMessage, error) {
			calls++
			return json.RawMessage(`[{"id":"` + strconv.Itoa(calls) + `"}]`), nil
		}), "/things", map[string]string{"page": "1", "per_page": "1"}, nil, true, "page", "page", "per_page", 1, "", "")
		var typed *paginationTruncationError
		if !errors.As(err, &typed) || calls != paginationMaxPages {
			t.Fatalf("err=%v calls=%d", err, calls)
		}
	})
}

func TestPaginatedGetReportedTotalsMustRemainConsistent(t *testing.T) {
	for _, tc := range []struct {
		name       string
		secondMeta string
		want       string
	}{
		{"drifting total", `"total":1,`, "total changed from 2 to 1"},
		{"zero total", `"total":0,`, "total changed from 2 to 0"},
		{"missing total", "", "total changed from 2 to missing"},
		{"malformed total", `"total":"many",`, "total changed from 2 to invalid"},
		{"consistent total", `"total":2,`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			_, err := paginatedGet(context.Background(), paginationClientFunc(func(context.Context, string, map[string]string, map[string]string) (json.RawMessage, error) {
				calls++
				if calls == 1 {
					return json.RawMessage(`{"data":[{"id":"a"},{"id":"b"}],"meta":{"total":2,"has_more":true}}`), nil
				}
				return json.RawMessage(fmt.Sprintf(`{"data":[],"meta":{%s"has_more":false}}`, tc.secondMeta)), nil
			}), "/things", map[string]string{"page": "1", "per_page": "2"}, nil, true, "page", "page", "per_page", 2, "", "meta.has_more")
			var typed *paginationTruncationError
			if (tc.want != "") != errors.As(err, &typed) || (tc.want != "" && !strings.Contains(err.Error(), tc.want)) || (tc.want == "" && err != nil) {
				t.Fatalf("err=%v calls=%d", err, calls)
			}
		})
	}
}

func TestPaginatedGetAbsentTotalsUseFetchedCount(t *testing.T) {
	calls := 0
	data, err := paginatedGet(context.Background(), paginationClientFunc(func(context.Context, string, map[string]string, map[string]string) (json.RawMessage, error) {
		calls++
		if calls == 1 {
			return json.RawMessage(`{"data":[{"id":"a"},{"id":"b"}],"meta":{"has_more":true}}`), nil
		}
		return json.RawMessage(`{"data":[],"meta":{"has_more":false}}`), nil
	}), "/things", map[string]string{"page": "1", "per_page": "2"}, nil, true, "page", "page", "per_page", 2, "", "meta.has_more")
	var items []json.RawMessage
	if err != nil || json.Unmarshal(data, &items) != nil || len(items) != 2 {
		t.Fatalf("err=%v data=%s calls=%d", err, data, calls)
	}
}

func TestNovelAutomationsHoursAuditBehavior(t *testing.T) {
	testenv.Isolate(t)
	path := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	a := json.RawMessage(`{"id":"a1","name":"Lead follow-up","isActive":true,"graphVersion":2,"publishedActions":{"nodes":[{"id":"sms","type":"action","action":{"type":"send_sms","config":{"runWindow":{"days":["Mon","Tue","Wed","Thu","Fri"],"startHour":9,"endHour":17}}}},{"id":"email","type":"action","action":{"type":"send_email","config":{"runWindow":{"days":["Mon","Tue","Wed","Thu","Fri"],"startHour":9.5,"endHour":17}}}},{"id":"assign","type":"action","action":{"type":"assign_to_user","config":{}}}]},"actions":{"nodes":[]}}`)
	b := json.RawMessage(`{"id":"a2","name":"Draft flow","isActive":false,"actions":{"nodes":[{"id":"draft-sms","type":"action","action":{"type":"send_sms","config":{}}}]}}`)
	if err := db.Upsert("automations", "a1", a); err != nil {
		t.Fatal(err)
	}
	if err := db.Upsert("automations", "a2", b); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveSyncState("automations", "", 2); err != nil {
		t.Fatal(err)
	}
	db.Close()
	for _, tc := range []struct {
		name           string
		published      bool
		steps, without int
	}{{"all", false, 4, 2}, {"published", true, 3, 1}} {
		t.Run(tc.name, func(t *testing.T) {
			flags := &rootFlags{asJSON: true, dataSource: "local"}
			cmd := newNovelAutomationsHoursAuditCmd(flags)
			cmd.SilenceUsage = true
			args := []string{"--db", path}
			if tc.published {
				args = append(args, "--published-only")
			}
			cmd.SetArgs(args)
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&bytes.Buffer{})
			err := cmd.Execute()
			if err != nil {
				t.Fatal(err)
			}
			var got hoursAuditView
			if err := json.Unmarshal(out.Bytes(), &got); err != nil {
				t.Fatalf("%q: %v", out.String(), err)
			}
			if got.Summary.Automations != 2 || got.Summary.Steps != tc.steps || got.Summary.WithWindow != 2 || got.Summary.WithoutWindow != tc.without {
				t.Fatalf("summary %+v", got.Summary)
			}
			if got.Steps[0].Window != "Mon–Fri · 9:00 AM–5:00 PM" || got.Steps[1].Window != "Mon–Fri · 9:30 AM–5:00 PM" {
				t.Fatalf("steps %+v", got.Steps)
			}
		})
	}
}

func TestAppendHoursAuditDocPublishedOnlyAccounting(t *testing.T) {
	tests := []struct {
		name         string
		nodes        string
		wantSteps    int
		wantExcluded int
	}{
		{"published non-action nodes", `[{"id":"trigger","type":"trigger"},{"id":"delay","type":"control"}]`, 0, 1},
		{"published audited action", `[{"id":"sms","type":"action","action":{"type":"send_sms","config":{}}}]`, 1, 0},
		{"fully unpublished", `[]`, 0, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var doc automationAuditDoc
			if err := json.Unmarshal([]byte(`{"id":"a1","name":"A","graphVersion":2,"publishedActions":{"nodes":`+tc.nodes+`}}`), &doc); err != nil {
				t.Fatal(err)
			}
			view := hoursAuditView{Steps: make([]hoursAuditStep, 0)}
			if err := appendHoursAuditDoc(&view, doc, true); err != nil {
				t.Fatal(err)
			}
			represented := map[string]bool{}
			for _, step := range view.Steps {
				represented[step.AutomationID] = true
			}
			if len(view.Steps) != tc.wantSteps || view.Summary.Steps != tc.wantSteps || view.Summary.Unpublished != tc.wantExcluded || view.Summary.Automations != len(represented)+view.Summary.Unpublished {
				t.Fatalf("view=%+v represented=%v", view, represented)
			}
		})
	}
}

func TestNovelAutomationsHoursAuditEmptyAndMissing(t *testing.T) {
	for _, exists := range []bool{true, false} {
		t.Run(map[bool]string{true: "empty", false: "missing"}[exists], func(t *testing.T) {
			testenv.Isolate(t)
			path := filepath.Join(t.TempDir(), "data.db")
			if exists {
				db, err := store.Open(path)
				if err != nil {
					t.Fatal(err)
				}
				db.Close()
			}
			flags := &rootFlags{asJSON: true, dataSource: "local"}
			cmd := newNovelAutomationsHoursAuditCmd(flags)
			cmd.SetArgs([]string{"--db", path})
			var out, stderr bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&stderr)
			err := cmd.Execute()
			if ExitCode(err) != 5 {
				t.Fatalf("err=%v", err)
			}
			var got hoursAuditView
			if err := json.NewDecoder(&out).Decode(&got); err != nil {
				t.Fatal(err)
			}
			if got.Steps == nil || len(got.Steps) != 0 || got.Summary.Steps != 0 {
				t.Fatalf("got %+v", got)
			}
			if got.Synced || !strings.Contains(got.Hint, "sync --resources automations --db ") {
				t.Fatalf("sync metadata %+v", got)
			}
			if !got.Partial || len(got.Failures) == 0 {
				t.Fatalf("partial metadata %+v", got)
			}
			if strings.Contains(err.Error(), "%!w") {
				t.Fatalf("malformed wrapped error: %v", err)
			}
			if !exists && !strings.Contains(stderr.String(), "no local mirror") {
				t.Fatalf("stderr %q", stderr.String())
			}
		})
	}
}

func TestNovelAutomationsHoursAuditMalformedRowPrintsPartial(t *testing.T) {
	path := syncedAutomationDB(t, map[string]string{
		"a1": `{"id":"a1","name":"kept","actions":{"nodes":[{"id":"sms","type":"action","action":{"type":"send_sms","config":{}}}]}}`,
		"a2": `{`,
	})
	assertHoursAuditFailureFormats(t, path, "decoding automation a2", 1, 2)
}

func TestNovelAutomationsHoursAuditSyncTotalMismatchIsPartial(t *testing.T) {
	testenv.Isolate(t)
	path := syncedAutomationDB(t, map[string]string{"a1": `{"id":"a1","name":"kept","actions":{"nodes":[{"id":"sms","type":"action","action":{"type":"send_sms","config":{}}}]}}`})
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveSyncState("automations", "", 2); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	assertHoursAuditFailureFormats(t, path, "sync state reports total 2", 1, 2)
}

func TestNovelAutomationsHoursAuditIterationFailurePrintsPartial(t *testing.T) {
	path := syncedAutomationDB(t, map[string]string{"seed": `{"id":"seed"}`})
	oldQuery := queryHoursAuditRows
	queryHoursAuditRows = func(*store.Store) (hoursAuditRows, error) {
		return &failingHoursAuditRows{err: errors.New("forced iteration failure")}, nil
	}
	t.Cleanup(func() { queryHoursAuditRows = oldQuery })
	assertHoursAuditFailureFormats(t, path, "forced iteration failure", 1, 1)
}

func syncedAutomationDB(t *testing.T, rows map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	for id, raw := range rows {
		if json.Valid([]byte(raw)) {
			if err := db.Upsert("automations", id, json.RawMessage(raw)); err != nil {
				t.Fatal(err)
			}
		} else if _, err := db.DB().Exec(`INSERT INTO resources(id,resource_type,data,synced_at,updated_at) VALUES(?, 'automations', ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, id, raw); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.SaveSyncState("automations", "", len(rows)); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertHoursAuditFailureFormats(t *testing.T, path, reason string, checked, total int) {
	t.Helper()
	for _, human := range []bool{false, true} {
		name := "json"
		flags := &rootFlags{asJSON: true, dataSource: "local"}
		if human {
			name = "human"
			flags = &rootFlags{dataSource: "local"}
		}
		t.Run(name, func(t *testing.T) {
			oldHumanFriendly := humanFriendly
			humanFriendly = human
			t.Cleanup(func() { humanFriendly = oldHumanFriendly })
			cmd := newNovelAutomationsHoursAuditCmd(flags)
			cmd.SilenceUsage = true
			cmd.SetArgs([]string{"--db", path})
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&bytes.Buffer{})
			err := cmd.Execute()
			if ExitCode(err) != 5 {
				t.Fatalf("err=%v", err)
			}
			if human {
				if !strings.Contains(out.String(), "WARNING:") || !strings.Contains(out.String(), reason) {
					t.Fatalf("out=%q", out.String())
				}
				return
			}
			var got hoursAuditView
			if err := json.Unmarshal(out.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if !got.Partial || got.Checked != checked || got.Total == nil || *got.Total != total || len(got.Failures) != 1 || !strings.Contains(got.Failures[0], reason) || len(got.Steps) != checked {
				t.Fatalf("got=%+v", got)
			}
		})
	}
}
