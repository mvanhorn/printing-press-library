// Sweep findings: mirrored-row and estimate-page decode failures, page caps/fetch failures, and verification batch/fetch/decode failures print partial results and exit API-class non-zero.
package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/store"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNovelContactsVerifyLineTypeHelpWires(t *testing.T) {
	testenv.Isolate(t)
	_, _, err := runNovel(t, []string{"contacts", "verify-line-type", "--help"}, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestContactLineTypeAliasesAndFallback(t *testing.T) {
	tests := []struct {
		name string
		row  map[string]any
		want string
	}{
		{name: "camel case wins", row: map[string]any{"lineType": " mobile ", "line_type": "landline", "customFields": map[string]any{"sms_line_type": "voip"}}, want: "mobile"},
		{name: "snake case after blank camel case", row: map[string]any{"lineType": " ", "line_type": " landline ", "customFields": map[string]any{"sms_line_type": "voip"}}, want: "landline"},
		{name: "custom field fallback", row: map[string]any{"customFields": map[string]any{"sms_line_type": " voip "}}, want: "voip"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := contactLineType(tc.row); got != tc.want {
				t.Fatalf("contactLineType()=%q, want %q", got, tc.want)
			}
		})
	}
}

func TestVerifyLineTypeEstimateStaleMirrorIsPartial(t *testing.T) {
	testenv.Isolate(t)
	dbPath := t.TempDir() + "/data.db"
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Upsert("contacts", "c1", json.RawMessage(`{"phone":"+15550000001"}`)); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveSyncState("contacts", "", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().Exec(`UPDATE sync_state SET last_synced_at = ? WHERE resource_type = 'contacts'`, time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	out, _, err := runNovel(t, []string{"contacts", "verify-line-type", "--estimate", "--db", dbPath, "--max-age", "1m", "--json"}, nil)
	var view verifyEstimate
	if decodeErr := json.Unmarshal([]byte(out), &view); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if ExitCode(err) != 5 || !view.Partial || view.Checked != 1 || view.Total == nil || *view.Total != 1 || len(view.Failures) != 1 || !strings.Contains(view.Failures[0], "older than --max-age") {
		t.Fatalf("err=%v view=%+v", err, view)
	}
}

func TestVerifyLineTypeEstimateLeafRejectsPaidLoopFlags(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"until-done", []string{"contacts", "verify-line-type", "--until-done"}},
		{"max-batches", []string{"contacts", "verify-line-type", "--max-batches", "2"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Isolate(t)
			var posts atomic.Int32
			_, _, err := runNovel(t, tc.args, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					posts.Add(1)
				}
				http.NotFound(w, r)
			})
			var ce *cliError
			if !errors.As(err, &ce) || ce.code != 2 {
				t.Fatalf("err=%v", err)
			}
			if !strings.Contains(err.Error(), "contacts verify-line-type run") {
				t.Fatalf("missing run pointer: %v", err)
			}
			if posts.Load() != 0 {
				t.Fatalf("POSTs=%d", posts.Load())
			}
		})
	}
}

func TestVerifyLineTypeEstimateNeverPosts(t *testing.T) {
	testenv.Isolate(t)
	var posts atomic.Int32
	_, _, err := runNovel(t, []string{"contacts", "verify-line-type", "--estimate", "--db", t.TempDir() + "/missing.db", "--json"}, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posts.Add(1)
		}
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	if err != nil {
		t.Fatal(err)
	}
	if posts.Load() != 0 {
		t.Fatalf("POSTs=%d", posts.Load())
	}
}

func TestVerifyLineTypeUntilDoneHonorsMaxBatches(t *testing.T) {
	testenv.Isolate(t)
	var posts atomic.Int32
	out, _, err := runNovel(t, []string{"contacts", "verify-line-type", "run", "--until-done", "--max-batches", "2", "--json"}, func(w http.ResponseWriter, r *http.Request) {
		posts.Add(1)
		_, _ = w.Write([]byte(`{"data":{"verified":1,"remaining":9,"done":false}}`))
	})
	var ce *cliError
	if !errors.As(err, &ce) || ce.code != 5 || !strings.Contains(compactTestJSON(out), `"partial":true`) {
		t.Fatalf("err=%v", err)
	}
	if posts.Load() != 2 {
		t.Fatalf("POSTs=%d", posts.Load())
	}
}

