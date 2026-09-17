// Sweep findings: row caps plus row/contact/side-effect fetch or decode failures make blame partial and API-class non-zero.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/cliutil/testenv"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

func TestNovelImportsBlameHelpWires(t *testing.T) {
	testenv.Isolate(t)
	_, _, err := runNovel(t, []string{"imports", "blame", "--help"}, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestImportsBlameRejectsInvalidPreferredSideEffectCounter(t *testing.T) {
	testenv.Isolate(t)
	out, _, err := runNovel(t, []string{"imports", "blame", "job", "--db", t.TempDir() + "/missing.db", "--json"}, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/side-effects") {
			_, _ = w.Write([]byte(`{"data":{"total":0,"completed":0,"failed":0,"retryableFailed":null,"retryable":0,"exhaustedFailed":0,"queued":0,"held":0,"actionRequired":false,"isComplete":true}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	compact := compactTestJSON(out)
	if ExitCode(err) != 5 || !strings.Contains(compact, `"partial":true`) || !strings.Contains(compact, "invalid retryableFailed") {
		t.Fatalf("err=%v out=%s", err, out)
	}
}

func TestImportsBlameStrictActionRequiredAliases(t *testing.T) {
	for _, tc := range []struct {
		name      string
		fields    string
		wantErr   string
		wantJSON  string
		wantTable string
	}{
		{"invalid preferred with valid legacy", `,"actionRequired":"bad","action_required":true`, "invalid actionRequired", "", ""},
		{"conflicting dual keys", `,"actionRequired":false,"action_required":true`, "conflicting actionRequired=false and action_required=true", "", ""},
		{"legacy only true", `,"action_required":true`, "", `"action_required":true`, "true"},
		{"legacy only false", `,"action_required":false`, "", `"action_required":false`, "false"},
		{"absent", ``, "", `"action_required":null`, "unknown"},
		{"preferred true", `,"actionRequired":true`, "", `"action_required":true`, "true"},
		{"preferred false", `,"actionRequired":false`, "", `"action_required":false`, "false"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/side-effects") {
					_, _ = w.Write([]byte(`{"data":{"total":0,"completed":0,"failed":0,"retryableFailed":0,"exhaustedFailed":0,"queued":0,"held":0,"isComplete":true` + tc.fields + `}}`))
					return
				}
				_, _ = w.Write([]byte(`{"data":[],"meta":{"total":0}}`))
			}
			testenv.Isolate(t)
			out, _, err := runNovel(t, []string{"imports", "blame", "job", "--db", t.TempDir() + "/missing.db", "--json"}, handler)
			if tc.wantErr != "" {
				if ExitCode(err) != 5 || !strings.Contains(out, tc.wantErr) {
					t.Fatalf("err=%v out=%s", err, out)
				}
				return
			}
			compact := compactTestJSON(out)
			if err != nil || !strings.Contains(compact, tc.wantJSON) {
				t.Fatalf("err=%v out=%s", err, out)
			}
			testenv.Isolate(t)
			table, _, err := runNovel(t, []string{"imports", "blame", "job", "--db", t.TempDir() + "/missing.db", "--human-friendly"}, handler)
			if err != nil || !strings.Contains(table, tc.wantTable) {
				t.Fatalf("err=%v table=%s", err, table)
			}
		})
	}
}

func TestImportsBlameRendersIsCompleteTrue(t *testing.T) {
	testenv.Isolate(t)
	out, _, err := runNovel(t, []string{"imports", "blame", "job", "--db", t.TempDir() + "/missing.db", "--json"}, importsBlameIsCompleteHandler(true))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"job_id":"job","rows":0,"partial":false,"checked":0,"total":0,"delivered":0,"delivery_correlation":"none","delivery_correlation_reason":"GET /reports/sms-delivery returns aggregate totals only (no per-message import, message or contact keys), so delivery is read from the import rows","skipped_by_reason":{},"side_effects":{"queued":0,"retryable":0,"exhausted":0,"total":0,"completed":0,"failed":0,"held":0,"action_required":null,"is_complete":true},"rows_unavailable":false,"fetch_failures":0}`
	if got := compactTestJSON(out); got != want {
		t.Fatalf("output mismatch\nwant: %s\n got: %s", want, got)
	}
}

func TestImportsBlameRendersIsCompleteFalse(t *testing.T) {
	testenv.Isolate(t)
	out, _, err := runNovel(t, []string{"imports", "blame", "job", "--db", t.TempDir() + "/missing.db", "--json"}, importsBlameIsCompleteHandler(false))
	want := `{"job_id":"job","rows":0,"partial":true,"checked":0,"total":0,"delivered":0,"delivery_correlation":"none","delivery_correlation_reason":"GET /reports/sms-delivery returns aggregate totals only (no per-message import, message or contact keys), so delivery is read from the import rows","skipped_by_reason":{},"side_effects":{"queued":0,"retryable":0,"exhausted":0,"total":0,"completed":0,"failed":0,"held":0,"action_required":null,"is_complete":false},"rows_unavailable":false,"fetch_failures":1,"failures":["import side effects are not complete"]}`
	if got := compactTestJSON(out); ExitCode(err) != 5 || got != want {
		t.Fatalf("err=%v\nwant: %s\n got: %s", err, want, got)
	}

	testenv.Isolate(t)
	table, _, err := runNovel(t, []string{"imports", "blame", "job", "--db", t.TempDir() + "/missing.db", "--human-friendly"}, importsBlameIsCompleteHandler(false))
	if ExitCode(err) != 5 || !strings.Contains(table, "WARNING: result is partial") || !strings.Contains(err.Error(), "import side effects are not complete") {
		t.Fatalf("err=%v table=%s", err, table)
	}
}

func importsBlameIsCompleteHandler(value bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/side-effects") {
			_, _ = fmt.Fprintf(w, `{"data":{"total":0,"completed":0,"failed":0,"retryableFailed":0,"exhaustedFailed":0,"queued":0,"held":0,"isComplete":%t}}`, value)
			return
		}
		_, _ = w.Write([]byte(`{"data":[],"meta":{"total":0}}`))
	}
}

func TestImportsBlameRejectsNegativeSideEffectCounters(t *testing.T) {
	for _, tc := range []struct {
		key   string
		value int
	}{{"queued", -1}, {"total", -2}, {"completed", -3}, {"failed", -4}, {"held", -5}, {"valid", 0}} {
		t.Run(tc.key, func(t *testing.T) {
			testenv.Isolate(t)
			counters := map[string]int{"queued": 0, "total": 0, "completed": 0, "failed": 0, "held": 0}
			if tc.key != "valid" {
				counters[tc.key] = tc.value
			}
			out, _, err := runNovel(t, []string{"imports", "blame", "job", "--db", t.TempDir() + "/missing.db", "--json"}, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/imports/job/rows":
					_, _ = w.Write([]byte(`{"data":[],"meta":{"total":0}}`))
				case "/imports/job/side-effects":
					_, _ = fmt.Fprintf(w, `{"data":{"queued":%d,"retryableFailed":0,"exhaustedFailed":0,"total":%d,"completed":%d,"failed":%d,"held":%d,"actionRequired":false,"isComplete":true}}`, counters["queued"], counters["total"], counters["completed"], counters["failed"], counters["held"])
				}
			})
			compact := compactTestJSON(out)
			if tc.key == "valid" {
				if err != nil || !strings.Contains(compact, `"partial":false`) {
					t.Fatalf("err=%v out=%s", err, out)
				}
				return
			}
			if ExitCode(err) != 5 || !strings.Contains(compact, `"partial":true`) || !strings.Contains(out, "invalid "+tc.key) {
				t.Fatalf("err=%v out=%s", err, out)
			}
		})
	}
}

func TestImportsBlameUsesStablePageSize(t *testing.T) {
	testenv.Isolate(t)
	var pageSizes []string
	h := func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/imports/job/rows":
			pageSizes = append(pageSizes, r.URL.Query().Get("per_page"))
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))
			perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
			start, count := (page-1)*perPage, perPage
			if start+count > 450 {
				count = 450 - start
			}
			items := make([]map[string]any, count)
			for i := range items {
				items[i] = map[string]any{"contactId": fmt.Sprintf("c-%d", start+i), "status": "delivered"}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": items})
		case "/reports/sms-delivery":
			t.Fatal("account-wide delivery report must not be used for import attribution")
		case "/imports/job/side-effects":
			_, _ = w.Write([]byte(`{"data":{"scope":"import_job","scopedControlsAvailable":true,"total":0,"pending":0,"processing":0,"completed":0,"skippedInactive":0,"failed":0,"retryableFailed":0,"exhaustedFailed":0,"queued":0,"held":0,"actionRequired":false,"progress":100,"isComplete":true,"byEvent":[]}}`))
		default:
			_, _ = w.Write([]byte(`{"data":{}}`))
		}
	}
	out, _, err := runNovel(t, []string{"imports", "blame", "job", "--limit", "450", "--db", t.TempDir() + "/missing.db", "--json"}, h)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(pageSizes, ",") != "200,200,200" || !strings.Contains(compactTestJSON(out), `"rows":450`) {
		t.Fatalf("page sizes=%v out=%s", pageSizes, out)
	}
}

