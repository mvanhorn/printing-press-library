package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/restaurant365-odata/internal/store"
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
	if plan.Chunks[0].Filter != "date ge 2026-05-01T00:00:00Z and date le 2026-05-31T23:59:59.999999999Z" {
		t.Fatalf("first filter=%q", plan.Chunks[0].Filter)
	}
	if plan.Chunks[1].From != "2026-06-01" || plan.Chunks[1].To != "2026-06-15" {
		t.Fatalf("second chunk=%+v", plan.Chunks[1])
	}
}

func TestR365ExportFiltersUsesEveryBackfillChunk(t *testing.T) {
	var filters []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/SalesDetail" {
			t.Fatalf("path=%q, want /SalesDetail", r.URL.Path)
		}
		filters = append(filters, r.URL.Query().Get("$filter"))
		fmt.Fprintf(w, `{"value":[{"salesDetailId":"row-%d"}]}`, len(filters))
	}))
	defer server.Close()

	t.Setenv("RESTAURANT365_ODATA_BASE_URL", server.URL)
	t.Setenv("RESTAURANT365_ODATA_USERNAME", "tenant\\user")
	t.Setenv("RESTAURANT365_ODATA_PASSWORD", "password")

	output := filepath.Join(t.TempDir(), "sales.jsonl")
	var flags rootFlags
	cmd := newRootCmd(&flags)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{
		"export",
		"--view", "SalesDetail",
		"--from", "2026-05-01",
		"--to", "2026-06-15",
		"--output", output,
		"--format", "jsonl",
		"--agent",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("export returned error: %v\nstdout=%s\nstderr=%s", err, stdout.String(), stderr.String())
	}

	want := []string{
		"date ge 2026-05-01T00:00:00Z and date le 2026-05-31T23:59:59.999999999Z",
		"date ge 2026-06-01T00:00:00Z and date le 2026-06-15T23:59:59.999999999Z",
	}
	if !reflect.DeepEqual(filters, want) {
		t.Fatalf("filters=%q, want %q", filters, want)
	}
	body, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(body), "\n"); got != 2 {
		t.Fatalf("exported lines=%d, want 2; body=%s", got, string(body))
	}
}

func TestR365ExportPaginatesWithinEachChunk(t *testing.T) {
	var skips []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/SalesDetail" {
			t.Fatalf("path=%q, want /SalesDetail", r.URL.Path)
		}
		skip := r.URL.Query().Get("$skip")
		skips = append(skips, skip)
		switch skip {
		case "":
			fmt.Fprint(w, `{"@odata.nextLink":"https://example.test/SalesDetail?$skip=2","value":[{"salesDetailId":"row-1"},{"salesDetailId":"row-2"}]}`)
		case "2":
			fmt.Fprint(w, `{"value":[{"salesDetailId":"row-3"}]}`)
		default:
			t.Fatalf("unexpected $skip=%q", skip)
		}
	}))
	defer server.Close()

	t.Setenv("RESTAURANT365_ODATA_BASE_URL", server.URL)
	t.Setenv("RESTAURANT365_ODATA_USERNAME", "tenant\\user")
	t.Setenv("RESTAURANT365_ODATA_PASSWORD", "password")

	output := filepath.Join(t.TempDir(), "sales.jsonl")
	var flags rootFlags
	cmd := newRootCmd(&flags)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{
		"export",
		"--view", "SalesDetail",
		"--filter", "date ge 2026-05-01T00:00:00Z",
		"--output", output,
		"--format", "jsonl",
		"--limit", "2",
		"--agent",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("export returned error: %v\nstdout=%s\nstderr=%s", err, stdout.String(), stderr.String())
	}
	if !reflect.DeepEqual(skips, []string{"", "2"}) {
		t.Fatalf("skips=%q, want initial page and $skip=2", skips)
	}
	body, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(body), "\n"); got != 3 {
		t.Fatalf("exported lines=%d, want 3; body=%s", got, string(body))
	}
}

func TestR365EndOfDayIncludesSubsecondCeiling(t *testing.T) {
	got := r365EndOfDay(time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC))
	if got != "2026-05-31T23:59:59.999999999Z" {
		t.Fatalf("r365EndOfDay=%q", got)
	}
}

func TestR365ExportDryRunDoesNotWriteOutput(t *testing.T) {
	output := filepath.Join(t.TempDir(), "sales.jsonl")
	var flags rootFlags
	cmd := newRootCmd(&flags)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{
		"export",
		"--view", "SalesDetail",
		"--from", "2026-05-01",
		"--to", "2026-05-31",
		"--output", output,
		"--dry-run",
		"--agent",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("export --dry-run returned error: %v\nstdout=%s\nstderr=%s", err, stdout.String(), stderr.String())
	}
	var summary map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &summary); err != nil {
		t.Fatalf("dry-run stdout is not JSON: %v\nstdout=%s", err, stdout.String())
	}
	if summary["dry_run"] != true {
		t.Fatalf("dry_run=%v, want true; stdout=%s", summary["dry_run"], stdout.String())
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("dry-run output file exists or stat failed: %v", err)
	}
}

func TestWriteR365ExportRemovesOutputOnEncodeError(t *testing.T) {
	output := filepath.Join(t.TempDir(), "sales.jsonl")
	rows := []map[string]any{{"bad": func() {}}}

	err := writeR365Export(output, "jsonl", rows)
	if err == nil {
		t.Fatalf("writeR365Export accepted a JSON-unsupported value")
	}
	if _, statErr := os.Stat(output); !os.IsNotExist(statErr) {
		t.Fatalf("output file exists after encode error or stat failed: %v", statErr)
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

func TestBackfillPlanRejectsInvalidWatermark(t *testing.T) {
	var flags rootFlags
	cmd := newRootCmd(&flags)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"backfill-plan", "--view", "Transaction", "--watermark", "0 or 1 eq 1", "--agent"})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("backfill-plan accepted invalid watermark\nstdout=%s\nstderr=%s", stdout.String(), stderr.String())
	}
	if !strings.Contains(err.Error(), "--watermark must be a non-negative integer") {
		t.Fatalf("error=%v, want watermark validation", err)
	}
}

