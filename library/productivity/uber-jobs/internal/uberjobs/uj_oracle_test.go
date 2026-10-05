// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package uberjobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// ujOracleRows returns the 25 captured requisitionList rows as raw JSON.
func ujOracleRows(t *testing.T) []map[string]any {
	t.Helper()
	var env struct {
		Items []struct {
			TotalJobsCount  int              `json:"TotalJobsCount"`
			RequisitionList []map[string]any `json:"requisitionList"`
		} `json:"items"`
	}
	if err := json.Unmarshal(ujReadTestdata(t, "oracle_list.json"), &env); err != nil {
		t.Fatalf("oracle_list.json: %v", err)
	}
	if len(env.Items) != 1 || len(env.Items[0].RequisitionList) != 25 || env.Items[0].TotalJobsCount != 578 {
		t.Fatalf("oracle_list.json shape changed")
	}
	return env.Items[0].RequisitionList
}

// ujOraclePage builds a requisitionList reply of n rows with ids from
// startID, cloned from a real fixture row. It is safe to call from a
// server handler (no t.Fatal).
func ujOraclePage(base map[string]any, startID, n, total int) []byte {
	list := make([]map[string]any, 0, n)
	for i := 0; i < n; i++ {
		row := map[string]any{}
		for k, v := range base {
			row[k] = v
		}
		row["Id"] = strconv.Itoa(startID + i)
		list = append(list, row)
	}
	b, _ := json.Marshal(map[string]any{"items": []any{map[string]any{"TotalJobsCount": total, "requisitionList": list}}})
	return b
}

// ujFinder extracts the raw (still percent-encoded) finder value from a
// request's raw query. url.Query() would drop it: it holds a literal ';'.
func ujFinder(t *testing.T, rawQuery string) string {
	t.Helper()
	v := ujQueryValues(rawQuery)["finder"]
	if len(v) != 1 {
		t.Fatalf("raw query %q has %d finder values, want 1", rawQuery, len(v))
	}
	return v[0]
}

// ujOffset reads offset=N from a raw finder; -1 when absent. Handler-safe.
func ujOffset(rawQuery string) int {
	v := ujQueryValues(rawQuery)["finder"]
	if len(v) != 1 {
		return -1
	}
	for _, part := range strings.Split(v[0], ",") {
		if n, ok := strings.CutPrefix(part, "offset="); ok {
			if off, err := strconv.Atoi(n); err == nil {
				return off
			}
		}
	}
	return -1
}

func ujOracleClient(t *testing.T, oracleURL string) *Client {
	t.Helper()
	c := ujClient(t, "http://127.0.0.1:9")
	c.OracleBase = oracleURL
	return c
}

// TestOracleSearchAllPaging pins the finder shape measured on the tenant:
// limit=200 inside the finder with literal ';' and ',', offsets 0, 200, 400,
// and a stop on the first empty page.
func TestOracleSearchAllPaging(t *testing.T) {
	base := ujOracleRows(t)[0]
	srv := ujNewFake(t, func(w http.ResponseWriter, r *http.Request) {
		off := ujOffset(r.URL.RawQuery)
		switch off {
		case 0, 200:
			_, _ = w.Write(ujOraclePage(base, 300000+off, 200, 400))
		default:
			_, _ = w.Write(ujOraclePage(base, 0, 0, 400))
		}
	})
	c := ujOracleClient(t, srv.URL)
	res, err := c.OracleSearchAll(context.Background(), "")
	if err != nil {
		t.Fatalf("OracleSearchAll: %v", err)
	}
	hits := srv.Hits()
	if len(hits) != 3 {
		t.Fatalf("requests = %d, want 3 (two full pages, one empty)", len(hits))
	}
	for i, h := range hits {
		if h.Path != oracleListPath {
			t.Errorf("request %d path = %q", i, h.Path)
		}
		want := fmt.Sprintf("findReqs;siteNumber=CX_1,limit=200,offset=%d,sortBy=POSTING_DATES_DESC", i*200)
		if got := ujFinder(t, h.RawQuery); got != want {
			t.Errorf("request %d raw finder = %q, want %q", i, got, want)
		}
		q := ujQueryValues(h.RawQuery)
		if q["onlyData"][0] != "true" || q["expand"][0] != "requisitionList.secondaryLocations" {
			t.Errorf("request %d query = %q", i, h.RawQuery)
		}
		if strings.Contains(h.RawQuery, "%3B") || strings.Contains(h.RawQuery, "%2C") {
			t.Errorf("request %d encoded the finder separators: %q", i, h.RawQuery)
		}
		if strings.Contains(h.RawQuery, "&limit=") || strings.Contains(h.RawQuery, "&offset=") {
			t.Errorf("request %d sent limit/offset as top-level params (ignored by Oracle)", i)
		}
		if ua := h.Header.Get("User-Agent"); ua != OracleUserAgent {
			t.Errorf("request %d User-Agent = %q, want the measured OracleUserAgent", i, ua)
		}
	}
	if len(res.Rows) != 400 || res.Total != 400 || !res.Complete || res.ScanCapHit || res.Requests != 3 {
		t.Errorf("rows=%d total=%d complete=%v cap=%v requests=%d, want 400/400/true/false/3", len(res.Rows), res.Total, res.Complete, res.ScanCapHit, res.Requests)
	}
}