func TestImportsBlameLimitCapIsPartialAndNonZero(t *testing.T) {
	testenv.Isolate(t)
	h := func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/imports/job/rows":
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))
			perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
			start := (page - 1) * perPage
			count := perPage
			if start+count > 3 {
				count = 3 - start
			}
			items := make([]map[string]any, count)
			for i := range items {
				status := "delivered"
				if start+i == 2 {
					status = "failed"
				}
				items[i] = map[string]any{"contactId": fmt.Sprintf("c-%d", start+i), "status": status}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": items, "meta": map[string]any{"total": 3}})
		case "/imports/job/side-effects":
			_, _ = w.Write([]byte(`{"data":{"scope":"import_job","scopedControlsAvailable":true,"total":0,"pending":0,"processing":0,"completed":0,"skippedInactive":0,"failed":0,"retryableFailed":0,"exhaustedFailed":0,"queued":0,"held":0,"actionRequired":false,"progress":100,"isComplete":true,"byEvent":[]}}`))
		default:
			_, _ = w.Write([]byte(`{"data":{}}`))
		}
	}
	out, _, err := runNovel(t, []string{"imports", "blame", "job", "--limit", "2", "--db", t.TempDir() + "/missing.db", "--json"}, h)
	if ExitCode(err) != 5 {
		t.Fatalf("err=%v code=%d", err, ExitCode(err))
	}
	out = compactTestJSON(out)
	for _, want := range []string{`"partial":true`, `"checked":2`, `"total":3`, `"delivered":2`} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %s in %s", want, out)
		}
	}
	if strings.Contains(out, `"failed":1`) {
		t.Fatalf("row beyond cap must not be included: %s", out)
	}
}

