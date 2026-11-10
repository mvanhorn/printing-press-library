// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestBrandQAColorExtraction(t *testing.T) {
	for _, tc := range []struct {
		name, html string
		want       []string
	}{
		{"short hex", `<p style="color:#aBc">Hi</p>`, []string{"#aabbcc"}},
		{"long hex", `<p style="color:#1122AA">Hi</p>`, []string{"#1122aa"}},
		{"alpha hex", `<p style="color:#1122AA80">Hi</p>`, []string{"#1122aa"}},
		{"rgb", `<p style="color:RGB(17, 34, 51)">Hi</p>`, []string{"#112233"}},
		{"rgba", `<p style="color:rgba(17, 34, 51, .2)">Hi</p>`, []string{"#112233"}},
		{"percent rgb", `<p style="color:rgb(100% 0% 0% / 50%)">Hi</p>`, []string{"#ff0000"}},
		{"attributes", `<TABLE BGCOLOR='#abc'><font color=#112233>Hi</font></TABLE>`, []string{"#112233", "#aabbcc"}},
		{"style blocks", `<style>#abc {color:#112233} @media screen {p {background:rgba(204, 187, 170, 1);border:1px solid #abc}}</style>`, []string{"#112233", "#aabbcc", "#ccbbaa"}},
		{"attributes and inline styles", `<td bgcolor="#123" style="background:#456; color:#789;box-shadow:0 0 1px #abc">Hi</td>`, []string{"#112233", "#445566", "#778899", "#aabbcc"}},
		{"ignore URLs selectors and prose", `<a href="https://example.com/#abcdef">#fedcba</a><style>#abc {background:url(https://example.com/#abcdef);content:'#aabbcc'}</style>`, []string{}},
		{"ignore comments", `<!-- <p style="color:#abc">Hidden</p> --><style>/* p {color:#112233} */ p {color:#445566}</style>`, []string{"#445566"}},
		{"invalid colors", `<p style="color:#abcd; background:#1234567; border:rgb(999,0,0);outline:rgba(0,0,0,2)">Hi</p>`, []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			colors := extractBrandQAColors(tc.html)
			got := make([]string, 0, len(colors))
			for color, occurrences := range colors {
				got = append(got, color)
				if len(occurrences) == 0 || !strings.Contains(occurrences[0], "line 1") {
					t.Fatalf("missing location: %v", occurrences)
				}
			}
			sort.Strings(got)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("colors=%v want=%v", got, tc.want)
			}
		})
	}
	colors := extractBrandQAColors(strings.Repeat("<p style='color:#abc'>Hello</p>\n", 6))
	if got := colors["#aabbcc"]; len(got) != brandColorOccurrenceLimit || !strings.Contains(got[2], "line 3") {
		t.Fatalf("occurrences=%v", got)
	}
	colors = extractBrandQAColors("<!--\ncomment\n-->\n<p style='color:#abc'>Visible</p>")
	if got := colors["#aabbcc"]; len(got) != 1 || !strings.Contains(got[0], "line 4") {
		t.Fatalf("comment changed occurrence line: %v", got)
	}
}

func brandQATestKit() *brandQAKit {
	return &brandQAKit{Palette: []string{"#000000", "#112233", "#ffffff"}, LogoURLs: []string{"https://cdn.example.com/current.png"}, RGBDistanceThreshold: brandColorDistanceThreshold, hasColors: true, oldLogoURLs: map[string]bool{"https://cdn.example.com/archived.png": true}}
}

func brandQAFindings(findings []map[string]any, check string) []map[string]any {
	out := []map[string]any{}
	for _, finding := range findings {
		if finding["check"] == check {
			out = append(out, finding)
		}
	}
	return out
}

