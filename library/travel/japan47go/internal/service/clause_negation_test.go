// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package service

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRequestConditionsStayWithTheirClause(t *testing.T) {
	for _, tc := range []struct{ evidence, status, notice string }{
		{"予約不要。ただし、雨天の場合は中止", "known", "supported"},
		{"当日予約可。ただし、雨天の場合は中止", "known", "supported"},
		{"予約不要。ただし、前日までに連絡がない場合は中止", "ambiguous", "unknown"},
		{"予約不要。ただし、前日までに連絡しない場合は中止", "ambiguous", "unknown"},
		{"予約不要。ただし、雨天の場合または前日までに連絡がない場合は中止", "ambiguous", "unknown"},
		{"予約不要。ただし、雨天の場合は中止。また、前日までに連絡がない場合は中止", "ambiguous", "unknown"},
		{"予約不要。ただし、都合がつかない場合は中止", "ambiguous", "unknown"},
		{"予約不要。ただし、雨天以外の理由でも中止となる場合があります", "ambiguous", "unknown"},
		{"予約不要。事前連絡不要", "known", "supported"},
		{"予約不要（空きがある場合のみ）", "ambiguous", "unknown"},
		{"予約不要。ただし、予約が必要な場合は事前に連絡", "ambiguous", "unknown"},
		{"予約不要。ただし、空きがある場合のみ", "ambiguous", "unknown"},
		{"予約不要ではありません", "unknown", "unknown"},
		{"当日予約不可", "unknown", "unknown"},
		{"予約不要", "known", "supported"},
		{"当日", "known", "supported"},
		{"10日前（当日予約不可）", "known", "excluded"},
	} {
		t.Run(tc.evidence, func(t *testing.T) {
			request := ParseRequest("【予約期限】" + tc.evidence)
			x := Compare(Service{Request: request, Availability: "unknown", Schedule: Schedule{Operation: "unknown"}}, Constraints{On: "2026-11-01", AsOf: "2026-11-01"})
			if request.Status != tc.status || x.Notice.State != tc.notice || request.Original != tc.evidence || x.Availability != "unknown" || x.Service.OpenNow != nil || x.Service.Schedule.Operation != "unknown" {
				t.Fatalf("reservation and operation facets crossed: %+v", x)
			}
			if tc.notice == "excluded" && (x.Notice.Deadline == nil || *x.Notice.Deadline != "2026-10-22") {
				t.Fatalf("ten-day deadline lost: %+v", x.Notice)
			}
		})
	}
}

func TestSeparateCostPolarityControlsZeroCost(t *testing.T) {
	for _, tc := range []struct{ raw, desc, status, free string }{
		{"ガイド料無料、交通費は別途不要", "", "free", "supported"},
		{"無料", "交通費は別途不要", "free", "supported"},
		{"ガイド料無料、別途交通費は不要", "", "free", "supported"},
		{"交通費は別途不要", "", "unknown", "unknown"},
		{"ガイド料無料、交通費別途", "", "expenses", "excluded"},
		{"ガイド料無料、交通費別途200円", "", "expenses", "excluded"},
		{"ガイド料無料、交通費は別途不要、資料代は別途200円", "", "expenses", "excluded"},
		{"ガイド料無料、交通費別途200円（予約不要）", "", "expenses", "excluded"},
		{"無料（費用が一切発生しない）", "", "free", "supported"},
		{"無料", "交通費無料、予約は別途受け付けます", "free", "supported"},
	} {
		t.Run(tc.raw+tc.desc, func(t *testing.T) {
			price := ParsePrice(tc.raw, tc.desc)
			x := Compare(Service{Price: price}, Constraints{RequireFree: true})
			if price.Status != tc.status || x.Free.State != tc.free || price.TotalJPY != nil || !strings.Contains(price.Original, tc.raw) || (tc.desc != "" && !strings.Contains(price.Original, tc.desc)) {
				t.Fatalf("waived and payable costs crossed: %+v", x)
			}
			expense := false
			for _, q := range price.Qualifiers {
				if q == "expenses" {
					expense = true
				}
			}
			if expense != (tc.status == "expenses") {
				t.Fatalf("unsupported expense qualifier: %+v", price)
			}
			if strings.Contains(tc.raw, "200円") && (len(price.Amounts) != 1 || price.Amounts[0].JPY != 200 || price.Amounts[0].Unit != nil) {
				t.Fatalf("stated amount/unit lost: %+v", price)
			}
		})
	}
}

