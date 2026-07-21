// Copyright 2026 Ryan Kelley and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
)

// reportingAPIFixture returns a realistic /reports/campaigns response payload.
// campaign c1: 2 days — impressions 1000+500, taps 80+40, installs 10+5, spend 40+20
func reportingAPIFixture() json.RawMessage {
	return json.RawMessage(`{
		"data": {
			"reportingDataResponse": {
				"row": [
					{
						"metadata": {"campaignId": "c1", "campaignName": "Brand"},
						"granularity": [
							{"date": "2026-07-01", "impressions": "1000", "taps": "80", "installs": "10", "localSpend": {"amount": "40.00"}},
							{"date": "2026-07-02", "impressions": "500",  "taps": "40", "installs": "5",  "localSpend": {"amount": "20.00"}}
						]
					},
					{
						"metadata": {"campaignId": "c2", "campaignName": "Retargeting"},
						"granularity": [
							{"date": "2026-07-01", "impressions": "0", "taps": "0", "installs": "0", "localSpend": {"amount": "0"}}
						]
					}
				]
			}
		}
	}`)
}

func TestParseReportingRows_TTR(t *testing.T) {
	rows, err := parseReportingRows(reportingAPIFixture())
	if err != nil {
		t.Fatal(err)
	}
	// Two campaigns × one or two granularity rows = 3 rows total.
	if len(rows) != 3 {
		t.Fatalf("want 3 rows, got %d", len(rows))
	}

	// Day 1 of Brand: impressions=1000, taps=80 → TTR=80/1000*100=8.0
	r0 := rows[0]
	if math.Abs(r0.TTR-8.0) > 0.001 {
		t.Errorf("want TTR=8.0 (taps/impressions*100), got %f", r0.TTR)
	}

	// Day 2 of Brand: impressions=500, taps=40 → TTR=40/500*100=8.0
	r1 := rows[1]
	if math.Abs(r1.TTR-8.0) > 0.001 {
		t.Errorf("want TTR=8.0, got %f", r1.TTR)
	}
}

func TestParseReportingRows_CVR(t *testing.T) {
	rows, err := parseReportingRows(reportingAPIFixture())
	if err != nil {
		t.Fatal(err)
	}

	// Day 1 of Brand: taps=80, installs=10 → CVR=10/80*100=12.5
	r0 := rows[0]
	if math.Abs(r0.CVR-12.5) > 0.001 {
		t.Errorf("want CVR=12.5 (installs/taps*100), got %f", r0.CVR)
	}

	// CVR must NOT equal TTR — confirms the two metrics are distinct.
	if math.Abs(r0.TTR-r0.CVR) < 0.001 {
		t.Errorf("TTR (%f) and CVR (%f) are equal — likely still computing the same formula", r0.TTR, r0.CVR)
	}
}

func TestParseReportingRows_ZeroImpressions(t *testing.T) {
	// Retargeting campaign has all-zero metrics; TTR must be 0, not NaN/div-by-zero.
	rows, err := parseReportingRows(reportingAPIFixture())
	if err != nil {
		t.Fatal(err)
	}
	zeroRow := rows[2] // third row = Retargeting day 1
	if zeroRow.TTR != 0 {
		t.Errorf("want TTR=0 when impressions=0, got %f", zeroRow.TTR)
	}
	if zeroRow.CVR != 0 {
		t.Errorf("want CVR=0 when taps=0, got %f", zeroRow.CVR)
	}
}

func TestParseReportingRows_CPA(t *testing.T) {
	rows, err := parseReportingRows(reportingAPIFixture())
	if err != nil {
		t.Fatal(err)
	}
	// Day 1: spend=40, installs=10 → CPA=4.0
	if math.Abs(rows[0].CPA-4.0) > 0.001 {
		t.Errorf("want CPA=4.0, got %f", rows[0].CPA)
	}
}

func TestParseReportingRows_CampaignFields(t *testing.T) {
	rows, err := parseReportingRows(reportingAPIFixture())
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].CampaignID != "c1" {
		t.Errorf("want campaignId=c1, got %q", rows[0].CampaignID)
	}
	if rows[0].CampaignName != "Brand" {
		t.Errorf("want campaignName=Brand, got %q", rows[0].CampaignName)
	}
	if rows[0].Date != "2026-07-01" {
		t.Errorf("want date=2026-07-01, got %q", rows[0].Date)
	}
}

// TestParseReportingRows_PaginationSentinel builds a 1000-row response and
// verifies that len(rows) >= 1000 is the correct sentinel for the sync-cache
// truncation warning ("reporting response hit the 1000-row limit").
func TestParseReportingRows_PaginationSentinel(t *testing.T) {
	var sb strings.Builder
	sb.WriteString(`{"data":{"reportingDataResponse":{"row":[`)
	for i := 0; i < 1000; i++ {
		if i > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb,
			`{"metadata":{"campaignId":"%d","campaignName":"C%d"},"granularity":[{"date":"2026-07-01","impressions":"100","taps":"5","installs":"1","localSpend":{"amount":"2.00"}}]}`,
			i, i)
	}
	sb.WriteString(`]}}}`)

	rows, err := parseReportingRows(json.RawMessage(sb.String()))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1000 {
		t.Fatalf("want 1000 rows, got %d", len(rows))
	}
	if !(len(rows) >= 1000) {
		t.Error("sentinel len >= 1000 should be true at capacity")
	}
}