func TestImportsBlameLimitCoveringAllRowsIsComplete(t *testing.T) {
	testenv.Isolate(t)
	out, _, err := runNovel(t, []string{"imports", "blame", "job", "--limit", "3", "--db", t.TempDir() + "/missing.db", "--json"}, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/imports/job/rows":
			_, _ = w.Write([]byte(`{"data":[{"contactId":"c1","status":"delivered"},{"contactId":"c2","status":"delivered"},{"contactId":"c3","status":"failed","reason":"bad"}],"meta":{"total":3}}`))
		case "/imports/job/side-effects":
			_, _ = w.Write([]byte(`{"data":{"scope":"import_job","scopedControlsAvailable":true,"total":0,"pending":0,"processing":0,"completed":0,"skippedInactive":0,"failed":0,"retryableFailed":0,"exhaustedFailed":0,"queued":0,"held":0,"actionRequired":false,"progress":100,"isComplete":true,"byEvent":[]}}`))
		default:
			_, _ = w.Write([]byte(`{"data":{}}`))
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	out = compactTestJSON(out)
	if !strings.Contains(out, `"partial":false`) || !strings.Contains(out, `"checked":3`) || !strings.Contains(out, `"bad":1`) {
		t.Fatalf("output=%s", out)
	}
}

func TestImportsBlameShortPageWithLargerTotalIsPartial(t *testing.T) {
	testenv.Isolate(t)
	out, _, err := runNovel(t, []string{"imports", "blame", "job", "--limit", "500", "--db", t.TempDir() + "/missing.db", "--json"}, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/imports/job/rows":
			rows := make([]map[string]any, 50)
			for i := range rows {
				rows[i] = map[string]any{"contactId": fmt.Sprintf("c-%d", i), "status": "delivered"}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": rows, "meta": map[string]any{"total": 100}})
		case "/imports/job/side-effects":
			_, _ = w.Write([]byte(`{"data":{"scope":"import_job","scopedControlsAvailable":true,"total":0,"pending":0,"processing":0,"completed":0,"skippedInactive":0,"failed":0,"retryableFailed":0,"exhaustedFailed":0,"queued":0,"held":0,"actionRequired":false,"progress":100,"isComplete":true,"byEvent":[]}}`))
		}
	})
	compact := compactTestJSON(out)
	if ExitCode(err) != 5 || !strings.Contains(compact, `"partial":true`) || !strings.Contains(compact, `"checked":50`) || !strings.Contains(compact, `"total":100`) || !strings.Contains(compact, "ended after 50 rows") {
		t.Fatalf("err=%v out=%s", err, out)
	}
}
func TestImportsBlameUsesOnlyImportOutcomesForDelivery(t *testing.T) {
	testenv.Isolate(t)
	h := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/imports/job/rows":
			_, _ = w.Write([]byte(`{"data":{"rows":[{"contactId":"c1","status":"skipped","reason":"landline"},{"contactId":"c2","status":"delivered"},{"contactId":"c3","status":"error","error":"invalid_phone"}]}}`))
		case "/contacts/c1", "/contacts/c2", "/contacts/c3":
			id := strings.TrimPrefix(r.URL.Path, "/contacts/")
			_, _ = w.Write([]byte(`{"data":{"id":"` + id + `","phone":"+15550000000"}}`))
		case "/reports/sms-delivery":
			t.Fatal("account-wide delivery report must not be used for import attribution")
		case "/imports/job/side-effects":
			_, _ = w.Write([]byte(`{"data":{"scope":"import_job","scopedControlsAvailable":true,"total":6,"pending":0,"processing":0,"completed":0,"skippedInactive":0,"failed":4,"retryableFailed":1,"exhaustedFailed":3,"queued":2,"held":0,"actionRequired":true,"progress":100,"isComplete":true,"byEvent":[]}}`))
		default:
			http.NotFound(w, r)
		}
	}
	out, _, err := runNovel(t, []string{"imports", "blame", "job", "--db", t.TempDir() + "/missing.db", "--json"}, h)
	if err != nil {
		t.Fatal(err)
	}
	out = compactTestJSON(out)
	for _, want := range []string{`"job_id":"job"`, `"rows":3`, `"delivered":1`, `"delivery_correlation":"none"`, `"delivery_correlation_reason":"GET /reports/sms-delivery returns aggregate totals only (no per-message import, message or contact keys), so delivery is read from the import rows"`, `"landline":1`, `"invalid_phone":1`, `"queued":2`, `"retryable":1`, `"exhausted":3`, `"action_required":true`} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %s in %s", want, out)
		}
	}
}

