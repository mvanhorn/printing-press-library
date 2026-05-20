// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package valuation

import (
	"errors"
	"strings"
	"testing"
)

func TestParseTPGTable_ExtractsAtmosCPP(t *testing.T) {
	html := `<html><body><table>
		<tr><th>Program</th><th>cpp</th></tr>
		<tr><td>Delta SkyMiles</td><td>1.2*</td></tr>
		<tr><td>Alaska Airlines Atmos Rewards</td><td>1.4*</td></tr>
		<tr><td>American Airlines AAdvantage</td><td>1.55*</td></tr>
	</table></body></html>`
	cpp, err := parseTPGTable([]byte(html), "alaska airlines atmos")
	if err != nil {
		t.Fatalf("parseTPGTable returned err=%v; want nil", err)
	}
	if cpp != 1.4 {
		t.Errorf("cpp = %v; want 1.4", cpp)
	}
}

func TestParseTPGTable_HandlesDagger(t *testing.T) {
	html := `<html><body><table>
		<tr><td>Alaska Airlines Atmos Rewards</td><td>1.5†</td></tr>
	</table></body></html>`
	cpp, err := parseTPGTable([]byte(html), "alaska airlines atmos")
	if err != nil {
		t.Fatalf("parseTPGTable returned err=%v; want nil", err)
	}
	if cpp != 1.5 {
		t.Errorf("cpp = %v; want 1.5", cpp)
	}
}

func TestParseTPGTable_CaseInsensitive(t *testing.T) {
	html := `<html><body><table>
		<tr><td>ALASKA AIRLINES ATMOS REWARDS</td><td>1.4</td></tr>
	</table></body></html>`
	cpp, err := parseTPGTable([]byte(html), "Alaska Airlines Atmos")
	if err != nil {
		t.Fatalf("parseTPGTable returned err=%v; want nil", err)
	}
	if cpp != 1.4 {
		t.Errorf("cpp = %v; want 1.4", cpp)
	}
}

func TestParseTPGTable_RowMissing(t *testing.T) {
	html := `<html><body><table>
		<tr><td>Delta SkyMiles</td><td>1.2*</td></tr>
	</table></body></html>`
	_, err := parseTPGTable([]byte(html), "alaska airlines atmos")
	if !errors.Is(err, ErrTPGParse) {
		t.Errorf("err = %v; want ErrTPGParse", err)
	}
}

func TestParseTPGTable_OutOfRangeRejected(t *testing.T) {
	// A cell value that parses to >10 is almost certainly the wrong column.
	html := `<html><body><table>
		<tr><td>Alaska Airlines Atmos Rewards</td><td>40000</td></tr>
	</table></body></html>`
	_, err := parseTPGTable([]byte(html), "alaska airlines atmos")
	if !errors.Is(err, ErrTPGParse) {
		t.Errorf("err = %v; want ErrTPGParse for out-of-range cpp", err)
	}
}

func TestLooksLikeCloudflareChallenge_PositiveCases(t *testing.T) {
	bodies := []string{
		`<title>Just a moment...</title>`,
		`<div id="cf-browser-verification">checking</div>`,
		`Attention Required! | Cloudflare`,
		`<html>Checking your browser before accessing example.com</html>`,
	}
	for _, b := range bodies {
		if !looksLikeCloudflareChallenge([]byte(b)) {
			t.Errorf("looksLikeCloudflareChallenge returned false for %q", b[:min(len(b), 60)])
		}
	}
}

func TestLooksLikeCloudflareChallenge_NegativeCase(t *testing.T) {
	body := `<html><body><h1>Monthly Valuations</h1></body></html>`
	if looksLikeCloudflareChallenge([]byte(body)) {
		t.Errorf("looksLikeCloudflareChallenge returned true for benign body")
	}
}

// Smoke test: confirm the regex catches both integers and floats from a
// realistic cell string with surrounding punctuation.
func TestCPPRegexp_Sanity(t *testing.T) {
	cases := map[string]string{
		"1.4*":     "1.4",
		"1.55†":    "1.55",
		"  2.0 ":   "2.0",
		"3":        "3",
		"1,400":    "1", // first run is "1" (TPG never uses thousands separators here)
	}
	for input, want := range cases {
		got := cppRegexp.FindString(strings.TrimSpace(input))
		if got != want {
			t.Errorf("cppRegexp on %q = %q; want %q", input, got, want)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
