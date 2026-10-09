// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package tickets

import (
	"strings"
	"testing"
	"time"
)

func mustDate(t *testing.T, s string) Date {
	t.Helper()
	d, err := ParseDate(s)
	if err != nil {
		t.Fatalf("ParseDate(%q): %v", s, err)
	}
	return d
}

func TestSunsetShibuya(t *testing.T) {
	cases := []struct {
		date     string
		wantHHMM string
	}{
		{"2026-11-24", "16:29"}, // published tables: about 16:29-16:30 in Tokyo
		{"2026-06-21", "19:00"},
		{"2026-12-05", "16:28"},
	}
	for _, c := range cases {
		got, ok := Sunset(mustDate(t, c.date), ShibuyaLat, ShibuyaLon)
		if !ok {
			t.Fatalf("%s: no sunset", c.date)
		}
		want, _ := time.ParseInLocation("2006-01-02 15:04", c.date+" "+c.wantHHMM, JST)
		if diff := got.Sub(want); diff < -2*time.Minute || diff > 2*time.Minute {
			t.Errorf("%s: sunset %s, want about %s", c.date, got.Format("15:04:05"), c.wantHHMM)
		}
	}
}

func TestSaleMoments(t *testing.T) {
	if got := FormatJST(GhibliSaleMoment(mustDate(t, "2026-11-24"))); got != "2026-10-10T10:00:00+09:00" {
		t.Errorf("Ghibli Nov visit: %s", got)
	}
	if got := FormatJST(GhibliSaleMoment(mustDate(t, "2027-01-03"))); got != "2026-12-10T10:00:00+09:00" {
		t.Errorf("Ghibli Jan visit crosses year: %s", got)
	}
	if got := FormatJST(SkySaleMoment(mustDate(t, "2026-11-24"))); got != "2026-11-10T00:00:00+09:00" {
		t.Errorf("SHIBUYA SKY D-14: %s", got)
	}
	if got := FormatJST(SkySaleMoment(mustDate(t, "2027-01-05"))); got != "2026-12-22T00:00:00+09:00" {
		t.Errorf("SHIBUYA SKY across year: %s", got)
	}
}

func TestPlanSunsetSlotsAndTiers(t *testing.T) {
	loc, _ := time.LoadLocation("America/Los_Angeles")
	plan, ok := PlanSunset(mustDate(t, "2026-11-24"), loc)
	if !ok {
		t.Fatal("no plan")
	}
	if len(plan.TargetSlots) == 0 {
		t.Fatal("no target slots")
	}
	for _, s := range plan.TargetSlots {
		if s.MinutesBefore < 20 || s.MinutesBefore > 60 {
			t.Errorf("slot %s is %d min before sunset", s.Start, s.MinutesBefore)
		}
		if s.WebAdultJPY != 3400 || s.CounterAdultJPY != 3700 {
			t.Errorf("slot %s after 15:00 must use the higher tier, got %d/%d", s.Start, s.WebAdultJPY, s.CounterAdultJPY)
		}
	}
	if !strings.HasSuffix(plan.SunsetLocal, "-08:00") {
		t.Errorf("sunset_local not in Los Angeles: %s", plan.SunsetLocal)
	}
}

func TestParseGhibliCalendar(t *testing.T) {
	page := `<p>Tickets go on sale at 10 a.m. (JST) on the 10th of each month.</p>
<h3>November 2026</h3><table><tr>
<td class="text-center"></td><td class="text-center">1</td><td class="text-center close-dates">2</td><td class="text-center">3</td>
</tr></table>
<h3>December 2026</h3><table><tr><td class="text-center close-dates">29</td></tr></table>`
	closed, known, rule := ParseGhibliCalendar(page)
	if !rule {
		t.Error("rule text not detected")
	}
	if !closed["2026-11-02"] || closed["2026-11-03"] || !closed["2026-12-29"] {
		t.Errorf("closed = %v", closed)
	}
	if !known["2026-11-01"] || known["2026-11-04"] {
		t.Errorf("known = %v", known)
	}
}