func TestVerifyLineTypeUnexpectedResponseStopsAfterOnePost(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{"message envelope", `{"message":"temporarily unavailable"}`},
		{"object error with valid data", `{"error":{"reason":"temporarily unavailable"},"data":{"verified":1,"remaining":0,"done":true}}`},
		{"wrongly typed field", `{"data":{"verified":1,"remaining":1,"done":"yes"}}`},
		{"wrong byLineType shape", `{"data":{"verified":1,"remaining":1,"done":false,"byLineType":["mobile"]}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Isolate(t)
			var posts atomic.Int32
			out, _, err := runNovel(t, []string{"contacts", "verify-line-type", "run", "--until-done", "--json"}, func(w http.ResponseWriter, r *http.Request) {
				posts.Add(1)
				_, _ = w.Write([]byte(tc.body))
			})
			var view verifyRunView
			if decodeErr := json.Unmarshal([]byte(out), &view); decodeErr != nil {
				t.Fatal(decodeErr)
			}
			if ExitCode(err) != 5 || !view.Partial || view.Batches != 0 || len(view.Failures) != 1 {
				t.Fatalf("err=%v view=%+v", err, view)
			}
			if posts.Load() != 1 {
				t.Fatalf("POSTs=%d", posts.Load())
			}
		})
	}
}

func TestVerifyLineTypePostFailuresAreTyped(t *testing.T) {
	for _, tc := range []struct{ status, code int }{{401, 4}, {403, 4}, {404, 3}, {500, 5}} {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			testenv.Isolate(t)
			out, _, err := runNovel(t, []string{"contacts", "verify-line-type", "run", "--until-done", "--json"}, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(`{"message":"failure"}`))
			})
			var ce *cliError
			if !errors.As(err, &ce) || ce.code != tc.code {
				t.Fatalf("err=%v", err)
			}
			if !strings.Contains(compactTestJSON(out), `"partial":true`) {
				t.Fatalf("out=%s", out)
			}
		})
	}
}

func TestVerifyLineTypeLiveEstimateFailuresAreTyped(t *testing.T) {
	for _, tc := range []struct{ status, code int }{{401, 4}, {403, 4}, {404, 3}, {500, 5}} {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			testenv.Isolate(t)
			out, _, err := runNovel(t, []string{"contacts", "verify-line-type", "--estimate", "--db", t.TempDir() + "/missing.db", "--json"}, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(`{"message":"failure"}`))
			})
			if ExitCode(err) != tc.code || !strings.Contains(compactTestJSON(out), `"partial":true`) {
				t.Fatalf("err=%v code=%d out=%s", err, ExitCode(err), out)
			}
		})
	}
}

func TestVerifyLineTypeInvalidProgressStopsAfterOnePost(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{"negative verified", `{"data":{"verified":-1,"remaining":2,"done":false}}`},
		{"negative remaining", `{"data":{"verified":1,"remaining":-1,"done":false}}`},
		{"negative line type", `{"data":{"verified":1,"remaining":2,"done":false,"byLineType":{"mobile":-1}}}`},
		{"fractional verified", `{"data":{"verified":0.5,"remaining":2,"done":false}}`},
		{"fractional remaining", `{"data":{"verified":1,"remaining":1.5,"done":false}}`},
		{"fractional line type", `{"data":{"verified":1,"remaining":2,"done":false,"byLineType":{"mobile":0.5}}}`},
		{"overflowing verified", `{"data":{"verified":1e100,"remaining":2,"done":false}}`},
		{"contradictory terminal state", `{"data":{"verified":0,"remaining":0,"done":false}}`},
		{"done with contacts remaining", `{"data":{"verified":1,"remaining":10,"done":true}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Isolate(t)
			var posts atomic.Int32
			out, _, err := runNovel(t, []string{"contacts", "verify-line-type", "run", "--until-done", "--json"}, func(w http.ResponseWriter, r *http.Request) {
				posts.Add(1)
				_, _ = w.Write([]byte(tc.body))
			})
			var view verifyRunView
			if decodeErr := json.Unmarshal([]byte(out), &view); decodeErr != nil {
				t.Fatal(decodeErr)
			}
			if ExitCode(err) != 5 || !view.Partial || view.Batches != 0 || len(view.Failures) != 1 {
				t.Fatalf("err=%v view=%+v", err, view)
			}
			if posts.Load() != 1 {
				t.Fatalf("POSTs=%d", posts.Load())
			}
		})
	}
}

