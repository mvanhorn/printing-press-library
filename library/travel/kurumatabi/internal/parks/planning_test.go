package parks

import (
	"math"
	"testing"
	"time"
)

func sample() Park {
	p := newPark("rvpark/1", observed)
	p.Name = "Evidence Park"
	p.SourceLevel = "detail"
	p.Vehicles = []string{"バンコン"}
	p.Dimensions = ParseDimensions("長さ7m 幅3m 高さ 無制限")
	p.Membership.Status = "not_required_stated"
	p.Membership.Evidence = "特になし"
	p.Facilities["electricity"] = Facility{"yes", "free", []string{"あり（無料）"}}
	return p
}
func TestFit(t *testing.T) {
	for _, x := range []struct {
		name string
		p    Park
		v    Vehicle
		want string
		err  bool
	}{{"known", sample(), Vehicle{6, 2.2, 3, "van", "nonmember"}, "compatible_with_published_limits", false}, {"oversize", sample(), Vehicle{8, 2.2, 3, "van", "nonmember"}, "exceeds_or_conflicts_with_published_limits", false}, {"unknown", newPark("rvpark/1", observed), Vehicle{6, 2.2, 3, "van", "nonmember"}, "needs_confirmation", false}, {"nan", sample(), Vehicle{math.NaN(), 2.2, 3, "van", "nonmember"}, "", true}} {
		r, e := Fit(x.p, x.v)
		if (e != nil) != x.err || !x.err && r.Decision != x.want {
			t.Errorf("%s %s %v", x.name, r.Decision, e)
		}
	}
	p := sample()
	p.Sections["駐車場"] = "複数区画利用 可 はみ出し可能"
	r, e := Fit(p, Vehicle{8, 2.2, 3, "van", "nonmember"})
	if e != nil || r.Decision != "needs_confirmation" {
		t.Error("flexible multi-pitch rule cannot become a rejection or guarantee")
	}
	p.Membership.Status = "required"
	p.Membership.Evidence = "会員証提示"
	r, _ = Fit(p, Vehicle{6, 2.2, 3, "van", "nonmember"})
	if r.Decision != "exceeds_or_conflicts_with_published_limits" {
		t.Error("nonmember restriction lost")
	}
}
func TestRequirementsAndMatch(t *testing.T) {
	for _, x := range []struct {
		raw string
		ok  bool
	}{{"electricity=free,pets", true}, {"moon_water", false}, {"water=0", false}, {"", false}} {
		_, e := ParseRequirements(x.raw)
		if (e == nil) != x.ok {
			t.Errorf("%s %v", x.raw, e)
		}
	}
	for _, x := range []struct {
		require string
		want    string
	}{{"electricity=free", "proven_match"}, {"water", "needs_confirmation"}, {"electricity=paid", "ruled_out"}} {
		req, e := ParseRequirements(x.require)
		if e != nil {
			t.Fatal(e)
		}
		r, e := Match(sample(), req, "nonmember")
		if e != nil || r.Decision != x.want {
			t.Errorf("%s %s", x.require, r.Decision)
		}
	}
	p := sample()
	p.SourceLevel = "search_card"
	req, _ := ParseRequirements("electricity")
	r, _ := Match(p, req, "nonmember")
	if r.Decision != "needs_confirmation" {
		t.Error("card-only candidate cannot be proven")
	}
}
func TestNear(t *testing.T) {
	a, b, c := sample(), sample(), sample()
	a.ID = "rvpark/1"
	a.Coordinates = &Coordinates{35, 135}
	b.ID = "rvpark/2"
	b.Coordinates = &Coordinates{36, 135}
	c.Coordinates = nil
	for _, x := range []struct {
		lat, lon, radius float64
		want             int
		err              bool
	}{{35, 135, 200, 2, false}, {0, 0, 1, 0, false}, {91, 0, 100, 0, true}, {math.Inf(1), 0, 100, 0, true}} {
		r, missing, e := Near([]Park{a, b, c}, x.lat, x.lon, x.radius, 10)
		if (e != nil) != x.err || !x.err && (len(r) != x.want || missing != 1) {
			t.Errorf("got %d %d %v", len(r), missing, e)
		}
		if !x.err && x.want == 2 && (r[0].ID != a.ID || r[0].DistanceKM != 0 || r[1].DistanceKM < 110 || r[1].DistanceKM > 112) {
			t.Error("distance/ranking incorrect")
		}
		if !x.err && r == nil {
			t.Error("zero-result array nil")
		}
	}
}
func TestAudit(t *testing.T) {
	p := sample()
	if rows := Audit(p, observed); rows == nil || len(rows) != 0 {
		t.Errorf("clean evidence must be []: %v", rows)
	}
	p.Warnings = []string{"legacy_icon_off_overrides_positive_alt: dump"}
	p.SourceUpdated = "2025-01-01"
	p.Sections["利用可能期間"] = "通年 冬期は休業"
	rows := Audit(p, time.Date(2026, 10, 3, 0, 0, 0, 0, JST))
	want := map[string]bool{"disabled_icon_positive_alt": false, "source_update_old": false, "opening_qualified": false}
	for _, r := range rows {
		if _, ok := want[r.Kind]; ok {
			want[r.Kind] = true
		}
		if r.URL != p.URL || r.Evidence == nil {
			t.Error("audit provenance missing")
		}
	}
	for k, v := range want {
		if !v {
			t.Error("missing", k)
		}
	}
}
func TestCompare(t *testing.T) {
	for _, ps := range [][]Park{{sample(), newPark("yypark/2", observed)}, {}} {
		rows := Compare(ps)
		if len(ps) == 0 && len(rows) != 0 {
			t.Error("empty input fabricated field rows")
		}
		if rows == nil {
			t.Error("nil matrix")
		}
		for _, r := range rows {
			if len(r.Values) != len(ps) || r.Values == nil {
				t.Error("matrix denominator includes phantom/missing record")
			}
		}
	}
}