func TestParseLawsonGhibli(t *testing.T) {
	page := `<p>毎月10日10:00より、翌月入場分チケットを一斉発売</p>
<!-- <p>2026/9/1(火)～9/30(水)入場分 ※9月入場券は8/1(土)発売</p> -->
<p>2026/10/1(木)～10/31(土)入場分</p>
<p>※10月入場券は9/10(木)発売</p>
<p>※10/1(木)・10/3(土)は市民デーの為、一般販売はございません</p>`
	n := ParseLawsonGhibli(page)
	if !n.RuleFound || n.GeneralHour != 10 {
		t.Errorf("rule: found=%v hour=%d", n.RuleFound, n.GeneralHour)
	}
	if got := n.SaleDate["2026-10"]; got.String() != "2026-09-10" {
		t.Errorf("October override = %s", got)
	}
	if _, ok := n.SaleDate["2026-09"]; ok {
		t.Error("commented-out block must be ignored")
	}
	if _, ok := n.NoGeneralSale["2026-10-03"]; !ok {
		t.Errorf("citizens' day missing: %v", n.NoGeneralSale)
	}
}

func TestParseLawsonSearch(t *testing.T) {
	page := `<div class="ResultBox boxContents prfSummaryItem">
<h3 class="ResultBox__title">三鷹の森ジブリ美術館　【１０月入場分】</h3>
<dt class="ResultBox__informationText">2026/10/9(金) ～ 2026/10/31(土)</dt>
<p class="orderAccepting ResultBox__status textSStat">発売中</p>
<p class="ResultBox__date orderEndDate" id="receiptDat">2026/9/10(木) 10:00 ～ 2026/10/31(土) 16:00</p>
</div>
<div class="ResultBox boxContents prfSummaryItem"><h3 class="ResultBox__title">別の公演</h3>
<dt>2026/10/9(金) ～ 2026/10/31(土)</dt><p id="receiptDat">2026/9/10(木) 10:00 ～ 2026/10/31(土) 16:00</p></div>`
	w := ParseLawsonSearch(page)
	if len(w) != 1 {
		t.Fatalf("windows = %d, want 1 (non-Ghibli listing must be skipped)", len(w))
	}
	if w[0].FirstVisit != "2026-10-09" || w[0].LastVisit != "2026-10-31" || w[0].Status != "発売中" {
		t.Errorf("window = %+v", w[0])
	}
	if FormatJST(*w[0].Opens) != "2026-09-10T10:00:00+09:00" || FormatJST(*w[0].Closes) != "2026-10-31T16:00:00+09:00" {
		t.Errorf("window times = %s .. %s", FormatJST(*w[0].Opens), FormatJST(*w[0].Closes))
	}
	if !strings.Contains(w[0].Label, "10月入場分") {
		t.Errorf("label not half-width: %q", w[0].Label)
	}
}

func TestSkyRuleChecks(t *testing.T) {
	bundle := DecodeJSEscapes(`"\u5165\u5834\u65e5\u306e2\u9031\u9593\u524d\u306e\u65e5\u672c\u6642\u9593\u5348\u524d0\u6642\u304b\u3089\u8ca9\u58f2" "20\u5206\u3054\u3068\u306e\u5165\u5834\u6642\u9593\u6307\u5b9a\u5236"`)
	rule, slot := CheckSkyFAQ(bundle)
	if !rule || !slot {
		t.Errorf("rule=%v slot=%v in %q", rule, slot, bundle)
	}
}

