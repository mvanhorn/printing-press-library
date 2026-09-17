// Sweep findings: either current/prior report fetch or decode failure prints a partial comparison envelope and exits API-class non-zero.
package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/cliutil/testenv"
)

func TestNovelReportsCompareHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"reports", "compare", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{"Usage:", "compare", "--pipeline", "--assigned-to"} {
		if !strings.Contains(out.String(), w) {
			t.Fatalf("missing %q: %s", w, out.String())
		}
	}
	if !strings.Contains(out.String(), "reports compare speed-to-lead --range 7d --json\n") || !strings.Contains(out.String(), "--assigned-to <userId> --json") {
		t.Fatalf("examples are not runnable: %s", out.String())
	}
}

func TestNovelReportsCompareBehavior(t *testing.T) {
	testenv.Isolate(t)
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		q := r.URL.Query()
		if r.URL.Path != "/reports/funnel" || q.Get("pipelineId") != "p1" || q.Get("assignedTo") != "u1" {
			t.Errorf("request=%s?%s", r.URL.Path, r.URL.RawQuery)
		}
		switch q.Get("from") {
		case "2026-09-10T00:00:00Z":
			if q.Get("to") != "2026-09-20T00:00:00Z" {
				t.Errorf("current to=%s", q.Get("to"))
			}
			_, _ = w.Write([]byte(`{"data":{"total":15,"nested":{"rate":4},"items":[{"count":0},{"count":6}],"account_id":999,"created_at":123}}`))
		case "2026-08-31T00:00:00Z":
			if q.Get("to") != "2026-09-10T00:00:00Z" {
				t.Errorf("prior to=%s", q.Get("to"))
			}
			_, _ = w.Write([]byte(`{"data":{"total":10,"nested":{"rate":2},"items":[{"count":0},{"count":3}],"account_id":111,"created_at":100}}`))
		default:
			t.Errorf("from=%s", q.Get("from"))
		}
	}))
	defer srv.Close()
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL)
	t.Setenv("CONDUYT_API_KEY", "test")
	flags := &rootFlags{asJSON: true}
	cmd := newNovelReportsCompareCmd(flags)
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"funnel", "--from", "2026-09-10", "--to", "2026-09-20", "--pipeline", "p1", "--assigned-to", "u1"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if ExitCode(err) != 5 {
		t.Fatalf("err=%v", err)
	}
	if calls != 2 {
		t.Fatalf("calls=%d", calls)
	}
	var view compareView
	if err := json.Unmarshal(out.Bytes(), &view); err != nil {
		t.Fatalf("decode %v: %s", err, out.String())
	}
	if view.Report != "funnel" || view.Current.From == "" || view.Prior.To == "" {
		t.Fatalf("envelope=%+v", view)
	}
	rows := view.Rows
	if len(rows) != 2 {
		t.Fatalf("rows=%+v", rows)
	}
	by := map[string]compareRow{}
	for _, r := range rows {
		by[r.Metric] = r
	}
	if by["total"].Delta != 5 || by["total"].DeltaPct == nil || *by["total"].DeltaPct != 50 {
		t.Fatalf("total=%+v", by["total"])
	}
	if by["nested.rate"].DeltaPct == nil || *by["nested.rate"].DeltaPct != 100 {
		t.Fatalf("nested=%+v", by["nested.rate"])
	}
	if len(view.NonComparable) != 1 || view.NonComparable[0].Path != "items" {
		t.Fatalf("non_comparable=%+v", view.NonComparable)
	}
	if !view.Partial || len(view.Failures) != 1 {
		t.Fatalf("partial metadata=%+v", view)
	}
	if _, ok := by["account_id"]; ok {
		t.Fatal("identifier was not ignored")
	}
}

