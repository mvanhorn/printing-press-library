package manifest

import (
	"strings"
	"testing"
)

func TestParseCSV(t *testing.T) {
	csv := `Redaction,Release Date,Title,Type,Video Pairing,PDF Pairing,Description Blurb,DVIDS Video ID,Video Title,Agency,Incident Date,Incident Location,PDF | Image Link,Modal Image
TRUE,5/8/26,DOW-UAP-D10 Mission Report,PDF,,,"A military mission report",,,Department of War,5/6/22,Iraq,https://www.war.gov/medialink/ufo/release_1/test.pdf,https://www.war.gov/medialink/ufo/release_1/thumbnail/test.jpg
FALSE,5/8/26,FBI-Case-001,PDF,,,"An FBI report",,,FBI,11/7/57,Germany,https://www.war.gov/medialink/ufo/release_1/fbi.pdf,
,5/8/26,NASA-Apollo-12,VID,,PR-19,"Apollo 12 transcript",12345,PR-19,NASA,1969,,https://www.war.gov/medialink/ufo/release_1/apollo.mp4,`

	files, err := ParseCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}

	if len(files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(files))
	}

	tests := []struct {
		idx      int
		title    string
		fileType string
		agency   string
		redacted bool
		hasURL   bool
	}{
		{0, "DOW-UAP-D10 Mission Report", "PDF", "DoD", true, true},
		{1, "FBI-Case-001", "PDF", "FBI", false, true},
		{2, "NASA-Apollo-12", "VID", "NASA", false, true},
	}

	for _, tt := range tests {
		f := files[tt.idx]
		if f.Title != tt.title {
			t.Errorf("[%d] title: got %q, want %q", tt.idx, f.Title, tt.title)
		}
		if f.Type != tt.fileType {
			t.Errorf("[%d] type: got %q, want %q", tt.idx, f.Type, tt.fileType)
		}
		if f.Agency != tt.agency {
			t.Errorf("[%d] agency: got %q, want %q", tt.idx, f.Agency, tt.agency)
		}
		if f.Redacted != tt.redacted {
			t.Errorf("[%d] redacted: got %v, want %v", tt.idx, f.Redacted, tt.redacted)
		}
		if tt.hasURL && f.DownloadURL == "" {
			t.Errorf("[%d] expected download URL to be set", tt.idx)
		}
		if f.ID == "" {
			t.Errorf("[%d] expected non-empty ID", tt.idx)
		}
	}
}

func TestNormalizeAgency(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Department of War", "DoD"},
		{"FBI", "FBI"},
		{"NASA", "NASA"},
		{"Department of State", "State"},
		{"", "Unknown"},
	}
	for _, tt := range tests {
		got := normalizeAgency(tt.input)
		if got != tt.want {
			t.Errorf("normalizeAgency(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseIncidentDate(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"12/30/47", "1947-12-30"},
		{"6/15/48", "1948-06-15"},
		{"1969", "1969-01-01"},
		{"N/A", ""},
		{"", ""},
		{"Late 2025", "2025-01-01"},
	}
	for _, tt := range tests {
		got := parseIncidentDate(tt.input)
		if got != tt.want {
			t.Errorf("parseIncidentDate(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