func TestParseTeamLab(t *testing.T) {
	cfg, err := ParseTeamLabConfig([]byte(`[{"key":"Calendar_Period","value":"2026-12"},{"key":"x","value":"y"}]`))
	if err != nil || cfg.CalendarPeriod != "2026-12" {
		t.Fatalf("config = %+v, %v", cfg, err)
	}
	last, ok := LastDayOfPeriod(cfg.CalendarPeriod)
	if !ok || last.String() != "2026-12-31" {
		t.Errorf("last day = %s", last)
	}
	now := time.Date(2026, 10, 9, 11, 0, 0, 0, JST)
	text, at, tentative, err := ParseTeamLabNotice([]byte(`[{"key":"unpublished_calendar_notice","text":"１１月２日 12時00分発売予定"}]`), now)
	if err != nil || at == nil || FormatJST(*at) != "2026-11-02T12:00:00+09:00" || !tentative || text == "" {
		t.Errorf("notice: %q %v %v %v", text, at, tentative, err)
	}
	_, at, _, _ = ParseTeamLabNotice([]byte(`[{"key":"unpublished_calendar_notice","text":"1月5日 12時00分発売"}]`), time.Date(2026, 12, 20, 0, 0, 0, 0, JST))
	if at == nil || at.Year() != 2027 {
		t.Errorf("notice year rollover: %v", at)
	}
	st, err := ParseTeamLabStatuses([]byte(`{"records":[{"date":"2026-11-01T00:00:00+09:00","stock_status":""},{"date":"2026-11-02T00:00:00+09:00","stock_status":"few"},{"date":"2026-11-03T00:00:00+09:00","stock_status":"sold_out"},{"date":"2026-11-04T00:00:00+09:00","stock_status":"closed"}]}`))
	if err != nil || st["2026-11-01"] != DayAvailable || st["2026-11-02"] != DayFew || st["2026-11-03"] != DaySoldOut || st["2026-11-04"] != DayClosed {
		t.Errorf("statuses = %v, %v", st, err)
	}
	if _, err := ParseTeamLabStatuses([]byte(`{"error":"bad range"}`)); err == nil {
		t.Error("error body must fail")
	}
	slots, err := ParseTeamLabSlots([]byte(`{"time_spans":[{"start_time":"2026-11-24T09:00:00+09:00","start_str":"09:00","end_str":"09:30","stock_count":0,"options":[{"name":"Adults","add_cost":4800}]},{"start_time":"2026-11-24T09:30:00+09:00","start_str":"09:30","end_str":"10:00","stock_count":3,"is_few_stock":true,"options":[{"name":"Adults","add_cost":4800}]}]}`), "2026-11-24", "Admission")
	if err != nil || len(slots) != 2 || slots[0].Status != "sold_out" || slots[1].Status != "few" || *slots[1].AdultPriceJPY != 4800 {
		t.Errorf("slots = %+v, %v", slots, err)
	}
}

func TestParseDMM(t *testing.T) {
	page := `<td class="p-purchaseCalendar_close">
<input type="radio" name="calendar" value="20261008" disabled></td>
<td class="p-purchaseCalendar_available is-few">
<input type="radio" name="calendar" value="20261009"><label>9<span class="p-purchaseCalendar_stockStatus is-few"></span><span class="p-purchaseCalendar_itemPrice">4,800~</span></label></td>
<td class="p-purchaseCalendar_available">
<input type="radio" name="calendar" value="20261010"><label>10<span class="p-purchaseCalendar_stockStatus is-soldOut"></span></label></td>
<td class="p-purchaseCalendar_noStock">
<input type="radio" name="calendar" value="20261105" disabled><label>5</label></td>
<p><span>1月のチケットは10月末頃販売予定</span></p>`
	now := time.Date(2026, 10, 9, 11, 0, 0, 0, JST)
	cal := ParseDMMCalendar(page, now)
	want := map[string]DayStatus{"2026-10-08": DayPast, "2026-10-09": DayFew, "2026-10-10": DaySoldOut, "2026-11-05": DayClosed}
	for k, v := range want {
		if cal.Days[k].Status != v {
			t.Errorf("%s = %s, want %s", k, cal.Days[k].Status, v)
		}
	}
	if p := cal.Days["2026-10-09"].PriceFrom; p == nil || *p != 4800 {
		t.Errorf("price from = %v", p)
	}
	if cal.Last == nil || cal.Last.String() != "2026-11-05" {
		t.Errorf("last = %v", cal.Last)
	}
	r := cal.Release
	if r == nil || r.Covers != "2027-01" || r.EarliestDay != "2026-10-21" || r.LatestDay != "2026-10-31" || r.Confidence != "estimated" {
		t.Errorf("release = %+v", r)
	}
	slots, err := ParseDMMSlots([]byte(`{"ticket_types":{"1":{"ticket_type_id":1,"entry_times":[{"start_time":"08:00","end_time":"08:30","max_stock":145,"instock":0,"ticket_infos":{"attribute_infos":[{"attribute_name":{"text_en":"Adults"},"unit_price":"5600"}]}},{"start_time":"08:30","end_time":"09:00","max_stock":145,"instock":12,"ticket_infos":{"attribute_infos":[{"attribute_name":{"text_en":"Adults"},"unit_price":"5600"}]}}]}}}`), mustDate(t, "2026-10-09"))
	if err != nil || len(slots) != 2 || slots[0].Status != "sold_out" || slots[1].Product != "Entrance Pass" || *slots[1].AdultPriceJPY != 5600 || slots[1].StartJST != "2026-10-09T08:30:00+09:00" {
		t.Errorf("dmm slots = %+v, %v", slots, err)
	}
}

