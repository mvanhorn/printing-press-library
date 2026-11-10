// Ported from the public library entry's imports_blame_test.go (sweep findings: row caps plus row/side-effect fetch or
// decode failures make blame partial and API-class non-zero), re-pinned to the real GET /imports/{id}/rows shape
// ({data:{rows,meta,tabs}}, rows carry row/identifier/outcome/message) and the error tab. PATCH(novel1219-imports-blame)
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/cliutil/testenv"
)

const blameTestJob = `{"data":{"id":"job","status":"completed","totalRows":3,"createdRows":2,"updatedRows":0,"skippedRows":0,"duplicateRows":0,"errorRows":1}}`
const blameTestSideEffects = `{"data":{"scope":"import_job","scopedControlsAvailable":true,"total":1,"pending":0,"processing":0,"completed":1,"skippedInactive":0,"failed":0,"retryableFailed":0,"exhaustedFailed":0,"queued":0,"held":0,"actionRequired":false,"progress":100,"isComplete":true,"byEvent":[]}}`

// blameErrorRows renders one GET /imports/{id}/rows page in the route's real envelope.
func blameErrorRows(rows []map[string]any, total int, tabs map[string]int) string {
	if tabs == nil {
		tabs = map[string]int{"all": total, "success": 0, "error": total, "warning": 0}
	}
	raw, _ := json.Marshal(map[string]any{"data": map[string]any{"rows": rows, "meta": map[string]any{"page": 1, "per_page": 100, "total": total}, "tabs": tabs}})
	return string(raw)
}

func blameErrorRow(n int, message string) map[string]any {
	return map[string]any{"row": n, "identifier": fmt.Sprintf("Row %d", n), "outcome": "error", "objectType": "contact", "message": message}
}

// blameHandler answers the three reads blame makes; a route override replaces one body, "" keeps the default.
func blameHandler(t *testing.T, job, rows, side string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/imports/job":
			if job == "" {
				job = blameTestJob
			}
			_, _ = w.Write([]byte(job))
		case "/imports/job/rows":
			if r.URL.Query().Get("tab") != "error" {
				t.Errorf("rows read without tab=error: %s", r.URL.RawQuery)
			}
			if rows == "" {
				rows = blameErrorRows([]map[string]any{}, 0, nil)
			}
			_, _ = w.Write([]byte(rows))
		case "/imports/job/side-effects":
			if side == "" {
				side = blameTestSideEffects
			}
			_, _ = w.Write([]byte(side))
		case "/reports/sms-delivery":
			t.Error("account-wide delivery report must not be used for import attribution")
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	}
}

func TestNovelImportsBlameHelpWires(t *testing.T) {
	testenv.Isolate(t)
	_, _, err := runNovel(t, []string{"imports", "blame", "--help"}, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestImportsBlameRejectsInvalidPreferredSideEffectCounter(t *testing.T) {
	testenv.Isolate(t)
	out, _, err := runNovel(t, []string{"imports", "blame", "job", "--json"}, blameHandler(t, "", "", `{"data":{"total":0,"completed":0,"failed":0,"retryableFailed":null,"retryable":0,"exhaustedFailed":0,"queued":0,"held":0,"actionRequired":false,"isComplete":true}}`))
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
		{"legacy only true", `,"action_required":true`, "action required", "", ""}, // novel1219 r2: actionRequired is never complete
		{"legacy only false", `,"action_required":false`, "", `"action_required":false`, "false"},
		{"absent", ``, "", `"action_required":null`, "unknown"},
		{"preferred true", `,"actionRequired":true`, "action required", "", ""},
		{"preferred false", `,"actionRequired":false`, "", `"action_required":false`, "false"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			side := `{"data":{"total":1,"completed":1,"failed":0,"retryableFailed":0,"exhaustedFailed":0,"queued":0,"held":0,"isComplete":true` + tc.fields + `}}`
			testenv.Isolate(t)
			out, _, err := runNovel(t, []string{"imports", "blame", "job", "--json"}, blameHandler(t, "", "", side))
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
			table, _, err := runNovel(t, []string{"imports", "blame", "job", "--human-friendly"}, blameHandler(t, "", "", side))
			if err != nil || !strings.Contains(table, tc.wantTable) {
				t.Fatalf("err=%v table=%s", err, table)
			}
		})
	}
}