func TestBrandQAColorDistanceAndNeutrals(t *testing.T) {
	if got := brandQAColorDistance(brandQAColor{0, 0, 0}, brandQAColor{3, 4, 0}); got != 5 {
		t.Fatalf("distance=%v", got)
	}
	for _, tc := range []struct {
		color  string
		strict bool
		warn   bool
	}{
		{"#112233", false, false}, {"#1d2233", true, false}, {"#1e2233", true, true},
		{"#888888", false, false}, {"#888888", true, true}, {"#888889", false, true},
		{"#ffffff", true, false}, {"#000000", true, false}, {"#11223380", true, false},
	} {
		t.Run(fmt.Sprintf("%s/strict=%t", tc.color, tc.strict), func(t *testing.T) {
			kit := brandQATestKit()
			kit.StrictNeutrals = tc.strict
			findings := brandQAFindings(kit.checkHTML(`<footer style="color:`+tc.color+`">Address</footer>`), "brand_colors")
			if (len(findings) > 0) != tc.warn {
				t.Fatalf("findings=%v", findings)
			}
			if tc.color == "#1e2233" && (findings[0]["nearest_brand_color"] != "#112233" || findings[0]["rgb_distance"] != 13.0 || findings[0]["status"] != "warn") {
				t.Fatalf("finding=%v", findings[0])
			}
		})
	}
}

func TestBrandQAOldLogos(t *testing.T) {
	for _, tc := range []struct {
		name, html string
		warn       bool
	}{
		{"library name", `<img src="https://cdn.example.com/archived.png">`, true},
		{"alt", `<img src="https://cdn.example.com/elsewhere.png" alt="Company LOGO">`, true},
		{"filename", `<img src="https://cdn.example.com/old-LoGo.png?v=1">`, true},
		{"encoded filename", `<img src="https://cdn.example.com/old-%6cogo.png">`, true},
		{"current", `<img src="https://cdn.example.com/current.png" alt="Logo">`, false},
		{"ordinary image", `<img src="https://logo.example.com/hero.png?logo=1">`, false},
		{"missing src", `<img alt="Logo">`, false},
		{"comment", `<!-- <img src="https://cdn.example.com/old-logo.png"> -->`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			findings := brandQAFindings(brandQATestKit().checkHTML(tc.html), "brand_logo")
			if (len(findings) > 0) != tc.warn {
				t.Fatalf("findings=%v", findings)
			}
			if tc.warn && (!strings.Contains(findings[0]["detail"].(string), "https://cdn.example.com/current.png") || findings[0]["url"] == "" || findings[0]["status"] != "warn") {
				t.Fatalf("finding=%v", findings[0])
			}
		})
	}
}

func TestBrandQAFooter(t *testing.T) {
	for _, tc := range []struct {
		html string
		want bool
	}{
		{`{{ organization.full_address }}`, true},
		{`{{organization.name}} {{ organization.street_address }}`, true},
		{`{{ organization.name|default:'Company' }} {{ organization.city }}`, true},
		{`<footer>Company, 123 Main Street</footer>`, true},
		{`<table class="kl-footer"><tr><td>Address</td></tr></table>`, true},
		{`<div id="email-footer">Address</div>`, true},
		{`<table data-block-type="footer">Address</table>`, true},
		{`<div role="contentinfo">Address</div>`, true},
		{`{% unsubscribe %}`, false},
		{`{{ organization.name }}`, false},
		{`{{ organization.city }}`, false},
		{`{{ organization.full_address_typo }}`, false},
		{`<!-- {{ organization.full_address }} <footer>Address</footer> -->`, false},
		{`<script>{{ organization.full_address }}</script>`, false},
		{`<div hidden><footer>Address</footer></div>`, false},
		{`<style>.footer { color:red; }</style>`, false},
		{"", false},
	} {
		t.Run(tc.html, func(t *testing.T) {
			got, detail := brandQAFooter(tc.html)
			if got != tc.want || detail == "" {
				t.Fatalf("present=%t want=%t detail=%s", got, tc.want, detail)
			}
			findings := brandQAFindings(brandQATestKit().checkHTML(tc.html), "brand_footer")
			if len(findings) != 1 || (findings[0]["status"] == "pass") != tc.want {
				t.Fatalf("findings=%v", findings)
			}
		})
	}
}