func TestVerifyLineTypeNonDecreasingRemainingStopsBeforeAggregation(t *testing.T) {
	for _, tc := range []struct {
		name      string
		remaining int
	}{
		{"unchanged remaining", 10},
		{"increasing remaining", 11},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Isolate(t)
			var posts atomic.Int32
			out, _, err := runNovel(t, []string{"contacts", "verify-line-type", "run", "--until-done", "--json"}, func(w http.ResponseWriter, r *http.Request) {
				if posts.Add(1) == 1 {
					_, _ = w.Write([]byte(`{"data":{"verified":2,"remaining":10,"done":false,"byLineType":{"mobile":2}}}`))
					return
				}
				_, _ = fmt.Fprintf(w, `{"data":{"verified":1,"remaining":%d,"done":false,"byLineType":{"landline":1}}}`, tc.remaining)
			})
			var view verifyRunView
			if decodeErr := json.Unmarshal([]byte(out), &view); decodeErr != nil {
				t.Fatal(decodeErr)
			}
			if ExitCode(err) != 5 || !view.Partial || view.Batches != 1 || view.Verified != 2 || view.Remaining != 10 || view.ByLineType["mobile"] != 2 || view.ByLineType["landline"] != 0 || len(view.Failures) != 1 {
				t.Fatalf("err=%v view=%+v", err, view)
			}
			if posts.Load() != 2 {
				t.Fatalf("POSTs=%d", posts.Load())
			}
		})
	}
}
func TestVerifyLineTypeEstimateMath(t *testing.T) {
	testenv.Isolate(t)
	dbPath := t.TempDir() + "/data.db"
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]string{"a": `{"phone":"+15550000001","smartListIds":["s1"]}`, "b": `{"phone":"+15550000002","lineType":"mobile","smartListIds":["s1"]}`, "c": `{"phone":"+15550000003","smartListIds":["s1"]}`, "d": `{"phone":"+15550000004","smartListIds":["other"]}`}
	for id, raw := range rows {
		if err := db.Upsert("contacts", id, json.RawMessage(raw)); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.SaveSyncState("contacts", "", len(rows)); err != nil {
		t.Fatal(err)
	}
	db.Close()
	out, _, err := runNovel(t, []string{"contacts", "verify-line-type", "--estimate", "--smart-list", "s1", "--db", dbPath, "--json"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	out = compactTestJSON(out)
	for _, want := range []string{`"unverified":2`, `"unit_cost_usd":0.008`, `"estimated_cost_usd":0.016`} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %s in %s", want, out)
		}
	}
}