func TestImportsBlameStrictRowStatusAliases(t *testing.T) {
	for _, tc := range []struct {
		name      string
		row       string
		wantError string
	}{
		{"non-string preferred does not fall back", `{"contactId":"c1","status":7,"outcome":"delivered"}`, "import row 1 status: invalid status"},
		{"conflicting aliases", `{"contactId":"c1","status":"delivered","outcome":"skipped"}`, "import row 1 status: conflicting status"},
		{"legacy only", `{"contactId":"c1","outcome":"delivered"}`, ""},
		{"preferred only", `{"contactId":"c1","status":"delivered"}`, ""},
		{"missing aliases", `{"contactId":"c1"}`, "import row 1 status: missing status"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Isolate(t)
			out, _, err := runNovel(t, []string{"imports", "blame", "job", "--db", t.TempDir() + "/missing.db", "--json"}, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/imports/job/rows":
					_, _ = w.Write([]byte(`{"data":[` + tc.row + `],"meta":{"total":1}}`))
				case "/contacts/c1":
					_, _ = w.Write([]byte(`{"data":{"id":"c1"}}`))
				case "/imports/job/side-effects":
					_, _ = w.Write([]byte(`{"data":{"total":1,"completed":1,"failed":0,"retryableFailed":0,"exhaustedFailed":0,"queued":0,"held":0,"actionRequired":false,"isComplete":true}}`))
				}
			})
			compact := compactTestJSON(out)
			if tc.wantError == "" {
				if err != nil || !strings.Contains(compact, `"partial":false`) || !strings.Contains(compact, `"delivered":1`) {
					t.Fatalf("err=%v out=%s", err, out)
				}
				return
			}
			if ExitCode(err) != 5 || !strings.Contains(compact, `"partial":true`) || !strings.Contains(out, tc.wantError) {
				t.Fatalf("err=%v out=%s", err, out)
			}
		})
	}
}

