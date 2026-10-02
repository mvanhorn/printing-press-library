package toyota

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func shopCard(id, name, jp, oneway string) string {
	parts := strings.Split(id, ":")
	return fmt.Sprintf(`<div id="divBoxShop"><h2><span class="box_shop__title__eng">%s<span> Shop</span></span><span class="box_shop__title__jp">%s</span></h2>
	<dl><dd><p>08:00-20:00(Jan.01-Dec.31)</p><p>Closed : Aug.22</p></dd><dd>03-5220-2220</dd><dd>Map code: 647594*83</dd></dl><dl><dd>Tokyo Address 東京都</dd></dl>
	<ul><li>%s</li></ul><input id="hdnRCode" value="%s"><input id="hdnECode" value="%s"></div>`, name, jp, oneway, parts[0], parts[1])
}

const classCards = `<div id="divClassSelect"><span id="lblCarClassCd">C1 Class</span><span id="lblEstimatePrice">11,990</span><span id="lblPassengers">5 seats</span><a id="lnkSelect" href="javascript:x()">Select<i></i></a><span id="lblOptionFee">JPY 2,200 / one time</span><span id="lblCarName">YARIS</span><span id="lblCarName">YARIS</span><span id="lblCarName">ROOMY</span></div>
<div id="divClassSelect"><span id="lblCarClassCd">C0 Class</span><span id="lblEstimatePrice">11,990</span><span id="lblPassengers">4 seats</span><a id="lnkSelect" class="is_disabled">Fully booked</a></div>`

func TestParseShopsPreservesDistinctSourceIdentities(t *testing.T) {
	for _, tc := range []struct {
		name, card string
		want       string
		ok         bool
	}{
		{"outside", shopCard("63601:01V", "Tokyo Nihonbashi", "東京駅日本橋口店", "*One-way:Available inside / outside the prefecture Vehicle height must be under 2.1 m"), "inside_and_outside_prefecture", true},
		{"inside", shopCard("63601:08U", "Nishi-Shimbashi", "西新橋店", "*One-way:Available only inside the prefecture"), "inside_prefecture_only", true},
		{"no return", shopCard("63601:018", "Tokyo Yaesu", "東京駅八重洲口店", "*One-way:One-way returns not possible"), "not_possible", true},
		{"unknown", shopCard("63601:01V", "Tokyo Nihonbashi", "東京駅日本橋口店", ""), "unknown", true},
		{"missing ID", strings.ReplaceAll(shopCard("63601:01V", "Tokyo", "東京", ""), `id="hdnRCode"`, `id="changed"`), "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, ctx, err := ParseShops([]byte("<h1>Around Tokyo Station</h1>" + tc.card))
			if (err == nil) != tc.ok {
				t.Fatalf("err=%v", err)
			}
			if !tc.ok {
				return
			}
			if len(out) != 1 || out[0].OneWayReturns != tc.want || out[0].NameJP == "" || len(out[0].Hours) != 1 || out[0].Closure != "Closed : Aug.22" || ctx != "Around Tokyo Station" {
				t.Fatalf("parsed=%+v context=%q", out, ctx)
			}
		})
	}
	out, _, err := ParseShops([]byte(shopCard("63601:01V", "Tokyo Station", "日本橋口", "") + shopCard("63601:018", "Tokyo Station", "八重洲口", "")))
	if err != nil || len(out) != 2 || out[0].ID == out[1].ID {
		t.Fatalf("merged distinct branches: %+v %v", out, err)
	}
}

func TestParseOffersAvailabilityPricesAndModelLimits(t *testing.T) {
	for _, tc := range []struct {
		name, data   string
		limit        int
		ok           bool
		availability string
	}{
		{"real shape", classCards, 1, true, "available"},
		{"unknown label", strings.Replace(classCards, ">Select<", ">Contact shop<", 1), 3, true, "unknown"},
		{"disabled", strings.Replace(classCards, `href="javascript:x()"`, `class="is_disabled"`, 1), 3, true, "unknown"},
		{"missing price", strings.Replace(classCards, "11,990", "Not quoted", 1), 3, false, ""},
		{"empty", "<p>No recognizable cards</p>", 3, false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			offers, err := ParseOffers([]byte(tc.data), tc.limit)
			if (err == nil) != tc.ok {
				t.Fatalf("err=%v", err)
			}
			if !tc.ok {
				return
			}
			if len(offers) != 2 || offers[0].Availability != tc.availability || offers[1].Availability != "fully_booked" || *offers[0].SourceEstimateJPY != 11990 || *offers[0].Capacity != 5 || offers[0].ModelGuaranteed || len(offers[0].RepresentativeModels) > tc.limit {
				t.Fatalf("offers=%+v", offers)
			}
		})
	}
}