func TestNovelReportsCompareUnknownIsUsage(t *testing.T) {
	testenv.Isolate(t)
	flags := &rootFlags{asJSON: true}
	cmd := newNovelReportsCompareCmd(flags)
	cmd.SetArgs([]string{"bogus"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if ExitCode(err) != 2 || !strings.Contains(err.Error(), "valid reports:") {
		t.Fatalf("err=%v code=%d", err, ExitCode(err))
	}
}

func TestCompareDocumentsEmpty(t *testing.T) {
	rows := compareDocuments(map[string]any{"name": "x"}, map[string]any{"name": "y"})
	if rows == nil || len(rows) != 0 {
		t.Fatalf("rows=%v", rows)
	}
}

func TestNovelReportsCompareMetricFreePayloadsArePartial(t *testing.T) {
	for _, tc := range []struct {
		name, prior, current string
		checked              int
	}{
		{"both envelopes", `{"message":"unavailable"}`, `{"error":"unavailable"}`, 0},
		{"prior only", `{"label":"none"}`, `{"count":2}`, 2},
		{"current only", `{"count":1}`, `{"ok":true,"value":null}`, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Isolate(t)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				payload := tc.prior
				if r.URL.Query().Get("from") == "2026-09-10T00:00:00Z" {
					payload = tc.current
				}
				fmt.Fprint(w, `{"data":`+payload+`}`)
			}))
			defer srv.Close()
			t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL)
			t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
			cmd := newNovelReportsCompareCmd(&rootFlags{asJSON: true})
			cmd.SilenceErrors, cmd.SilenceUsage = true, true
			cmd.SetArgs([]string{"speed-to-lead", "--from", "2026-09-10", "--to", "2026-09-20"})
			var out bytes.Buffer
			cmd.SetOut(&out)
			err := cmd.Execute()
			var view compareView
			if decodeErr := json.Unmarshal(out.Bytes(), &view); decodeErr != nil {
				t.Fatal(decodeErr)
			}
			if ExitCode(err) != 5 || !view.Partial || view.Checked != tc.checked || view.Total != 2 || len(view.Failures) != 1 {
				t.Fatalf("err=%v view=%+v", err, view)
			}
		})
	}
}

func TestNovelReportsCompareRejectsErrorEnvelopesBeforeUnwrapping(t *testing.T) {
	for _, tc := range []struct {
		name        string
		failingSide string
		payload     string
		checked     int
	}{
		{"current top-level string", "current", `{"error":"upstream unavailable","data":{"count":42}}`, 0},
		{"current top-level object", "current", `{"error":{"reason":"unavailable"},"data":{"count":42}}`, 0},
		{"current nested string", "current", `{"data":{"error":"upstream unavailable","count":42}}`, 0},
		{"current nested object", "current", `{"data":{"error":{"reason":"unavailable"},"count":42}}`, 0},
		{"prior top-level string", "prior", `{"error":"upstream unavailable","data":{"count":42}}`, 1},
		{"prior top-level object", "prior", `{"error":{"reason":"unavailable"},"data":{"count":42}}`, 1},
		{"prior nested string", "prior", `{"data":{"error":"upstream unavailable","count":42}}`, 1},
		{"prior nested object", "prior", `{"data":{"error":{"reason":"unavailable"},"count":42}}`, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Isolate(t)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				side := "prior"
				if r.URL.Query().Get("from") == "2026-09-10T00:00:00Z" {
					side = "current"
				}
				if side == tc.failingSide {
					fmt.Fprint(w, tc.payload)
					return
				}
				fmt.Fprint(w, `{"data":{"count":7}}`)
			}))
			defer srv.Close()
			t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL)
			t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
			cmd := newNovelReportsCompareCmd(&rootFlags{asJSON: true})
			cmd.SilenceErrors, cmd.SilenceUsage = true, true
			cmd.SetArgs([]string{"speed-to-lead", "--from", "2026-09-10", "--to", "2026-09-20"})
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&bytes.Buffer{})
			err := cmd.Execute()
			var view compareView
			if decodeErr := json.Unmarshal(out.Bytes(), &view); decodeErr != nil {
				t.Fatalf("decode %v: %s", decodeErr, out.String())
			}
			if ExitCode(err) != 5 || !view.Partial || view.Checked != tc.checked || view.Total != 2 || len(view.Rows) != 0 || len(view.Failures) != 1 {
				t.Fatalf("err=%v view=%+v", err, view)
			}
		})
	}
}

func TestRejectResponseErrorEnvelopeObjectErrorsWithoutMessageOrCode(t *testing.T) {
	for _, raw := range []string{
		`{"error":{"reason":"unavailable"},"data":{"count":42}}`,
		`{"data":{"error":{"reason":"unavailable"},"count":42}}`,
	} {
		if err := rejectResponseErrorEnvelope(json.RawMessage(raw)); err == nil {
			t.Fatalf("accepted error envelope: %s", raw)
		}
	}
}

