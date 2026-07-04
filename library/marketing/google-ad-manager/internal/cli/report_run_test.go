// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestBuildReportDefinition(t *testing.T) {
	tests := []struct {
		name       string
		dimensions string
		metrics    string
		dateRange  string
		reportType string
		want       map[string]any
	}{
		{
			name:       "basic three-part definition",
			dimensions: "AD_UNIT_NAME,DATE",
			metrics:    "IMPRESSIONS,CLICKS",
			dateRange:  "LAST_7_DAYS",
			reportType: "HISTORICAL",
			want: map[string]any{
				"dimensions": []string{"AD_UNIT_NAME", "DATE"},
				"metrics":    []string{"IMPRESSIONS", "CLICKS"},
				"dateRange":  map[string]any{"relative": "LAST_7_DAYS"},
				"reportType": "HISTORICAL",
			},
		},
		{
			name:       "trims whitespace, uppercases, drops blanks from trailing comma",
			dimensions: " ad_unit_name , date , ",
			metrics:    "impressions",
			dateRange:  "YESTERDAY",
			reportType: "",
			want: map[string]any{
				"dimensions": []string{"AD_UNIT_NAME", "DATE"},
				"metrics":    []string{"IMPRESSIONS"},
				"dateRange":  map[string]any{"relative": "YESTERDAY"},
			},
		},
		{
			name:       "empty date range omits dateRange key",
			dimensions: "DATE",
			metrics:    "IMPRESSIONS",
			dateRange:  "",
			reportType: "HISTORICAL",
			want: map[string]any{
				"dimensions": []string{"DATE"},
				"metrics":    []string{"IMPRESSIONS"},
				"reportType": "HISTORICAL",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildReportDefinition(tt.dimensions, tt.metrics, tt.dateRange, tt.reportType)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("buildReportDefinition() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestReportResultFromOperation(t *testing.T) {
	tests := []struct {
		name string
		op   string
		want string
	}{
		{
			name: "extracts reportResult from completed operation response",
			op:   `{"done":true,"response":{"@type":"x","reportResult":"networks/123/reports/9/results/7"}}`,
			want: "networks/123/reports/9/results/7",
		},
		{
			name: "missing response yields empty",
			op:   `{"done":true}`,
			want: "",
		},
		{
			name: "malformed json yields empty",
			op:   `{not json`,
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := reportResultFromOperation(json.RawMessage(tt.op)); got != tt.want {
				t.Fatalf("reportResultFromOperation() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMetricValueAt(t *testing.T) {
	row := json.RawMessage(`{"metricValueGroups":[{"primaryValues":[{"intValue":"42"},{"doubleValue":3.5}]}]}`)
	tests := []struct {
		name   string
		idx    int
		want   float64
		wantOK bool
	}{
		{"int metric", 0, 42, true},
		{"double metric", 1, 3.5, true},
		{"index out of range", 2, 0, false},
		{"negative index", -1, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := metricValueAt(row, tt.idx)
			if ok != tt.wantOK || got != tt.want {
				t.Fatalf("metricValueAt(idx=%d) = (%v,%v), want (%v,%v)", tt.idx, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}