func TestPartyFit(t *testing.T) {
	p, err := ParseParty("adults=2,children=1,infants=1")
	if err != nil || p.Total() != 4 {
		t.Fatalf("party = %+v, %v", p, err)
	}
	for _, bad := range []string{"adults", "adults=-1", "pets=2", "adults=0"} {
		if _, err := ParseParty(bad); err == nil {
			t.Errorf("ParseParty(%q) should fail", bad)
		}
	}
	// Every --party error must say how to write a valid value.
	for _, bad := range []string{"adults", "adults=x", "adults=-1", "adults=51", "pets=2", "adults=0"} {
		_, err := ParseParty(bad)
		if err == nil || !strings.Contains(err.Error(), "adults=N") {
			t.Errorf("ParseParty(%q) error %v does not show the adults=N,children=N,infants=N form", bad, err)
		}
	}
	if _, err := ParseParty("adults=x"); err == nil || !strings.Contains(err.Error(), "0 to 50") {
		t.Errorf("bad count error %v does not give the allowed range", err)
	}
	three, five := 3, 5
	kept, fit := ApplyParty([]Slot{{Start: "09:00", Stock: &three}, {Start: "09:30", Stock: &five}, {Start: "10:00"}}, p)
	if fit != 1 || len(kept) != 2 || kept[0].Start != "09:30" || kept[0].FitsParty == nil || !*kept[0].FitsParty || kept[1].FitsParty != nil {
		t.Errorf("kept = %+v; want 09:30 fits and 10:00 (unknown stock) kept without a verdict", kept)
	}
	ghibli, _ := Lookup("ghibli-museum")
	if note := PartyNote(ghibli, &Party{Adults: 7, Infants: 1}); !strings.Contains(note, "free") || !strings.Contains(note, "up to 6") {
		t.Errorf("Ghibli note = %q, want infant and 6-ticket notes", note)
	}
	if note := PartyNote(ghibli, &Party{Adults: 5, Infants: 2}); strings.Contains(note, "up to 6") {
		t.Errorf("infants need no ticket: %q", note)
	}
	sky, _ := Lookup("shibuya-sky")
	if note := PartyNote(sky, p); !strings.Contains(note, "counter") {
		t.Errorf("SHIBUYA SKY child note = %q", note)
	}
}

func TestSelectAliases(t *testing.T) {
	s, err := Select("sky,ghibli,sky")
	if err != nil || len(s) != 2 {
		t.Fatalf("select = %v, %v", s, err)
	}
	tl, _ := Select("teamlab")
	if len(tl) != 5 {
		t.Errorf("teamlab expands to %d venues, want 5", len(tl))
	}
	if _, err := Select("tokyo-tower"); err == nil {
		t.Error("unknown sight must fail")
	}
}

