package omakase

import (
	"strings"
	"testing"
)

func TestPriceSemantics(t *testing.T) {
	cases := []struct {
		raw     string
		amount  int
		minimum bool
		tax     bool
		basis   string
	}{{"JPY52,800 (Tax included)～ /guest(s)", 52800, true, true, "per_guest"}, {"33,000円（税込） / 人", 33000, false, true, "per_guest"}, {"JPY12,000 (Tax excluded) /guest(s)", 12000, false, false, "per_guest"}, {"JPY5,000 /bottle", 5000, false, false, "unknown"}}
	for _, c := range cases {
		t.Run(c.raw, func(t *testing.T) {
			p := ParsePrice(c.raw)
			if p.Amount == nil || *p.Amount != c.amount || p.Minimum != c.minimum || p.Basis != c.basis {
				t.Fatalf("%+v", p)
			}
			if strings.Contains(c.raw, "Tax") || strings.Contains(c.raw, "税込") {
				if p.TaxIncluded == nil || *p.TaxIncluded != c.tax {
					t.Fatal(p)
				}
			}
		})
	}
	if p := ParsePrice("market price"); p.Amount != nil || p.TaxIncluded != nil {
		t.Fatal("invented missing values")
	}
}
func TestReleaseStates(t *testing.T) {
	for _, c := range []struct{ next, state, at string }{{"TBD", "undetermined", ""}, {"未定", "undetermined", ""}, {"Irregular", "irregular", ""}, {"Oct 1, 2026, 12:00 JST(UTC+9)", "scheduled", "2026-10-01T12:00:00+09:00"}, {"2026年10月1日 12:00", "scheduled", "2026-10-01T12:00:00+09:00"}, {"First day of each month", "schedule_text", ""}, {"", "unknown", ""}} {
		r := ParseRelease(nil, strptr(c.next), nil)
		if r.State != c.state || deref(r.NextRoundAt) != c.at {
			t.Fatalf("%s: %+v", c.next, r)
		}
	}
}
func TestActionStateDoesNotInferSeats(t *testing.T) {
	for _, c := range []struct{ raw, method, access string }{{"Log in to check availability", "unknown", "login_required"}, {"ログインして空き枠を確認", "unknown", "login_required"}, {"Join waitlist", "waitlist", "unknown"}, {"Request reservation", "request", "unknown"}, {"Enter lottery", "lottery", "unknown"}, {"Reserve now", "unknown", "unknown"}, {"Sold out? Please request information", "unknown", "unknown"}} {
		m, a := ActionState(c.raw)
		if m != c.method || a != c.access {
			t.Fatalf("%q: %s %s", c.raw, m, a)
		}
	}
}

const detailHTML = `<h1 class="p-r_title">Test Sushi</h1><div class="p-r_main_subInfo"><span>Tokyo, Tokyo</span><span>Sushi</span></div><div class="p-r_text"><h2>Reservation rules</h2><p>Please note request is not guaranteed.</p></div><div class="p-r_course_upper"><h3>Lunch</h3><div class="p-r_course_price">JPY20,000 (Tax included)～ /guest(s)</div></div><div class="p-r_course_notice"><p>Additional 10% service charge.<br>Price may vary.</p></div><div class="p-r-list"><table><tr><td>Seats</td><td>8 seats</td></tr></table></div><div class="p-r_reserve"><div class="p-r_reserve_action_reserve">Log in to check availability</div><span class="rsv_notice">Reservation fee (390JPY /seat)</span><table><tr><td>Current reservation period</td><td>Irregular</td></tr><tr><td>Next round opens</td><td>TBD</td></tr></table><table><thead><tr><td>Cancellation policy</td></tr></thead><tbody><tr><td>3 days prior</td><td>50%</td></tr><tr><td>on the day</td><td>100%</td></tr></tbody></table><p class="p-r_reserve_cpDesc">Advance payments have a different policy.</p></div>`

