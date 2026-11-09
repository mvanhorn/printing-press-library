// Copyright 2026 Paul Taramona and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

// PATCH(novel1219-restored-commands): regressions for the four commands restored from the public library entry
// (Greptile PR #2450 round 2 P1) and for the hardening applied while porting them.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/store"
)

func TestNovel1219RestoredCommandsAreServed(t *testing.T) {
	root := RootCmd()
	for _, path := range [][]string{
		{"reports", "compare"},
		{"reports", "scorecard"},
		{"dialer", "coverage"},
		{"automations", "hours-audit"},
	} {
		cmd, _, err := root.Find(path)
		if err != nil || cmd == nil || cmd.Name() != path[1] || cmd.Parent() == nil || cmd.Parent().Name() != path[0] {
			t.Fatalf("%s is not served: %v", strings.Join(path, " "), err)
		}
		if cmd.RunE == nil || isNovelScaffoldCommand(cmd) {
			t.Fatalf("%s is a scaffold, not the ported command", strings.Join(path, " "))
		}
		if cmd.Annotations["mcp:read-only"] != "true" || cmd.Annotations["pp:data-source"] == "" {
			t.Fatalf("%s lost its annotations: %v", strings.Join(path, " "), cmd.Annotations)
		}
	}
}

// Hardening: the public entry sent assignedTo to every report and pipelineId to any report, so a report without
// that filter silently answered for the whole account.
func TestNovel1219ReportsCompareRefusesAFilterTheReportDoesNotHave(t *testing.T) {
	for _, args := range [][]string{
		{"reports", "compare", "funnel", "--pipeline", "p1", "--assigned-to", "u1", "--json"},
		{"reports", "compare", "attribution", "--assigned-to", "u1", "--json"},
		{"reports", "compare", "sms-delivery", "--assigned-to", "u1", "--json"},
		{"reports", "compare", "speed-to-lead", "--pipeline", "p1", "--json"},
	} {
		t.Run(strings.Join(args[2:], " "), func(t *testing.T) {
			testenv.Isolate(t)
			calls := 0
			_, _, err := runNovel(t, args, func(w http.ResponseWriter, r *http.Request) { calls++ })
			if ExitCode(err) != 2 || calls != 0 {
				t.Fatalf("want a usage error before any request; err=%v calls=%d", err, calls)
			}
		})
	}
}

