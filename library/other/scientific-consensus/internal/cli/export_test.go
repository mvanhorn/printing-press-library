// Hand-authored tests for the Excel report engine (report.go + export_out.go).
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
)

// fakeReportGetter satisfies apiGetter with a canned payload or error.
type fakeReportGetter struct {
	payload json.RawMessage
	err     error
}

func (f *fakeReportGetter) Get(_ context.Context, _ string, _ map[string]string) (json.RawMessage, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.payload, nil
}

// twoWorksPayload is a minimal OpenAlex /works response: one supporting
// meta-analysis and one refuting RCT-shaped work.
const twoWorksPayload = `{
  "meta": {"count": 2},
  "results": [
    {
      "id": "https://openalex.org/W1",
      "doi": "https://doi.org/10.1000/xyz1",
      "title": "Vitamin D supplementation reduces the risk of respiratory infections: a meta-analysis",
      "publication_year": 2021,
      "cited_by_count": 500,
      "type": "article",
      "open_access": {"is_oa": true},
      "ids": {"pmid": "https://pubmed.ncbi.nlm.nih.gov/111"},
      "primary_topic": {"display_name": "Nutrition"},
      "primary_location": {"source": {"display_name": "BMJ"}},
      "authorships": [{"author": {"display_name": "A. Author"}, "institutions": [{"country_code": "GB"}]}]
    },
    {
      "id": "https://openalex.org/W2",
      "doi": "https://doi.org/10.1000/xyz2",
      "title": "Vitamin D shows no effect on respiratory infection rates: a randomized controlled trial",
      "publication_year": 2022,
      "cited_by_count": 120,
      "type": "article",
      "open_access": {"is_oa": false},
      "ids": {},
      "primary_topic": {"display_name": "Nutrition"},
      "primary_location": {"source": {"display_name": "JAMA"}},
      "authorships": [{"author": {"display_name": "B. Author"}, "institutions": []}]
    }
  ]
}`

const emptyWorksPayload = `{"meta": {"count": 0}, "results": []}`

func TestBuildReportData(t *testing.T) {
	tests := []struct {
		name         string
		payload      string
		err          error
		wantErr      bool
		wantRows     int
		wantTotal    int
		wantAnalyzed int
	}{
		{name: "two works", payload: twoWorksPayload, wantRows: 2, wantTotal: 2, wantAnalyzed: 2},
		{name: "empty result set", payload: emptyWorksPayload, wantRows: 0, wantTotal: 0, wantAnalyzed: 0},
		{name: "api error", err: errors.New("boom"), wantErr: true},
		{name: "malformed payload", payload: `{"meta":`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &fakeReportGetter{payload: json.RawMessage(tt.payload), err: tt.err}
			data, err := buildReportData(context.Background(), g, "vitamin d respiratory", "", "", 50)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(data.Rows) != tt.wantRows {
				t.Errorf("rows = %d, want %d", len(data.Rows), tt.wantRows)
			}
			if data.TotalMatches != tt.wantTotal {
				t.Errorf("total = %d, want %d", data.TotalMatches, tt.wantTotal)
			}
			if data.Analyzed != tt.wantAnalyzed {
				t.Errorf("analyzed = %d, want %d", data.Analyzed, tt.wantAnalyzed)
			}
			if data.Claim != "vitamin d respiratory" {
				t.Errorf("claim should default to query, got %q", data.Claim)
			}
			if sum := data.Supporting + data.Refuting + data.Mixed + data.Inconclusive; sum != tt.wantAnalyzed {
				t.Errorf("stance counts sum to %d, want %d", sum, tt.wantAnalyzed)
			}
		})
	}
}

func TestBuildReportData_RowContents(t *testing.T) {
	g := &fakeReportGetter{payload: json.RawMessage(twoWorksPayload)}
	data, err := buildReportData(context.Background(), g, "vitamin d", "", "vitamin d reduces respiratory infections", 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data.Rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(data.Rows))
	}
	r := data.Rows[0]
	if r.Title == "" || r.FirstAuthor != "A. Author" || r.Year != 2021 || r.DOI != "10.1000/xyz1" || r.PMID != "111" || r.Venue != "BMJ" || r.CitedBy != 500 || !r.OpenAccess {
		t.Errorf("first row not flattened correctly: %+v", r)
	}
	if r.Design == "" || r.Stance == "" {
		t.Errorf("classification missing: design=%q stance=%q", r.Design, r.Stance)
	}
	if data.ApexDesign == "" {
		t.Errorf("apex design should be set when rows exist")
	}
	if data.StanceMethod == "" {
		t.Errorf("stance method should never be empty")
	}
}

func TestWriteReportXLSX(t *testing.T) {
	g := &fakeReportGetter{payload: json.RawMessage(twoWorksPayload)}
	data, err := buildReportData(context.Background(), g, "vitamin d", "", "", 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tests := []struct {
		name    string
		path    func(t *testing.T) string
		data    *reportData
		wantErr bool
	}{
		{name: "valid export", path: func(t *testing.T) string { return filepath.Join(t.TempDir(), "out.xlsx") }, data: data},
		{name: "empty result set still writes a valid file", path: func(t *testing.T) string { return filepath.Join(t.TempDir(), "empty.xlsx") }, data: &reportData{Query: "q", Claim: "q"}},
		{name: "unwritable path", path: func(t *testing.T) string { return filepath.Join(t.TempDir(), "no-such-dir", "out.xlsx") }, data: data, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := tt.path(t)
			err := writeReportXLSX(p, tt.data)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// Reopen and verify structure: both sheets, header row, data rows.
			f, err := excelize.OpenFile(p)
			if err != nil {
				t.Fatalf("reopening workbook: %v", err)
			}
			defer f.Close()
			sheets := f.GetSheetList()
			if len(sheets) != 2 || sheets[0] != "Works" || sheets[1] != "Summary" {
				t.Errorf("sheets = %v, want [Works Summary]", sheets)
			}
			a1, err := f.GetCellValue("Works", "A1")
			if err != nil || a1 != "Title" {
				t.Errorf("Works!A1 = %q (err %v), want Title", a1, err)
			}
			rows, err := f.GetRows("Works")
			if err != nil {
				t.Fatalf("reading Works rows: %v", err)
			}
			if got, want := len(rows)-1, len(tt.data.Rows); got != want {
				t.Errorf("data rows = %d, want %d", got, want)
			}
			q, err := f.GetCellValue("Summary", "B1")
			if err != nil || q != tt.data.Query {
				t.Errorf("Summary!B1 = %q (err %v), want %q", q, err, tt.data.Query)
			}
		})
	}
}

func TestReportCmd_Validation(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "missing output flag", args: []string{"report", "some topic"}},
		{name: "non-xlsx output", args: []string{"report", "some topic", "--output", "out.csv"}},
		{name: "non-positive limit", args: []string{"report", "some topic", "--output", "out.xlsx", "--limit", "0"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var flags rootFlags
			root := newRootCmd(&flags)
			root.SetArgs(tt.args)
			root.SetOut(nil)
			root.SetErr(nil)
			err := root.Execute()
			if err == nil {
				t.Fatalf("expected usage error, got nil")
			}
			if ExitCode(err) != 2 {
				t.Errorf("exit code = %d, want 2 (usage error)", ExitCode(err))
			}
		})
	}
}
