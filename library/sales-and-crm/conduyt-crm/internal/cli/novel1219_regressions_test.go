// Copyright 2026 Paul Taramona and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

// PATCH(novel1219-regressions): one regression per Greptile finding on PR #2450 (round 1: 6 P1 + 2 P2), each against
// the behaviour the reprint's simple novel commands had and the public library entry's hardened ones restore.

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/store"
	"gopkg.in/yaml.v3"
)

// Greptile P1 "Smart-list checks fail": members are read through GET /contacts?smartListId, never the add/remove-only
// /smart-lists/{id}/contacts route.
func TestNovel1219SendCheckListReadsMembersThroughTheContactsRoute(t *testing.T) {
	testenv.Isolate(t)
	const list = "7c1d2e3f-0000-4000-8000-000000000000"
	var paths []string
	out, _, err := runNovel(t, []string{"send-check", "--list", list, "--json"}, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path+"?"+r.URL.RawQuery)
		switch r.URL.Path {
		case "/contacts":
			if r.URL.Query().Get("smartListId") != list {
				t.Errorf("contacts read without smartListId: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"data":[{"id":"c1","phone":"+12125550101","lineType":"mobile"}],"meta":{"total":1}}`))
		case "/contacts/dnc-status":
			writeDNCResponse(t, w, r)
		default:
			http.NotFound(w, r)
		}
	})
	if err != nil {
		t.Fatalf("err=%v out=%s", err, out)
	}
	for _, p := range paths {
		if strings.HasPrefix(p, "/smart-lists/") {
			t.Fatalf("send-check called the add/remove-only members route: %v", paths)
		}
	}
	if !strings.Contains(compactTestJSON(out), `"verdict":"go"`) {
		t.Fatalf("out=%s", out)
	}
}

// sendCheckTagAudience serves n tagged contacts, 200 per page; the LAST contact is SMS-blocked in dnc-status.
func sendCheckTagAudience(t *testing.T, n int, pages *atomic.Int32) http.HandlerFunc {
	last := fmt.Sprintf("c-%d", n-1)
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/contacts":
			pages.Add(1)
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))
			perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
			start := (page - 1) * perPage
			items := []map[string]any{}
			for i := start; i < n && i < start+perPage; i++ {
				items = append(items, map[string]any{"id": fmt.Sprintf("c-%d", i), "phone": fmt.Sprintf("+1212555%04d", i), "lineType": "mobile"})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": items, "meta": map[string]any{"total": n}})
		case "/contacts/dnc-status":
			writeDNCResponse(t, w, r, last)
		default:
			http.NotFound(w, r)
		}
	}
}

// Greptile P1 "Unchecked contacts pass the gate": a tag of 201 contacts is paged past page 1, so a blocked 201st
// contact makes the verdict no_go (exit 2) instead of a go over the first 200.
func TestNovel1219SendCheckTagPaginatesPastPageOne(t *testing.T) {
	testenv.Isolate(t)
	var pages atomic.Int32
	out, _, err := runNovel(t, []string{"send-check", "--tag", "big", "--limit", "500", "--json"}, sendCheckTagAudience(t, 201, &pages))
	var view sendCheckView
	if decodeErr := json.Unmarshal([]byte(out), &view); decodeErr != nil {
		t.Fatalf("out=%s", out)
	}
	if ExitCode(err) != 2 || view.Verdict != "no_go" || view.Checked != 201 || view.Summary.DNC != 1 || pages.Load() != 2 {
		t.Fatalf("err=%v code=%d verdict=%s checked=%d dnc=%d pages=%d", err, ExitCode(err), view.Verdict, view.Checked, view.Summary.DNC, pages.Load())
	}
}

// Greptile P1 "Unchecked contacts pass the gate", second half: when --limit leaves contacts unchecked the result is
// inconclusive and non-zero, never a go.
func TestNovel1219SendCheckLimitLeavingContactsUncheckedIsInconclusive(t *testing.T) {
	testenv.Isolate(t)
	var pages atomic.Int32
	out, _, err := runNovel(t, []string{"send-check", "--tag", "big", "--limit", "200", "--json"}, sendCheckTagAudience(t, 201, &pages))
	var view sendCheckView
	if decodeErr := json.Unmarshal([]byte(out), &view); decodeErr != nil {
		t.Fatalf("out=%s", out)
	}
	if ExitCode(err) != 5 || view.Verdict != "inconclusive" || !view.Partial || view.Checked != 200 || view.Total == nil || *view.Total != 201 {
		t.Fatalf("err=%v code=%d view=%+v", err, ExitCode(err), view)
	}
}

// Greptile P1 "DNC checks miss blocked numbers": DNC verdicts come from the contact-specific dnc-status lookup, so a
// contact whose number only shares the last 7 digits with a blocked one is NOT blocked, and the /dnc list is never read.
func TestNovel1219SendCheckRefusesALastSevenDigitMatch(t *testing.T) {
	testenv.Isolate(t)
	var paths []string
	out, _, err := runNovel(t, []string{"send-check", "--tag", "pair", "--json"}, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/contacts":
			// Same last 7 digits (555-0101), different area codes: only "blocked" is on the DNC list.
			_, _ = w.Write([]byte(`{"data":[{"id":"blocked","phone":"+12125550101","lineType":"mobile"},{"id":"clean","phone":"+13105550101","lineType":"mobile"}],"meta":{"total":2}}`))
		case "/contacts/dnc-status":
			writeDNCResponse(t, w, r, "blocked")
		default:
			http.NotFound(w, r)
		}
	})
	var view sendCheckView
	if decodeErr := json.Unmarshal([]byte(out), &view); decodeErr != nil {
		t.Fatalf("out=%s", out)
	}
	verdicts := map[string]string{}
	for _, v := range view.Verdicts {
		verdicts[v.ContactID] = v.Verdict
	}
	if ExitCode(err) != 2 || verdicts["blocked"] != "dnc" || verdicts["clean"] != "ok" {
		t.Fatalf("err=%v verdicts=%v", err, verdicts)
	}
	for _, p := range paths {
		if p == "/dnc" {
			t.Fatalf("send-check read the paged /dnc list: %v", paths)
		}
	}
}

// Greptile P1 "DNC checks miss blocked numbers", completeness: a dnc-status answer that omits a requested contact is
// inconclusive, never a clean verdict for it.
func TestNovel1219SendCheckIncompleteDNCAnswerIsInconclusive(t *testing.T) {
	testenv.Isolate(t)
	out, _, err := runNovel(t, []string{"send-check", "--tag", "pair", "--json"}, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/contacts":
			_, _ = w.Write([]byte(`{"data":[{"id":"a","phone":"+12125550101","lineType":"mobile"},{"id":"b","phone":"+13105550102","lineType":"mobile"}],"meta":{"total":2}}`))
		case "/contacts/dnc-status":
			_, _ = w.Write([]byte(`{"data":{"statuses":{"a":{"dnc":false,"litigator":false,"voiceBlocked":false,"smsBlocked":false}},"requested":2,"checked":1,"missing":1,"truncated":false}}`))
		default:
			http.NotFound(w, r)
		}
	})
	if ExitCode(err) != 5 || !strings.Contains(compactTestJSON(out), `"verdict":"inconclusive"`) {
		t.Fatalf("err=%v out=%s", err, out)
	}
}

