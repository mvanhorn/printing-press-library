// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package service

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProhibitedAndConditionalRequestIsNotSupported(t *testing.T) {
	for _, evidence := range []string{"当日予約不可", "当日受付不可", "当日予約はできません", "予約不要ではありません", "当日予約可（空きがある場合のみ）", "予約不要（当日予約不可）"} {
		t.Run(evidence, func(t *testing.T) {
			request := ParseRequest("【予約期限】" + evidence)
			x := Compare(Service{Request: request}, Constraints{On: "2026-11-01", AsOf: "2026-11-01"})
			if x.Notice.State != "unknown" || x.Compatibility != "unknown" || !strings.Contains(x.Service.Request.Original, evidence) {
				t.Fatalf("prohibited/conditional evidence became support: %+v", x)
			}
		})
	}
	for _, evidence := range []string{"当日", "予約不要"} {
		x := Compare(Service{Request: ParseRequest("【予約期限】" + evidence)}, Constraints{On: "2026-11-01", AsOf: "2026-11-01"})
		if x.Notice.State != "supported" {
			t.Fatalf("explicit positive lost: %+v", x)
		}
	}
	x := Compare(Service{Request: ParseRequest("【予約期限】10日前（当日予約不可）")}, Constraints{On: "2026-11-01", AsOf: "2026-11-01"})
	if x.Notice.State != "excluded" || x.Notice.Deadline == nil || *x.Notice.Deadline != "2026-10-22" {
		t.Fatalf("explicit advance rule lost: %+v", x)
	}
}

func TestSeparateExpenseDoesNotMeetZeroCost(t *testing.T) {
	for _, tc := range []struct{ raw, desc string }{{"ガイド料無料、交通費別途", ""}, {"無料", "ガイド料無料\n交通費は別途必要"}, {"無料", "別途資料代が必要"}} {
		x := Compare(Service{Price: ParsePrice(tc.raw, tc.desc)}, Constraints{RequireFree: true})
		if x.Free.State != "excluded" || x.Service.Price.Status != "expenses" || !strings.Contains(strings.Join(x.Service.Price.Qualifiers, ","), "expenses") || x.Service.Price.TotalJPY != nil {
			t.Fatalf("extra cost became free: %+v", x)
		}
	}
	x := Compare(Service{Price: ParsePrice("無料（費用が一切発生しない）", "")}, Constraints{RequireFree: true})
	if x.Free.State != "supported" {
		t.Fatalf("explicit zero-cost lost: %+v", x)
	}
	x = Compare(Service{Price: ParsePrice("無料", "交通費無料、予約は別途受け付けます")}, Constraints{RequireFree: true})
	if x.Free.State != "supported" {
		t.Fatalf("unrelated separate reservation became expense: %+v", x)
	}
}

func TestLegacyCacheFacetsAreRecheckedWithoutWriteOrClockChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "observations.sqlite")
	updated := "2026-04-30T11:58:29+09:00"
	old := Service{ID: tsumago, NameJA: "fixture", ObservedAt: "2026-10-04T12:00:00Z", SourceUpdatedAt: &updated, Request: Request{Status: "known", Options: []string{"当日"}, LeadTimes: []LeadTime{}, Original: "当日予約不可"}, Price: Price{Status: "free", Amounts: []Amount{}, Qualifiers: []string{"guide_fee"}, Original: "無料"}, DescriptionEvidence: "ガイド料無料\n交通費別途"}
	if e := Save(context.Background(), path, old); e != nil {
		t.Fatal(e)
	}
	before, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	for _, query := range []string{"", tsumago, "fixture"} {
		rows, n, e := Cached(context.Background(), path, query, 5)
		if e != nil || n != 1 || len(rows) != 1 {
			t.Fatal(rows, n, e)
		}
		x := Compare(rows[0], Constraints{On: "2026-11-01", AsOf: "2026-11-01", RequireFree: true})
		if x.Notice.State != "unknown" || x.Free.State != "excluded" || x.Service.CacheWarning == nil {
			t.Fatalf("old facets survived cached inspect/compare/saved: %+v", x)
		}
		if x.Service.ObservedAt != old.ObservedAt || x.Service.SourceUpdatedAt == nil || *x.Service.SourceUpdatedAt != updated || x.Service.Transport != "local" || x.Service.Price.TotalJPY != nil {
			t.Fatalf("clock or provenance changed: %+v", x)
		}
	}
	after, e := os.ReadFile(path)
	if e != nil || sha256.Sum256(before) != sha256.Sum256(after) {
		t.Fatal("read changed stored observation", e)
	}
}

func TestConditionalNoReservationLegacyMatchesFresh(t *testing.T) {
	old := Service{Request: Request{Status: "known", Options: []string{"予約不要"}, LeadTimes: []LeadTime{}, Original: "予約不要（空きがある場合のみ）"}}
	fresh := ParseRequest("【予約期限】" + old.Request.Original)
	fixed := recheckSavedFacets(old)
	if fresh.Status != "ambiguous" || fixed.Request.Status != fresh.Status || Compare(fixed, Constraints{On: "2026-11-01", AsOf: "2026-11-01"}).Notice.State != "unknown" || fixed.CacheWarning == nil {
		t.Fatalf("legacy differs from fresh: %+v", fixed)
	}
	already := old
	already.Request = fresh
	if recheckSavedFacets(already).CacheWarning != nil {
		t.Fatal("unchanged derived fact warned")
	}
}
func TestExplicitSeparateAmountExcludesZeroCost(t *testing.T) {
	price := ParsePrice("ガイド料無料、交通費別途200円", "")
	x := Compare(Service{Price: price}, Constraints{RequireFree: true})
	if x.Free.State != "excluded" || len(price.Amounts) != 1 || price.Amounts[0].JPY != 200 || price.Amounts[0].Unit != nil || price.TotalJPY != nil || !strings.Contains(price.Original, "交通費別途200円") {
		t.Fatalf("explicit extra cost lost: %+v", x)
	}
	old := Service{Price: price}
	old.Price.Status = "ambiguous"
	fixed := recheckSavedFacets(old)
	if Compare(fixed, Constraints{RequireFree: true}).Free.State != "excluded" || fixed.Price.Amounts[0].JPY != 200 || fixed.Price.Amounts[0].Unit != nil || fixed.Price.TotalJPY != nil {
		t.Fatalf("cached extra amount lost: %+v", fixed)
	}
}