func TestRejectResponseErrorEnvelopeRejectsEveryNonNullErrorType(t *testing.T) {
	for _, value := range []string{`"failed"`, `{"reason":"failed"}`, `["failed"]`, `42`, `true`, `false`} {
		raw := json.RawMessage(`{"error":` + value + `,"data":{"count":42}}`)
		if err := rejectResponseErrorEnvelope(raw); err == nil {
			t.Fatalf("accepted error value %s", value)
		}
	}
	if err := rejectResponseErrorEnvelope(json.RawMessage(`{"error":null,"data":{"count":42}}`)); err != nil {
		t.Fatalf("rejected null error: %v", err)
	}
}

func TestRejectResponseErrorEnvelopeRejectsMessageObjects(t *testing.T) {
	for _, raw := range []string{
		`{"message":{"reason":"unavailable"},"data":{"count":42}}`,
		`{"data":{"message":{"reason":"unavailable"},"count":42}}`,
	} {
		if err := rejectResponseErrorEnvelope(json.RawMessage(raw)); err == nil {
			t.Fatalf("accepted message envelope: %s", raw)
		}
	}
}

func TestCompareDocumentsCorrelatesReorderedArraysByIdentity(t *testing.T) {
	prior := map[string]any{"items": []any{
		map[string]any{"userId": "abc", "calls": json.Number("2")},
		map[string]any{"userId": "xyz", "calls": json.Number("7")},
	}}
	current := map[string]any{"items": []any{
		map[string]any{"userId": "xyz", "calls": json.Number("10")},
		map[string]any{"userId": "abc", "calls": json.Number("5")},
	}}
	rows, nonComparable := compareDocumentsDetailed(prior, current)
	if len(nonComparable) != 0 || len(rows) != 2 {
		t.Fatalf("rows=%+v non_comparable=%+v", rows, nonComparable)
	}
	by := map[string]compareRow{}
	for _, row := range rows {
		by[row.Metric] = row
	}
	if by["items[userId=abc].calls"].Delta != 3 || by["items[userId=xyz].calls"].Delta != 3 {
		t.Fatalf("rows=%+v", rows)
	}
}

func TestCompareDocumentsReportsUnkeyedObjectArrayNonComparable(t *testing.T) {
	prior := map[string]any{"items": []any{map[string]any{"calls": json.Number("2")}}}
	current := map[string]any{"items": []any{map[string]any{"calls": json.Number("5")}}}
	rows, nonComparable := compareDocumentsDetailed(prior, current)
	if len(rows) != 0 {
		t.Fatalf("rows=%+v", rows)
	}
	if len(nonComparable) != 1 || nonComparable[0].Path != "items" || !strings.Contains(nonComparable[0].Reason, "stable identity") {
		t.Fatalf("non_comparable=%+v", nonComparable)
	}
}

func TestNovelReportsCompareAllSkippedArraysArePartial(t *testing.T) {
	for _, tc := range []struct{ name, payload, reason string }{
		{"unkeyed objects", `{"items":[{"calls":2}]}`, "stable identity"},
		{"duplicate identities", `{"items":[{"id":"same","calls":2},{"id":"same","calls":3}]}`, "not unique"},
		{"primitive values", `{"items":[2,3]}`, "stable identity"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Isolate(t)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `{"data":`+tc.payload+`}`)
			}))
			defer srv.Close()
			t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL)
			t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
			cmd := newNovelReportsCompareCmd(&rootFlags{asJSON: true})
			cmd.SilenceErrors, cmd.SilenceUsage = true, true
			cmd.SetArgs([]string{"speed-to-lead", "--from", "2026-09-10", "--to", "2026-09-20"})
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&bytes.Buffer{})
			err := cmd.Execute()
			var view compareView
			if decodeErr := json.Unmarshal(out.Bytes(), &view); decodeErr != nil {
				t.Fatal(decodeErr)
			}
			if ExitCode(err) != 5 || !view.Partial || len(view.NonComparable) != 1 || view.NonComparable[0].Path != "items" || !strings.Contains(view.NonComparable[0].Reason, tc.reason) || len(view.Failures) != 1 {
				t.Fatalf("err=%v view=%+v", err, view)
			}
		})
	}
}