// Greptile P1 "Import error counts are wrong": blame reads the documented error tab with a page size the route allows
// (at most 100), and an ordinary row is never counted as an error "(no reason recorded)".
func TestNovel1219ImportsBlameReadsTheErrorTabWithASupportedPageSize(t *testing.T) {
	testenv.Isolate(t)
	var queries []string
	out, _, err := runNovel(t, []string{"imports", "blame", "job", "--json"}, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/imports/job/rows" {
			queries = append(queries, r.URL.Query().Get("tab")+":"+r.URL.Query().Get("per_page"))
		}
		blameHandler(t, "", blameErrorRows([]map[string]any{blameErrorRow(1, "invalid phone")}, 1, nil), "")(w, r)
	})
	if err != nil {
		t.Fatalf("err=%v out=%s", err, out)
	}
	if len(queries) != 1 || queries[0] != "error:100" {
		t.Fatalf("rows queries=%v, want one error-tab page of 100", queries)
	}
	if c := compactTestJSON(out); !strings.Contains(c, `"errors_by_reason":{"invalid phone":1}`) || strings.Contains(c, "no reason recorded") {
		t.Fatalf("out=%s", c)
	}
}

func watchTestHandler(t *testing.T, side func(w http.ResponseWriter)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/imports/job":
			_, _ = w.Write([]byte(`{"data":{"id":"job","status":"completed","totalRows":1,"createdRows":1,"updatedRows":0,"skippedRows":0,"errorRows":0,"processedRows":1}}`))
		case "/imports/job/launch-monitor":
			_, _ = w.Write([]byte(`{"data":{"stage":"complete"}}`))
		case "/imports/job/side-effects":
			side(w)
		case "/reports/sms-delivery":
			t.Error("--verify must read the job-scoped side effects, not the account-wide delivery report")
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	}
}

// Greptile P1 "Failed verification reports success": a verification read that fails, or side effects with queued or
// failed sends, make `imports watch --verify` partial and non-zero; complete side effects verify with exit 0.
func TestNovel1219ImportsWatchVerifyFailsNonZero(t *testing.T) {
	for _, tc := range []struct {
		name string
		side func(w http.ResponseWriter)
		code int
		want string
	}{
		{"side effects fetch fails", func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusNotFound) // not retried; the failed fetch keeps its typed class (3)
			_, _ = w.Write([]byte(`{"error":"gone"}`))
		}, 3, "missing job side effects"},
		{"queued sends", func(w http.ResponseWriter) {
			_, _ = w.Write([]byte(`{"data":{"total":2,"completed":1,"failed":0,"retryableFailed":0,"exhaustedFailed":0,"queued":1,"held":0,"actionRequired":false,"isComplete":false}}`))
		}, 5, "side effects are not complete"},
		{"failed sends", func(w http.ResponseWriter) {
			_, _ = w.Write([]byte(`{"data":{"total":2,"completed":1,"failed":1,"retryableFailed":1,"exhaustedFailed":0,"queued":0,"held":0,"actionRequired":false,"isComplete":false}}`))
		}, 5, "side effects are not complete"},
		{"complete", func(w http.ResponseWriter) {
			_, _ = w.Write([]byte(`{"data":{"total":1,"completed":1,"failed":0,"retryableFailed":0,"exhaustedFailed":0,"queued":0,"held":0,"actionRequired":false,"isComplete":true}}`))
		}, 0, `"verified":true`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Isolate(t)
			out, _, err := runNovel(t, []string{"imports", "watch", "job", "--verify", "--interval", "1", "--json"}, watchTestHandler(t, tc.side))
			code := 0
			if err != nil {
				code = ExitCode(err)
			}
			if code != tc.code || !strings.Contains(compactTestJSON(out), tc.want) {
				t.Fatalf("err=%v code=%d out=%s", err, code, out)
			}
		})
	}
}

// --interval/--timeout accept both the public entry's durations and the reprint's bare seconds.
func TestNovel1219ImportsWatchIntervalTakesDurationsAndSeconds(t *testing.T) {
	for _, value := range []string{"10", "10s", "1m"} {
		var d secondsOrDuration
		if err := d.Set(value); err != nil || d <= 0 {
			t.Fatalf("Set(%q) = %v, %v", value, d, err)
		}
	}
	var d secondsOrDuration
	if d.Set("ten") == nil {
		t.Fatal("Set(ten) accepted a non-duration")
	}
	if err := d.Set("10"); err != nil || d.String() != "10s" {
		t.Fatalf("bare seconds = %s, %v", d.String(), err)
	}
}

// Greptile P1 "Different messages look duplicated": two different bodies to one contact that share an 80-character
// introduction are not a double send; the same full body twice is.
func TestNovel1219DripsAuditGroupsByTheFullBody(t *testing.T) {
	testenv.Isolate(t)
	dbPath := t.TempDir() + "/data.db"
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	intro := strings.Repeat("Hi Sam, this is a reminder about your appointment with our office. ", 2)
	for id, body := range map[string]string{"m1": intro + "Reply C to confirm.", "m2": intro + "Reply R to reschedule.", "m3": "Same body", "m4": "Same body"} {
		contact := "c-intro"
		if id == "m3" || id == "m4" {
			contact = "c-dup"
		}
		raw, _ := json.Marshal(map[string]any{"id": id, "contactId": contact, "direction": "outbound", "channel": "sms", "body": body, "createdAt": "2026-07-02T10:00:00Z"})
		if err := db.Upsert("messages", id, raw); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	out, _, err := runNovel(t, []string{"drips", "audit", "--db", dbPath, "--json"}, nil)
	if ExitCode(err) != 2 {
		t.Fatalf("err=%v out=%s, want exit 2 for the one real double send", err, out)
	}
	var got struct {
		Summary     map[string]any   `json:"summary"`
		DoubleSends []map[string]any `json:"doubleSends"`
	}
	if json.Unmarshal([]byte(out), &got) != nil || len(got.DoubleSends) != 1 || got.DoubleSends[0]["contactId"] != "c-dup" || got.DoubleSends[0]["count"] != float64(2) {
		t.Fatalf("out=%s, want only c-dup flagged", out)
	}
}

// Greptile P2 "Cost estimate can undercount": a mirror that exists but was never synced is not counted as the
// audience; the estimate counts live instead, and an incomplete live count is a partial lower bound with a non-zero exit.
func TestNovel1219VerifyLineTypeEstimateNeverTrustsAnUnsyncedMirror(t *testing.T) {
	testenv.Isolate(t)
	dbPath := t.TempDir() + "/data.db"
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	var contactsReads atomic.Int32
	out, _, err := runNovel(t, []string{"contacts", "verify-line-type", "--estimate", "--db", dbPath, "--json"}, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/contacts" {
			http.NotFound(w, r)
			return
		}
		contactsReads.Add(1)
		// One short page while the metadata reports 5 contacts: the count is a lower bound.
		_, _ = w.Write([]byte(`{"data":[{"id":"c1","phone":"+12125550101"}],"meta":{"total":5}}`))
	})
	var view verifyEstimate
	if decodeErr := json.Unmarshal([]byte(out), &view); decodeErr != nil {
		t.Fatalf("out=%s", out)
	}
	if contactsReads.Load() == 0 || view.Synced || ExitCode(err) != 5 || !view.Partial || view.Unverified != 1 {
		t.Fatalf("err=%v reads=%d view=%+v", err, contactsReads.Load(), view)
	}
	testenv.Isolate(t)
	table, _, _ := runNovel(t, []string{"contacts", "verify-line-type", "--estimate", "--db", dbPath, "--human-friendly"}, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":"c1","phone":"+12125550101"}],"meta":{"total":5}}`))
	})
	if !strings.Contains(table, "lower bound") {
		t.Fatalf("human estimate does not label the partial count a lower bound: %s", table)
	}
}

