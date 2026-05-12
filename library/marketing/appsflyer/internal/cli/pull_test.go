package cli

import (
	"encoding/json"
	"testing"
)

func TestChooseAggEndpoint(t *testing.T) {
	tests := []struct {
		breakdown string
		want      string
	}{
		{"media_source", "partners_report"},
		{"campaign", "partners_report"},
		{"date,media_source", "partners_by_date_report"},
		{"date,campaign", "partners_by_date_report"},
		{"country", "geo_report"},
		{"geo", "geo_report"},
		{"date,country", "geo_by_date_report"},
		{"date", "daily_report"},
		{"date,geo", "geo_by_date_report"},
		{"", "partners_report"},
	}
	for _, tt := range tests {
		t.Run(tt.breakdown, func(t *testing.T) {
			if got := chooseAggEndpoint(tt.breakdown); got != tt.want {
				t.Errorf("chooseAggEndpoint(%q) = %q, want %q", tt.breakdown, got, tt.want)
			}
		})
	}
}

func TestValidatePullDates(t *testing.T) {
	tests := []struct {
		name    string
		from    string
		to      string
		wantErr bool
	}{
		{"valid same day", "2026-05-10", "2026-05-10", false},
		{"valid range", "2026-05-04", "2026-05-10", false},
		{"to before from", "2026-05-10", "2026-05-04", true},
		{"bad from format", "2026/05/10", "2026-05-10", true},
		{"bad to format", "2026-05-04", "May 10", true},
		{"empty from", "", "2026-05-10", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePullDates(tt.from, tt.to)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePullDates(%q,%q) err=%v wantErr=%v", tt.from, tt.to, err, tt.wantErr)
			}
		})
	}
}

func TestParseAggCSV(t *testing.T) {
	csv := `Date,Media Source,Campaign,Country,Installs,Clicks,Impressions,Total Cost,Total Revenue,ROAS
2026-05-04,Facebook Ads,fb_install_us,US,150,1000,50000,500.00,750.00,1.5
2026-05-04,Google Ads,uac_install_us,US,200,1500,80000,800.00,1200.00,1.5
2026-05-05,Facebook Ads,fb_install_us,US,160,1100,52000,520.00,820.00,1.58
`
	rows, err := parseAggCSV(csv)
	if err != nil {
		t.Fatalf("parseAggCSV: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3", len(rows))
	}
	first := rows[0]
	if first.MediaSource != "Facebook Ads" || first.Installs != 150 || first.Cost != 500 || first.ROAS != 1.5 {
		t.Errorf("row[0] = %+v, unexpected values", first)
	}
}

func TestParseAggCSV_RestrictedRow(t *testing.T) {
	csv := `Date,Media Source,Installs,Total Cost,Total Revenue,ROAS
2026-05-04,restricted,,,,
2026-05-04,Facebook Ads,150,500,750,1.5
`
	rows, err := parseAggCSV(csv)
	if err != nil {
		t.Fatalf("parseAggCSV: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	// Restricted row should be present but with zeroed numeric fields
	if rows[0].MediaSource != "restricted" || rows[0].Installs != 0 || rows[0].Cost != 0 {
		t.Errorf("restricted row should have zero numerics, got %+v", rows[0])
	}
	if rows[1].Installs != 150 {
		t.Errorf("real row Installs wrong: %d", rows[1].Installs)
	}
}

func TestParsePullResponse_JSONArray(t *testing.T) {
	body := `[{"app_id":"id123","media_source":"Facebook Ads","installs":42,"cost":100,"revenue":150,"roas":1.5}]`
	rows, err := parsePullResponse(json.RawMessage(body))
	if err != nil {
		t.Fatalf("parsePullResponse: %v", err)
	}
	if len(rows) != 1 || rows[0].AppID != "id123" || rows[0].Installs != 42 {
		t.Errorf("unexpected JSON array parsing: %+v", rows)
	}
}

func TestFilterPullRows_BySource(t *testing.T) {
	rows := []pullRow{
		{MediaSource: "Facebook Ads", Installs: 100},
		{MediaSource: "Google Ads", Installs: 200},
		{MediaSource: "TikTok For Business", Installs: 80},
	}
	got := filterPullRows(rows, "facebook_int", nil, "")
	if len(got) != 1 || got[0].MediaSource != "Facebook Ads" {
		t.Errorf("filter by source: %+v", got)
	}
}

func TestFilterPullRows_ByGroup(t *testing.T) {
	rows := []pullRow{
		{MediaSource: "Facebook Ads", Installs: 100},
		{MediaSource: "TikTok For Business", Installs: 80},
		{MediaSource: "Google Ads", Installs: 200},
	}
	got := filterPullRows(rows, "", []string{"facebook_int", "tiktokglobal_int"}, "")
	if len(got) != 2 {
		t.Errorf("filter by group: got %d, want 2: %+v", len(got), got)
	}
}

func TestFilterPullRows_NoFilters(t *testing.T) {
	rows := []pullRow{{MediaSource: "X"}, {MediaSource: "Y"}}
	got := filterPullRows(rows, "", nil, "")
	if len(got) != 2 {
		t.Errorf("no filters should pass through, got %d", len(got))
	}
}

func TestSourceMatches(t *testing.T) {
	tests := []struct {
		observed  string
		canonical string
		want      bool
	}{
		{"facebook ads", "facebook_int", true},
		{"meta ads", "facebook_int", true},
		{"google ads (adwords)", "googleadwords_int", true},
		{"tiktok for business", "tiktokglobal_int", true},
		{"foo bar", "facebook_int", false},
	}
	for _, tt := range tests {
		t.Run(tt.observed, func(t *testing.T) {
			got := sourceMatches(tt.observed, tt.canonical)
			if got != tt.want {
				t.Errorf("sourceMatches(%q,%q) = %v, want %v", tt.observed, tt.canonical, got, tt.want)
			}
		})
	}
}