func TestMultiPitchDoesNotExpandHeight(t *testing.T) {
	p := sample()
	h := 3.0
	p.Dimensions.HeightM = &h
	p.Dimensions.HeightUnrestricted = false
	p.Sections["駐車場"] = "複数区画利用 可"
	r, e := Fit(p, Vehicle{6, 2, 4, "van", "nonmember"})
	if e != nil || r.Decision != "exceeds_or_conflicts_with_published_limits" {
		t.Error("multiple pitches cannot increase explicit height bound")
	}
}

func TestCompareRetainsCanonicalSourceURLs(t *testing.T) {
	ps := []Park{{ID: "rvpark/1", URL: Origin + "/park/rvpark/1.html"}, {ID: "rvpark/2", URL: Origin + "/park/rvpark/2.html"}}
	for _, row := range Compare(ps) {
		if row.Field == "url" {
			if len(row.Values) != 2 || row.Values[0].Value != ps[0].URL || row.Values[1].Value != ps[1].URL {
				t.Fatal("canonical URL row incomplete")
			}
			return
		}
	}
	t.Fatal("comparison missing canonical URLs")
}

func TestRequiredPremiumCardTierFitAndMatch(t *testing.T) {
	for _, condition := range []string{"プレミアム会員証提示（必須）", "プレミアム会員証の提示が必要", "プレミアム会員限定"} {
		body := `<h3 class="commonIcoTitle"><span>検証パーク</span></h3><ul class="cartypedetail"><li>バンコン</li></ul><dl><dt>駐車場</dt><dd>長さ7m 幅3m 高さ3m</dd></dl><dl><dt>電源の有無</dt><dd>あり（無料）</dd></dl><dl><dt>利用条件</dt><dd>` + condition + `</dd></dl>`
		p, err := ParseDetail("rvpark/1", []byte(body), observed)
		if err != nil || p.Membership.Status != "required" {
			t.Fatalf("condition=%s status=%s err=%v", condition, p.Membership.Status, err)
		}
		reqs, _ := ParseRequirements("electricity")
		for _, tc := range []struct{ member, fit, match string }{
			{"premium", "compatible_with_published_limits", "proven_match"},
			{"standard", "exceeds_or_conflicts_with_published_limits", "ruled_out"},
			{"member", "needs_confirmation", "needs_confirmation"},
			{"nonmember", "exceeds_or_conflicts_with_published_limits", "ruled_out"},
		} {
			fit, err := Fit(p, Vehicle{6, 2.2, 3, "van", tc.member})
			if err != nil || fit.Decision != tc.fit {
				t.Errorf("%s member=%s fit=%s err=%v", condition, tc.member, fit.Decision, err)
			}
			match, err := Match(p, reqs, tc.member)
			if err != nil || match.Decision != tc.match {
				t.Errorf("%s member=%s match=%s err=%v", condition, tc.member, match.Decision, err)
			}
		}
	}
}

func TestMembershipWaiverDoesNotRejectNonmember(t *testing.T) {
	for _, tc := range []struct{ condition, status, fit, match string }{
		{"くるま旅クラブ会員証の提示は不要です", "not_required_stated", "compatible_with_published_limits", "proven_match"},
		{"プレミアム会員証は必要ありません", "unknown", "needs_confirmation", "needs_confirmation"},
		{"会員証提示は必須ではありません", "not_required_stated", "compatible_with_published_limits", "proven_match"},
		{"会員証の提示は不要ですが、プレミアム会員限定です", "unknown", "needs_confirmation", "needs_confirmation"},
	} {
		body := `<h3 class="commonIcoTitle"><span>検証パーク</span></h3><ul class="cartypedetail"><li>バンコン</li></ul><dl><dt>駐車場</dt><dd>長さ7m 幅3m 高さ3m</dd></dl><dl><dt>電源の有無</dt><dd>あり（無料）</dd></dl><dl><dt>利用条件</dt><dd>` + tc.condition + `</dd></dl>`
		p, err := ParseDetail("rvpark/1", []byte(body), observed)
		if err != nil || p.Membership.Status != tc.status {
			t.Fatalf("%s status=%s err=%v", tc.condition, p.Membership.Status, err)
		}
		fit, err := Fit(p, Vehicle{6, 2.2, 3, "van", "nonmember"})
		if err != nil || fit.Decision != tc.fit {
			t.Errorf("%s fit=%s err=%v", tc.condition, fit.Decision, err)
		}
		reqs, _ := ParseRequirements("electricity")
		match, err := Match(p, reqs, "nonmember")
		if err != nil || match.Decision != tc.match {
			t.Errorf("%s match=%s err=%v", tc.condition, match.Decision, err)
		}
	}
}