func TestDeletedRecordsRejectsInvalidSinceRowVersion(t *testing.T) {
	var flags rootFlags
	cmd := newRootCmd(&flags)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"deleted-records", "--since-row-version", "0 or 1 eq 1", "--agent"})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("deleted-records accepted invalid since-row-version\nstdout=%s\nstderr=%s", stdout.String(), stderr.String())
	}
	if !strings.Contains(err.Error(), "--since-row-version must be a non-negative integer") {
		t.Fatalf("error=%v, want since-row-version validation", err)
	}
}

func TestR365CustomCommandHelpIncludesExamples(t *testing.T) {
	commands := [][]string{
		{"list-views"},
		{"describe-view"},
		{"sample"},
		{"backfill-plan"},
		{"deleted-records"},
		{"export"},
	}

	for _, command := range commands {
		var flags rootFlags
		cmd := newRootCmd(&flags)
		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs(append(command, "--help"))

		if err := cmd.Execute(); err != nil {
			t.Fatalf("%s --help returned error: %v\nstdout=%s\nstderr=%s", strings.Join(command, " "), err, stdout.String(), stderr.String())
		}
		if !strings.Contains(stdout.String(), "Examples:") {
			t.Fatalf("%s --help missing Examples section:\n%s", strings.Join(command, " "), stdout.String())
		}
	}
}

func TestDefaultSyncResourcesUseCanonicalGLAccountOnce(t *testing.T) {
	resources := defaultSyncResources()
	count := 0
	for _, resource := range resources {
		if resource == "glaccount" || resource == "gl-account" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("GLAccount resources=%d in %q, want exactly one canonical resource", count, resources)
	}
	if containsString(strings.Join(resources, ","), "gl-account") {
		t.Fatalf("default sync resources should not include gl-account alias: %q", resources)
	}
}

func TestKnownSyncResourceNamesUseCanonicalGLAccountOnce(t *testing.T) {
	names := knownSyncResourceNames()
	count := 0
	for _, name := range names {
		if name == "glaccount" || name == "gl-account" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("GLAccount names=%d in %q, want exactly one canonical resource", count, names)
	}
}

func TestCanonicalSyncResourceNamesDedupesAliases(t *testing.T) {
	got := canonicalSyncResourceNames([]string{"company", "gl-account", "glaccount", "location"})
	want := []string{"company", "glaccount", "location"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("canonicalSyncResourceNames=%q, want %q", got, want)
	}
}

func TestSyncResourceAddsDeterministicOrderBy(t *testing.T) {
	db, err := store.OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	client := &recordingSyncClient{}
	result := syncResource(context.Background(), client, db, "company", "", true, 1, false, &syncUserParams{}, io.Discard)
	if result.Err != nil {
		t.Fatalf("syncResource returned error: %v", result.Err)
	}
	if len(client.params) != 1 {
		t.Fatalf("requests=%d, want 1", len(client.params))
	}
	if got := client.params[0]["$orderby"]; got != "companyId asc" {
		t.Fatalf("$orderby=%q, want companyId asc", got)
	}
}

func TestPayrollSummaryPromotedCommandAddsExplicitDateRangeFilter(t *testing.T) {
	filter := runPayrollSummaryCommand(t, []string{
		"payroll-summary",
		"--from", "2026-05-01",
		"--to", "2026-05-31",
		"--agent",
	})
	want := "payrollStart le 2026-05-31T23:59:59.999999999Z and payrollEnd ge 2026-05-01T00:00:00Z"
	if filter != want {
		t.Fatalf("$filter=%q, want %q", filter, want)
	}
}

func TestPayrollSummaryPromotedCommandDefaultsToBoundedFilter(t *testing.T) {
	filter := runPayrollSummaryCommand(t, []string{"payroll-summary", "--agent"})
	if !strings.Contains(filter, "payrollStart le ") || !strings.Contains(filter, " and payrollEnd ge ") {
		t.Fatalf("$filter=%q, want bounded payroll overlap filter", filter)
	}
}

func runPayrollSummaryCommand(t *testing.T, args []string) string {
	t.Helper()
	var filter string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/PayrollSummary" {
			t.Fatalf("path=%q, want /PayrollSummary", r.URL.Path)
		}
		filter = r.URL.Query().Get("$filter")
		fmt.Fprint(w, `{"value":[{"payrollSummaryId":"payroll-1"}]}`)
	}))
	defer server.Close()

	t.Setenv("RESTAURANT365_ODATA_BASE_URL", server.URL)
	t.Setenv("RESTAURANT365_ODATA_USERNAME", "tenant\\user")
	t.Setenv("RESTAURANT365_ODATA_PASSWORD", "password")

	var flags rootFlags
	cmd := newRootCmd(&flags)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("payroll-summary returned error: %v\nstdout=%s\nstderr=%s", err, stdout.String(), stderr.String())
	}
	return filter
}

type recordingSyncClient struct {
	params []map[string]string
}

func (c *recordingSyncClient) Get(_ context.Context, _ string, params map[string]string) (json.RawMessage, error) {
	copied := make(map[string]string, len(params))
	for k, v := range params {
		copied[k] = v
	}
	c.params = append(c.params, copied)
	return json.RawMessage(`{"value":[{"companyId":"company-1","name":"Example"}]}`), nil
}

func (c *recordingSyncClient) RateLimit() float64 {
	return 0
}

func containsString(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}
