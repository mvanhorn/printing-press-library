package agentic

import (
	"testing"
	"time"
)

func TestNormalizeTarget(t *testing.T) {
	cases := []struct {
		name, input, want string
		ok                bool
	}{
		{"domain", "example.com", "https://example.com", true},
		{"https", "https://example.com/path", "https://example.com/path", true},
		{"http", "http://example.com", "http://example.com", true},
		{"empty", "", "", false},
		{"scheme", "ftp://example.com", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeTarget(tc.input)
			if tc.ok {
				if err != nil || got != tc.want {
					t.Fatalf("NormalizeTarget(%q)=(%q,%v), want %q", tc.input, got, err, tc.want)
				}
			} else if err == nil {
				t.Fatalf("NormalizeTarget(%q) succeeded", tc.input)
			}
		})
	}
}

func TestParseReport(t *testing.T) {
	raw := []byte(`{"target":"https://example.com","display_target":"example.com","score":52,"score_label":"Needs work","scanned_at":"2026-08-24T00:00:00Z","report_url":"https://is-agentic.com/scan/example.com","eligible_checks":3,"score_breakdown":{"essential":{"earned":2,"available":3,"passing":1,"total":2}},"issues":[{"id":"content-no-js","name":"Content without JavaScript","tier":"essential","result":"partial","details":"shell","recommendation":"SSR"}]}`)
	report, err := ParseReport(raw, time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if report.Parsed.Score == nil || *report.Parsed.Score != 52 || len(report.Issues) != 1 || report.Issues[0].Id != "content-no-js" {
		t.Fatalf("unexpected parsed report: %+v", report)
	}
	if report.ScoreBreakdown["essential"].Passing != 1 {
		t.Fatalf("missing score breakdown: %+v", report.ScoreBreakdown)
	}
}