type brandQAFixture struct {
	t            *testing.T
	calls        map[string]int
	empty        bool
	singleton    bool
	organization string
	fallback     http.Handler
}

func (f *brandQAFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	endpoint := strings.TrimRight(r.URL.Path, "/")
	if !strings.HasPrefix(endpoint, "/api/brand-") && endpoint != "/api/images" && endpoint != "/api/accounts" {
		f.fallback.ServeHTTP(w, r)
		return
	}
	if r.Method != http.MethodGet || r.Header.Get("revision") != "2026-10-15" {
		f.t.Errorf("request=%s revision=%s", r.Method, r.Header.Get("revision"))
	}
	if endpoint == "/api/brand-logos" && r.URL.Query().Get("include") != "image" {
		f.t.Error("brand logos must include image")
	}
	if endpoint == "/api/images" && (r.URL.Query().Get("filter") != `contains(name,"logo")` || r.URL.Query().Get("page[size]") != "100") {
		f.t.Errorf("image request must filter logo names with page size 100: %s", r.URL)
	}
	f.calls[endpoint]++
	w.Header().Set("Content-Type", "application/json")
	if endpoint == "/api/accounts" {
		json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"id": "account", "attributes": map[string]any{"contact_information": map[string]any{"organization_name": f.organization}}}}, "links": map[string]any{"next": nil}})
		return
	}
	if f.empty {
		fmt.Fprint(w, `{"data":[],"links":{"next":null}}`)
		return
	}
	second := r.URL.Query().Get("page[cursor]") != ""
	page := map[string]any{"data": []any{}, "links": map[string]any{"next": nil}}
	if !second && !(endpoint == "/api/brand-email-defaults" && f.singleton) {
		page["links"] = map[string]any{"next": "https://a.klaviyo.com" + endpoint + "?page%5Bcursor%5D=next"}
	}
	attrs := map[string]any{}
	id := "first"
	switch endpoint {
	case "/api/brand-colors":
		hex := "#112233"
		if second {
			hex = "#445566"
		}
		attrs["colors"] = []any{map[string]any{"hex": hex, "name": "Primary"}}
	case "/api/brand-email-defaults":
		attrs["body"] = map[string]any{"h4_text": map[string]any{"color": "#778899"}, "background_color": "#ffffff"}
		attrs["footer"] = map[string]any{"link_text": map[string]any{"color": "#abcdef"}, "social_group_color": "grey"}
	case "/api/brand-buttons":
		attrs["style"] = map[string]any{"background_color": "#123456"}
		attrs["border"] = map[string]any{"border_color": "#234567"}
		attrs["text"] = map[string]any{"color": "#345678"}
	case "/api/brand-logos":
		if second {
			id = "second"
		}
		page["included"] = []any{map[string]any{"type": "image", "id": "image-" + id, "attributes": map[string]any{"image_url": "https://cdn.example.com/" + id + ".png", "name": "Logo"}}}
	case "/api/images":
		attrs["name"], attrs["image_url"] = "Journal LOGO", "https://cdn.example.com/hero.png"
		if second {
			attrs["name"], attrs["image_url"] = "Acme Archived LOGO", "https://cdn.example.com/archived.png"
		}
	default:
		f.t.Errorf("unexpected brand endpoint: %s", endpoint)
	}
	record := map[string]any{"id": id, "attributes": attrs}
	if endpoint == "/api/brand-logos" {
		record["relationships"] = map[string]any{"image": map[string]any{"data": map[string]any{"type": "image", "id": "image-" + id}}}
	}
	page["data"] = []any{record}
	if f.singleton && endpoint == "/api/brand-email-defaults" {
		page["data"] = record
	}
	json.NewEncoder(w).Encode(page)
}