func TestVerifyLineTypeEstimateMalformedMirrorRowIsPartial(t *testing.T) {
	testenv.Isolate(t)
	dbPath := t.TempDir() + "/data.db"
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Upsert("contacts", "good", json.RawMessage(`{"phone":"+15550000001"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().Exec(`INSERT INTO resources(id,resource_type,data,synced_at,updated_at) VALUES('broken','contacts','{',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveSyncState("contacts", "", 2); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	out, _, err := runNovel(t, []string{"contacts", "verify-line-type", "--estimate", "--db", dbPath, "--json"}, nil)
	if ExitCode(err) != 5 {
		t.Fatalf("err=%v", err)
	}
	var view verifyEstimate
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatal(err)
	}
	if !view.Partial || view.Unverified != 1 || view.Checked != 1 || view.Total == nil || *view.Total != 2 || len(view.Failures) != 2 || !strings.Contains(strings.Join(view.Failures, "; "), "broken") {
		t.Fatalf("view=%+v", view)
	}
}

func TestVerifyLineTypeEstimateUnmappablePhoneIsPartial(t *testing.T) {
	testenv.Isolate(t)
	dbPath := t.TempDir() + "/data.db"
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Upsert("contacts", "short", json.RawMessage(`{"phone":"5551212"}`)); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveSyncState("contacts", "", 1); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	out, _, err := runNovel(t, []string{"contacts", "verify-line-type", "--estimate", "--db", dbPath, "--json"}, nil)
	var view verifyEstimate
	if decodeErr := json.Unmarshal([]byte(out), &view); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if ExitCode(err) != 5 || !view.Partial || view.Unverified != 0 || !strings.Contains(strings.Join(view.Failures, "; "), "unmappable phone identity") {
		t.Fatalf("err=%v view=%+v", err, view)
	}
}

func TestVerifyLineTypeLocalSyncTotalMismatchIsPartial(t *testing.T) {
	testenv.Isolate(t)
	dbPath := t.TempDir() + "/data.db"
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Upsert("contacts", "c1", json.RawMessage(`{"phone":"+15550000001"}`)); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveSyncState("contacts", "", 2); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	out, _, err := runNovel(t, []string{"contacts", "verify-line-type", "--estimate", "--db", dbPath, "--json"}, nil)
	var view verifyEstimate
	if decodeErr := json.Unmarshal([]byte(out), &view); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if ExitCode(err) != 5 || !view.Partial || view.Checked != 1 || view.Total == nil || *view.Total != 2 || !strings.Contains(strings.Join(view.Failures, "; "), "sync state reports total 2") {
		t.Fatalf("err=%v view=%+v", err, view)
	}
}
func TestVerifyLineTypeMissingMirrorFallsBackLive(t *testing.T) {
	testenv.Isolate(t)
	h := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/contacts" || r.URL.Query().Get("per_page") != "200" {
			t.Fatalf("request=%s?%s", r.URL.Path, r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"data":[{"phone":"+15550000001"},{"phone":"+15550000002","line_type":"mobile"}]}`))
	}
	out, _, err := runNovel(t, []string{"contacts", "verify-line-type", "--estimate", "--db", t.TempDir() + "/missing.db", "--json"}, h)
	if err != nil {
		t.Fatal(err)
	}
	compact := compactTestJSON(out)
	for _, want := range []string{`"unverified":1`, `"synced":false`, `"hint":"run: conduyt-crm-pp-cli sync --resources contacts --db `} {
		if !strings.Contains(compact, want) {
			t.Fatalf("missing %s in %s", want, out)
		}
	}
}

func TestVerifyLineTypeLiveEstimateUsesCustomFieldLineType(t *testing.T) {
	testenv.Isolate(t)
	out, _, err := runNovel(t, []string{"contacts", "verify-line-type", "--estimate", "--db", t.TempDir() + "/missing.db", "--json"}, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"phone":"+15550000001","customFields":{"sms_line_type":" mobile "}}]}`))
	})
	if err != nil {
		t.Fatal(err)
	}
	var view verifyEstimate
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatal(err)
	}
	if view.Checked != 1 || view.Unverified != 0 || view.EstimatedCostUSD != 0 {
		t.Fatalf("view=%+v", view)
	}
}

func TestVerifyLineTypeEstimateLocalLiveParity(t *testing.T) {
	testenv.Isolate(t)
	payload := `{"phone":"+15550000001","customFields":{"sms_line_type":"mobile"}}`
	dbPath := t.TempDir() + "/data.db"
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Upsert("contacts", "contact-1", json.RawMessage(payload)); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveSyncState("contacts", "", 1); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	localOut, _, err := runNovel(t, []string{"contacts", "verify-line-type", "--estimate", "--db", dbPath, "--json"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	liveOut, _, err := runNovel(t, []string{"contacts", "verify-line-type", "--estimate", "--db", t.TempDir() + "/missing.db", "--json"}, func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, `{"data":[%s]}`, payload)
	})
	if err != nil {
		t.Fatal(err)
	}
	var local, live verifyEstimate
	if err := json.Unmarshal([]byte(localOut), &local); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(liveOut), &live); err != nil {
		t.Fatal(err)
	}
	if local.Checked != live.Checked || local.Unverified != live.Unverified || local.EstimatedCostUSD != live.EstimatedCostUSD {
		t.Fatalf("local=%+v live=%+v", local, live)
	}
}

func TestVerifyLineTypeLiveEstimateReportsCheckedAndTotal(t *testing.T) {
	testenv.Isolate(t)
	h := func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		count := 200
		if page == "2" {
			count = 200
		}
		if page == "3" {
			count = 0
		}
		items := make([]map[string]any, count)
		for i := range items {
			items[i] = map[string]any{"phone": "+15550000001"}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": items, "meta": map[string]any{"total": 400}})
	}
	out, _, err := runNovel(t, []string{"contacts", "verify-line-type", "--estimate", "--db", t.TempDir() + "/missing.db", "--json"}, h)
	if err != nil {
		t.Fatal(err)
	}
	var view verifyEstimate
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatal(err)
	}
	if view.Checked != 400 || view.Total == nil || *view.Total != 400 || view.Partial {
		t.Fatalf("view=%+v", view)
	}
}

func TestVerifyLineTypeLiveEstimateShortPageWithLargerTotalIsPartial(t *testing.T) {
	testenv.Isolate(t)
	out, _, err := runNovel(t, []string{"contacts", "verify-line-type", "--estimate", "--db", t.TempDir() + "/missing.db", "--json"}, func(w http.ResponseWriter, r *http.Request) {
		items := make([]map[string]any, 50)
		for i := range items {
			items[i] = map[string]any{"phone": "+15550000001"}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": items, "meta": map[string]any{"total": 100}})
	})
	var view verifyEstimate
	if decodeErr := json.Unmarshal([]byte(out), &view); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if ExitCode(err) != 5 || !view.Partial || view.Checked != 50 || view.Total == nil || *view.Total != 100 || !strings.Contains(strings.Join(view.Failures, "; "), "ended after 50 rows") {
		t.Fatalf("err=%v view=%+v", err, view)
	}
}

func TestVerifyLineTypeEstimatePaginatesPastFivePages(t *testing.T) {
	testenv.Isolate(t)
	var calls atomic.Int32
	h := func(w http.ResponseWriter, r *http.Request) {
		page := calls.Add(1)
		count := 200
		if page == 7 {
			count = 3
		}
		items := make([]map[string]any, count)
		for i := range items {
			items[i] = map[string]any{"phone": "+15550000001"}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": items})
	}
	out, _, err := runNovel(t, []string{"contacts", "verify-line-type", "--estimate", "--db", t.TempDir() + "/missing.db", "--json"}, h)
	if err != nil {
		t.Fatal(err)
	}
	var view verifyEstimate
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 7 || view.Unverified != 1203 || view.Partial {
		t.Fatalf("calls=%d view=%+v", calls.Load(), view)
	}
}

func TestVerifyLineTypeEstimateHardCapWarnsInHumanOutput(t *testing.T) {
	testenv.Isolate(t)
	h := func(w http.ResponseWriter, r *http.Request) {
		items := make([]map[string]any, 200)
		for i := range items {
			items[i] = map[string]any{"phone": "+15550000001"}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": items})
	}
	out, _, err := runNovel(t, []string{"contacts", "verify-line-type", "--estimate", "--db", t.TempDir() + "/missing.db", "--human-friendly"}, h)
	if ExitCode(err) != 5 {
		t.Fatalf("err=%v", err)
	}
	if !bytes.Contains([]byte(out), []byte("WARNING:")) || !strings.Contains(out, "lower bound") {
		t.Fatalf("out=%s", out)
	}
}
func TestVerifyLineTypeRunsUntilDone(t *testing.T) {
	testenv.Isolate(t)
	var calls atomic.Int32
	h := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/contacts/verify-line-type" {
			t.Fatalf("%s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"smartListId":"s1"`) {
			t.Fatalf("body=%s", body)
		}
		if calls.Add(1) == 1 {
			_, _ = w.Write([]byte(`{"data":{"verified":2,"remaining":1,"done":false,"byLineType":{"mobile":2}}}`))
		} else {
			_, _ = w.Write([]byte(`{"data":{"verified":1,"remaining":0,"done":true,"byLineType":{"landline":1}}}`))
		}
	}
	out, _, err := runNovel(t, []string{"contacts", "verify-line-type", "run", "--smart-list", "s1", "--until-done", "--json"}, h)
	if err != nil {
		t.Fatal(err)
	}
	out = compactTestJSON(out)
	for _, want := range []string{`"verified":3`, `"batches":2`, `"done":true`, `"mobile":2`, `"landline":1`} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %s in %s", want, out)
		}
	}
}
