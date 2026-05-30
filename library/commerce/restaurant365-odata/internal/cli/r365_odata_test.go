package cli

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

func TestCanonicalR365ViewAcceptsCommonSpellings(t *testing.T) {
	tests := map[string]string{
		"sales-detail":       "SalesDetail",
		"SalesDetail":        "SalesDetail",
		"sales_detail":       "SalesDetail",
		"transaction detail": "TransactionDetail",
		"gl-account":         "GLAccount",
		"glaccount":          "GLAccount",
	}

	for input, want := range tests {
		got, err := canonicalR365View(input)
		if err != nil {
			t.Fatalf("canonicalR365View(%q) returned error: %v", input, err)
		}
		if got.Name != want {
			t.Fatalf("canonicalR365View(%q)=%q, want %q", input, got.Name, want)
		}
	}
}

func TestExtractODataRowsUsesValueEnvelope(t *testing.T) {
	rows, err := extractODataRows(json.RawMessage(`{"@odata.context":"x","value":[{"locationId":"loc-1","name":"Example Location"}]}`))
	if err != nil {
		t.Fatalf("extractODataRows returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("len(rows)=%d, want 1", len(rows))
	}
	if rows[0]["locationId"] != "loc-1" {
		t.Fatalf("locationId=%v, want loc-1", rows[0]["locationId"])
	}
}

func TestSyncExtractPageItemsUsesODataValueEnvelope(t *testing.T) {
	items, nextCursor, hasMore := extractPageItems(json.RawMessage(`{"@odata.context":"x","value":[{"locationId":"loc-1"}]}`), "$skip")
	if len(items) != 1 {
		t.Fatalf("len(items)=%d, want 1", len(items))
	}
	if nextCursor != "" {
		t.Fatalf("nextCursor=%q, want empty", nextCursor)
	}
	if hasMore {
		t.Fatal("hasMore=true, want false")
	}
}

func TestR365FieldsForViewUsesCanonicalNameMatching(t *testing.T) {
	metadata := map[string][]r365Field{
		"GlAccount":   {{Name: "glAccountId", Type: "Edm.Guid"}},
		"PosEmployee": {{Name: "posEmployeeId", Type: "Edm.Guid"}},
	}

	if got := r365FieldsForView(metadata, "GLAccount"); len(got) != 1 || got[0].Name != "glAccountId" {
		t.Fatalf("GLAccount fields=%+v", got)
	}
	if got := r365FieldsForView(metadata, "POSEmployee"); len(got) != 1 || got[0].Name != "posEmployeeId" {
		t.Fatalf("POSEmployee fields=%+v", got)
	}
}

func TestRedactedSampleSummaryDoesNotExposeValues(t *testing.T) {
	summary := newR365SampleSummary("Location", 5, []map[string]any{
		{"locationId": "real-location-id", "name": "Real Location Name"},
	}, false)

	body, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) == "" {
		t.Fatal("expected JSON body")
	}
	for _, forbidden := range []string{"real-location-id", "Real Location Name"} {
		if containsString(string(body), forbidden) {
			t.Fatalf("sample summary leaked %q in %s", forbidden, string(body))
		}
	}
}

func TestBackfillPlanSplitsDateWindow(t *testing.T) {
	from := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)

	plan, err := planR365Backfill("SalesDetail", from, to, 31, "")
	if err != nil {
		t.Fatalf("planR365Backfill returned error: %v", err)
	}
	if len(plan.Chunks) != 2 {
		t.Fatalf("len(chunks)=%d, want 2", len(plan.Chunks))
	}
	if plan.Chunks[0].From != "2026-05-01" || plan.Chunks[0].To != "2026-05-31" {
		t.Fatalf("first chunk=%+v", plan.Chunks[0])
	}
	if plan.Chunks[0].Filter != "date ge 2026-05-01T00:00:00Z and date le 2026-05-31T23:59:59Z" {
		t.Fatalf("first filter=%q", plan.Chunks[0].Filter)
	}
	if plan.Chunks[1].From != "2026-06-01" || plan.Chunks[1].To != "2026-06-15" {
		t.Fatalf("second chunk=%+v", plan.Chunks[1])
	}
}

func TestDeletedRecordsDryRunDoesNotParseSyntheticBody(t *testing.T) {
	var flags rootFlags
	cmd := newRootCmd(&flags)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"deleted-records", "--dry-run", "--agent"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run returned error: %v\nstdout=%s\nstderr=%s", err, stdout.String(), stderr.String())
	}
	if !containsString(stdout.String(), `"values_redacted": true`) {
		t.Fatalf("stdout=%q, want redacted dry-run summary", stdout.String())
	}
}