func TestNovelReportsCompareOneSidedMetricsArePartial(t *testing.T) {
	for _, tc := range []struct{ name, prior, current, path string }{
		{"added", `{"shared":1}`, `{"shared":2,"added":3}`, "added"},
		{"removed", `{"shared":1,"removed":3}`, `{"shared":2}`, "removed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Isolate(t)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("from") == "2026-09-10T00:00:00Z" {
					fmt.Fprint(w, `{"data":`+tc.current+`}`)
				} else {
					fmt.Fprint(w, `{"data":`+tc.prior+`}`)
				}
			}))
			defer srv.Close()
			t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL)
			t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
			cmd := newNovelReportsCompareCmd(&rootFlags{asJSON: true})
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
			cmd.SetArgs([]string{"speed-to-lead", "--from", "2026-09-10", "--to", "2026-09-20"})
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&bytes.Buffer{})
			err := cmd.Execute()
			var view compareView
			if decodeErr := json.Unmarshal(out.Bytes(), &view); decodeErr != nil {
				t.Fatal(decodeErr)
			}
			if ExitCode(err) != 5 || !view.Partial || view.Checked != 2 || view.Total != 2 || len(view.NonComparable) != 1 || view.NonComparable[0].Path != tc.path || len(view.Failures) != 1 {
				t.Fatalf("err=%v view=%+v", err, view)
			}
		})
	}
}

func TestNovelReportsCompareOneSidedDimensionMemberIsNotPartial(t *testing.T) {
	testenv.Isolate(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload := `{"shared":1,"byAssignee":[{"name":"Assigned","channels":{"call":2}}]}`
		if r.URL.Query().Get("from") == "2026-09-10T00:00:00Z" {
			payload = `{"shared":2,"byAssignee":[{"name":"Assigned","channels":{"call":3}},{"name":"Unassigned","channels":{"call":1}}]}`
		}
		fmt.Fprint(w, `{"data":`+payload+`}`)
	}))
	defer srv.Close()
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL)
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	cmd := newNovelReportsCompareCmd(&rootFlags{asJSON: true})
	cmd.SilenceErrors, cmd.SilenceUsage = true, true
	cmd.SetArgs([]string{"speed-to-lead", "--from", "2026-09-10", "--to", "2026-09-20"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	err := cmd.Execute()
	var view compareView
	if decodeErr := json.Unmarshal(out.Bytes(), &view); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if err != nil || view.Partial || len(view.Failures) != 0 || len(view.NonComparable) != 1 || view.NonComparable[0].Path != "byAssignee[name=Unassigned].channels.call" || view.NonComparable[0].Reason != "dimension member present in current window only" {
		t.Fatalf("err=%v view=%+v", err, view)
	}
}

func TestNovelReportsCompareMissingDimensionCollectionIsPartial(t *testing.T) {
	testenv.Isolate(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload := `{"shared":1}`
		if r.URL.Query().Get("from") == "2026-09-10T00:00:00Z" {
			payload = `{"shared":2,"byAssignee":[{"name":"Assigned","calls":3}]}`
		}
		fmt.Fprint(w, `{"data":`+payload+`}`)
	}))
	defer srv.Close()
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL)
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	cmd := newNovelReportsCompareCmd(&rootFlags{asJSON: true})
	cmd.SilenceErrors, cmd.SilenceUsage = true, true
	cmd.SetArgs([]string{"speed-to-lead", "--from", "2026-09-10", "--to", "2026-09-20"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	err := cmd.Execute()
	var view compareView
	if decodeErr := json.Unmarshal(out.Bytes(), &view); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	want := "byAssignee: dimension collection byAssignee is absent in the prior window and array in the current window"
	if ExitCode(err) != 5 || !view.Partial || len(view.NonComparable) != 1 || len(view.Failures) != 1 || view.Failures[0] != want || !strings.Contains(err.Error(), "dimension collection byAssignee is absent in the prior window") {
		t.Fatalf("err=%v view=%+v", err, view)
	}
}

func TestCompareDocumentsNestedDimensionUsesNearestCollection(t *testing.T) {
	prior := map[string]any{"shared": json.Number("1")}
	current := map[string]any{"shared": json.Number("2"), "groups": []any{
		map[string]any{"name": "Sales", "members": []any{map[string]any{"name": "Ada", "calls": json.Number("3")}}},
	}}
	_, nonComparable := compareDocumentsDetailed(prior, current)
	if len(nonComparable) != 1 || nonComparable[0].Reason != "dimension collection groups[name=Sales].members is absent in the prior window and array in the current window" {
		t.Fatalf("non_comparable=%+v", nonComparable)
	}
}

func TestCompareDocumentsNestedDimensionCollectionCompatibility(t *testing.T) {
	prior := map[string]any{"groups": []any{map[string]any{"name": "Sales", "members": []any{map[string]any{"name": "Ada", "calls": json.Number("1")}}}}}
	for _, tc := range []struct {
		name    string
		current any
		want    string
	}{
		{"nested collection missing", map[string]any{"groups": []any{map[string]any{"name": "Sales"}}}, "dimension collection groups[name=Sales].members is array in the prior window and absent in the current window"},
		{"nested members differ", map[string]any{"groups": []any{map[string]any{"name": "Sales", "members": []any{map[string]any{"name": "Grace", "calls": json.Number("2")}}}}}, "dimension member present in prior window only"},
		{"nested collection becomes scalar", map[string]any{"groups": []any{map[string]any{"name": "Sales", "members": "unavailable"}}}, "dimension collection groups[name=Sales].members is array in the prior window and scalar in the current window"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, nonComparable := compareDocumentsDetailed(prior, tc.current)
			found := false
			for _, item := range nonComparable {
				found = found || item.Reason == tc.want
			}
			if !found {
				t.Fatalf("non_comparable=%+v", nonComparable)
			}
		})
	}
}