// TestOracleSearchAllDedupes: rows repeated across pages count once, so the
// read is short of the total and reported incomplete.
func TestOracleSearchAllDedupes(t *testing.T) {
	base := ujOracleRows(t)[0]
	srv := ujNewFake(t, func(w http.ResponseWriter, r *http.Request) {
		switch ujOffset(r.URL.RawQuery) {
		case 0:
			_, _ = w.Write(ujOraclePage(base, 1000, 200, 400))
		case 200:
			_, _ = w.Write(ujOraclePage(base, 1100, 200, 400)) // ids 1100-1199 repeat
		default:
			_, _ = w.Write(ujOraclePage(base, 0, 0, 400))
		}
	})
	c := ujOracleClient(t, srv.URL)
	res, err := c.OracleSearchAll(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 300 || res.Complete {
		t.Errorf("rows=%d complete=%v, want 300 unique and incomplete", len(res.Rows), res.Complete)
	}
	seen := map[string]bool{}
	for _, r := range res.Rows {
		if seen[string(r.ID)] {
			t.Fatalf("duplicate id %s in result", r.ID)
		}
		seen[string(r.ID)] = true
	}
}

// TestOracleSearchAllScanCap: a tenant that never returns an empty page
// stops after 10 pages and says the scan was capped.
func TestOracleSearchAllScanCap(t *testing.T) {
	base := ujOracleRows(t)[0]
	srv := ujNewFake(t, func(w http.ResponseWriter, r *http.Request) {
		off := ujOffset(r.URL.RawQuery)
		_, _ = w.Write(ujOraclePage(base, 500000+off, 200, 9999))
	})
	c := ujOracleClient(t, srv.URL)
	res, err := c.OracleSearchAll(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if srv.Count() != 10 || !res.ScanCapHit || res.Complete || len(res.Rows) != 2000 {
		t.Errorf("requests=%d cap=%v complete=%v rows=%d, want 10/true/false/2000", srv.Count(), res.ScanCapHit, res.Complete, len(res.Rows))
	}
	last := ujFinder(t, srv.Hits()[9].RawQuery)
	if !strings.Contains(last, "offset=1800") {
		t.Errorf("tenth finder = %q, want offset=1800", last)
	}
}

// TestOracleSearchAllKeywordQuoted: the keyword is a quoted finder value,
// with the quotes and space percent-encoded and inner quotes escaped.
func TestOracleSearchAllKeywordQuoted(t *testing.T) {
	base := ujOracleRows(t)[0]
	srv := ujNewFake(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(ujOraclePage(base, 0, 0, 0))
	})
	c := ujOracleClient(t, srv.URL)
	for _, tc := range []struct{ kw, raw, decoded string }{
		{"data engineer", `,keyword=%22data%20engineer%22`, `,keyword="data engineer"`},
		{`say "hi"`, `,keyword=%22say%20%5C%22hi%5C%22%22`, `,keyword="say \"hi\""`},
		{"a&b=c", `,keyword=%22a%26b=c%22`, `,keyword="a&b=c"`},
	} {
		if _, err := c.OracleSearchAll(context.Background(), "  "+tc.kw+" "); err != nil {
			t.Fatal(err)
		}
		hits := srv.Hits()
		raw := ujFinder(t, hits[len(hits)-1].RawQuery)
		if !strings.HasSuffix(raw, tc.raw) {
			t.Errorf("keyword %q: raw finder = %q, want suffix %q", tc.kw, raw, tc.raw)
		}
		dec, err := url.PathUnescape(raw)
		if err != nil || !strings.HasSuffix(dec, tc.decoded) {
			t.Errorf("keyword %q: decoded finder = %q (%v), want suffix %q", tc.kw, dec, err, tc.decoded)
		}
	}
	// A blank keyword adds no keyword clause. (This fake's empty list then
	// fails the unfiltered-corpus content check, which is expected.)
	var ce *ContentError
	if _, err := c.OracleSearchAll(context.Background(), "   "); !errors.As(err, &ce) {
		t.Fatalf("blank keyword over an empty list: err = %v, want a content error", err)
	}
	hits := srv.Hits()
	if raw := ujFinder(t, hits[len(hits)-1].RawQuery); strings.Contains(raw, "keyword") {
		t.Errorf("blank keyword sent: %q", raw)
	}
}

// TestOracleSearchAllEmptyFirstPage: an empty tenant is complete with zero
// rows after one request.
func TestOracleSearchAllEmptyFirstPage(t *testing.T) {
	base := ujOracleRows(t)[0]
	srv := ujNewFake(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(ujOraclePage(base, 0, 0, 0)) })
	c := ujOracleClient(t, srv.URL)
	res, err := c.OracleSearchAll(context.Background(), "nothing")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Complete || len(res.Rows) != 0 || srv.Count() != 1 {
		t.Errorf("complete=%v rows=%d requests=%d, want true/0/1", res.Complete, len(res.Rows), srv.Count())
	}
}