func TestSuccessfulFormMatchesBrowserControls(t *testing.T) {
	doc, _ := parseHTML([]byte(`<form id="PageForm">
	<input type="hidden" name="state" value="synthetic"><input type="text" name="disabled" disabled value="x">
	<input type="button" name="refresh" value="Refresh"><input type="submit" name="submit" value="Send">
	<input type="checkbox" name="on" checked><input type="checkbox" name="off">
	<input type="radio" name="radio" value="0"><input type="radio" name="radio" checked value="1">
	<select name="choose"><option value="0">Zero</option><option value="1" selected>One</option></select>
	<select name="first"><option value="a">A</option></select><textarea name="note">synthetic text</textarea>
	</form>`))
	f := successfulForm(doc)
	for _, tc := range []struct{ k, want string }{{"state", "synthetic"}, {"on", "on"}, {"off", ""}, {"disabled", ""}, {"refresh", ""}, {"submit", ""}, {"radio", "1"}, {"choose", "1"}, {"first", "a"}, {"note", "synthetic text"}} {
		if got := f.Get(tc.k); got != tc.want {
			t.Errorf("%s=%q want=%q", tc.k, got, tc.want)
		}
	}
}

func TestDateAndShopContextDriftWithholdsOffers(t *testing.T) {
	p := Period{Pickup: time.Date(2026, 10, 20, 9, 0, 0, 0, jst), Dropoff: time.Date(2026, 10, 21, 9, 0, 0, 0, jst)}
	for _, tc := range []struct {
		name, changed string
		ok            bool
	}{{"same", "", true}, {"date", "hdDepDate", false}, {"return branch", "txtHdnRetECode", false}} {
		t.Run(tc.name, func(t *testing.T) {
			var b strings.Builder
			for k, v := range map[string]string{"hdDepDate": "2026_10_20_0900", "hdRetDate": "2026_10_21_0900", "txtHdnDepRCode": "63601", "txtHdnDepECode": "01V", "txtHdnRetRCode": "63601", "txtHdnRetECode": "095"} {
				if tc.changed == k {
					v = "changed"
				}
				fmt.Fprintf(&b, `<input id="%s" value="%s">`, k, v)
			}
			doc, _ := parseHTML([]byte(b.String()))
			err := checkContext(doc, p, Shop{ID: "63601:01V"}, Shop{ID: "63601:095"})
			if (err == nil) != tc.ok {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestPublishedOperatingWindowsNeverBecomeStockSlots(t *testing.T) {
	for _, tc := range []struct {
		name, calendar string
		hour           int
		ok             bool
	}{
		{"open", `{"open":{"20261020":["0800","2000"]}}`, 9, true},
		{"closed hours", `{"open":{"20261020":["0800","2000"]}}`, 3, false},
		{"missing date", `{"open":{"20261021":["0800","2000"]}}`, 9, false},
		{"missing calendar", "{}", 9, false},
		{"changed format", `{"open":{"20261020":["8am","8pm"]}}`, 9, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc, _ := parseHTML([]byte(`<span id="calendar"><script type="application/json">` + tc.calendar + "</script></span>"))
			window, err := checkOperatingWindow(doc, "calendar", "--pickup", time.Date(2026, 10, 20, tc.hour, 0, 0, 0, jst))
			if (err == nil) != tc.ok {
				t.Fatalf("err=%v", err)
			}
			if tc.ok && (window.Opens != "08:00" || window.Closes != "20:00") {
				t.Fatalf("window=%+v", window)
			}
		})
	}
}