// Greptile P2 "Stalled verification keeps running": a batch with missing fields, or one whose remaining does not
// decrease, stops the paid loop at once instead of re-requesting up to --max-batches.
func TestNovel1219VerifyLineTypeRunStopsOnAnInvalidOrStalledBatch(t *testing.T) {
	for _, tc := range []struct {
		name    string
		batches []string
		posts   int32
		want    string
	}{
		{"missing fields", []string{`{"data":{"done":false}}`}, 1, "missing or invalid verified number"},
		{"stalled", []string{`{"data":{"done":false,"verified":50,"remaining":100,"byLineType":{"mobile":50}}}`, `{"data":{"done":false,"verified":0,"remaining":100,"byLineType":{}}}`}, 2, "verification made no progress"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Isolate(t)
			var posts atomic.Int32
			out, _, err := runNovel(t, []string{"contacts", "verify-line-type", "run", "--until-done", "--smart-list", "7c1d2e3f-0000-4000-8000-000000000000", "--json"}, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/contacts/verify-line-type" {
					http.NotFound(w, r)
					return
				}
				n := posts.Add(1)
				body := tc.batches[len(tc.batches)-1]
				if int(n) <= len(tc.batches) {
					body = tc.batches[n-1]
				}
				_, _ = w.Write([]byte(body))
			})
			if ExitCode(err) != 5 || posts.Load() != tc.posts || !strings.Contains(out, tc.want) {
				t.Fatalf("err=%v posts=%d out=%s", err, posts.Load(), out)
			}
		})
	}
}