// TestOracleContentChecks: an Oracle reply without items or a total is a
// content error, never an empty fallback result.
func TestOracleContentChecks(t *testing.T) {
	for _, body := range []string{
		`{"items":[]}`,
		`{"items":[{"requisitionList":[]}]}`,
		`{"count":0}`,
		`<html><body>Sign in</body></html>`,
		`[]`,
	} {
		srv := ujNewFake(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) })
		c := ujOracleClient(t, srv.URL)
		res, err := c.OracleSearchAll(context.Background(), "")
		var ce *ContentError
		if !errors.As(err, &ce) || res != nil {
			t.Errorf("body %q: res=%v err=%v, want nil result and *ContentError", body, res, err)
		}
	}
}

// TestOracleNotConfigured: with no Oracle base the fallback refuses to run
// rather than sending to a default host.
func TestOracleNotConfigured(t *testing.T) {
	c := ujClient(t, "http://127.0.0.1:9")
	c.OracleBase = ""
	if c.OracleEnabled() {
		t.Fatal("OracleEnabled with an empty base")
	}
	if _, err := c.OracleSearchAll(context.Background(), ""); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Errorf("OracleSearchAll err = %v", err)
	}
	if _, err := c.OracleGet(context.Background(), "1"); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Errorf("OracleGet err = %v", err)
	}
	var nilClient *Client
	if nilClient.OracleEnabled() {
		t.Error("nil client reports Oracle enabled")
	}
}

// TestOracleRefusalIsTyped: an Oracle 403 is a refusal like the site's,
// so a refused fallback never looks like "no postings".
func TestOracleRefusalIsTyped(t *testing.T) {
	srv := ujRefusingServer(t, http.StatusForbidden, nil, "denied")
	c := ujOracleClient(t, srv.URL)
	_, err := c.OracleSearchAll(context.Background(), "")
	if !IsRefusal(err) || srv.Count() != 1 {
		t.Errorf("err=%v requests=%d, want a refusal after one request", err, srv.Count())
	}
}

func ujDetailServer(t *testing.T, body []byte) *ujFake {
	t.Helper()
	return ujNewFake(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != oracleDetailPath {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(body)
	})
}