func TestImportsBlameDecodesRealRowsAndSideEffectsShapes(t *testing.T) {
	testenv.Isolate(t)
	out, _, err := runNovel(t, []string{"imports", "blame", "job", "--db", t.TempDir() + "/missing.db", "--json"}, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/imports/job/rows":
			_, _ = w.Write([]byte(`{"data":{"rows":[{"contactId":"c1","status":"delivered"}],"meta":{"page":1,"per_page":200,"total":1},"tabs":{"all":2,"success":1,"error":1,"warning":0}}}`))
		case "/imports/job/side-effects":
			_, _ = w.Write([]byte(`{"data":{"scope":"import_job","scopedControlsAvailable":true,"total":2,"pending":0,"processing":0,"completed":1,"skippedInactive":0,"failed":1,"retryableFailed":1,"exhaustedFailed":0,"queued":0,"held":1,"actionRequired":true,"progress":50,"isComplete":false,"byEvent":[]}}`))
		case "/contacts/c1":
			_, _ = w.Write([]byte(`{"data":{"id":"c1"}}`))
		}
	})
	if ExitCode(err) != 5 || !strings.Contains(out, "import side effects are not complete") {
		t.Fatalf("err=%v out=%s", err, out)
	}
	compact := compactTestJSON(out)
	for _, want := range []string{`"row_tabs":{"all":2`, `"rows_unavailable":true`, `"retryable":1`, `"held":1`, `"is_complete":false`} {
		if !strings.Contains(compact, want) {
			t.Fatalf("missing %s in %s", want, out)
		}
	}
}

func TestImportsBlameMissingSideEffectCountersIsPartial(t *testing.T) {
	testenv.Isolate(t)
	out, _, err := runNovel(t, []string{"imports", "blame", "job", "--db", t.TempDir() + "/missing.db", "--json"}, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/imports/job/rows":
			_, _ = w.Write([]byte(`{"data":[],"meta":{"total":0}}`))
		case "/imports/job/side-effects":
			_, _ = w.Write([]byte(`{"data":{}}`))
		}
	})
	compact := compactTestJSON(out)
	if ExitCode(err) != 5 || !strings.Contains(compact, `"partial":true`) || !strings.Contains(compact, "missing or invalid counters: completed, exhausted, failed, held, queued, retryable, total") {
		t.Fatalf("err=%v out=%s", err, out)
	}
}