func TestNovel1219ReportsCompareSendsEachReportsAssigneeParameter(t *testing.T) {
	for report, param := range map[string]string{"call-performance": "userId", "appointments": "assignedTo", "speed-to-lead": "assignedTo"} {
		t.Run(report, func(t *testing.T) {
			testenv.Isolate(t)
			_, _, err := runNovel(t, []string{"reports", "compare", report, "--range", "7d", "--assigned-to", "u1", "--json"}, func(w http.ResponseWriter, r *http.Request) {
				q := r.URL.Query()
				if q.Get(param) != "u1" {
					t.Errorf("%s: %s not sent: %s", report, param, r.URL.RawQuery)
				}
				for _, other := range []string{"assignedTo", "userId"} {
					if other != param && q.Has(other) {
						t.Errorf("%s: unexpected %s: %s", report, other, r.URL.RawQuery)
					}
				}
				_, _ = w.Write([]byte(`{"data":{"total":3}}`))
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func automationsPage(ids []string, total int) []byte {
	rows := make([]map[string]any, len(ids))
	for i, id := range ids {
		rows[i] = map[string]any{"id": id, "name": "A " + id, "actions": map[string]any{"nodes": []any{}}}
	}
	body, _ := json.Marshal(map[string]any{"data": rows, "meta": map[string]any{"total": total}})
	return body
}

func automationsPageWithMeta(ids []string, meta map[string]any) []byte {
	rows := make([]map[string]any, len(ids))
	for i, id := range ids {
		rows[i] = map[string]any{"id": id, "name": "A " + id, "actions": map[string]any{"nodes": []any{}}}
	}
	body, _ := json.Marshal(map[string]any{"data": rows, "meta": meta})
	return body
}

func automationIDs(prefix string, n int) []string {
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("%s%03d", prefix, i)
	}
	return ids
}

// The public entry's paginatedGet tests, pinned against the reprint's own page walk.
func TestNovel1219HoursAuditLivePageWalk(t *testing.T) {
	cases := []struct {
		name        string
		handler     func(http.ResponseWriter, *http.Request)
		wantPartial bool
		wantChecked int
		reason      string
	}{
		{"two full pages then a short one", func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Query().Get("page") {
			case "1":
				_, _ = w.Write(automationsPage(automationIDs("a", 50), 120))
			case "2":
				_, _ = w.Write(automationsPage(automationIDs("b", 50), 120))
			default:
				_, _ = w.Write(automationsPage(automationIDs("c", 20), 120))
			}
		}, false, 120, ""},
		{"absent totals use the fetched count", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("page") == "1" {
				body, _ := json.Marshal(map[string]any{"data": []map[string]any{{"id": "x1", "name": "X"}}})
				_, _ = w.Write(body)
				return
			}
			t.Errorf("a short page without a total must end the walk")
		}, false, 1, ""},
		{"a total that changes between pages", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("page") == "1" {
				_, _ = w.Write(automationsPage(automationIDs("a", 50), 120))
				return
			}
			_, _ = w.Write(automationsPage(automationIDs("b", 50), 90))
		}, true, 50, "total changed from 120 to 90"},
		// r15 (Codex novel1219-r14 P2): a total that disappears, turns malformed or appears after page 1 is truncation,
		// as in the public entry's paginatedGet.
		{"a total that disappears on a later page", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("page") == "1" {
				_, _ = w.Write(automationsPage(automationIDs("a", 50), 100))
				return
			}
			_, _ = w.Write(automationsPageWithMeta(automationIDs("b", 50), map[string]any{"page": 2}))
		}, true, 50, "total changed from 100 to missing"},
		{"a total that turns malformed on a later page", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("page") == "1" {
				_, _ = w.Write(automationsPage(automationIDs("a", 50), 100))
				return
			}
			_, _ = w.Write(automationsPageWithMeta(automationIDs("b", 50), map[string]any{"total": "one hundred"}))
		}, true, 50, "total changed from 100 to invalid"},
		{"a negative total on a later page", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("page") == "1" {
				_, _ = w.Write(automationsPage(automationIDs("a", 50), 100))
				return
			}
			_, _ = w.Write(automationsPageWithMeta(automationIDs("b", 50), map[string]any{"total": -1}))
		}, true, 50, "total changed from 100 to invalid"},
		{"a total that appears after a page without one", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("page") == "1" {
				_, _ = w.Write(automationsPageWithMeta(automationIDs("a", 50), map[string]any{"page": 1}))
				return
			}
			_, _ = w.Write(automationsPage(automationIDs("b", 20), 70))
		}, true, 50, "total changed from missing to 70"},
		{"a page that repeats an earlier automation", func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write(automationsPage(automationIDs("a", 50), 120))
		}, true, 50, "repeated automation"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Isolate(t)
			out, _, err := runNovel(t, []string{"automations", "hours-audit", "--data-source", "live", "--json"}, tc.handler)
			var got hoursAuditView
			if decodeErr := json.Unmarshal([]byte(out), &got); decodeErr != nil {
				t.Fatalf("%v: %s", decodeErr, out)
			}
			if tc.wantPartial {
				if ExitCode(err) != 5 || !got.Partial || !strings.Contains(strings.Join(got.Failures, "; "), tc.reason) {
					t.Fatalf("err=%v got=%+v", err, got)
				}
				return
			}
			if err != nil || got.Partial || got.Checked != tc.wantChecked || got.Total == nil || *got.Total != tc.wantChecked {
				t.Fatalf("err=%v got=%+v", err, got)
			}
		})
	}
}

// Hardening: like the contacts mirror (novel1219 r3), the automations mirror counts only after a finished sync attempt.
func TestNovel1219HoursAuditUnfinishedSyncAttemptIsPartial(t *testing.T) {
	testenv.Isolate(t)
	path := syncedAutomationDB(t, map[string]string{"a1": `{"id":"a1","name":"kept","actions":{"nodes":[{"id":"sms","type":"action","action":{"type":"send_sms","config":{}}}]}}`})
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveSyncProgress("automations", "", 1); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	assertHoursAuditFailureFormats(t, path, "sync attempt did not finish", 1, 1)
}