func TestImportsBlameRendersIsCompleteTrue(t *testing.T) {
	testenv.Isolate(t)
	out, _, err := runNovel(t, []string{"imports", "blame", "job", "--json"}, blameHandler(t, "", blameErrorRows([]map[string]any{blameErrorRow(3, "invalid phone")}, 1, nil), ""))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"job_id":"job","row_outcomes":{"status":"completed","total":3,"created":2,"updated":0,"skipped":0,"duplicates":0,"errors":1},"error_rows":1,"partial":false,"checked":1,"total":1,"side_effects_completed":1,"note":"provider delivery is not available from this data; side_effects_completed counts the import's side effects that fired","errors_by_reason":{"invalid phone":1},"side_effects":{"queued":0,"retryable":0,"exhausted":0,"total":1,"completed":1,"failed":0,"held":0,"action_required":false,"is_complete":true,"skipped_inactive":0,"pending":0,"processing":0},"row_tabs":{"all":1,"error":1,"success":0,"warning":0},"rows_unavailable":false,"fetch_failures":0}`
	if got := compactTestJSON(out); got != want {
		t.Fatalf("output mismatch\nwant: %s\n got: %s", want, got)
	}
}

func TestImportsBlameRendersIsCompleteFalse(t *testing.T) {
	side := `{"data":{"total":1,"completed":0,"failed":0,"retryableFailed":0,"exhaustedFailed":0,"queued":1,"held":0,"isComplete":false}}`
	testenv.Isolate(t)
	out, _, err := runNovel(t, []string{"imports", "blame", "job", "--json"}, blameHandler(t, "", "", side))
	compact := compactTestJSON(out)
	if ExitCode(err) != 5 || !strings.Contains(compact, `"partial":true`) || !strings.Contains(compact, `"is_complete":false`) || !strings.Contains(compact, `"failures":["import side effects are not complete","import side effects 1 queued sends","import side effects 0 of 1 side effects completed"]`) {
		t.Fatalf("err=%v out=%s", err, compact)
	}
	testenv.Isolate(t)
	table, _, err := runNovel(t, []string{"imports", "blame", "job", "--human-friendly"}, blameHandler(t, "", "", side))
	if ExitCode(err) != 5 || !strings.Contains(table, "WARNING: result is partial") || !strings.Contains(err.Error(), "import side effects are not complete") {
		t.Fatalf("err=%v table=%s", err, table)
	}
}

func TestImportsBlameRejectsNegativeSideEffectCounters(t *testing.T) {
	for _, tc := range []struct {
		key   string
		value int
	}{{"queued", -1}, {"total", -2}, {"completed", -3}, {"failed", -4}, {"held", -5}, {"valid", 0}} {
		t.Run(tc.key, func(t *testing.T) {
			testenv.Isolate(t)
			counters := map[string]int{"queued": 0, "total": 1, "completed": 1, "failed": 0, "held": 0}
			if tc.key != "valid" {
				counters[tc.key] = tc.value
			}
			side := fmt.Sprintf(`{"data":{"queued":%d,"retryableFailed":0,"exhaustedFailed":0,"total":%d,"completed":%d,"failed":%d,"held":%d,"actionRequired":false,"isComplete":true}}`, counters["queued"], counters["total"], counters["completed"], counters["failed"], counters["held"])
			out, _, err := runNovel(t, []string{"imports", "blame", "job", "--json"}, blameHandler(t, "", "", side))
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

// pagedBlameRows serves n error rows across pages of the requested size (capped at the route's 100).
func pagedBlameRows(t *testing.T, n int, pageSizes *[]string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/imports/job/rows" {
			blameHandler(t, "", "", "")(w, r)
			return
		}
		if r.URL.Query().Get("tab") != "error" {
			t.Errorf("rows read without tab=error: %s", r.URL.RawQuery)
		}
		if pageSizes != nil {
			*pageSizes = append(*pageSizes, r.URL.Query().Get("per_page"))
		}
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
		perPage = minInt(perPage, importRowsMaxPerPage)
		start, count := (page-1)*perPage, perPage
		if start+count > n {
			count = max(0, n-start)
		}
		items := make([]map[string]any, count)
		for i := range items {
			items[i] = blameErrorRow(start+i+1, "invalid phone")
		}
		_, _ = w.Write([]byte(blameErrorRows(items, n, nil)))
	}
}

// Greptile #2450 "Import error counts are wrong": the rows route clamps per_page to 100, so a larger page size reads a
// clamped page as the last one; blame asks for at most 100 and the error tab only.
func TestImportsBlameUsesTheRoutesPageSizeAndErrorTab(t *testing.T) {
	testenv.Isolate(t)
	var pageSizes []string
	out, _, err := runNovel(t, []string{"imports", "blame", "job", "--limit", "450", "--json"}, pagedBlameRows(t, 250, &pageSizes))
	if err != nil {
		t.Fatalf("err=%v out=%s", err, out)
	}
	compact := compactTestJSON(out)
	if strings.Join(pageSizes, ",") != "100,100,100" || !strings.Contains(compact, `"error_rows":250`) || !strings.Contains(compact, `"invalid phone":250`) {
		t.Fatalf("page sizes=%v out=%s", pageSizes, compact)
	}
}

// Greptile #2450 "Import error counts are wrong": only error rows are grouped; a non-error row the error tab returns
// is never counted as an error with "(no reason recorded)", it makes the result partial.
func TestImportsBlameGroupsOnlyErrorRows(t *testing.T) {
	testenv.Isolate(t)
	rows := blameErrorRows([]map[string]any{
		blameErrorRow(1, "invalid phone"),
		{"row": 2, "identifier": "Row 2", "outcome": "created", "objectType": "contact"},
		blameErrorRow(3, "invalid phone"),
	}, 3, nil)
	out, _, err := runNovel(t, []string{"imports", "blame", "job", "--json"}, blameHandler(t, "", rows, ""))
	compact := compactTestJSON(out)
	if ExitCode(err) != 5 || !strings.Contains(compact, `"errors_by_reason":{"invalid phone":2}`) || !strings.Contains(compact, `has outcome \"created\", not error`) || strings.Contains(compact, "no reason recorded") {
		t.Fatalf("err=%v out=%s", err, compact)
	}
}

