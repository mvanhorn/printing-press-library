// Copyright 2026 Kieran Maynard and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored tests for the novel archive commands' pure helpers.

package cli

import (
	"encoding/json"
	"testing"
	"time"
)

func TestKakaoServiceSlug(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"카카오", "kakao"},
		{"다음", "daum"},
		{" 카카오 ", "kakao"},
		{"Kakao", "kakao"},
	}
	for _, tc := range cases {
		if got := kakaoServiceSlug(tc.in); got != tc.want {
			t.Errorf("kakaoServiceSlug(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestHalfLabel(t *testing.T) {
	if got := halfLabel(1); got != "1H" {
		t.Errorf("halfLabel(1) = %q, want 1H", got)
	}
	if got := halfLabel(2); got != "2H" {
		t.Errorf("halfLabel(2) = %q, want 2H", got)
	}
}

func TestKakaoSeriesStart(t *testing.T) {
	now := time.Now().Year()
	cases := []struct {
		since int
		want  int
	}{
		{0, kakaoArchiveStartYear},
		{2005, kakaoArchiveStartYear},
		{2020, 2020},
		{now + 5, now},
	}
	for _, tc := range cases {
		if got := kakaoSeriesStart(tc.since); got != tc.want {
			t.Errorf("kakaoSeriesStart(%d) = %d, want %d", tc.since, got, tc.want)
		}
	}
}

func jsonUnmarshalForTest(b []byte, v any) error { return json.Unmarshal(b, v) }

func TestFlexStringAbsorbsBothPayloadTypes(t *testing.T) {
	var row kakaoReportRow
	koSnippet := []byte(`{"serviceCorp":"카카오","numberOfRequests":"517","numberOfProcesses":"-1","numberOfAccounts":"0"}`)
	if err := jsonUnmarshalForTest(koSnippet, &row); err != nil {
		t.Fatalf("korean-style strings failed: %v", err)
	}
	if row.NumberOfRequests != "517" || row.NumberOfProcesses != "-1" {
		t.Errorf("korean-style parse got %q / %q", row.NumberOfRequests, row.NumberOfProcesses)
	}
	enSnippet := []byte(`{"serviceCorp":"다음","numberOfRequests":517,"numberOfProcesses":-1,"numberOfAccounts":0}`)
	if err := jsonUnmarshalForTest(enSnippet, &row); err != nil {
		t.Fatalf("english-style numbers failed: %v", err)
	}
	if row.NumberOfRequests != "517" || row.NumberOfAccounts != "0" {
		t.Errorf("english-style parse got %q / %q", row.NumberOfRequests, row.NumberOfAccounts)
	}
}
