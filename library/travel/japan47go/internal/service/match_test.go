// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package service

import (
	"testing"
)

func TestCompareNoticePartyAndFree(t *testing.T) {
	min := 2
	s := Service{Request: ParseRequest("【予約期限】10日前"), MinimumParty: &min, Price: ParsePrice("実費負担", "")}
	for _, x := range []struct {
		on, asOf       string
		party          int
		want, deadline string
	}{{"2026-11-01", "2026-10-22", 2, "supported_by_published_rules", "2026-10-22"}, {"2026-11-01", "2026-10-23", 2, "excluded", "2026-10-22"}, {"2026-11-01", "2026-10-21", 1, "excluded", "2026-10-22"}} {
		v := Compare(s, Constraints{On: x.on, AsOf: x.asOf, Party: x.party})
		if v.Compatibility != x.want || v.Notice.Deadline == nil || *v.Notice.Deadline != x.deadline || v.Availability != "unknown" {
			t.Fatalf("%+v", v)
		}
	}
	v := Compare(s, Constraints{RequireFree: true})
	if v.Free.State != "excluded" {
		t.Fatal(v)
	}
	s.Request = ParseRequest("【予約期限】当日・1ヶ月前")
	v = Compare(s, Constraints{On: "2026-11-01", AsOf: "2026-10-01"})
	if v.Notice.State != "unknown" {
		t.Fatal(v)
	}
}
func TestCalendarMonthsAndArchivedHiatus(t *testing.T) {
	for _, x := range []struct{ on, want string }{{"2026-03-31", "2026-02-28"}, {"2028-03-31", "2028-02-29"}, {"2026-01-31", "2025-12-31"}} {
		s := Service{Request: ParseRequest("【予約期限】1ヶ月前"), Schedule: Schedule{Original: "冬季活動なし2025/11/4-2026/4/28", Operation: "unknown"}}
		v := Compare(s, Constraints{On: x.on, AsOf: "2025-01-01"})
		if v.Notice.Deadline == nil || *v.Notice.Deadline != x.want || v.PublishedSpan.State != "unknown" {
			t.Fatal(v)
		}
	}
}
func TestConstraintValidation(t *testing.T) {
	for _, x := range []struct {
		c     Constraints
		valid bool
	}{{Constraints{On: "2026-11-01", AsOf: "2026-10-01", Party: 2}, true}, {Constraints{On: "2026-02-30"}, false}, {Constraints{On: "2026-11-01", AsOf: "2026-11-02"}, false}, {Constraints{Party: -1}, false}} {
		e := ValidateConstraints(x.c)
		if (e == nil) != x.valid {
			t.Fatal(x, e)
		}
	}
}