func TestImportsBlameLimitCapIsPartialAndNonZero(t *testing.T) {
	testenv.Isolate(t)
	out, _, err := runNovel(t, []string{"imports", "blame", "job", "--limit", "2", "--json"}, pagedBlameRows(t, 3, nil))
	if ExitCode(err) != 5 {
		t.Fatalf("err=%v code=%d", err, ExitCode(err))
	}
	out = compactTestJSON(out)
	for _, want := range []string{`"partial":true`, `"checked":2`, `"total":3`, `"invalid phone":2`, "--limit 2 capped"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %s in %s", want, out)
		}
	}
}

func TestImportsBlameLimitCoveringAllRowsIsComplete(t *testing.T) {
	testenv.Isolate(t)
	rows := blameErrorRows([]map[string]any{blameErrorRow(1, "bad"), blameErrorRow(2, "bad"), blameErrorRow(3, "worse")}, 3, nil)
	out, _, err := runNovel(t, []string{"imports", "blame", "job", "--limit", "3", "--json"}, blameHandler(t, "", rows, ""))
	if err != nil {
		t.Fatal(err)
	}
	out = compactTestJSON(out)
	if !strings.Contains(out, `"partial":false`) || !strings.Contains(out, `"checked":3`) || !strings.Contains(out, `"bad":2`) || !strings.Contains(out, `"worse":1`) {
		t.Fatalf("output=%s", out)
	}
}