func TestCompareDocumentsDimensionCollectionKinds(t *testing.T) {
	for _, tc := range []struct {
		name, want     string
		prior, current any
	}{
		{"empty array to object", "dimension collection dimensions is array in the prior window and object in the current window", map[string]any{"dimensions": []any{}}, map[string]any{"dimensions": map[string]any{}}},
		{"empty object to array", "dimension collection dimensions is object in the prior window and array in the current window", map[string]any{"dimensions": map[string]any{}}, map[string]any{"dimensions": []any{}}},
		{"array member churn", "dimension member present in prior window only", map[string]any{"dimensions": []any{map[string]any{"name": "Old", "count": json.Number("1")}}}, map[string]any{"dimensions": []any{map[string]any{"name": "New", "count": json.Number("2")}}}},
		{"plain object members are fixed", "numeric metric is present in prior window only", map[string]any{"dimensions": map[string]any{"old": json.Number("1")}}, map[string]any{"dimensions": map[string]any{"new": json.Number("2")}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, nonComparable := compareDocumentsDetailed(tc.prior, tc.current)
			found := false
			for _, item := range nonComparable {
				found = found || item.Reason == tc.want
			}
			if !found {
				t.Fatalf("non_comparable=%+v", nonComparable)
			}
		})
	}
}

