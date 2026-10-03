package cli

import (
	"github.com/mvanhorn/printing-press-library/library/travel/michi-no-eki/internal/michi"
	"strings"
	"testing"
)

func TestMichiDetailMetadata(t *testing.T) {
	station := `<html><head><title>一般的なサイト見出し</title></head><main><div class="viewTitle"><span>長野県</span></div><div class="viewContent"><dl><dt>道の駅名</dt><dd>架空の検証駅</dd></dl></div></main></html>`
	notice := `<html><head><title>一般的なサイト見出し</title></head><article class="noticesView__content"><h3>架空の日程通知</h3><a href="/stations/views/10001">架空の検証駅</a><p class="createdDate">2026年9月30日</p></article></html>`
	for _, tc := range []struct{ kind, body, field, want, status string }{
		{"stations", station, "name", "架空の検証駅", "observed"},
		{"bulletins", notice, "notice_title", "架空の日程通知", "observed"},
		{"stations", `<html><title>Generic source page</title></html>`, "name", "unknown", "unknown"},
		{"bulletins", `<html><title>Generic source page</title></html>`, "notice_title", "unknown", "unknown"},
	} {
		t.Run(tc.kind+tc.status, func(t *testing.T) {
			out, e := michiPageMetadata([]byte(tc.body), tc.kind, "10001", "https://source.example/request", "2026-10-03T12:00:00+09:00")
			if e != nil {
				t.Fatal(e)
			}
			path := "/stations/views/10001"
			if tc.kind == "bulletins" {
				path = "/notices/views/10001"
			}
			if out[tc.field] != tc.want || out["entity_fields_status"] != tc.status || out["canonical_url"] != michi.Origin+path || out["source_url"] != "https://source.example/request" || out["observed_at"] != "2026-10-03T12:00:00+09:00" {
				t.Fatalf("metadata lost: %+v", out)
			}
			if _, ok := out["links"]; ok {
				t.Fatal("unreliable generic link images exposed")
			}
			if tc.kind == "bulletins" && tc.status == "observed" && out["published_date"] != "2026-09-30" {
				t.Fatal("publication provenance lost", out)
			}
		})
	}
	if _, e := michiPageMetadata([]byte(`<html><title>Access denied</title><body>Verify you are human</body></html>`), "stations", "10001", michi.Origin, "seen"); e == nil {
		t.Fatal("challenge accepted")
	}
	if _, e := michiPageMetadata([]byte(strings.Repeat("x", 5*1024*1024+1)), "stations", "10001", michi.Origin, "seen"); e == nil {
		t.Fatal("oversize accepted")
	}
}