// TestOracleGetFound maps the real detail record for 302906 and checks the
// ById finder shape.
func TestOracleGetFound(t *testing.T) {
	srv := ujDetailServer(t, ujReadTestdata(t, "oracle_detail.json"))
	c := ujOracleClient(t, srv.URL)
	d, err := c.OracleGet(context.Background(), "302906")
	if err != nil {
		t.Fatalf("OracleGet: %v", err)
	}
	h := srv.Hits()[0]
	if got := ujFinder(t, h.RawQuery); got != "ById;Id=%22302906%22,siteNumber=CX_1" {
		t.Errorf("raw finder = %q", got)
	}
	if q := ujQueryValues(h.RawQuery); q["expand"][0] != "all" || q["onlyData"][0] != "true" {
		t.Errorf("query = %q", h.RawQuery)
	}
	if d.ID != "302906" || d.Title != "Senior - FRM Advisory" || d.Category != "Finance" || d.JobSchedule != "Full time" {
		t.Errorf("detail = %+v", d)
	}
	if d.ExternalPostedStartDate != "2026-10-02T21:46:44+00:00" || d.PrimaryLocation != "New York City, NY, United States" || d.PrimaryLocationCountry != "US" {
		t.Errorf("dates/location = %q %q %q", d.ExternalPostedStartDate, d.PrimaryLocation, d.PrimaryLocationCountry)
	}
	for name, text := range map[string]string{"Description": d.Description, "Responsibilities": d.Responsibilities} {
		if strings.Contains(text, "<") || strings.Contains(text, "&nbsp;") {
			t.Errorf("%s still has HTML: %.120q", name, text)
		}
	}
	if !strings.HasPrefix(d.Description, "About the role and team") {
		t.Errorf("Description starts %.60q", d.Description)
	}
	if !strings.Contains(d.Responsibilities, "The base salary range for this role is USD $122,000 per year - USD $135,000 per year") {
		t.Errorf("Responsibilities = %.200q", d.Responsibilities)
	}
	if d.Qualifications != "" {
		t.Errorf("Qualifications = %q, want empty (fixture has \"\")", d.Qualifications)
	}
}

// TestOracleGetNotFound: items:[] and an id mismatch both mean "not
// listed", typed so get exits 3.
func TestOracleGetNotFound(t *testing.T) {
	cases := map[string]struct {
		body []byte
		id   string
	}{
		"empty items": {[]byte(`{"items":[],"count":0}`), "302906"},
		"id mismatch": {ujReadTestdata(t, "oracle_detail.json"), "999999"},
	}
	for name, tc := range cases {
		srv := ujDetailServer(t, tc.body)
		c := ujOracleClient(t, srv.URL)
		d, err := c.OracleGet(context.Background(), tc.id)
		var nf *NotFoundError
		if !errors.As(err, &nf) || nf.ID != tc.id || d != nil {
			t.Errorf("%s: d=%v err=%v, want *NotFoundError for %s", name, d, err, tc.id)
		}
	}
	srv := ujDetailServer(t, []byte(`<html>oops</html>`))
	c := ujOracleClient(t, srv.URL)
	var ce *ContentError
	if _, err := c.OracleGet(context.Background(), "1"); !errors.As(err, &ce) {
		t.Errorf("HTML detail reply: err = %v, want *ContentError", err)
	}
}

// TestOracleDetailNeverReadsBannedFields: hiring-manager, contact, and
// Internal* fields must never reach the output, even when Oracle sends them.
func TestOracleDetailNeverReadsBannedFields(t *testing.T) {
	var env map[string]any
	if err := json.Unmarshal(ujReadTestdata(t, "oracle_detail.json"), &env); err != nil {
		t.Fatal(err)
	}
	item := env["items"].([]any)[0].(map[string]any)
	item["HiringManager"] = "Jane Sentinel-Manager"
	item["HiringManagerEmail"] = "sentinel.manager@example.invalid"
	item["ExternalContactName"] = "Sentinel Contact"
	item["ExternalContactEmail"] = "sentinel.contact@example.invalid"
	item["InternalDescriptionStr"] = "SENTINEL-INTERNAL-ONLY"
	item["InternalQualificationsStr"] = "SENTINEL-INTERNAL-QUALS"
	body, _ := json.Marshal(env)

	srv := ujDetailServer(t, body)
	c := ujOracleClient(t, srv.URL)
	d, err := c.OracleGet(context.Background(), "302906")
	if err != nil {
		t.Fatal(err)
	}
	out, _ := json.Marshal(struct {
		D *OracleDetail
		P Posting
	}{d, NormalizeOracleDetail(d, "")})
	for _, banned := range []string{"Sentinel", "sentinel", "SENTINEL"} {
		if strings.Contains(string(out), banned) {
			t.Fatalf("banned field value leaked into output: %s", out)
		}
	}
	typ := reflect.TypeOf(OracleDetail{})
	for i := 0; i < typ.NumField(); i++ {
		name := typ.Field(i).Name
		if strings.Contains(name, "Hiring") || strings.Contains(name, "Contact") || strings.HasPrefix(name, "Internal") {
			t.Errorf("OracleDetail declares banned field %s", name)
		}
	}
}