func TestLegacyClauseFacetsCorrectWithoutCacheWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "observations.sqlite")
	updated := "2026-04-30T11:58:29+09:00"
	weather := Service{ID: tsumago, NameJA: "weather fixture", ObservedAt: "2026-10-04T12:00:00Z", SourceUpdatedAt: &updated, Availability: "unknown", Schedule: Schedule{Operation: "unknown"}, Request: Request{Status: "ambiguous", Options: []string{"予約不要"}, LeadTimes: []LeadTime{}, Original: "予約不要。ただし、雨天の場合は中止"}, Price: Price{Status: "expenses", Amounts: []Amount{}, Qualifiers: []string{"expenses", "guide_fee"}, Original: "ガイド料無料、交通費は別途不要"}}
	paid := weather
	paid.ID = "0ad62a4e-2987-4e83-af63-7a6dd69e0d98"
	paid.NameJA = "paid fixture"
	paid.Request = ParseRequest("【予約期限】予約不要")
	paid.Price = ParsePrice("ガイド料無料、交通費は別途不要、資料代は別途1組200円", "")
	for _, row := range []Service{weather, paid} {
		if e := Save(context.Background(), path, row); e != nil {
			t.Fatal(e)
		}
	}
	before, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	stat, e := os.Stat(path)
	if e != nil {
		t.Fatal(e)
	}
	for _, query := range []string{"", tsumago, "fixture"} {
		rows, n, e := Cached(context.Background(), path, query, 5)
		want := 2
		if query == tsumago {
			want = 1
		}
		if e != nil || n != 2 || len(rows) != want {
			t.Fatalf("bounded cache read: rows=%d scanned=%d err=%v", len(rows), n, e)
		}
		for _, row := range rows {
			original := weather
			if row.ID == paid.ID {
				original = paid
			}
			x := Compare(row, Constraints{On: "2026-11-01", AsOf: "2026-11-01", RequireFree: true})
			wantFree := "supported"
			if row.ID == paid.ID {
				wantFree = "excluded"
			}
			if x.Notice.State != "supported" || x.Free.State != wantFree || x.Availability != "unknown" || row.OpenNow != nil || row.Schedule.Operation != "unknown" {
				t.Fatalf("cached facets differ from fresh: %+v", x)
			}
			freshRequest := ParseRequest("【予約期限】" + original.Request.Original)
			freshPrice := ParsePrice(original.Price.Original, original.DescriptionEvidence)
			if !reflect.DeepEqual(row.Request, freshRequest) || row.Price.Status != freshPrice.Status || !reflect.DeepEqual(row.Price.Qualifiers, freshPrice.Qualifiers) || !reflect.DeepEqual(row.Price.Amounts, original.Price.Amounts) || row.Price.Original != original.Price.Original || row.ObservedAt != original.ObservedAt || row.SourceUpdatedAt == nil || *row.SourceUpdatedAt != updated || row.Price.TotalJPY != nil || row.Transport != "local" {
				t.Fatalf("cached evidence or clocks changed: %+v", row)
			}
			if (row.CacheWarning != nil) != (row.ID == weather.ID) {
				t.Fatalf("warning without actual delta or missing warning: %+v", row)
			}
		}
	}
	after, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	finalStat, e := os.Stat(path)
	if e != nil {
		t.Fatal(e)
	}
	if sha256.Sum256(before) != sha256.Sum256(after) || !stat.ModTime().Equal(finalStat.ModTime()) {
		t.Fatal("cached read wrote database")
	}
}

