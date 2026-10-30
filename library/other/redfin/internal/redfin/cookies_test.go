package redfin

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

var cookieTestNow = time.Unix(1_800_000_000, 0)

func TestParseCookieExportFormats(t *testing.T) {
	tests := []struct {
		name string
		file string
	}{
		{"json array", `[
			{"name":"_abck","value":"a1","domain":".redfin.com","expirationDate":1900000000},
			{"name":"bm_sz","value":"b2","domain":"www.redfin.com"},
			{"name":"old","value":"x","domain":".redfin.com","expirationDate":1000},
			{"name":"other","value":"y","domain":".example.com"}
		]`},
		{"storage state", `{"cookies":[
			{"name":"_abck","value":"a1","domain":".redfin.com","expires":1900000000},
			{"name":"bm_sz","value":"b2","domain":"www.redfin.com","expires":-1},
			{"name":"old","value":"x","domain":".redfin.com","expires":1000}
		],"origins":[]}`},
		{"netscape", "# Netscape HTTP Cookie File\n" +
			".redfin.com\tTRUE\t/\tTRUE\t1900000000\t_abck\ta1\n" +
			"#HttpOnly_www.redfin.com\tFALSE\t/\tTRUE\t0\tbm_sz\tb2\n" +
			".redfin.com\tTRUE\t/\tTRUE\t1000\told\tx\n" +
			".example.com\tTRUE\t/\tTRUE\t1900000000\tother\ty\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCookieExport([]byte(tt.file), cookieTestNow)
			if err != nil {
				t.Fatalf("ParseCookieExport: %v", err)
			}
			names := []string{}
			for _, c := range got {
				names = append(names, c.Name)
			}
			if !reflect.DeepEqual(names, []string{"_abck", "bm_sz"}) {
				t.Fatalf("names = %v, want _abck and bm_sz", names)
			}
			if h := CookieHeader(got); h != "_abck=a1; bm_sz=b2" {
				t.Fatalf("CookieHeader = %q", h)
			}
			if !HasChallengeCookies(got) {
				t.Fatal("HasChallengeCookies = false")
			}
		})
	}
}

func TestParseCookieExportRejects(t *testing.T) {
	tests := []struct {
		name, file, want string
	}{
		{"empty", "  \n", "empty"},
		{"other site", `[{"name":"a","value":"1","domain":".notredfin.com"}]`, "no unexpired"},
		{"all expired", `[{"name":"a","value":"1","domain":".redfin.com","expires":1000}]`, "no unexpired"},
		{"garbage", "cookie=value", "not a recognized"},
		{"bad json", "{not json", "parsing JSON"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseCookieExport([]byte(tt.file), cookieTestNow)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want it to mention %q", err, tt.want)
			}
		})
	}
}

func TestParseCookieExportPicksCookieForRequestHost(t *testing.T) {
	file := `[
		{"name":"_abck","value":"parent","domain":".redfin.com"},
		{"name":"_abck","value":"host","domain":"www.redfin.com"},
		{"name":"_abck","value":"parent2","domain":"redfin.com"},
		{"name":"_abck","value":"sibling","domain":"insights.redfin.com"},
		{"name":"bm_sz","value":"sibling-only","domain":"insights.redfin.com"}
	]`
	got, err := ParseCookieExport([]byte(file), cookieTestNow)
	if err != nil {
		t.Fatalf("ParseCookieExport: %v", err)
	}
	if h := CookieHeader(got); h != "_abck=host" {
		t.Fatalf("CookieHeader = %q, want the www.redfin.com value only", h)
	}
}