// The reprint's --contact-ids scope stays available on the paid loop and is sent as contactIds.
func TestNovel1219VerifyLineTypeRunSendsContactIDs(t *testing.T) {
	testenv.Isolate(t)
	var body map[string]any
	_, _, err := runNovel(t, []string{"contacts", "verify-line-type", "run", "--until-done", "--contact-ids", "9e778307-e575-420b-b02c-dc9cc564febd,3f8a0c4e-1111-4222-8333-444455556666", "--json"}, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"data":{"done":true,"verified":2,"remaining":0,"byLineType":{"mobile":2}}}`))
	})
	if err != nil || fmt.Sprint(body["contactIds"]) != "[9e778307-e575-420b-b02c-dc9cc564febd 3f8a0c4e-1111-4222-8333-444455556666]" {
		t.Fatalf("err=%v body=%v", err, body)
	}
	testenv.Isolate(t)
	if _, _, err := runNovel(t, []string{"contacts", "verify-line-type", "run", "--until-done", "--contact-ids", "9e778307-e575-420b-b02c-dc9cc564febd", "--smart-list", "7c1d2e3f-0000-4000-8000-000000000000"}, nil); ExitCode(err) != 2 {
		t.Fatalf("--contact-ids with --smart-list: err=%v, want a usage error", err)
	}
}

// The terminal job's `errors` field is its issue log (an array), never a row counter: watch reads errorRows only, so a
// job carrying an issue log is not "missing invalid errors" (seen live on the demo workspace).
func TestNovel1219ImportsWatchIgnoresTheJobsIssueLog(t *testing.T) {
	testenv.Isolate(t)
	out, _, err := runNovel(t, []string{"imports", "watch", "job", "--verify", "--interval", "1", "--json"}, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/imports/job" {
			_, _ = w.Write([]byte(`{"data":{"id":"job","status":"completed","totalRows":1,"createdRows":0,"updatedRows":1,"skippedRows":0,"errorRows":0,"processedRows":1,"errors":[{"row":1,"code":"info","message":"junk email dropped"}]}}`))
			return
		}
		watchTestHandler(t, func(w http.ResponseWriter) {
			_, _ = w.Write([]byte(`{"data":{"total":0,"completed":0,"failed":0,"retryableFailed":0,"exhaustedFailed":0,"queued":0,"held":0,"actionRequired":false,"isComplete":false}}`))
		})(w, r)
	})
	if c := compactTestJSON(out); err != nil || !strings.Contains(c, `"verified":true`) || !strings.Contains(c, `"no_side_effects":true`) || strings.Contains(c, "invalid errors") {
		t.Fatalf("err=%v out=%s", err, c)
	}
}

// ---- Codex novel1219-r1 regressions (r2) ----

// syncedContactsMirror writes a synced, fresh contacts mirror holding the given contact rows (id -> JSON).
func syncedContactsMirror(t *testing.T, rows map[string]string) string {
	t.Helper()
	dbPath := t.TempDir() + "/data.db"
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	for id, raw := range rows {
		if err := db.Upsert("contacts", id, json.RawMessage(raw)); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.SaveSyncState("contacts", "", len(rows)); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	return dbPath
}

// sendCheckLiveHandler serves one live contact for any /contacts read and clean dnc-status answers, counting reads.
func sendCheckLiveHandler(t *testing.T, contactsReads *atomic.Int32) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/contacts":
			contactsReads.Add(1)
			_, _ = w.Write([]byte(`{"data":[{"id":"live-1","phone":"+12125550101","lineType":"mobile"}],"meta":{"total":1}}`))
		case "/contacts/dnc-status":
			writeDNCResponse(t, w, r)
		default:
			http.NotFound(w, r)
		}
	}
}

// Codex r1 P1 (send-check audience): --data-source live never reads the mirror, even a synced, fresh one.
func TestNovel1219SendCheckDataSourceLiveSkipsTheMirror(t *testing.T) {
	testenv.Isolate(t)
	db := syncedContactsMirror(t, map[string]string{"local-1": `{"id":"local-1","phone":"+12125550102","lineType":"mobile","tags":[{"id":"t1","name":"leads"}]}`})
	var reads atomic.Int32
	out, _, err := runNovel(t, []string{"send-check", "--tag", "leads", "--db", db, "--data-source", "live", "--json"}, sendCheckLiveHandler(t, &reads))
	if err != nil || reads.Load() == 0 || !strings.Contains(out, "live-1") || strings.Contains(out, "local-1") {
		t.Fatalf("err=%v reads=%d out=%s", err, reads.Load(), out)
	}
}

// Codex r1 P1: smart-list membership is not in synced contacts, so --list is read live even from a fresh mirror,
// and --list under --data-source local is inconclusive (exit 5), never an empty audience.
func TestNovel1219SendCheckListIsAlwaysLive(t *testing.T) {
	testenv.Isolate(t)
	db := syncedContactsMirror(t, map[string]string{"local-1": `{"id":"local-1","phone":"+12125550102","lineType":"mobile"}`})
	var reads atomic.Int32
	out, _, err := runNovel(t, []string{"send-check", "--list", "7c1d2e3f-0000-4000-8000-000000000000", "--db", db, "--json"}, sendCheckLiveHandler(t, &reads))
	if err != nil || reads.Load() == 0 || !strings.Contains(out, "live-1") {
		t.Fatalf("auto: err=%v reads=%d out=%s", err, reads.Load(), out)
	}
	testenv.Isolate(t)
	reads.Store(0)
	out, _, err = runNovel(t, []string{"send-check", "--list", "7c1d2e3f-0000-4000-8000-000000000000", "--db", db, "--data-source", "local", "--json"}, sendCheckLiveHandler(t, &reads))
	c := compactTestJSON(out)
	if ExitCode(err) != 5 || reads.Load() != 0 || !strings.Contains(c, `"verdict":"inconclusive"`) || strings.Contains(c, `"verdict":"empty"`) {
		t.Fatalf("local: err=%v reads=%d out=%s", err, reads.Load(), c)
	}
}

// Codex r1 P1: the contacts route serializes tags as objects, so a fresh mirror matches a tag by name (or id) and
// answers locally, with DNC still checked live.
func TestNovel1219SendCheckLocalTagMatchesTagObjects(t *testing.T) {
	testenv.Isolate(t)
	db := syncedContactsMirror(t, map[string]string{
		"local-1": `{"id":"local-1","phone":"+12125550102","lineType":"mobile","tags":[{"id":"t1","name":"leads","color":"#fff"}]}`,
		"local-2": `{"id":"local-2","phone":"+12125550103","lineType":"mobile","tags":[{"id":"t2","name":"other"}]}`,
	})
	var reads, dnc atomic.Int32
	out, _, err := runNovel(t, []string{"send-check", "--tag", "leads", "--db", db, "--json"}, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/contacts":
			reads.Add(1)
			http.NotFound(w, r)
		case "/contacts/dnc-status":
			dnc.Add(1)
			writeDNCResponse(t, w, r)
		default:
			http.NotFound(w, r)
		}
	})
	if err != nil || reads.Load() != 0 || dnc.Load() != 1 || !strings.Contains(out, "local-1") || strings.Contains(out, "local-2") {
		t.Fatalf("err=%v reads=%d dnc=%d out=%s", err, reads.Load(), dnc.Load(), out)
	}
}

// Codex r1 P1: a fresh mirror with no contact in the tag cannot prove an empty audience: auto reads the tag live,
// local answers inconclusive (exit 5), never the clean-empty exit 0 without DNC.
func TestNovel1219SendCheckUnresolvedLocalTagIsNeverEmpty(t *testing.T) {
	testenv.Isolate(t)
	db := syncedContactsMirror(t, map[string]string{"local-1": `{"id":"local-1","phone":"+12125550102","lineType":"mobile","tags":[{"id":"t2","name":"other"}]}`})
	var reads atomic.Int32
	out, _, err := runNovel(t, []string{"send-check", "--tag", "leads", "--db", db, "--json"}, sendCheckLiveHandler(t, &reads))
	if err != nil || reads.Load() == 0 || !strings.Contains(out, "live-1") {
		t.Fatalf("auto: err=%v reads=%d out=%s", err, reads.Load(), out)
	}
	testenv.Isolate(t)
	reads.Store(0)
	out, _, err = runNovel(t, []string{"send-check", "--tag", "leads", "--db", db, "--data-source", "local", "--json"}, sendCheckLiveHandler(t, &reads))
	if c := compactTestJSON(out); ExitCode(err) != 5 || reads.Load() != 0 || strings.Contains(c, `"verdict":"empty"`) || !strings.Contains(c, "cannot resolve this audience") {
		t.Fatalf("local: err=%v reads=%d out=%s", err, reads.Load(), c)
	}
}

// Codex r1 P1 (held side effects): the route reports held-only as isComplete:true with actionRequired:true; watch
// --verify and blame fail on any held, queued, retryable, exhausted or failed send and on actionRequired.
func TestNovel1219HeldSideEffectsNeverVerify(t *testing.T) {
	for _, tc := range []struct{ name, side, want string }{
		{"held only", `{"data":{"total":1,"completed":0,"failed":0,"retryableFailed":0,"exhaustedFailed":0,"queued":0,"held":1,"actionRequired":true,"isComplete":true}}`, "1 held sends"},
		{"action required", `{"data":{"total":1,"completed":1,"failed":0,"retryableFailed":0,"exhaustedFailed":0,"queued":0,"held":0,"actionRequired":true,"isComplete":true}}`, "action required"},
		{"exhausted", `{"data":{"total":2,"completed":1,"failed":1,"retryableFailed":0,"exhaustedFailed":1,"queued":0,"held":0,"actionRequired":false,"isComplete":true}}`, "1 exhausted failed sends"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Isolate(t)
			out, _, err := runNovel(t, []string{"imports", "watch", "job", "--verify", "--interval", "1", "--json"}, watchTestHandler(t, func(w http.ResponseWriter) { _, _ = w.Write([]byte(tc.side)) }))
			if ExitCode(err) != 5 || !strings.Contains(out, tc.want) || strings.Contains(compactTestJSON(out), `"verified":true`) {
				t.Fatalf("watch: err=%v out=%s", err, out)
			}
			testenv.Isolate(t)
			out, _, err = runNovel(t, []string{"imports", "blame", "job", "--json"}, blameHandler(t, "", "", tc.side))
			if ExitCode(err) != 5 || !strings.Contains(out, tc.want) {
				t.Fatalf("blame: err=%v out=%s", err, out)
			}
		})
	}
}

// Codex r1 P2: a stale or incomplete mirror never answers the estimate under auto (it counts live), a smart-list
// estimate is always live, --data-source live skips the mirror, and --data-source local reports a lower bound (exit 5).
func TestNovel1219VerifyLineTypeEstimateSourceRules(t *testing.T) {
	staleMirror := func(t *testing.T) string {
		db := syncedContactsMirror(t, map[string]string{"c1": `{"id":"c1","phone":"+12125550101"}`})
		s, err := store.Open(db)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.DB().Exec(`UPDATE sync_state SET last_synced_at = ? WHERE resource_type = 'contacts'`, time.Now().Add(-2*time.Hour)); err != nil {
			t.Fatal(err)
		}
		_ = s.Close()
		return db
	}
	live := func(reads *atomic.Int32) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			reads.Add(1)
			_, _ = w.Write([]byte(`{"data":[{"id":"x1","phone":"+12125550111"},{"id":"x2","phone":"+12125550112"}],"meta":{"total":2}}`))
		}
	}
	for _, tc := range []struct {
		name      string
		mirror    func(t *testing.T) string
		args      []string
		wantLive  bool
		wantCode  int
		wantCount int
	}{
		{"stale mirror under auto counts live", staleMirror, []string{"--max-age", "1m"}, true, 0, 2},
		{"fresh mirror smart-list is live", func(t *testing.T) string {
			return syncedContactsMirror(t, map[string]string{"c1": `{"id":"c1","phone":"+12125550101"}`})
		}, []string{"--smart-list", "7c1d2e3f-0000-4000-8000-000000000000"}, true, 0, 2},
		{"data-source live skips a fresh mirror", func(t *testing.T) string {
			return syncedContactsMirror(t, map[string]string{"c1": `{"id":"c1","phone":"+12125550101"}`})
		}, []string{"--data-source", "live"}, true, 0, 2},
		{"fresh complete mirror answers locally", func(t *testing.T) string {
			return syncedContactsMirror(t, map[string]string{"c1": `{"id":"c1","phone":"+12125550101"}`})
		}, nil, false, 0, 1},
		{"data-source local with a stale mirror is a lower bound", staleMirror, []string{"--max-age", "1m", "--data-source", "local"}, false, 5, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Isolate(t)
			var reads atomic.Int32
			args := append([]string{"contacts", "verify-line-type", "--estimate", "--db", tc.mirror(t), "--json"}, tc.args...)
			out, _, err := runNovel(t, args, live(&reads))
			code := 0
			if err != nil {
				code = ExitCode(err)
			}
			var view verifyEstimate
			_ = json.Unmarshal([]byte(out), &view)
			if code != tc.wantCode || (reads.Load() > 0) != tc.wantLive || view.Unverified != tc.wantCount {
				t.Fatalf("code=%d reads=%d view=%+v out=%s", code, reads.Load(), view, out)
			}
		})
	}
}

// Codex r1 P2: a batch without byLineType is invalid and stops the paid loop after one POST.
func TestNovel1219VerifyLineTypeRunRequiresByLineType(t *testing.T) {
	testenv.Isolate(t)
	var posts atomic.Int32
	out, _, err := runNovel(t, []string{"contacts", "verify-line-type", "run", "--until-done", "--smart-list", "7c1d2e3f-0000-4000-8000-000000000000", "--json"}, func(w http.ResponseWriter, r *http.Request) {
		posts.Add(1)
		_, _ = w.Write([]byte(`{"data":{"done":true,"verified":1,"remaining":0}}`))
	})
	if ExitCode(err) != 5 || posts.Load() != 1 || !strings.Contains(out, "missing byLineType") {
		t.Fatalf("err=%v posts=%d out=%s", err, posts.Load(), out)
	}
}

// Codex r1 P2: the watch deadline is --timeout from the command context; a short root request timeout never caps it.
func TestNovel1219ImportsWatchTimeoutIsNotCappedByTheRequestTimeout(t *testing.T) {
	testenv.Isolate(t)
	var polls atomic.Int32
	h := func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/imports/job":
			status := "processing"
			if polls.Add(1) > 4 {
				status = "completed"
			}
			_, _ = fmt.Fprintf(w, `{"data":{"id":"job","status":%q,"totalRows":1,"createdRows":1,"updatedRows":0,"skippedRows":0,"errorRows":0}}`, status)
		case "/imports/job/launch-monitor":
			_, _ = w.Write([]byte(`{"data":{"stage":"complete"}}`))
		default:
			http.NotFound(w, r)
		}
	}
	srv := httptest.NewServer(http.HandlerFunc(h))
	t.Cleanup(srv.Close)
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL)
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	// The root request timeout (300ms) is shorter than the whole watch (5 polls x 150ms); only --timeout bounds it.
	cmd := newNovelImportsWatchCmd(&rootFlags{timeout: 300 * time.Millisecond, asJSON: true})
	cmd.SetArgs([]string{"job", "--interval", "150ms", "--timeout", "10s"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil || polls.Load() < 5 {
		t.Fatalf("err=%v polls=%d out=%s", err, polls.Load(), out.String())
	}
}

// Codex r1 P2: under the dogfood poll cap a job that is still running exits 5 with a partial result, never 0.
func TestNovel1219ImportsWatchDogfoodCapOnARunningJobIsPartial(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("PRINTING_PRESS_DOGFOOD", "1")
	out, _, err := runNovel(t, []string{"imports", "watch", "job", "--interval", "1ms", "--json"}, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/imports/job":
			_, _ = w.Write([]byte(`{"data":{"id":"job","status":"processing","totalRows":1,"createdRows":0,"updatedRows":0,"skippedRows":0,"errorRows":0}}`))
		case "/imports/job/launch-monitor":
			_, _ = w.Write([]byte(`{"data":{"stage":"running"}}`))
		default:
			http.NotFound(w, r)
		}
	})
	if c := compactTestJSON(out); ExitCode(err) != 5 || !strings.Contains(c, `"partial":true`) || !strings.Contains(c, "still processing after 3 polls") {
		t.Fatalf("err=%v out=%s", err, c)
	}
}

// Codex r1 P2: the all-zero side-effects exception applies only after a terminal job read.
func TestNovel1219ImportsBlameZeroSideEffectsOnARunningJobIsPartial(t *testing.T) {
	testenv.Isolate(t)
	job := `{"data":{"id":"job","status":"processing","totalRows":3,"createdRows":0,"updatedRows":0,"skippedRows":0,"duplicateRows":0,"errorRows":0}}`
	side := `{"data":{"total":0,"completed":0,"failed":0,"retryableFailed":0,"exhaustedFailed":0,"queued":0,"held":0,"actionRequired":false,"isComplete":false}}`
	out, _, err := runNovel(t, []string{"imports", "blame", "job", "--json"}, blameHandler(t, job, "", side))
	if c := compactTestJSON(out); ExitCode(err) != 5 || strings.Contains(c, `"no_side_effects":true`) || !strings.Contains(c, "import side effects are not complete") {
		t.Fatalf("err=%v out=%s", err, c)
	}
}

// Codex r1 P2: job-level error entries (row 0, identified only by identifier) are counted, not rejected.
func TestNovel1219ImportsBlameAcceptsRowZeroErrors(t *testing.T) {
	testenv.Isolate(t)
	rows := `{"data":{"rows":[{"row":0,"identifier":"Import","outcome":"error","objectType":"contact","message":"mapping missing"},{"row":0,"identifier":"Tags","outcome":"error","objectType":"contact","message":"tag settle failed"},{"row":2,"identifier":"Row 2","outcome":"error","objectType":"contact","message":"invalid phone"}],"meta":{"total":3},"tabs":{"error":3}}}`
	out, _, err := runNovel(t, []string{"imports", "blame", "job", "--json"}, blameHandler(t, "", rows, ""))
	if c := compactTestJSON(out); err != nil || !strings.Contains(c, `"error_rows":3`) || !strings.Contains(c, `"mapping missing":1`) || !strings.Contains(c, `"tag settle failed":1`) {
		t.Fatalf("err=%v out=%s", err, c)
	}
}

// Codex r1 P3: the typed-exit annotations list every reachable class (404 = 3, auth = 4).
func TestNovel1219ImportsTypedExitCodesCoverNotFoundAndAuth(t *testing.T) {
	root := newRootCmd(&rootFlags{})
	for _, path := range []string{"imports watch", "imports blame"} {
		cmd := findCommandPath(root, path)
		if cmd == nil {
			t.Fatalf("%s not found", path)
		}
		if got := cmd.Annotations["pp:typed-exit-codes"]; got != "0,2,3,4,5" {
			t.Fatalf("%s typed exits = %q, want 0,2,3,4,5", path, got)
		}
	}
}

// ---- Codex novel1219-r2 regressions (r3) ----

// interruptedContactsMirror is a mirror whose last sync completed and whose next (incremental) attempt was
// interrupted: last_synced_at stays recent but last_attempt_complete is 0.
func interruptedContactsMirror(t *testing.T, rows map[string]string) string {
	t.Helper()
	dbPath := syncedContactsMirror(t, rows)
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveSyncProgress("contacts", "cursor-2", len(rows)); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	return dbPath
}

// Codex r2 P1: an interrupted sync attempt is not a mirror: send-check reads the audience live under auto and is
// inconclusive (exit 5) under local; the estimate counts live under auto and is a lower bound (exit 5) under local.
func TestNovel1219InterruptedSyncIsNotAMirror(t *testing.T) {
	rows := map[string]string{"local-1": `{"id":"local-1","phone":"+12125550102","lineType":"mobile","tags":[{"id":"t1","name":"leads"}]}`}
	testenv.Isolate(t)
	db := interruptedContactsMirror(t, rows)
	var reads atomic.Int32
	out, _, err := runNovel(t, []string{"send-check", "--tag", "leads", "--db", db, "--json"}, sendCheckLiveHandler(t, &reads))
	if err != nil || reads.Load() == 0 || !strings.Contains(out, "live-1") {
		t.Fatalf("send-check auto: err=%v reads=%d out=%s", err, reads.Load(), out)
	}
	testenv.Isolate(t)
	reads.Store(0)
	out, _, err = runNovel(t, []string{"send-check", "--tag", "leads", "--db", db, "--data-source", "local", "--json"}, sendCheckLiveHandler(t, &reads))
	if ExitCode(err) != 5 || reads.Load() != 0 || !strings.Contains(out, "did not finish") {
		t.Fatalf("send-check local: err=%v reads=%d out=%s", err, reads.Load(), out)
	}
	testenv.Isolate(t)
	reads.Store(0)
	out, _, err = runNovel(t, []string{"contacts", "verify-line-type", "--estimate", "--db", db, "--json"}, func(w http.ResponseWriter, r *http.Request) {
		reads.Add(1)
		_, _ = w.Write([]byte(`{"data":[{"id":"x1","phone":"+12125550111"},{"id":"x2","phone":"+12125550112"}],"meta":{"total":2}}`))
	})
	if err != nil || reads.Load() == 0 || !strings.Contains(compactTestJSON(out), `"unverified":2`) {
		t.Fatalf("estimate auto: err=%v reads=%d out=%s", err, reads.Load(), out)
	}
	testenv.Isolate(t)
	out, _, err = runNovel(t, []string{"contacts", "verify-line-type", "--estimate", "--db", db, "--data-source", "local", "--json"}, nil)
	if ExitCode(err) != 5 || !strings.Contains(out, "did not finish") {
		t.Fatalf("estimate local: err=%v out=%s", err, out)
	}
}

// Codex r2 P1 (counts reconcile): a mirror whose row count differs from its last sync total is not a mirror either.
func TestNovel1219SendCheckUnreconciledMirrorFallsBackLive(t *testing.T) {
	testenv.Isolate(t)
	db := syncedContactsMirror(t, map[string]string{"local-1": `{"id":"local-1","phone":"+12125550102","lineType":"mobile","tags":[{"id":"t1","name":"leads"}]}`})
	s, err := store.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSyncState("contacts", "", 5); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	var reads atomic.Int32
	out, _, err := runNovel(t, []string{"send-check", "--tag", "leads", "--db", db, "--json"}, sendCheckLiveHandler(t, &reads))
	if err != nil || reads.Load() == 0 || !strings.Contains(out, "live-1") {
		t.Fatalf("err=%v reads=%d out=%s", err, reads.Load(), out)
	}
}

// Codex r2 P1: skippedInactive side effects (terminal, nothing fired; outside the route's isComplete terms) never
// verify; neither does pending, processing, or a completed count below total.
func TestNovel1219OnlyCompletedSideEffectsVerify(t *testing.T) {
	for _, tc := range []struct{ name, side, want string }{
		{"skipped inactive", `{"data":{"total":1,"pending":0,"processing":0,"completed":0,"skippedInactive":1,"failed":0,"retryableFailed":0,"exhaustedFailed":0,"queued":0,"held":0,"actionRequired":false,"isComplete":true}}`, "1 skipped inactive sends"},
		{"unclassified status", `{"data":{"total":2,"completed":1,"failed":0,"retryableFailed":0,"exhaustedFailed":0,"queued":0,"held":0,"actionRequired":false,"isComplete":true}}`, "1 of 2 side effects completed"},
		{"processing", `{"data":{"total":1,"pending":0,"processing":1,"completed":0,"skippedInactive":0,"failed":0,"retryableFailed":0,"exhaustedFailed":0,"queued":1,"held":0,"actionRequired":false,"isComplete":false}}`, "1 processing sends"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Isolate(t)
			out, _, err := runNovel(t, []string{"imports", "watch", "job", "--verify", "--interval", "1", "--json"}, watchTestHandler(t, func(w http.ResponseWriter) { _, _ = w.Write([]byte(tc.side)) }))
			if ExitCode(err) != 5 || !strings.Contains(out, tc.want) || strings.Contains(compactTestJSON(out), `"verified":true`) {
				t.Fatalf("watch: err=%v out=%s", err, out)
			}
			testenv.Isolate(t)
			out, _, err = runNovel(t, []string{"imports", "blame", "job", "--json"}, blameHandler(t, "", "", tc.side))
			if ExitCode(err) != 5 || !strings.Contains(out, tc.want) {
				t.Fatalf("blame: err=%v out=%s", err, out)
			}
		})
	}
	testenv.Isolate(t)
	out, _, err := runNovel(t, []string{"imports", "watch", "job", "--verify", "--interval", "1", "--json"}, watchTestHandler(t, func(w http.ResponseWriter) {
		_, _ = w.Write([]byte(`{"data":{"total":2,"pending":0,"processing":0,"completed":2,"skippedInactive":0,"failed":0,"retryableFailed":0,"exhaustedFailed":0,"queued":0,"held":0,"actionRequired":false,"isComplete":true}}`))
	}))
	if c := compactTestJSON(out); err != nil || !strings.Contains(c, `"verified":true`) || !strings.Contains(c, `"skipped_inactive":0`) {
		t.Fatalf("all completed: err=%v out=%s", err, c)
	}
}

// Codex r2 P2: a local read with a decode failure is not an answer: auto reads the audience live.
func TestNovel1219SendCheckPartialLocalReadFallsBackLive(t *testing.T) {
	testenv.Isolate(t)
	db := syncedContactsMirror(t, map[string]string{"local-1": `{"id":"local-1","phone":"+12125550102","lineType":"mobile","tags":[{"id":"t1","name":"leads"}]}`})
	s, err := store.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().Exec(`INSERT INTO resources(id,resource_type,data,synced_at,updated_at) VALUES('broken','contacts','{"id":"broken","tags":[{"name":"leads"}],',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`); err != nil {
		t.Skipf("cannot plant a malformed row through the store's JSON checks: %v", err)
	}
	if err := s.SaveSyncState("contacts", "", 2); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	var reads atomic.Int32
	out, _, err := runNovel(t, []string{"send-check", "--tag", "leads", "--db", db, "--json"}, sendCheckLiveHandler(t, &reads))
	if err != nil || reads.Load() == 0 || !strings.Contains(out, "live-1") || strings.Contains(out, "inconclusive") {
		t.Fatalf("err=%v reads=%d out=%s", err, reads.Load(), out)
	}
}

// Codex r2 P3: a smart-list estimate never suggests a sync (it cannot make membership local): auto counts live with no
// sync hint, local tells the user to use live or auto.
func TestNovel1219SmartListEstimateHintNamesARealAction(t *testing.T) {
	testenv.Isolate(t)
	db := syncedContactsMirror(t, map[string]string{"c1": `{"id":"c1","phone":"+12125550101"}`})
	out, _, err := runNovel(t, []string{"contacts", "verify-line-type", "--estimate", "--smart-list", "s1", "--db", db, "--json"}, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":"x1","phone":"+12125550111"}],"meta":{"total":1}}`))
	})
	if err != nil || strings.Contains(out, "sync --resources") {
		t.Fatalf("auto: err=%v out=%s", err, out)
	}
	testenv.Isolate(t)
	out, _, err = runNovel(t, []string{"contacts", "verify-line-type", "--estimate", "--smart-list", "s1", "--db", db, "--data-source", "local", "--json"}, nil)
	if ExitCode(err) != 5 || strings.Contains(out, "sync --resources") || !strings.Contains(out, "--data-source live or auto") {
		t.Fatalf("local: err=%v out=%s", err, out)
	}
}