func TestImportsBlameRejectsSideEffectErrorEnvelopeWithData(t *testing.T) {
	testenv.Isolate(t)
	out, _, err := runNovel(t, []string{"imports", "blame", "job", "--db", t.TempDir() + "/missing.db", "--json"}, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/imports/job/rows":
			_, _ = w.Write([]byte(`{"data":[],"meta":{"total":0}}`))
		case "/imports/job/side-effects":
			_, _ = w.Write([]byte(`{"error":{"code":"unavailable"},"data":{"queued":0,"retryable":0,"exhausted":0}}`))
		}
	})
	if ExitCode(err) != 5 || !strings.Contains(compactTestJSON(out), `"partial":true`) || !strings.Contains(out, "unexpected error envelope") {
		t.Fatalf("err=%v out=%s", err, out)
	}
}

func TestImportsBlameFetchFailuresAreTyped(t *testing.T) {
	for _, endpoint := range []string{"rows", "contact", "side-effects"} {
		for _, tc := range []struct{ status, code int }{{401, 4}, {403, 4}, {404, 3}, {500, 5}} {
			t.Run(endpoint+"/"+http.StatusText(tc.status), func(t *testing.T) {
				testenv.Isolate(t)
				out, _, err := runNovel(t, []string{"imports", "blame", "job", "--db", t.TempDir() + "/missing.db", "--json"}, func(w http.ResponseWriter, r *http.Request) {
					failedPath := map[string]string{"rows": "/imports/job/rows", "contact": "/contacts/c1", "side-effects": "/imports/job/side-effects"}[endpoint]
					if r.URL.Path == failedPath {
						w.WriteHeader(tc.status)
						_, _ = w.Write([]byte(`{"message":"failure"}`))
						return
					}
					switch r.URL.Path {
					case "/imports/job/rows":
						_, _ = w.Write([]byte(`{"data":[{"contactId":"c1","status":"skipped"}]}`))
					case "/contacts/c1":
						_, _ = w.Write([]byte(`{"data":{"id":"c1"}}`))
					case "/imports/job/side-effects":
						_, _ = w.Write([]byte(`{"data":{"scope":"import_job","scopedControlsAvailable":true,"total":0,"pending":0,"processing":0,"completed":0,"skippedInactive":0,"failed":0,"retryableFailed":0,"exhaustedFailed":0,"queued":0,"held":0,"actionRequired":false,"progress":100,"isComplete":true,"byEvent":[]}}`))
					}
				})
				if ExitCode(err) != tc.code || !strings.Contains(compactTestJSON(out), `"partial":true`) {
					t.Fatalf("err=%v code=%d out=%s", err, ExitCode(err), out)
				}
			})
		}
	}
}
func TestImportsBlameEmptyRows(t *testing.T) {
	testenv.Isolate(t)
	out, _, err := runNovel(t, []string{"imports", "blame", "empty", "--db", t.TempDir() + "/missing.db", "--json"}, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/side-effects") {
			_, _ = w.Write([]byte(`{"data":{"scope":"import_job","scopedControlsAvailable":true,"total":0,"pending":0,"processing":0,"completed":0,"skippedInactive":0,"failed":0,"retryableFailed":0,"exhaustedFailed":0,"queued":0,"held":0,"actionRequired":false,"progress":100,"isComplete":true,"byEvent":[]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	if err != nil {
		t.Fatal(err)
	}
	out = compactTestJSON(out)
	if !strings.Contains(out, `"rows":0`) || !strings.Contains(out, `"skipped_by_reason":{}`) {
		t.Fatalf("output=%s", out)
	}
}

func TestImportsBlameRejectsRowsWithoutContactIdentity(t *testing.T) {
	for name, row := range map[string]string{
		"missing":     `{"status":"delivered"}`,
		"blank":       `{"contactId":"","status":"delivered"}`,
		"whitespace":  `{"contactId":"  ","phone":" \t ","status":"delivered"}`,
		"short phone": `{"phone":"5551212","status":"delivered"}`,
	} {
		t.Run(name, func(t *testing.T) {
			testenv.Isolate(t)
			out, _, err := runNovel(t, []string{"imports", "blame", "job", "--db", t.TempDir() + "/missing.db", "--json"}, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/imports/job/rows":
					_, _ = w.Write([]byte(`{"data":[` + row + `]}`))
				case "/imports/job/side-effects":
					_, _ = w.Write([]byte(`{"data":{"scope":"import_job","scopedControlsAvailable":true,"total":0,"pending":0,"processing":0,"completed":0,"skippedInactive":0,"failed":0,"retryableFailed":0,"exhaustedFailed":0,"queued":0,"held":0,"actionRequired":false,"progress":100,"isComplete":true,"byEvent":[]}}`))
				default:
					http.NotFound(w, r)
				}
			})
			if ExitCode(err) != 5 || !strings.Contains(compactTestJSON(out), `"partial":true`) || !strings.Contains(out, "missing a stable contact id or normalizable phone") {
				t.Fatalf("err=%v out=%s", err, out)
			}
		})
	}
}

func TestImportsBlameRejectsNonPositiveLimitBeforeIO(t *testing.T) {
	for _, limit := range []string{"0", "-1"} {
		t.Run(limit, func(t *testing.T) {
			testenv.Isolate(t)
			requests := 0
			_, _, err := runNovel(t, []string{"imports", "blame", "job", "--limit", limit, "--db", t.TempDir() + "/missing.db", "--json"}, func(w http.ResponseWriter, r *http.Request) {
				requests++
				http.NotFound(w, r)
			})
			var ce *cliError
			if !errors.As(err, &ce) || ce.code != 2 {
				t.Fatalf("err=%v", err)
			}
			if requests != 0 {
				t.Fatalf("requests=%d, want 0", requests)
			}
		})
	}
}

func TestImportsBlameNeverClaimsReportCorrelation(t *testing.T) {
	testenv.Isolate(t)
	paths := []string{}
	out, _, err := runNovel(t, []string{"imports", "blame", "job", "--db", t.TempDir() + "/missing.db", "--json"}, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if strings.HasSuffix(r.URL.Path, "/side-effects") {
			_, _ = w.Write([]byte(`{"data":{"scope":"import_job","scopedControlsAvailable":true,"total":0,"pending":0,"processing":0,"completed":0,"skippedInactive":0,"failed":0,"retryableFailed":0,"exhaustedFailed":0,"queued":0,"held":0,"actionRequired":false,"progress":100,"isComplete":true,"byEvent":[]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	if err != nil {
		t.Fatal(err)
	}
	out = compactTestJSON(out)
	if !strings.Contains(out, `"delivery_correlation":"none"`) || !strings.Contains(out, "aggregate totals only") {
		t.Fatalf("output=%s", out)
	}
	for _, p := range paths {
		if strings.Contains(p, "sms-delivery") {
			t.Fatalf("blame must not fetch the aggregate delivery report: %v", paths)
		}
	}
}

func TestImportsBlameRepeatedFullPageIsPartial(t *testing.T) {
	testenv.Isolate(t)
	requests := 0
	out, _, err := runNovel(t, []string{"imports", "blame", "job", "--limit", "500", "--db", t.TempDir() + "/missing.db", "--json"}, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/imports/job/rows":
			requests++
			items := make([]map[string]any, 200)
			for i := range items {
				items[i] = map[string]any{"contactId": fmt.Sprintf("c-%d", i), "status": "delivered"}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": items})
		case "/imports/job/side-effects":
			_, _ = w.Write([]byte(`{"data":{"scope":"import_job","scopedControlsAvailable":true,"total":0,"pending":0,"processing":0,"completed":0,"skippedInactive":0,"failed":0,"retryableFailed":0,"exhaustedFailed":0,"queued":0,"held":0,"actionRequired":false,"progress":100,"isComplete":true,"byEvent":[]}}`))
		default:
			_, _ = w.Write([]byte(`{"data":{}}`))
		}
	})
	if ExitCode(err) != 5 || requests != 2 {
		t.Fatalf("err=%v code=%d requests=%d", err, ExitCode(err), requests)
	}
	if compact := compactTestJSON(out); !strings.Contains(compact, `"partial":true`) || !strings.Contains(compact, "zero new ids") {
		t.Fatalf("out=%s", out)
	}
}