// TestNormalizeOracleRowFixtures maps all 25 real listing rows: ISO2 to
// ISO3, the site's country spelling, null category and description, and an
// absolute site URL with a trailing slash.
func TestNormalizeOracleRowFixtures(t *testing.T) {
	iso := map[string]string{"MY": "MYS", "US": "USA", "IN": "IND", "DE": "DEU", "CA": "CAN", "BR": "BRA", "JP": "JPN", "ES": "ESP", "CL": "CHL", "PH": "PHL", "ZA": "ZAF", "TW": "TWN"}
	names := map[string]string{"MYS": "Malaysia", "USA": "United States", "IND": "India", "DEU": "Germany", "CAN": "Canada", "BRA": "Brazil", "JPN": "Japan", "ESP": "Spain", "CHL": "Chile", "PHL": "Philippines", "ZAF": "South Africa", "TWN": "Taiwan"}
	for _, raw := range ujOracleRows(t) {
		b, _ := json.Marshal(raw)
		var r OracleRow
		if err := json.Unmarshal(b, &r); err != nil {
			t.Fatalf("row decode: %v", err)
		}
		p := NormalizeOracleRow(r, "")
		id := raw["Id"].(string)
		want3, ok := iso[raw["PrimaryLocationCountry"].(string)]
		if !ok {
			t.Fatalf("fixture country %v not in the test table", raw["PrimaryLocationCountry"])
		}
		if p.ID != id || p.Title != strings.TrimSpace(raw["Title"].(string)) || p.Source != SourceOracle || p.Employer != "uber" {
			t.Errorf("%s: id/title/source = %q %q %q", id, p.ID, p.Title, p.Source)
		}
		if ujS(p.CountryCode) != want3 || ujS(p.Country) != names[want3] {
			t.Errorf("%s: country = %s %s, want %s %s", id, ujS(p.CountryCode), ujS(p.Country), want3, names[want3])
		}
		if ujS(p.Location) != raw["PrimaryLocation"].(string) || ujS(p.NormalizedLocation) != raw["PrimaryLocation"].(string) {
			t.Errorf("%s: location = %s", id, ujS(p.Location))
		}
		if len(p.Locations) != 1 || ujS(p.Locations[0].CountryCode) != want3 {
			t.Errorf("%s: locations = %+v", id, p.Locations)
		}
		if p.JobCategory != nil || p.Description != nil || p.SubTeam != nil {
			t.Errorf("%s: category/description/sub_team must be null on Oracle rows", id)
		}
		if p.URL != "https://jobs.uber.com/en/jobs/"+id+"/" || p.JobPath != "/en/jobs/"+id+"/" {
			t.Errorf("%s: url = %q path = %q", id, p.URL, p.JobPath)
		}
		if p.SalaryRanges == nil || len(p.SalaryRanges) != 0 {
			t.Errorf("%s: salary_ranges = %v, want []", id, p.SalaryRanges)
		}
		if ujS(p.PostedRaw) != raw["PostedDate"].(string) || ujS(p.PostedOn) != raw["PostedDate"].(string) || p.PostedDateIsFloor {
			t.Errorf("%s: posted raw/on = %s %s", id, ujS(p.PostedRaw), ujS(p.PostedOn))
		}
	}
	// A known row, end to end.
	var r OracleRow
	b, _ := json.Marshal(ujOracleRows(t)[0])
	_ = json.Unmarshal(b, &r)
	p := NormalizeOracleRow(r, "http://127.0.0.1:1/")
	if p.ID != "153856" || ujS(p.PostedDate) != "October 5, 2026" || p.URL != "http://127.0.0.1:1/en/jobs/153856/" {
		t.Errorf("row 153856 = id %q posted %s url %q", p.ID, ujS(p.PostedDate), p.URL)
	}
}