// Codex r2 P3: no surface still claims the import commands report SMS delivery.
func TestNovel1219NoSMSDeliveryClaimsForImportCommands(t *testing.T) {
	root := newRootCmd(&rootFlags{})
	for _, path := range []string{"imports blame", "imports watch"} {
		cmd := findCommandPath(root, path)
		if cmd == nil {
			t.Fatalf("%s not found", path)
		}
		for _, text := range []string{cmd.Short, cmd.Long} {
			for _, claim := range []string{"who got the SMS", "got its texts", "delivery report"} {
				if strings.Contains(text, claim) {
					t.Fatalf("%s help claims %q: %s", path, claim, text)
				}
			}
		}
	}
	for _, f := range []string{"../../README.md", "../../SKILL.md"} {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, claim := range []string{"who actually got the SMS", "get its texts", "who got the SMS"} {
			if strings.Contains(string(raw), claim) {
				t.Fatalf("%s still claims %q", f, claim)
			}
		}
	}
}

// ---- Codex novel1219-r3 regressions (r4) ----

// faultyMirror wraps the real mirror reader and fails at one stage.
type faultyMirror struct {
	inner         mirrorReader
	stage         string // "query", "scan", "iterate", "close", "count-capped"
	cappedCountOn string
}

type faultyRows struct {
	mirrorRows
	stage string
}