func TestLegacyPriorContactRemainsUnknownWithoutCacheWrites(t *testing.T) {
	var evidences []string
	for _, connector := range []string{"ただし", "また", "なお", "但し", "尚"} {
		for _, body := range []string{
			"前日までに連絡がない場合は中止",
			"前日までに連絡しない場合は中止",
			"雨天の場合または前日までに連絡がない場合は中止",
			"雨天の場合は中止。また、前日までに連絡がない場合は中止",
		} {
			evidences = append(evidences, "予約不要。"+connector+"、"+body)
		}
	}
	for _, evidence := range evidences {
		t.Run(evidence, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "observations.sqlite")
			updated := "2026-04-30T11:58:29+09:00"
			unit := "per_group"
			old := Service{ID: tsumago, NameJA: "contact fixture", ObservedAt: "2026-10-04T12:00:00Z", SourceUpdatedAt: &updated, Availability: "unknown", Schedule: Schedule{Operation: "unknown"}, Request: Request{Status: "known", Options: []string{"予約不要"}, LeadTimes: []LeadTime{}, Original: evidence}, Price: Price{Status: "paid", Amounts: []Amount{{JPY: 200, Qualifier: "stated", Unit: &unit, Original: "200円"}}, Qualifiers: []string{}, TotalJPY: nil, Original: "1組200円"}}
			if e := Save(context.Background(), path, old); e != nil {
				t.Fatal(e)
			}
			before, e := os.ReadFile(path)
			if e != nil {
				t.Fatal(e)
			}
			stat, e := os.Stat(path)
			if e != nil {
				t.Fatal(e)
			}
			for _, query := range []string{"", tsumago, "fixture"} {
				rows, n, e := Cached(context.Background(), path, query, 5)
				if e != nil || n != 1 || len(rows) != 1 {
					t.Fatal(rows, n, e)
				}
				row := rows[0]
				fresh := ParseRequest("【予約期限】" + evidence)
				x := Compare(row, Constraints{On: "2026-11-01", AsOf: "2026-11-01"})
				if x.Notice.State != "unknown" || x.Notice.Deadline != nil || x.Compatibility != "unknown" || x.Availability != "unknown" || row.OpenNow != nil || row.Schedule.Operation != "unknown" || len(row.Request.LeadTimes) != 0 || !reflect.DeepEqual(row.Request, fresh) {
					t.Fatalf("contact condition became unconditional or invented a deadline: %+v", x)
				}
				if row.Request.Original != evidence || row.ObservedAt != old.ObservedAt || row.SourceUpdatedAt == nil || *row.SourceUpdatedAt != updated || !reflect.DeepEqual(row.Price, old.Price) || row.CacheWarning == nil || row.Transport != "local" {
					t.Fatalf("read lost recorded evidence, money, clocks or warning: %+v", row)
				}
			}
			after, e := os.ReadFile(path)
			if e != nil {
				t.Fatal(e)
			}
			final, e := os.Stat(path)
			if e != nil {
				t.Fatal(e)
			}
			if sha256.Sum256(before) != sha256.Sum256(after) || !stat.ModTime().Equal(final.ModTime()) {
				t.Fatal("cached contact projection wrote database")
			}
			correct := old
			correct.Request = ParseRequest("【予約期限】" + evidence)
			if recheckSavedFacets(correct).CacheWarning != nil {
				t.Fatal("unchanged contact projection warned")
			}
		})
	}
}