func TestNovelReportsCompareFixedVsIndexedPathIsPartial(t *testing.T) {
	testenv.Isolate(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload := `{"shared":1,"dimensions":{"count":2}}`
		if r.URL.Query().Get("from") == "2026-09-10T00:00:00Z" {
			payload = `{"shared":2,"dimensions":[{"name":"All","count":3}]}`
		}
		fmt.Fprint(w, `{"data":`+payload+`}`)
	}))
	defer srv.Close()
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL)
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	cmd := newNovelReportsCompareCmd(&rootFlags{asJSON: true})
	cmd.SilenceErrors, cmd.SilenceUsage = true, true
	cmd.SetArgs([]string{"speed-to-lead", "--from", "2026-09-10", "--to", "2026-09-20"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	err := cmd.Execute()
	var view compareView
	if decodeErr := json.Unmarshal(out.Bytes(), &view); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	want := "dimensions: dimension collection dimensions is object in the prior window and array in the current window"
	if ExitCode(err) != 5 || !view.Partial || len(view.Failures) != 1 || view.Failures[0] != want {
		t.Fatalf("err=%v view=%+v", err, view)
	}
}

func TestNovelReportsCompareDimensionAndFixedOneSidedMetrics(t *testing.T) {
	testenv.Isolate(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload := `{"shared":1,"removed":4,"byAssignee":[{"name":"Prior only","calls":2}]}`
		if r.URL.Query().Get("from") == "2026-09-10T00:00:00Z" {
			payload = `{"shared":2,"byAssignee":[{"name":"Current only","calls":3}]}`
		}
		fmt.Fprint(w, `{"data":`+payload+`}`)
	}))
	defer srv.Close()
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL)
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	cmd := newNovelReportsCompareCmd(&rootFlags{asJSON: true})
	cmd.SilenceErrors, cmd.SilenceUsage = true, true
	cmd.SetArgs([]string{"speed-to-lead", "--from", "2026-09-10", "--to", "2026-09-20"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	err := cmd.Execute()
	var view compareView
	if decodeErr := json.Unmarshal(out.Bytes(), &view); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if ExitCode(err) != 5 || !view.Partial || len(view.NonComparable) != 3 || len(view.Failures) != 1 || !strings.Contains(view.Failures[0], "removed") {
		t.Fatalf("err=%v view=%+v", err, view)
	}
}

func runReportsCompareCommand(t *testing.T, prior, current string) (error, compareView) {
	t.Helper()
	testenv.Isolate(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload := prior
		if r.URL.Query().Get("from") == "2026-09-10T00:00:00Z" {
			payload = current
		}
		fmt.Fprint(w, `{"data":`+payload+`}`)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL)
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	cmd := newNovelReportsCompareCmd(&rootFlags{asJSON: true})
	cmd.SilenceErrors, cmd.SilenceUsage = true, true
	cmd.SetArgs([]string{"speed-to-lead", "--from", "2026-09-10", "--to", "2026-09-20"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	var view compareView
	if decodeErr := json.Unmarshal(out.Bytes(), &view); decodeErr != nil {
		t.Fatalf("decode %v: %s", decodeErr, out.String())
	}
	return err, view
}

func TestNovelReportsCompareNullDimensionCollectionKindsArePartial(t *testing.T) {
	for _, tc := range []struct {
		name, prior, current, failure string
	}{
		{"null to object", `{"shared":1,"dimensions":null}`, `{"shared":2,"dimensions":{}}`, "dimensions: dimension collection dimensions is null in the prior window and object in the current window"},
		{"object to null", `{"shared":1,"dimensions":{}}`, `{"shared":2,"dimensions":null}`, "dimensions: dimension collection dimensions is object in the prior window and null in the current window"},
		{"null to array", `{"shared":1,"dimensions":null}`, `{"shared":2,"dimensions":[]}`, "dimensions: dimension collection dimensions is null in the prior window and array in the current window"},
		{"array to null", `{"shared":1,"dimensions":[]}`, `{"shared":2,"dimensions":null}`, "dimensions: dimension collection dimensions is array in the prior window and null in the current window"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err, view := runReportsCompareCommand(t, tc.prior, tc.current)
			wantNC := []compareNonComparable{{Path: "dimensions", Reason: strings.TrimPrefix(tc.failure, "dimensions: ")}}
			if ExitCode(err) != 5 || !view.Partial || len(view.Failures) != 1 || view.Failures[0] != tc.failure || fmt.Sprint(view.NonComparable) != fmt.Sprint(wantNC) {
				t.Fatalf("err=%v view=%+v", err, view)
			}
		})
	}
}

func TestNovelReportsCompareNestedFixedMetricIsPartial(t *testing.T) {
	err, view := runReportsCompareCommand(t, `{"summary":{"shared":1,"removed":2}}`, `{"summary":{"shared":2}}`)
	wantNC := []compareNonComparable{{Path: "summary.removed", Reason: "numeric metric is present in prior window only"}}
	wantFailure := "summary.removed: numeric metric is present in prior window only"
	if ExitCode(err) != 5 || !view.Partial || len(view.Failures) != 1 || view.Failures[0] != wantFailure || fmt.Sprint(view.NonComparable) != fmt.Sprint(wantNC) {
		t.Fatalf("err=%v view=%+v", err, view)
	}
}

func TestNovelReportsCompareSameKindMemberChurnIsNonFatal(t *testing.T) {
	err, view := runReportsCompareCommand(t,
		`{"shared":1,"byAssignee":[{"userId":"old","calls":2}]}`,
		`{"shared":2,"byAssignee":[{"userId":"new","calls":3}]}`)
	want := []compareNonComparable{
		{Path: "byAssignee[userId=new].calls", Reason: "dimension member present in current window only"},
		{Path: "byAssignee[userId=old].calls", Reason: "dimension member present in prior window only"},
	}
	if err != nil || view.Partial || len(view.Failures) != 0 || fmt.Sprint(view.NonComparable) != fmt.Sprint(want) {
		t.Fatalf("err=%v view=%+v", err, view)
	}
}

func TestNovelReportsCompareRootArrayMemberChurnIsNonFatal(t *testing.T) {
	for _, tc := range []struct {
		name, prior, current string
		want                 []compareNonComparable
	}{
		{
			name:    "members change",
			prior:   `[{"name":"Old","count":1}]`,
			current: `[{"name":"New","count":2}]`,
			want: []compareNonComparable{
				{Path: "[name=New].count", Reason: "dimension member present in current window only"},
				{Path: "[name=Old].count", Reason: "dimension member present in prior window only"},
			},
		},
		{
			name:    "empty prior array",
			prior:   `[]`,
			current: `[{"name":"New","count":2}]`,
			want: []compareNonComparable{
				{Path: "[name=New].count", Reason: "dimension member present in current window only"},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err, view := runReportsCompareCommand(t, tc.prior, tc.current)
			if err != nil || view.Partial || len(view.Failures) != 0 || fmt.Sprint(view.NonComparable) != fmt.Sprint(tc.want) {
				t.Fatalf("err=%v view=%+v", err, view)
			}
		})
	}
}

func TestNovelReportsCompareRootCollectionKindChangesArePartial(t *testing.T) {
	for _, tc := range []struct {
		name, prior, current, reason string
	}{
		{
			name:    "array to object",
			prior:   `[{"name":"Old","count":1}]`,
			current: `{"count":2}`,
			reason:  "dimension collection $ is array in the prior window and object in the current window",
		},
		{
			name:    "scalar to array",
			prior:   `1`,
			current: `[{"name":"New","count":2}]`,
			reason:  "dimension collection $ is scalar in the prior window and array in the current window",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err, view := runReportsCompareCommand(t, tc.prior, tc.current)
			wantNC := []compareNonComparable{{Path: "$", Reason: tc.reason}}
			wantFailure := "$: " + tc.reason
			if ExitCode(err) != 5 || !view.Partial || fmt.Sprint(view.Failures) != fmt.Sprint([]string{wantFailure}) || fmt.Sprint(view.NonComparable) != fmt.Sprint(wantNC) {
				t.Fatalf("err=%v code=%d view=%+v", err, ExitCode(err), view)
			}
		})
	}
}

func TestNovelReportsCompareLiteralDollarPropertyDoesNotCollideWithRoot(t *testing.T) {
	for _, property := range []struct {
		name, prior, current, metric string
	}{
		{"array", `[{"name":"x","count":1}]`, `[{"name":"x","count":2}]`, "$[name=x].count"},
		{"object", `{"count":1}`, `{"count":2}`, "$.count"},
		{"scalar", `1`, `2`, "$"},
		{"null", `null`, `null`, ""},
	} {
		for _, root := range []struct {
			name, current string
		}{
			{"array root", `[{"name":"root","count":2}]`},
			{"object root", `{"shared":2,"$":` + property.current + `}`},
			{"scalar root", `2`},
		} {
			t.Run(property.name+"/"+root.name, func(t *testing.T) {
				prior := `{"shared":1,"$":` + property.prior + `}`
				err, view := runReportsCompareCommand(t, prior, root.current)
				if root.name != "object root" {
					wantReason := "dimension collection $ is object in the prior window and " + strings.TrimSuffix(root.name, " root") + " in the current window"
					wantNC := []compareNonComparable{{Path: "$", Reason: wantReason}}
					if ExitCode(err) != 5 || !view.Partial || fmt.Sprint(view.Failures) != fmt.Sprint([]string{"$: " + wantReason}) || fmt.Sprint(view.NonComparable) != fmt.Sprint(wantNC) {
						t.Fatalf("err=%v code=%d view=%+v", err, ExitCode(err), view)
					}
					return
				}
				if err != nil || view.Partial || len(view.NonComparable) != 0 {
					t.Fatalf("err=%v view=%+v", err, view)
				}
				if property.metric != "" {
					found := false
					for _, row := range view.Rows {
						found = found || row.Metric == property.metric
					}
					if !found {
						t.Fatalf("literal dollar metric %q missing from rows=%+v", property.metric, view.Rows)
					}
				}
			})
		}
	}
}

func TestNovelReportsCompareNestedSelectorUsesInnermostCollection(t *testing.T) {
	err, view := runReportsCompareCommand(t,
		`{"shared":1,"groups":[{"name":"Sales","members":[{"name":"Ada","calls":2}]}]}`,
		`{"shared":2,"groups":[{"name":"Sales"}]}`)
	wantReason := "dimension collection groups[name=Sales].members is array in the prior window and absent in the current window"
	wantNC := []compareNonComparable{{Path: "groups[name=Sales].members", Reason: wantReason}}
	if ExitCode(err) != 5 || !view.Partial || len(view.Failures) != 1 || view.Failures[0] != "groups[name=Sales].members: "+wantReason || fmt.Sprint(view.NonComparable) != fmt.Sprint(wantNC) {
		t.Fatalf("err=%v view=%+v", err, view)
	}
}

func TestNovelReportsCompareKeepsDottedPropertyDistinctFromNestedPath(t *testing.T) {
	err, view := runReportsCompareCommand(t,
		`{"a.b":1,"a":{"b":2}}`,
		`{"a.b":4,"a":{"b":2}}`)
	if err != nil || view.Partial || len(view.Rows) != 2 {
		t.Fatalf("err=%v view=%+v", err, view)
	}
	byMetric := map[string]compareRow{}
	for _, row := range view.Rows {
		byMetric[row.Metric] = row
	}
	if byMetric[`a\.b`].Delta != 3 || byMetric["a.b"].Delta != 0 {
		t.Fatalf("rows=%+v", view.Rows)
	}
}

func TestNovelReportsCompareLiteralSelectorPropertyStaysFixed(t *testing.T) {
	err, view := runReportsCompareCommand(t,
		`{"groups[name=Sales]":1,"groups":[{"name":"Sales","calls":2}]}`,
		`{"groups[name=Sales]":3,"groups":[{"name":"Sales","calls":5},{"name":"Support","calls":1}]}`)
	if err != nil || view.Partial || len(view.Rows) != 2 || len(view.NonComparable) != 1 {
		t.Fatalf("err=%v view=%+v", err, view)
	}
	byMetric := map[string]compareRow{}
	for _, row := range view.Rows {
		byMetric[row.Metric] = row
	}
	if byMetric[`groups\[name=Sales\]`].Delta != 2 || byMetric["groups[name=Sales].calls"].Delta != 3 {
		t.Fatalf("rows=%+v", view.Rows)
	}
	want := compareNonComparable{Path: "groups[name=Support].calls", Reason: "dimension member present in current window only"}
	if view.NonComparable[0] != want {
		t.Fatalf("non_comparable=%+v", view.NonComparable)
	}
}

func TestNovelReportsCompareEscapesClosingBracketInLiteralProperty(t *testing.T) {
	err, view := runReportsCompareCommand(t, `{"total]count":2}`, `{"total]count":7}`)
	if err != nil || view.Partial || len(view.Rows) != 1 || view.Rows[0].Metric != `total\]count` || view.Rows[0].Delta != 5 {
		t.Fatalf("err=%v view=%+v", err, view)
	}
}

func TestNovelReportsCompareEmptyPropertyPathsAreDistinct(t *testing.T) {
	t.Run("distinct sibling metrics survive", func(t *testing.T) {
		err, view := runReportsCompareCommand(t,
			`{"x":1,"":{"x":2}}`,
			`{"x":4,"":{"x":7}}`)
		if err != nil || view.Partial || len(view.Rows) != 2 {
			t.Fatalf("err=%v view=%+v", err, view)
		}
		byMetric := map[string]compareRow{}
		for _, row := range view.Rows {
			byMetric[row.Metric] = row
		}
		if len(byMetric) != 2 || byMetric["x"].Delta != 3 || byMetric[`[""].x`].Delta != 5 {
			t.Fatalf("rows=%+v", view.Rows)
		}
	})

	t.Run("root-level property differs from document root", func(t *testing.T) {
		err, propertyView := runReportsCompareCommand(t, `{"":2}`, `{"":5}`)
		if err != nil || propertyView.Partial || len(propertyView.Rows) != 1 || propertyView.Rows[0].Metric != `[""]` || propertyView.Rows[0].Delta != 3 {
			t.Fatalf("empty property: err=%v view=%+v", err, propertyView)
		}
		err, rootView := runReportsCompareCommand(t, `2`, `5`)
		if err != nil || rootView.Partial || len(rootView.Rows) != 1 || rootView.Rows[0].Metric != "" || rootView.Rows[0].Delta != 3 {
			t.Fatalf("document root: err=%v view=%+v", err, rootView)
		}
	})

	t.Run("nested under indexed member", func(t *testing.T) {
		err, view := runReportsCompareCommand(t,
			`{"groups":[{"name":"Sales","":2}]}`,
			`{"groups":[{"name":"Sales","":6}]}`)
		if err != nil || view.Partial || len(view.Rows) != 1 || view.Rows[0].Metric != `groups[name=Sales].[""]` || view.Rows[0].Delta != 4 {
			t.Fatalf("err=%v view=%+v", err, view)
		}
	})
}