func (r faultyRows) Scan(dest ...any) error {
	if r.stage == "scan" {
		return fmt.Errorf("injected scan failure")
	}
	return r.mirrorRows.Scan(dest...)
}
func (r faultyRows) Err() error {
	if r.stage == "iterate" {
		return fmt.Errorf("injected iteration failure")
	}
	return r.mirrorRows.Err()
}
func (r faultyRows) Close() error {
	err := r.mirrorRows.Close()
	if r.stage == "close" {
		return fmt.Errorf("injected close failure")
	}
	return err
}

func (f faultyMirror) Query(ctx context.Context, q string, args ...any) (mirrorRows, error) {
	if f.stage == "query" && strings.Contains(q, "FROM resources") {
		return nil, fmt.Errorf("injected query failure")
	}
	if f.stage == "sync-state" && strings.Contains(q, "FROM sync_state") {
		return nil, fmt.Errorf("injected sync-state failure")
	}
	rows, err := f.inner.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	return faultyRows{mirrorRows: rows, stage: f.stage}, nil
}

func (f faultyMirror) Count(ctx context.Context, q string, args ...any) (int, error) {
	if f.stage == "count-capped" && strings.Contains(q, f.cappedCountOn) {
		return 0, fmt.Errorf("injected count failure")
	}
	return f.inner.Count(ctx, q, args...)
}