func TestLeadingWeatherConnectorKeepsClauseBoundary(t *testing.T) {
	cases := []struct{ evidence, status, notice string }{}
	for _, connector := range []string{"ただし", "また", "なお", "但し", "尚"} {
		for _, prefix := range []string{connector + "、", " " + connector + ", ", connector} {
			cases = append(cases, struct{ evidence, status, notice string }{"予約不要。" + prefix + "雨天の場合は中止", "known", "supported"})
		}
		for _, body := range []string{
			"前日までに連絡がない場合は中止",
			"前日までに連絡しない場合は中止",
			"雨天の場合または前日までに連絡がない場合は中止",
			"雨天の場合は中止。また、前日までに連絡がない場合は中止",
			"都合がつかない場合は中止",
			"予約が必要な場合は事前に連絡",
		} {
			cases = append(cases, struct{ evidence, status, notice string }{"予約不要。" + connector + "、" + body, "ambiguous", "unknown"})
		}
	}
	for _, evidence := range []string{
		"予約不要。また、ただし、雨天の場合は中止",
		"予約不要。ただし、また、雨天の場合は中止",
		"予約不要。雨天の場合はまた、中止",
		"予約不要。そして、雨天の場合は中止",
	} {
		cases = append(cases, struct{ evidence, status, notice string }{evidence, "ambiguous", "unknown"})
	}
	for _, tc := range cases {
		t.Run(tc.evidence, func(t *testing.T) {
			request := ParseRequest("【予約期限】" + tc.evidence)
			x := Compare(Service{Request: request, Availability: "unknown", Schedule: Schedule{Operation: "unknown"}}, Constraints{On: "2026-11-01", AsOf: "2026-11-01"})
			if request.Status != tc.status || x.Notice.State != tc.notice || x.Notice.Deadline != nil || request.Original != tc.evidence || x.Availability != "unknown" || x.Service.OpenNow != nil || x.Service.Schedule.Operation != "unknown" {
				t.Fatalf("connector changed request or operation evidence: %+v", x)
			}
		})
	}
}

func TestLegacyLeadingWeatherConnectorProjectsWithoutWrites(t *testing.T) {
	for _, connector := range []string{"ただし", "また", "なお", "但し", "尚"} {
		t.Run(connector, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "observations.sqlite")
			updated := "2026-04-30T11:58:29+09:00"
			unit := "per_group"
			evidence := "予約不要。" + connector + "、雨天の場合は中止"
			old := Service{ID: tsumago, NameJA: "connector fixture", ObservedAt: "2026-10-04T12:00:00Z", SourceUpdatedAt: &updated, Availability: "unknown", Schedule: Schedule{Operation: "unknown"}, Request: Request{Status: "ambiguous", Options: []string{"予約不要"}, LeadTimes: []LeadTime{}, Original: evidence}, Price: Price{Status: "paid", Amounts: []Amount{{JPY: 200, Qualifier: "stated", Unit: &unit, Original: "200円"}}, Qualifiers: []string{}, TotalJPY: nil, Original: "1組200円"}}
			if e := Save(context.Background(), path, old); e != nil {
				t.Fatal(e)
			}
			before, e := os.ReadFile(path)
			if e != nil {
				t.Fatal(e)
			}
			stat, e := os.Stat(path)
			if e != nil {
				t.Fatal(e)
			}
			for _, query := range []string{"", tsumago, "fixture"} {
				rows, n, e := Cached(context.Background(), path, query, 5)
				if e != nil || n != 1 || len(rows) != 1 {
					t.Fatal(rows, n, e)
				}
				row := rows[0]
				fresh := ParseRequest("【予約期限】" + evidence)
				x := Compare(row, Constraints{On: "2026-11-01", AsOf: "2026-11-01"})
				if fresh.Status != "known" || x.Notice.State != "supported" || x.Notice.Deadline != nil || x.Availability != "unknown" || row.OpenNow != nil || row.Schedule.Operation != "unknown" || !reflect.DeepEqual(row.Request, fresh) {
					t.Fatalf("legacy connector differs from fresh weather: %+v", x)
				}
				if row.Request.Original != evidence || row.ObservedAt != old.ObservedAt || row.SourceUpdatedAt == nil || *row.SourceUpdatedAt != updated || !reflect.DeepEqual(row.Price, old.Price) || row.CacheWarning == nil || row.Transport != "local" {
					t.Fatalf("connector projection changed evidence, money, clocks or warning: %+v", row)
				}
			}
			after, e := os.ReadFile(path)
			if e != nil {
				t.Fatal(e)
			}
			final, e := os.Stat(path)
			if e != nil {
				t.Fatal(e)
			}
			if sha256.Sum256(before) != sha256.Sum256(after) || !stat.ModTime().Equal(final.ModTime()) {
				t.Fatal("connector projection wrote database")
			}
			correct := old
			correct.Request = ParseRequest("【予約期限】" + evidence)
			if recheckSavedFacets(correct).CacheWarning != nil {
				t.Fatal("unchanged connector projection warned")
			}
		})
	}
}