func TestImportsBlameShortPageWithLargerTotalIsPartial(t *testing.T) {
	testenv.Isolate(t)
	items := make([]map[string]any, 50)
	for i := range items {
		items[i] = blameErrorRow(i+1, "invalid phone")
	}
	out, _, err := runNovel(t, []string{"imports", "blame", "job", "--limit", "500", "--json"}, blameHandler(t, "", blameErrorRows(items, 100, nil), ""))
	compact := compactTestJSON(out)
	if ExitCode(err) != 5 || !strings.Contains(compact, `"partial":true`) || !strings.Contains(compact, `"checked":50`) || !strings.Contains(compact, `"total":100`) || !strings.Contains(compact, "ended after 50 rows") {
		t.Fatalf("err=%v out=%s", err, out)
	}
}

func TestImportsBlameReadsSideEffectsForDelivery(t *testing.T) {
	testenv.Isolate(t)
	side := `{"data":{"scope":"import_job","scopedControlsAvailable":true,"total":6,"pending":0,"processing":0,"completed":0,"skippedInactive":0,"failed":4,"retryableFailed":1,"exhaustedFailed":3,"queued":2,"held":0,"actionRequired":true,"progress":100,"isComplete":true,"byEvent":[]}}`
	rows := blameErrorRows([]map[string]any{blameErrorRow(4, "landline"), blameErrorRow(5, "invalid_phone")}, 2, nil)
	out, _, err := runNovel(t, []string{"imports", "blame", "job", "--json"}, blameHandler(t, "", rows, side))
	if ExitCode(err) != 5 { // novel1219 r2: queued, failed and exhausted sends make the report partial
		t.Fatalf("err=%v code=%d", err, ExitCode(err))
	}
	out = compactTestJSON(out)
	for _, want := range []string{`"job_id":"job"`, `"error_rows":2`, `"side_effects_completed":0`, `"note":"provider delivery is not available from this data`, `"landline":1`, `"invalid_phone":1`, `"queued":2`, `"retryable":1`, `"exhausted":3`, `"action_required":true`} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %s in %s", want, out)
		}
	}
}