func TestBrandQAFetchKitAndPaginateEveryList(t *testing.T) {
	for _, singleton := range []bool{false, true} {
		t.Run(fmt.Sprintf("singleton=%t", singleton), func(t *testing.T) {
			f := &brandQAFixture{t: t, calls: map[string]int{}, singleton: singleton, organization: "  AcMe   Co.  "}
			kit, err := fetchBrandQAKit(campaignFixtureClient(t, f), brandQAOptions{Enabled: true})
			if err != nil {
				t.Fatal(err)
			}
			want := []string{"#000000", "#112233", "#123456", "#234567", "#345678", "#445566", "#778899", "#808080", "#abcdef", "#ffffff"}
			if !reflect.DeepEqual(kit.Palette, want) || len(kit.LogoURLs) != 2 || !kit.oldLogoURLs["https://cdn.example.com/archived.png"] {
				t.Fatalf("kit=%+v", kit)
			}
			if kit.organizationWord != "acme" || kit.oldLogoURLs["https://cdn.example.com/hero.png"] {
				t.Fatalf("organization narrowing failed: %+v", kit)
			}
			for _, endpoint := range []string{"/api/brand-colors", "/api/brand-email-defaults", "/api/brand-buttons", "/api/brand-logos", "/api/accounts", "/api/images"} {
				wantCalls := 2
				if endpoint == "/api/accounts" || (singleton && endpoint == "/api/brand-email-defaults") {
					wantCalls = 1
				}
				if f.calls[endpoint] != wantCalls {
					t.Fatalf("%s calls=%d want=%d", endpoint, f.calls[endpoint], wantCalls)
				}
			}
		})
	}
}

func TestBrandQALogoOrganizationNarrowingAndFallback(t *testing.T) {
	for _, organization := range []string{"  AcMe   Co.  ", "", " \t\n "} {
		t.Run(fmt.Sprintf("organization=%q", organization), func(t *testing.T) {
			f := &brandQAFixture{t: t, calls: map[string]int{}, organization: organization}
			kit, err := fetchBrandQAKit(campaignFixtureClient(t, f), brandQAOptions{Enabled: true})
			if err != nil {
				t.Fatal(err)
			}
			fallback := strings.TrimSpace(organization) == ""
			for _, tc := range []struct {
				name, html string
				warn       bool
			}{
				{"brand library name", `<img src="https://cdn.example.com/archived.png">`, true},
				{"product library name", `<img src="https://cdn.example.com/hero.png">`, fallback},
				{"brand alt", `<img src="https://cdn.example.com/unknown.png" alt="Old ACME LOGO">`, true},
				{"product alt", `<img src="https://cdn.example.com/unknown.png" alt="Journal logo">`, fallback},
				{"brand filename", `<img src="https://cdn.example.com/old-Acme-LoGo.png">`, true},
				{"product filename", `<img src="https://cdn.example.com/Journal-logo.png">`, fallback},
				{"encoded brand filename", `<img src="https://cdn.example.com/old-%41cme%20logo.png">`, true},
				{"organization name alone", `<img src="https://cdn.example.com/Acme.png" alt="Acme Co.">`, false},
				{"second organization word", `<img src="https://cdn.example.com/unknown.png" alt="Co. logo">`, fallback},
				{"current logo", `<img src="https://cdn.example.com/first.png" alt="Acme logo">`, false},
			} {
				t.Run(tc.name, func(t *testing.T) {
					findings := brandQAFindings(kit.checkHTML(tc.html), "brand_logo")
					if (len(findings) > 0) != tc.warn {
						t.Fatalf("findings=%v want warning=%t", findings, tc.warn)
					}
				})
			}
			if f.calls["/api/accounts"] != 1 || f.calls["/api/images"] != 2 {
				t.Fatalf("HTML checks refetched account or images: %v", f.calls)
			}
		})
	}
}