func TestDetailParsingConsequentialFields(t *testing.T) {
	for _, loc := range []string{"en", "ja"} {
		d, e := ParseDetail([]byte(detailHTML), "hc778124", loc)
		if e != nil {
			t.Fatal(e)
		}
		if len(d.Courses) != 1 || *d.Courses[0].Price.Amount != 20000 || !d.Courses[0].Price.Variable || *d.ServiceCharge.Percent != 10 || *d.ReservationFee.Amount != 390 || len(d.Cancellation) != 2 || *d.Cancellation[1].Percent != 100 || d.SeatState != "unknown" || d.BookingMethod != "unknown" || d.Access != "login_required" || d.Release.State != "undetermined" {
			t.Fatalf("%+v", d)
		}
	}
	for _, body := range []string{"<title>Just a moment...</title>", "<h1>Log in</h1>"} {
		if _, e := ParseDetail([]byte(body), "hc778124", "en"); e == nil {
			t.Fatal("accepted challenge/login")
		}
	}
}

const pageHTML = `<form><select name="area"><option value="kanto">Tokyo Area</option></select><select name="cuisine"><option value="sushi">Sushi</option></select></form><a href="/en/r/hc778124"><div class="c-restaurant_item_detail"><h3>Test &amp; Sushi</h3><span>Sushi / Tokyo</span></div></a><a rel="next" href="/en/r/page/2">Next</a><div>Search result 1-32 / 721 店舗</div>`

func TestCatalogueParsing(t *testing.T) {
	for _, loc := range []string{"en", "ja"} {
		p, e := ParsePage([]byte(pageHTML), loc)
		if e != nil || len(p.Results) != 1 || p.Results[0].Name != "Test & Sushi" || p.Total == nil || *p.Total != 721 || p.Next == nil {
			t.Fatalf("%+v %v", p, e)
		}
		if p.Results[0].SeatState != "unknown" {
			t.Fatal("cards are not seats")
		}
	}
	if _, e := ParsePage([]byte("<html>unrelated</html>"), "en"); e == nil {
		t.Fatal("accepted wrong source shape")
	}
}
func TestNameRelevanceAndIDs(t *testing.T) {
	for _, c := range []struct {
		name, q string
		want    bool
	}{{"Nihonbashi Kakigaracho Sugita", "sugita", true}, {"SUGATA", "Sugita", false}, {"東麻布 天本", "東麻布天本", true}, {"acá", "acá", true}} {
		if NameMatches(c.name, c.q) != c.want {
			t.Fatal(c)
		}
	}
	for _, id := range []string{"../users/sign_in", "hc778124?x=y", "http://other.test", "HC778124"} {
		if ValidateID(id) == nil {
			t.Fatal(id)
		}
	}
	for _, l := range []string{"en", "ja"} {
		if _, e := LocalePath(l); e != nil {
			t.Fatal(e)
		}
	}
	if _, e := LocalePath("fr"); e == nil {
		t.Fatal("unknown locale")
	}
}
func TestMembershipMissingEligibility(t *testing.T) {
	m, e := ParseMembership([]byte(`<h1>Premium</h1><div>Green 4,980 Yen(tax inc.)/mo Green 49,980 Yen(tax inc.)/yr</div><h3>Search for available seats</h3><p>Premium feature</p>`))
	if e != nil || m.MonthlyJPY == nil || *m.MonthlyJPY != 4980 || m.AnnualJPY == nil || *m.AnnualJPY != 49980 || m.AccountEligibility != nil || m.GoldPriceJPY != nil {
		t.Fatalf("%+v %v", m, e)
	}
}

func TestObservedFinancialLanguageVariants(t *testing.T) {
	for _, raw := range []string{"JPY27,500 (Tax and service charge included) /guest(s)", "27,500円（税サ込） /人"} {
		p := ParsePrice(raw)
		if p.TaxIncluded == nil || !*p.TaxIncluded || p.ServiceChargeIncluded == nil || !*p.ServiceChargeIncluded {
			t.Fatalf("%s => %+v", raw, p)
		}
	}
	d, e := ParseDetail([]byte(strings.Replace(strings.Replace(detailHTML, "～", "", 1), "Price may vary.", "※その時々の季節の食材によってお値段上下いたします。", 1)), "jv742052", "ja")
	if e != nil || !d.Courses[0].Price.Variable || d.Courses[0].Price.Minimum {
		t.Fatalf("variable note lost: %+v %v", d, e)
	}
	p := ParsePrice("JPY20,000 - JPY30,000 (Tax included) /guest(s)")
	if p.Amount == nil || *p.Amount != 20000 || p.MaximumAmount == nil || *p.MaximumAmount != 30000 || !p.Minimum || !p.Variable {
		t.Fatalf("range lost %+v", p)
	}
}
