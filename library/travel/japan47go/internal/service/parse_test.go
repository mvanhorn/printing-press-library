// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package service

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

const tsumago = "2980022e-ef99-4115-95e5-be5227cdc74e"

func TestRequestRules(t *testing.T) {
	for _, x := range []struct {
		raw, status string
		n           int
	}{{"【予約期限】・10日前", "known", 1}, {"【予約期限】・2週間前 ・3週間前・1ヶ月前", "ambiguous", 3}, {"【予約期限】・予約不要・当日・1ヶ月前", "ambiguous", 1}, {"【受付時間】9:00-17:00", "unknown", 0}, {"【予約期限】・予約不要", "known", 0}, {"【予約期限】・10日前【定休日】土日", "known", 1}} {
		t.Run(x.raw, func(t *testing.T) {
			r := ParseRequest(x.raw)
			if r.Status != x.status || len(r.LeadTimes) != x.n {
				t.Fatalf("%+v", r)
			}
		})
	}
}
func TestPricesRemainQualified(t *testing.T) {
	for _, x := range []struct {
		raw, desc, status string
		n                 int
	}{{"実費負担（保険料、資料代、施設入場料、ガイド交通費・弁当代等）", "", "expenses", 0}, {"無料（費用が一切発生しない）", "", "free", 0}, {"", "【料金】 体験料金200円～", "paid", 1}, {"無料", "料金500円", "ambiguous", 1}, {"有料（ガイド料及び実費）", "", "paid", 0}, {"1人500円", "", "paid", 1}} {
		t.Run(x.raw+x.desc, func(t *testing.T) {
			p := ParsePrice(x.raw, x.desc)
			if p.Status != x.status || len(p.Amounts) != x.n || p.TotalJPY != nil {
				t.Fatalf("%+v", p)
			}
			if x.raw == "" && p.Amounts[0].Unit != nil {
				t.Fatal("invented fee unit")
			}
		})
	}
}
func TestDurationAndPartyEvidence(t *testing.T) {
	for _, x := range []struct {
		raw     string
		count   int
		minimum int
	}{{"1.お気軽コース30分\n2.本陣コース45分\n3.博物館60分\n4.歴史探訪90分\nガイド数：12\n平均年齢：77", 4, 0}, {"最低催行人数2名から\n徒歩約2時間半", 1, 2}, {"通常1-2時間", 1, 0}} {
		ds, ms := ParseDurations(x.raw)
		if len(ds) != x.count {
			t.Fatalf("durations%+v", ds)
		}
		if x.count == 4 && len(ms) != 4 {
			t.Fatal(ms)
		}
		p, _ := ParseMinimumParty(x.raw)
		if x.minimum > 0 && (p == nil || *p != x.minimum) {
			t.Fatal(p)
		}
	}
	p, _ := ParseMinimumParty("最低催行人数2名\n最少催行人数3名")
	if p != nil {
		t.Fatal("ambiguous minimum")
	}
}
func TestDetailPrivacyAndUnknowns(t *testing.T) {
	raw := `{"article":{"articleType":"location","slug":"` + tsumago + `","data":{"slug":"` + tsumago + `","name":"妻籠宿案内人の会","description":"モデルコース：30分\nガイド数：12\n平均年齢：77\n担当者：Synthetic Person\nmail: private@example.invalid","openingDateNotes":"【予約期限】10日前","closed":false,"categoryTag":{"id2":617},"price":{"price":"有料（ガイド料及び実費）"},"contact":[{"contactEmail":"private@example.invalid"}],"updateDate":"2026-04-30T11:58:29+09:00"}}}`
	s, e := ParseDetail([]byte(raw), tsumago, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	b, _ := json.Marshal(s)
	for _, secret := range []string{"平均年齢", "ガイド数", "private@example.invalid", "Synthetic Person", "contactEmail"} {
		if strings.Contains(string(b), secret) {
			t.Fatalf("private field %s", secret)
		}
	}
	if s.OpenNow != nil || s.Availability != "unknown" || s.SourceClosed == nil || *s.SourceClosed {
		t.Fatalf("inference%+v", s)
	}
	if len(s.Durations) != 1 || s.Request.LeadTimes[0].Value != 10 {
		t.Fatal(s)
	}
	if _, e = ParseDetail([]byte(raw), "0ad62a4e-2987-4e83-af63-7a6dd69e0d98", time.Now()); e == nil {
		t.Fatal("identity mismatch accepted")
	}
}
func TestStructuredPageAndListing(t *testing.T) {
	for _, x := range []struct {
		body  string
		valid bool
	}{{`<html><script id="__NEXT_DATA__">{"props":{"pageProps":{"x":1}}}</script></html>`, true}, {`<html>client error</html>`, false}, {`<script id="__NEXT_DATA__">notjson</script>`, false}} {
		_, e := PageProps([]byte(x.body))
		if (e == nil) != x.valid {
			t.Fatal(e)
		}
	}
	raw := `{"totalCount":107769,"items":[],"pageInfo":{"totalCnt":0,"perPage":20,"totalPageCnt":0,"pageNo":1}}`
	xs, c, e := ParseListing([]byte(raw), 1, time.Now())
	if e != nil || xs == nil || c.MatchingTotal != 0 {
		t.Fatalf("%+v %v", c, e)
	}
	if _, _, e = ParseListing([]byte(raw), 2, time.Now()); e == nil {
		t.Fatal("repeated source page accepted")
	}
}
func TestIDsBoundToSource(t *testing.T) {
	for _, x := range []struct {
		s     string
		valid bool
	}{{tsumago, true}, {BaseURL + "/ja/detail/" + tsumago, true}, {"https://elsewhere.invalid/ja/detail/" + tsumago, false}, {BaseURL + "/ja/detail/" + tsumago + "?x=1", false}, {"../../private", false}} {
		_, e := ID(x.s)
		if (e == nil) != x.valid {
			t.Fatal(x, e)
		}
	}
}

func TestTempleNameIsNotFeeEvidence(t *testing.T) {
	p := ParsePrice("実費負担", "博多千年門→円覚寺")
	if p.Original != "実費負担" || p.Status != "expenses" || len(p.Amounts) != 0 {
		t.Fatalf("irrelevant fee evidence %+v", p)
	}
}

func TestMixedFeeUnitsAndNoticeQuantitiesRemainUnresolved(t *testing.T) {
	for _, raw := range []string{"1人500円、1組1000円", "1人500円\n1組1000円"} {
		p := ParsePrice(raw, "")
		if len(p.Amounts) != 2 {
			t.Fatal(p)
		}
		for _, a := range p.Amounts {
			if a.Unit != nil {
				t.Fatalf("mixed-unit assumption %+v", p)
			}
		}
	}
	for _, raw := range []string{"【予約期限】1.5ヶ月前", "【予約期限】10～20日前", "【予約期限】2/3週間前"} {
		r := ParseRequest(raw)
		if r.Status != "ambiguous" || len(r.LeadTimes) != 0 {
			t.Fatalf("range/fraction parsed as one rule %+v", r)
		}
	}
}

// Source observations: 江差観光ガイド協会 and 龍岡城五稜郭保存会.
func TestPublishedMixedUnitDurationRanges(t *testing.T) {
	for _, x := range []struct {
		raw    string
		lo, hi int
	}{{"それぞれ30分～2時間程度", 30, 120}, {"30分～1時間", 30, 60}, {"徒歩1時間半～2時間", 90, 120}, {"30分～60分", 30, 60}, {"通常1-2時間", 60, 120}} {
		t.Run(x.raw, func(t *testing.T) {
			ds, exact := ParseDurations(x.raw)
			if len(ds) != 1 || ds[0].MinMinutes != x.lo || ds[0].MaxMinutes != x.hi || len(exact) != 0 {
				t.Fatalf("range became exact alternatives: %+v %v", ds, exact)
			}
		})
	}
	ds, exact := ParseDurations("1.お気軽コース30分\n2.本陣コース45分\n3.博物館60分\n4.歴史探訪90分")
	if len(ds) != 4 || len(exact) != 4 || exact[0] != 30 || exact[1] != 45 || exact[2] != 60 || exact[3] != 90 {
		t.Fatalf("Tsumago course durations changed: %+v %v", ds, exact)
	}
}