func TestBrandQAEmptyKit(t *testing.T) {
	f := &brandQAFixture{t: t, calls: map[string]int{}, empty: true}
	kit, err := fetchBrandQAKit(campaignFixtureClient(t, f), brandQAOptions{Enabled: true, StrictNeutrals: true})
	if err != nil {
		t.Fatal(err)
	}
	findings := append(kit.availabilityFindings(), kit.checkHTML(`<footer style="color:#fedcba"><img src="https://cdn.example.com/old-logo.png">Address</footer>`)...)
	if len(findings) != 2 || findings[0]["status"] != "warn" || findings[1]["status"] != "pass" || qaVerdict(findings) != "warn" {
		t.Fatalf("findings=%v", findings)
	}
	if len(kit.LogoURLs) != 0 || !reflect.DeepEqual(kit.Palette, []string{"#000000", "#ffffff"}) {
		t.Fatalf("empty kit=%+v", kit)
	}
}

func executeBrandQAGate(t *testing.T, args ...string) (map[string]any, error) {
	t.Helper()
	root := newRootCmd(&rootFlags{})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(io.Discard)
	root.SetArgs(append([]string{"plan", "qa-gate", "--json", "--no-cache", "--rate-limit", "10000", "--no-learn"}, args...))
	err := root.Execute()
	var result map[string]any
	if out.Len() > 0 {
		if decodeErr := json.Unmarshal(out.Bytes(), &result); decodeErr != nil {
			t.Fatalf("output=%s error=%v", out.String(), decodeErr)
		}
	}
	return result, err
}

func decodedBrandQAFindings(t *testing.T, result map[string]any) []map[string]any {
	t.Helper()
	raw, _ := json.Marshal(result["findings"])
	var findings []map[string]any
	if err := json.Unmarshal(raw, &findings); err != nil {
		t.Fatal(err)
	}
	return findings
}

func TestBrandQAGateCampaignTwoEmailVariations(t *testing.T) {
	for _, footerMissing := range []bool{false, true} {
		t.Run(fmt.Sprintf("footer_missing=%t", footerMissing), func(t *testing.T) {
			second := `<p style="color:#ff00ff"><img src="https://cdn.example.com/archived.png"></p>{% unsubscribe %}`
			if !footerMissing {
				second += `{{ organization.name }} {{ organization.street_address }}`
			}
			campaign := &campaignModelFixture{t: t, multipleEmail: true, bodies: map[string]map[string]any{}, html: map[string]string{
				"TPL0001":         `<footer style="color:#112233"><img src="https://cdn.example.com/first.png" alt="Logo">Address {% unsubscribe %}</footer>`,
				"template-extra": second,
			}}
			f := &brandQAFixture{t: t, calls: map[string]int{}, organization: "Acme Co.", fallback: campaign}
			campaignFixtureClient(t, f)
			result, err := executeBrandQAGate(t, "--brand", "--campaign-id", modelContainerID)
			if (err != nil) != footerMissing || (result["verdict"] == "fail") != footerMissing || result["brand"] == nil || result["campaign_id"] != modelContainerID || result["evidence"] == nil {
				t.Fatalf("result=%v err=%v", result, err)
			}
			findings := decodedBrandQAFindings(t, result)
			for _, check := range []string{"brand_colors", "brand_logo"} {
				got := brandQAFindings(findings, check)
				if len(got) != 1 || got[0]["variation_id"] != "variation-extra" || got[0]["message_id"] != modelMessageID || got[0]["template_id"] != "template-extra" {
					t.Fatalf("%s=%v", check, got)
				}
			}
			if len(brandQAFindings(findings, "brand_footer")) != 2 || len(brandQAFindings(findings, "unsubscribe")) != 2 || len(brandQAFindings(findings, "subject")) != 2 {
				t.Fatalf("lost variation checks: %v", findings)
			}
			for _, endpoint := range []string{"/api/brand-colors", "/api/brand-email-defaults", "/api/brand-buttons", "/api/brand-logos", "/api/accounts", "/api/images"} {
				want := 2
				if endpoint == "/api/accounts" {
					want = 1
				}
				if count := f.calls[endpoint]; count != want {
					t.Fatalf("kit fetched more than once or incomplete: %s count=%d want=%d", endpoint, count, want)
				}
			}
		})
	}
}