func injectMirrorFault(t *testing.T, stage string) {
	t.Helper()
	orig := newMirrorReader
	newMirrorReader = func(db *sql.DB) mirrorReader {
		return faultyMirror{inner: orig(db), stage: stage, cappedCountOn: "' AND "}
	}
	t.Cleanup(func() { newMirrorReader = orig })
}

// Codex r3 P2: a failure at any local read stage makes the mirror unusable: send-check auto reads live, local exits 5;
// the estimate counts live under auto and is a lower bound (exit 5) under local. The capped-audience count is covered
// with --limit 1 over a two-contact tag.
func TestNovel1219EveryLocalReadStageFailureFallsBackLive(t *testing.T) {
	mirrorRows := map[string]string{
		"local-1": `{"id":"local-1","phone":"+12125550102","lineType":"mobile","tags":[{"id":"t1","name":"leads"}]}`,
		"local-2": `{"id":"local-2","phone":"+12125550103","lineType":"mobile","tags":[{"id":"t1","name":"leads"}]}`,
	}
	for _, stage := range []string{"sync-state", "query", "scan", "iterate", "close", "count-capped"} {
		t.Run(stage, func(t *testing.T) {
			testenv.Isolate(t)
			db := syncedContactsMirror(t, mirrorRows)
			injectMirrorFault(t, stage)
			var reads atomic.Int32
			out, _, err := runNovel(t, []string{"send-check", "--tag", "leads", "--limit", "1", "--db", db, "--json"}, sendCheckLiveHandler(t, &reads))
			if err != nil || reads.Load() == 0 || !strings.Contains(out, "live-1") {
				t.Fatalf("send-check auto: err=%v reads=%d out=%s", err, reads.Load(), out)
			}
			testenv.Isolate(t)
			reads.Store(0)
			out, _, err = runNovel(t, []string{"send-check", "--tag", "leads", "--limit", "1", "--db", db, "--data-source", "local", "--json"}, sendCheckLiveHandler(t, &reads))
			if ExitCode(err) != 5 || reads.Load() != 0 || !strings.Contains(out, "the local mirror is unusable") {
				t.Fatalf("send-check local: err=%v reads=%d out=%s", err, reads.Load(), out)
			}
			if stage == "count-capped" {
				return // the estimate has no capped count
			}
			testenv.Isolate(t)
			reads.Store(0)
			out, _, err = runNovel(t, []string{"contacts", "verify-line-type", "--estimate", "--db", db, "--json"}, func(w http.ResponseWriter, r *http.Request) {
				reads.Add(1)
				_, _ = w.Write([]byte(`{"data":[{"id":"x1","phone":"+12125550111"},{"id":"x2","phone":"+12125550112"},{"id":"x3","phone":"+12125550113"}],"meta":{"total":3}}`))
			})
			if err != nil || reads.Load() == 0 || !strings.Contains(compactTestJSON(out), `"unverified":3`) {
				t.Fatalf("estimate auto: err=%v reads=%d out=%s", err, reads.Load(), out)
			}
			testenv.Isolate(t)
			out, _, err = runNovel(t, []string{"contacts", "verify-line-type", "--estimate", "--db", db, "--data-source", "local", "--json"}, nil)
			if ExitCode(err) != 5 || !strings.Contains(out, "the local mirror is unusable") || !strings.Contains(compactTestJSON(out), `"partial":true`) {
				t.Fatalf("estimate local: err=%v out=%s", err, out)
			}
		})
	}
}

// Codex r3 P2: drips audit declares pp:data-source local, so root rejects --data-source live before any store access.
func TestNovel1219DripsAuditRejectsDataSourceLiveBeforeTheStore(t *testing.T) {
	testenv.Isolate(t)
	dir := t.TempDir()
	dbPath := dir + "/never-created.db"
	_, _, err := runNovel(t, []string{"drips", "audit", "--db", dbPath, "--data-source", "live", "--json"}, nil)
	if err == nil || !strings.Contains(err.Error(), "data-source") {
		t.Fatalf("err=%v, want the --data-source live refusal", err)
	}
	if _, statErr := os.Stat(dbPath); !os.IsNotExist(statErr) {
		t.Fatalf("drips audit touched the store before refusing --data-source live (stat err=%v)", statErr)
	}
	if got := findCommandPath(newRootCmd(&rootFlags{}), "drips audit").Annotations["pp:data-source"]; got != "local" {
		t.Fatalf("drips audit pp:data-source = %q, want local", got)
	}
}