func TestImportsBlameStrictRowOutcomeAliases(t *testing.T) {
	for _, tc := range []struct {
		name      string
		row       string
		wantError string
	}{
		{"non-string preferred does not fall back", `{"row":1,"outcome":7,"status":"error","message":"x"}`, "import error row 1 outcome: invalid outcome"},
		{"conflicting aliases", `{"row":1,"outcome":"error","status":"skipped","message":"x"}`, "import error row 1 outcome: conflicting outcome"},
		{"legacy only", `{"row":1,"status":"error","message":"x"}`, ""},
		{"preferred only", `{"row":1,"outcome":"error","message":"x"}`, ""},
		{"missing aliases", `{"row":1,"message":"x"}`, "import error row 1 outcome: missing outcome"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Isolate(t)
			rows := `{"data":{"rows":[` + tc.row + `],"meta":{"total":1},"tabs":{"error":1}}}`
			out, _, err := runNovel(t, []string{"imports", "blame", "job", "--json"}, blameHandler(t, "", rows, ""))
			compact := compactTestJSON(out)
			if tc.wantError == "" {
				if err != nil || !strings.Contains(compact, `"partial":false`) || !strings.Contains(compact, `"x":1`) {
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
	rows := blameErrorRows([]map[string]any{blameErrorRow(2, "invalid phone")}, 1, map[string]int{"all": 4, "success": 1, "error": 3, "warning": 0})
	side := `{"data":{"scope":"import_job","scopedControlsAvailable":true,"total":2,"pending":0,"processing":0,"completed":1,"skippedInactive":0,"failed":1,"retryableFailed":1,"exhaustedFailed":0,"queued":0,"held":1,"actionRequired":true,"progress":50,"isComplete":false,"byEvent":[]}}`
	out, _, err := runNovel(t, []string{"imports", "blame", "job", "--json"}, blameHandler(t, "", rows, side))
	if ExitCode(err) != 5 || !strings.Contains(out, "import side effects are not complete") {
		t.Fatalf("err=%v out=%s", err, out)
	}
	compact := compactTestJSON(out)
	for _, want := range []string{`"row_tabs":{"all":4`, `"rows_unavailable":true`, "3 error rows were counted but only 1 were itemized", `"retryable":1`, `"held":1`, `"is_complete":false`} {
		if !strings.Contains(compact, want) {
			t.Fatalf("missing %s in %s", want, out)
		}
	}
}

func TestImportsBlameMissingSideEffectCountersIsPartial(t *testing.T) {
	testenv.Isolate(t)
	out, _, err := runNovel(t, []string{"imports", "blame", "job", "--json"}, blameHandler(t, "", "", `{"data":{}}`))
	compact := compactTestJSON(out)
	if ExitCode(err) != 5 || !strings.Contains(compact, `"partial":true`) || !strings.Contains(compact, "missing or invalid counters: completed, exhausted, failed, held, queued, retryable, total") {
		t.Fatalf("err=%v out=%s", err, out)
	}
}

func TestImportsBlameMissingJobCountersIsPartial(t *testing.T) {
	testenv.Isolate(t)
	out, _, err := runNovel(t, []string{"imports", "blame", "job", "--json"}, blameHandler(t, `{"data":{"id":"job","status":"completed","errorRows":-1}}`, "", ""))
	compact := compactTestJSON(out)
	if ExitCode(err) != 5 || !strings.Contains(compact, "import job missing or invalid fields") || !strings.Contains(compact, "invalid errorRows") {
		t.Fatalf("err=%v out=%s", err, out)
	}
}

func TestImportsBlameRejectsSideEffectErrorEnvelopeWithData(t *testing.T) {
	testenv.Isolate(t)
	out, _, err := runNovel(t, []string{"imports", "blame", "job", "--json"}, blameHandler(t, "", "", `{"error":{"code":"unavailable"},"data":{"queued":0,"retryable":0,"exhausted":0}}`))
	if ExitCode(err) != 5 || !strings.Contains(compactTestJSON(out), `"partial":true`) || !strings.Contains(out, "unexpected error envelope") {
		t.Fatalf("err=%v out=%s", err, out)
	}
}

func TestImportsBlameFetchFailuresAreTyped(t *testing.T) {
	for _, endpoint := range []string{"rows", "side-effects"} {
		for _, tc := range []struct{ status, code int }{{401, 4}, {403, 4}, {404, 3}, {500, 5}} {
			t.Run(endpoint+"/"+http.StatusText(tc.status), func(t *testing.T) {
				testenv.Isolate(t)
				failedPath := map[string]string{"rows": "/imports/job/rows", "side-effects": "/imports/job/side-effects"}[endpoint]
				out, _, err := runNovel(t, []string{"imports", "blame", "job", "--json"}, func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == failedPath {
						w.WriteHeader(tc.status)
						_, _ = w.Write([]byte(`{"message":"failure"}`))
						return
					}
					blameHandler(t, "", "", "")(w, r)
				})
				if ExitCode(err) != tc.code || !strings.Contains(compactTestJSON(out), `"partial":true`) {
					t.Fatalf("err=%v code=%d out=%s", err, ExitCode(err), out)
				}
			})
		}
	}
}

func TestImportsBlameUnknownJobIsNotFound(t *testing.T) {
	testenv.Isolate(t)
	_, _, err := runNovel(t, []string{"imports", "blame", "missing", "--json"}, nil)
	if ExitCode(err) != 3 {
		t.Fatalf("err=%v code=%d, want the API's 404 (exit 3)", err, ExitCode(err))
	}
}

func TestImportsBlameEmptyRows(t *testing.T) {
	testenv.Isolate(t)
	out, _, err := runNovel(t, []string{"imports", "blame", "job", "--json"}, blameHandler(t, "", "", ""))
	if err != nil {
		t.Fatal(err)
	}
	out = compactTestJSON(out)
	if !strings.Contains(out, `"error_rows":0`) || !strings.Contains(out, `"errors_by_reason":{}`) {
		t.Fatalf("output=%s", out)
	}
}

func TestImportsBlameRejectsRowsWithoutIdentity(t *testing.T) {
	for name, row := range map[string]string{
		"missing":      `{"outcome":"error","message":"x"}`,
		"negative row": `{"row":-1,"outcome":"error","message":"x"}`,
		"whitespace":   `{"contactId":"  ","phone":" \t ","outcome":"error","message":"x"}`,
		"short phone":  `{"phone":"5551212","outcome":"error","message":"x"}`,
	} {
		t.Run(name, func(t *testing.T) {
			testenv.Isolate(t)
			rows := `{"data":{"rows":[` + row + `],"meta":{"total":1},"tabs":{"error":1}}}`
			out, _, err := runNovel(t, []string{"imports", "blame", "job", "--json"}, blameHandler(t, "", rows, ""))
			if ExitCode(err) != 5 || !strings.Contains(compactTestJSON(out), `"partial":true`) || !strings.Contains(out, "has no row number or contact identity") {
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
			_, _, err := runNovel(t, []string{"imports", "blame", "job", "--limit", limit, "--json"}, func(w http.ResponseWriter, r *http.Request) {
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

func TestImportsBlameRowsAliasSetsTheLimit(t *testing.T) {
	testenv.Isolate(t)
	var pageSizes []string
	_, _, err := runNovel(t, []string{"imports", "blame", "job", "--rows", "5", "--json"}, pagedBlameRows(t, 3, &pageSizes))
	if err != nil || strings.Join(pageSizes, ",") != "6" {
		t.Fatalf("err=%v page sizes=%v, want one page of --rows+1", err, pageSizes)
	}
}

func TestImportsBlameNeverClaimsReportCorrelation(t *testing.T) {
	testenv.Isolate(t)
	paths := []string{}
	out, _, err := runNovel(t, []string{"imports", "blame", "job", "--json"}, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		blameHandler(t, "", "", "")(w, r)
	})
	if err != nil {
		t.Fatal(err)
	}
	out = compactTestJSON(out)
	if !strings.Contains(out, "provider delivery is not available from this data") || strings.Contains(out, `"delivered"`) {
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
	items := make([]map[string]any, 100)
	for i := range items {
		items[i] = blameErrorRow(i+1, "invalid phone")
	}
	out, _, err := runNovel(t, []string{"imports", "blame", "job", "--limit", "500", "--json"}, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/imports/job/rows" {
			requests++
			_, _ = w.Write([]byte(blameErrorRows(items, 0, nil)))
			return
		}
		blameHandler(t, "", "", "")(w, r)
	})
	if ExitCode(err) != 5 || requests != 2 {
		t.Fatalf("err=%v code=%d requests=%d", err, ExitCode(err), requests)
	}
	if compact := compactTestJSON(out); !strings.Contains(compact, `"partial":true`) || !strings.Contains(compact, "zero new ids") {
		t.Fatalf("out=%s", out)
	}
}

// novel1219: the route answers isComplete=false for a job that recorded no side effects (total 0); with every counter
// zero that is "nothing to deliver", reported as no_side_effects, not an unfinished job. Any recorded send still counts.
func TestImportsBlameNoRecordedSideEffectsIsComplete(t *testing.T) {
	testenv.Isolate(t)
	out, _, err := runNovel(t, []string{"imports", "blame", "job", "--json"}, blameHandler(t, "", "", `{"data":{"total":0,"completed":0,"failed":0,"retryableFailed":0,"exhaustedFailed":0,"queued":0,"held":0,"actionRequired":false,"isComplete":false}}`))
	compact := compactTestJSON(out)
	if err != nil || !strings.Contains(compact, `"partial":false`) || !strings.Contains(compact, `"no_side_effects":true`) {
		t.Fatalf("err=%v out=%s", err, compact)
	}
	testenv.Isolate(t)
	out, _, err = runNovel(t, []string{"imports", "blame", "job", "--json"}, blameHandler(t, "", "", `{"data":{"total":0,"completed":0,"failed":0,"retryableFailed":0,"exhaustedFailed":0,"queued":0,"held":1,"actionRequired":true,"isComplete":false}}`))
	if ExitCode(err) != 5 || !strings.Contains(compactTestJSON(out), "import side effects are not complete") {
		t.Fatalf("held send: err=%v out=%s", err, out)
	}
}