func TestBrandQAGateHTMLAndTemplateModes(t *testing.T) {
	for _, mode := range []string{"html", "template", "campaign_with_html"} {
		t.Run(mode, func(t *testing.T) {
			htmlBody := `<footer style="color:#888888">Address {% unsubscribe %}</footer>`
			campaign := &campaignModelFixture{t: t, bodies: map[string]map[string]any{}, html: map[string]string{"TPL0001": htmlBody}}
			f := &brandQAFixture{t: t, calls: map[string]int{}, fallback: campaign}
			campaignFixtureClient(t, f)
			args := []string{"--brand", "--brand-strict-neutrals"}
			if mode == "template" {
				args = append(args, "--template-id", "TPL0001")
			} else {
				file := filepath.Join(t.TempDir(), "email.html")
				if err := os.WriteFile(file, []byte(htmlBody), 0600); err != nil {
					t.Fatal(err)
				}
				args = append(args, "--html", file)
				if mode == "campaign_with_html" {
					args = append(args, "--campaign-id", modelContainerID)
				}
			}
			result, err := executeBrandQAGate(t, args...)
			if err != nil || result["brand"] == nil {
				t.Fatalf("result=%v err=%v", result, err)
			}
			want := 1
			if mode == "campaign_with_html" {
				want = 2
			}
			if got := brandQAFindings(decodedBrandQAFindings(t, result), "brand_colors"); len(got) != want {
				t.Fatalf("colors=%v", got)
			}
		})
	}
}

func TestBrandQATestSendSharesGateAndDefaultsOff(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("enabled=%t", enabled), func(t *testing.T) {
			campaign := &campaignModelFixture{t: t, multipleEmail: true, bodies: map[string]map[string]any{}, html: map[string]string{
				"TPL0001": `<footer>{% unsubscribe %}</footer>`, "template-extra": `<p>No footer</p>{% unsubscribe %}`,
			}}
			f := &brandQAFixture{t: t, calls: map[string]int{}, fallback: campaign}
			campaignFixtureClient(t, f)
			args := []string{"--campaign-id", modelContainerID, "--to", "one@example.com"}
			if enabled {
				args = append(args, "--brand")
			}
			result, output, err := executeCampaignTestSend(t, &rootFlags{asJSON: true, dryRun: true, rateLimit: 10000}, args...)
			if (err != nil) != enabled || (result.Brand != nil) != enabled || len(result.Variations) != 2 {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if !enabled && (len(f.calls) != 0 || strings.Contains(output, `"brand"`)) {
				t.Fatalf("brand ran without flag: calls=%v output=%s", f.calls, output)
			}
			if enabled {
				gate, gateErr := executeBrandQAGate(t, "--brand", "--campaign-id", modelContainerID)
				if gateErr == nil || ExitCode(gateErr) != ExitCode(err) || !reflect.DeepEqual(result.Findings, decodedBrandQAFindings(t, gate)) {
					t.Fatalf("test-send and qa-gate differ: gate=%v test-send=%+v errors=%v/%v", gate, result, gateErr, err)
				}
			}
			if len(campaign.bodies) != 0 {
				t.Fatalf("unexpected mutation: %v", campaign.bodies)
			}
		})
	}
}

func TestBrandQAPaginationRejectsLoopsAndMalformedData(t *testing.T) {
	for _, body := range []string{
		`{"data":[],"links":{"next":"/api/images?page=again"}}`,
		`{"data":[],"links":{"next":"/api/other"}}`,
		`{"links":{"next":null}}`,
		`{"data":"invalid","links":{"next":null}}`,
	} {
		t.Run(body, func(t *testing.T) {
			calls := 0
			c := campaignFixtureClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				fmt.Fprint(w, body)
			}))
			if _, _, err := brandQAResources(c, "/api/images", nil); err == nil || calls > 3 {
				t.Fatalf("calls=%d err=%v", calls, err)
			}
		})
	}
}