// Codex r3 P3: blame's rendered output (JSON and table) makes no delivery claim: side_effects_completed and the
// provider note, no delivered field or DELIVERED column.
func TestNovel1219ImportsBlameRenderedOutputMakesNoDeliveryClaim(t *testing.T) {
	testenv.Isolate(t)
	out, _, err := runNovel(t, []string{"imports", "blame", "job", "--json"}, blameHandler(t, "", "", ""))
	c := compactTestJSON(out)
	if err != nil || !strings.Contains(c, `"side_effects_completed":1`) || !strings.Contains(c, "provider delivery is not available from this data") || strings.Contains(c, `"delivered"`) || strings.Contains(c, "delivery_correlation") {
		t.Fatalf("json: err=%v out=%s", err, c)
	}
	testenv.Isolate(t)
	table, _, err := runNovel(t, []string{"imports", "blame", "job", "--human-friendly"}, blameHandler(t, "", "", ""))
	if err != nil || !strings.Contains(table, "SIDE EFFECTS COMPLETED") || !strings.Contains(table, "provider delivery is not available from this data") || strings.Contains(table, "DELIVERED") {
		t.Fatalf("table: err=%v out=%s", err, table)
	}
}

// Codex r3 P3: the rendered help of every novel command and subcommand (its own Short, Long, Example and flags;
// the inherited global --deliver output-routing flag is not a delivery claim) never uses delivered / delivery.
func TestNovel1219NovelCommandHelpNeverClaimsDelivery(t *testing.T) {
	root := newRootCmd(&rootFlags{})
	for _, path := range []string{"send-check", "imports blame", "imports watch", "contacts verify-line-type", "contacts verify-line-type run", "drips audit"} {
		cmd := findCommandPath(root, path)
		if cmd == nil {
			t.Fatalf("%s not found", path)
		}
		text := strings.ToLower(cmd.Short + "\n" + cmd.Long + "\n" + cmd.Example + "\n" + cmd.LocalNonPersistentFlags().FlagUsages())
		if strings.Contains(text, "deliver") {
			t.Fatalf("%s help mentions delivery:\n%s", path, text)
		}
	}
}

// ---- Codex novel1219-r4 regressions (r5) ----

// Codex r4 P2 #1: the freshness decision uses the one sync-state read: an old mirror is never trusted when the
// sync_state read succeeds, and a failing sync_state read is "mirror unusable" (covered by the sync-state stage of
// TestNovel1219EveryLocalReadStageFailureFallsBackLive), never "not stale".
func TestNovel1219FreshnessComesFromTheOneSyncStateRead(t *testing.T) {
	testenv.Isolate(t)
	db := syncedContactsMirror(t, map[string]string{"local-1": `{"id":"local-1","phone":"+12125550102","lineType":"mobile","tags":[{"id":"t1","name":"leads"}]}`})
	s, err := store.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().Exec(`UPDATE sync_state SET last_synced_at = ? WHERE resource_type = 'contacts'`, time.Now().Add(-3*time.Hour)); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	var reads atomic.Int32
	out, stderr, err := runNovel(t, []string{"send-check", "--tag", "leads", "--db", db, "--max-age", "1h", "--json"}, sendCheckLiveHandler(t, &reads))
	if err != nil || reads.Load() == 0 || !strings.Contains(out, "live-1") || !strings.Contains(stderr, "older than --max-age") {
		t.Fatalf("err=%v reads=%d out=%s stderr=%s", err, reads.Load(), out, stderr)
	}
	st, err := readContactsSyncState(context.Background(), faultyMirror{inner: sqlMirrorReader{}, stage: "sync-state"})
	if err == nil || !st.SyncedAt.IsZero() {
		t.Fatalf("a failing sync_state read must be an error, got %+v, %v", st, err)
	}
}

// specOperationBodyProperties reads the requestBody property names of one operation from the repo's spec.yaml.
func specOperationBodyProperties(t *testing.T, path, method string) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile("../../spec.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Paths map[string]map[string]struct {
			RequestBody struct {
				Required bool `yaml:"required"`
				Content  map[string]struct {
					Schema struct {
						Properties map[string]any `yaml:"properties"`
					} `yaml:"schema"`
				} `yaml:"content"`
			} `yaml:"requestBody"`
		} `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	op, ok := doc.Paths[path][method]
	if !ok {
		t.Fatalf("spec has no %s %s", method, path)
	}
	props := map[string]bool{}
	for name := range op.RequestBody.Content["application/json"].Schema.Properties {
		props[name] = true
	}
	return props
}

// Codex r4 P2 #2: the paid loop follows the route contract (POST /contacts/verify-line-type, operation
// postContactsVerifyLineType in spec.yaml): exactly one stateless scope, smartListId a UUID or contactIds 1 to
// 50,000 UUIDs, validated before any request; the body carries only that scope, a property the spec declares.
func TestNovel1219VerifyLineTypeRunFollowsTheRouteContract(t *testing.T) {
	props := specOperationBodyProperties(t, "/contacts/verify-line-type", "post")
	for _, want := range []string{"smartListId", "contactIds"} {
		if !props[want] {
			t.Fatalf("spec postContactsVerifyLineType does not declare %s (props %v)", want, props)
		}
	}
	const list = "7c1d2e3f-0000-4000-8000-000000000000"
	tooMany := make([]string, verifyRunMaxContactIDs+1)
	for i := range tooMany {
		tooMany[i] = fmt.Sprintf("00000000-0000-4000-8000-%012d", i)
	}
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"no scope", nil, "pass exactly one scope"},
		{"both scopes", []string{"--smart-list", list, "--contact-ids", "9e778307-e575-420b-b02c-dc9cc564febd"}, "not both"},
		{"invalid smart list", []string{"--smart-list", "s1"}, "--smart-list must be a UUID"},
		{"mixed contact ids", []string{"--contact-ids", "9e778307-e575-420b-b02c-dc9cc564febd,not-a-uuid"}, "--contact-ids[1] must be a UUID"},
		{"too many contact ids", []string{"--contact-ids", strings.Join(tooMany, ",")}, "at most 50000"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Isolate(t)
			var posts atomic.Int32
			_, _, err := runNovel(t, append([]string{"contacts", "verify-line-type", "run", "--until-done", "--json"}, tc.args...), func(w http.ResponseWriter, r *http.Request) {
				posts.Add(1)
				_, _ = w.Write([]byte(`{"data":{"done":true,"verified":0,"remaining":0,"byLineType":{}}}`))
			})
			if ExitCode(err) != 2 || posts.Load() != 0 || err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err=%v posts=%d", err, posts.Load())
			}
		})
	}
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"smart list", []string{"--smart-list", list}, `{"smartListId":"` + list + `"}`},
		{"contact ids", []string{"--contact-ids", "9e778307-e575-420b-b02c-dc9cc564febd, 3f8a0c4e-1111-4222-8333-444455556666"}, `{"contactIds":["9e778307-e575-420b-b02c-dc9cc564febd","3f8a0c4e-1111-4222-8333-444455556666"]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Isolate(t)
			var body []byte
			_, _, err := runNovel(t, append([]string{"contacts", "verify-line-type", "run", "--until-done", "--json"}, tc.args...), func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/contacts/verify-line-type" {
					http.NotFound(w, r)
					return
				}
				body, _ = io.ReadAll(r.Body)
				_, _ = w.Write([]byte(`{"data":{"done":true,"verified":1,"remaining":0,"byLineType":{"mobile":1}}}`))
			})
			if err != nil || compactTestJSON(string(body)) != tc.want {
				t.Fatalf("err=%v body=%s want=%s", err, body, tc.want)
			}
			var sent map[string]any
			_ = json.Unmarshal(body, &sent)
			for key := range sent {
				if !props[key] {
					t.Fatalf("body key %q is not a spec property of postContactsVerifyLineType", key)
				}
			}
		})
	}
}