func TestBuildOnsaleStatesAndICS(t *testing.T) {
	now := time.Date(2026, 10, 9, 11, 0, 0, 0, JST)
	ghibli, _ := Lookup("ghibli-museum")
	sky, _ := Lookup("shibuya-sky")
	bl, _ := Lookup("teamlab-borderless")

	gd := newSightData(ghibli)
	gd.RuleConfirmed = true
	gd.Days["2026-11-02"] = Day{Date: "2026-11-02", Status: DayClosed, Reason: "museum closed (official calendar)"}
	sd := newSightData(sky)
	sd.RuleConfirmed = true
	td := newSightData(bl)
	until := mustDate(t, "2026-12-31")
	td.CalendarUntil = &until
	td.Days["2026-11-03"] = Day{Date: "2026-11-03", Status: DayFew}
	opens := time.Date(2026, 11, 2, 12, 0, 0, 0, JST)
	td.Release = &Release{Covers: "2027-01", OnSale: &opens, Confidence: "announced", Text: "11月2日 12時00分発売予定", Tentative: true}

	data := map[string]*SightData{ghibli.ID: gd, sky.ID: sd, bl.ID: td}
	rows := BuildOnsale(data, []Sight{ghibli, sky, bl}, mustDate(t, "2026-11-02"), mustDate(t, "2026-11-03"), now, nil)
	by := map[string]OnsaleRow{}
	for _, r := range rows {
		by[r.Sight+"|"+r.VisitDate] = r
	}
	if r := by["ghibli-museum|2026-11-02"]; r.State != StateClosed {
		t.Errorf("Ghibli closed day state = %s", r.State)
	}
	if r := by["ghibli-museum|2026-11-03"]; r.State != StateOpensAt || ptrS(r.OnSaleJST) != "2026-10-10T10:00:00+09:00" || r.Confidence != "exact" {
		t.Errorf("Ghibli open day = %+v", r)
	}
	if r := by["shibuya-sky|2026-11-03"]; r.State != StateOpensAt || r.Sunset == nil || ptrS(r.OnSaleJST) != "2026-10-20T00:00:00+09:00" {
		t.Errorf("SHIBUYA SKY row = %+v", r)
	}
	if r := by["teamlab-borderless|2026-11-03"]; r.State != StateFew {
		t.Errorf("teamLab few state = %s", r.State)
	}
	if r := by["teamlab-borderless|2026-11-02"]; r.State != StateUnknown {
		t.Errorf("teamLab date missing from calendar data = %s", r.State)
	}
	if rows[0].State != StateFew {
		t.Errorf("act-now rows must sort first, got %s", rows[0].State)
	}

	// A date beyond the calendar uses the announced release.
	jan := BuildOnsale(data, []Sight{bl}, mustDate(t, "2027-01-04"), mustDate(t, "2027-01-04"), now, nil)
	if len(jan) != 1 || jan[0].State != StateOpensAt || ptrS(jan[0].OnSaleJST) != "2026-11-02T12:00:00+09:00" || jan[0].Confidence != "announced" {
		t.Errorf("teamLab January row = %+v", jan[0])
	}
	feb := BuildOnsale(data, []Sight{bl}, mustDate(t, "2027-02-04"), mustDate(t, "2027-02-04"), now, nil)
	if feb[0].State != StateUnknown || feb[0].Confidence != "unknown" {
		t.Errorf("teamLab unannounced month = %+v", feb[0])
	}

	ics, _ := BuildICS(append(rows, jan...), now)
	if !strings.HasPrefix(ics, "BEGIN:VCALENDAR\r\n") || strings.Count(ics, "BEGIN:VEVENT") != 4 {
		t.Errorf("ICS events = %d:\n%s", strings.Count(ics, "BEGIN:VEVENT"), ics)
	}
	if !strings.Contains(ics, "DTSTART:20261010T010000Z") {
		t.Error("Ghibli 10:00 JST must be 01:00Z")
	}
	for _, line := range strings.Split(ics, "\r\n") {
		if len(line) > 75 {
			t.Errorf("unfolded line: %q", line)
		}
	}
}

func ptrS(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