// TestNormalizeOracleRowSecondaryLocations: secondary locations are kept in
// order with their own ISO3 codes; the primary stays first.
func TestNormalizeOracleRowSecondaryLocations(t *testing.T) {
	raw := `{"Id":" 302500 ","Title":" Software Engineer II ","PostedDate":"2026-10-03","PrimaryLocation":"San Francisco, CA, United States","PrimaryLocationCountry":"US",
		"secondaryLocations":[{"Name":"London, United Kingdom","CountryCode":"GB"},{"Name":"Atlantis","CountryCode":"QQ"},{"Name":"Remote","CountryCode":null}]}`
	var r OracleRow
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatal(err)
	}
	p := NormalizeOracleRow(r, "")
	if p.ID != "302500" || p.Title != "Software Engineer II" {
		t.Errorf("id/title not trimmed: %q %q", p.ID, p.Title)
	}
	if len(p.Locations) != 4 {
		t.Fatalf("locations = %d, want 4 (primary + 3 secondary)", len(p.Locations))
	}
	if ujS(p.Locations[0].CountryCode) != "USA" || ujS(p.Locations[1].CountryCode) != "GBR" || ujS(p.Locations[1].Country) != "United Kingdom" || ujS(p.Locations[1].Address) != "London, United Kingdom" {
		t.Errorf("first two locations = %+v %+v", p.Locations[0], p.Locations[1])
	}
	if ujS(p.Locations[2].CountryCode) != "QQ" || p.Locations[2].Country != nil {
		t.Errorf("unknown code location = %s %s, want QQ and no invented country name", ujS(p.Locations[2].CountryCode), ujS(p.Locations[2].Country))
	}
	if p.Locations[3].CountryCode != nil || ujS(p.Locations[3].Address) != "Remote" {
		t.Errorf("codeless location = %+v", p.Locations[3])
	}
	if ujS(p.CountryCode) != "USA" {
		t.Errorf("country_code = %s, want the primary's USA", ujS(p.CountryCode))
	}
}

// TestNormalizeOracleDetailFixture maps the real 302906 detail record.
func TestNormalizeOracleDetailFixture(t *testing.T) {
	srv := ujDetailServer(t, ujReadTestdata(t, "oracle_detail.json"))
	c := ujOracleClient(t, srv.URL)
	d, err := c.OracleGet(context.Background(), "302906")
	if err != nil {
		t.Fatal(err)
	}
	p := NormalizeOracleDetail(d, "")
	if p.ID != "302906" || p.Title != "Senior - FRM Advisory" || p.Source != SourceOracle || p.Employer != "uber" {
		t.Errorf("identity = %q %q %q %q", p.ID, p.Title, p.Source, p.Employer)
	}
	if ujS(p.PostedDate) != "October 2, 2026" || ujS(p.PostedOn) != "2026-10-02" || ujS(p.PostedRaw) != "2026-10-02T21:46:44+00:00" || p.PostedDateIsFloor {
		t.Errorf("posted = %s / %s / %s", ujS(p.PostedDate), ujS(p.PostedOn), ujS(p.PostedRaw))
	}
	if ujS(p.CountryCode) != "USA" || ujS(p.Country) != "United States" || ujS(p.Location) != "New York City, NY, United States" {
		t.Errorf("location = %s %s %s", ujS(p.CountryCode), ujS(p.Country), ujS(p.Location))
	}
	if p.JobCategory != nil {
		t.Errorf("job_category = %s, want null (Oracle Category is its own taxonomy)", ujS(p.JobCategory))
	}
	if ujS(p.ContractType) != "Full time" {
		t.Errorf("contract_type = %s, want Full time from JobSchedule", ujS(p.ContractType))
	}
	if p.Description == nil || strings.Contains(*p.Description, "<") {
		t.Errorf("description missing or still HTML")
	}
	if p.URL != "https://jobs.uber.com/en/jobs/302906/" || p.SalaryRanges == nil {
		t.Errorf("url = %q salary_ranges nil = %v", p.URL, p.SalaryRanges == nil)
	}
	if len(p.Locations) != 1 {
		t.Errorf("locations = %d, want 1", len(p.Locations))
	}
}
