package evidence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNuxtReferencesAndMalformed(t *testing.T) {
	b := []byte(`<script id="__NUXT_DATA__" type="application/json">[["ShallowReactive",1],{"data":2},["ShallowReactive",3],{"x":4},{"data":5}, {"name":6,"lat":7,"empty":-1},"京都",35.1]</script>`)
	d, e := Nuxt(b)
	if e != nil {
		t.Fatal(e)
	}
	got := object(object(d["x"])["data"])
	if got["name"] != "京都" || got["lat"] != 35.1 || got["empty"] != nil {
		t.Fatalf("wrong decode: %#v", got)
	}
	for _, v := range []string{`[["Ref",1],{"data":2},{"x":99}]`, `[["Ref",1],{"data":2},{"x":2}]`, `[["Ref",1],{"data":2},{"x":-22}]`} {
		if _, e := Nuxt([]byte(`<script id="__NUXT_DATA__">` + v + `</script>`)); e == nil {
			t.Fatal("accepted malformed/cyclic reference")
		}
	}
}
func TestSeasonYearAndDates(t *testing.T) {
	b := []byte(`<title>桜【2025】</title><div class="close_msg">2025年の更新は終了しました</div>`)
	s, e := SeasonState(b, "sakura")
	if e != nil || s["year"] != 2025 || s["updates_ended"] != true {
		t.Fatal(s, e)
	}
	if dateEvidence("2/29", 2025) != nil || dateEvidence("2/29", 2024) != "2024-02-29" || dateEvidence("11月26日", 2026) != "2026-11-26" {
		t.Fatal("invalid day or year rollover")
	}
	if _, e := SeasonState([]byte(`<title>2026 seasonal photos</title>`), "koyo"); e == nil {
		t.Fatal("inferred year without title marker")
	}
}
func TestCriteriaUnknownDoesNotPass(t *testing.T) {
	pop := 40.0
	c := Criteria{MaxPOP: &pop}
	s, checks := CheckDaily(map[string]any{}, c)
	if s != "unknown" || object(checks[0])["passes"] != nil {
		t.Fatal(s, checks)
	}
	s, _ = CheckDaily(map[string]any{"precipitation_probability_percent": float64(41)}, c)
	if s != "does_not_meet" {
		t.Fatal(s)
	}
	s, _ = CheckDaily(map[string]any{"precipitation_probability_percent": float64(40)}, c)
	if s != "meets" {
		t.Fatal(s)
	}
}
func TestPeakDoesNotUseNormOrEnded(t *testing.T) {
	d := map[string]any{"season": map[string]any{"year": 2026, "updates_ended": true}, "normal": map[string]any{"period_ja": "11月下旬"}}
	if s, _ := PeakCheck(d, "2026-11-26", 3); s != "unknown" {
		t.Fatal(s)
	}
	d["season"] = map[string]any{"year": 2026, "updates_ended": false}
	if s, _ := PeakCheck(d, "2026-11-26", 3); s != "unknown" {
		t.Fatal(s)
	}
	d["predictions"] = []any{map[string]any{"event": "peak", "status": "available", "valid_date": "2026-11-26"}}
	for _, v := range []struct{ date, want string }{{"2026-11-29", "meets"}, {"2026-11-30", "does_not_meet"}, {"2027-11-26", "unknown"}} {
		if s, _ := PeakCheck(d, v.date, 3); s != v.want {
			t.Fatal(v, s)
		}
	}
}
func TestStrictCoordinateAndMissingValue(t *testing.T) {
	for _, s := range []string{"NaN", "Inf", "35foo", ""} {
		if num(s) != nil {
			t.Fatal("invalid number accepted", s)
		}
	}
	if number(float64(-9999)) != nil || number(float64(0)) != float64(0) {
		t.Fatal("missing sentinel vs zero")
	}
	if _, e := ParsePoint("Kyoto=35.01foo,135.7"); e == nil {
		t.Fatal("invalid coordinate accepted")
	}
}
func TestFreshOfflineCacheAndOriginBound(t *testing.T) {
	c := NewClient(t.TempDir(), time.Second)
	c.Offline = true
	u := "https://weathernews.jp/koyo/"
	f, e := c.Get(context.Background(), u, time.Hour)
	if e == nil {
		t.Fatal(f)
	}
	// Domain state tests use synthetic data and never count as live verification.
	p := cacheFile(c.CacheDir, u)
	if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		t.Fatal(e)
	}
	for _, delta := range []time.Duration{-2 * time.Hour, time.Hour, -time.Minute} {
		v := cacheEntry{u, time.Now().Add(delta), []byte("{}")}
		b, _ := json.Marshal(v)
		os.WriteFile(p, b, 0600)
		f, e := c.Get(context.Background(), u, time.Hour)
		if delta == -time.Minute {
			if e != nil || !f.CacheHit {
				t.Fatal("fresh cache not served", e)
			}
		} else if e == nil {
			t.Fatal("expired or future cache served", delta)
		}
	}
	for _, x := range []string{"https://evil.example/koyo/", "http://weathernews.jp/", "https://weathernews.jp.evil.example/", "https://weathernews.jp:443/"} {
		if _, e := c.Get(context.Background(), x, time.Hour); e == nil {
			t.Fatal("origin accepted", x)
		}
	}
	if e := bounds(51, 0); e == nil {
		t.Fatal("limit unbounded")
	}
	if x, _ := paginate([]any{1, 2}, 1, 9); len(x) != 0 {
		t.Fatal(x)
	}
	if strings.Contains(c.CacheDir, "weathernews.jp") {
		t.Fatal("test cache failed isolation")
	}
}

func TestExplicitSourceYearMustMatchSeason(t *testing.T) {
	if dateEvidence("2025/11/26", 2026) != nil || dateEvidence("2025-11-26", 2026) != nil || dateEvidence("2025年11月26日", 2026) != nil {
		t.Fatal("prior-year source date relabeled as current")
	}
	if dateEvidence("2026/11/26", 2026) != "2026-11-26" || dateEvidence("11月26日", 2026) != "2026-11-26" {
		t.Fatal("matching or yearless source date rejected")
	}
}
func TestSourceIdentityFailsClosed(t *testing.T) {
	valid := map[string]any{"is_japan": float64(1), "geo": map[string]any{"JCODE": "26104", "lat": float64(35.01), "lon": float64(135.76)}}
	if e := validateForecastIdentity(valid); e != nil {
		t.Fatal(e)
	}
	for _, d := range []map[string]any{{"is_japan": float64(1)}, {"is_japan": float64(1), "geo": map[string]any{"JCODE": "", "lat": float64(-50), "lon": "bad"}}, {"geo": valid["geo"]}, {"is_japan": "1", "geo": valid["geo"]}} {
		e := validateForecastIdentity(d)
		x, ok := e.(*Error)
		if !ok || x.Code != 5 {
			t.Fatal("malformed identity must be schema failure", e)
		}
	}
	if e := validateForecastIdentity(map[string]any{"is_japan": float64(0)}); e.(*Error).Code != 3 {
		t.Fatal("explicit exclusion misclassified", e)
	}
}

func TestPlacesFirstPartyMountainURL(t *testing.T) {
	for _, tc := range []struct {
		source string
		valid  bool
	}{
		{"/mountain/kanto/30858/?fm=onebox", true},
		{"https://weathernews.jp/mountain/kanto/30858/?fm=onebox", true},
		{"/onebox/tenki/tokyo/13201/", true},
		{"https://evil.example/mountain/kanto/30858/", false},
		{"http://weathernews.jp/mountain/kanto/30858/", false},
		{"https://user:password@weathernews.jp/mountain/kanto/30858/", false},
		{"/mountain/", false},
		{"/mountain/kanto/not-an-id/", false},
		{"/mountain/kanto/30858/extra/", false},
		{"/news/202610/010001/", false},
	} {
		t.Run(tc.source, func(t *testing.T) {
			c := NewClient(t.TempDir(), time.Second)
			c.Offline = true
			u := "https://weathernews.jp/onebox/api_search.cgi?" + url.Values{"callback": {""}, "query": {"高尾山"}, "lang": {"ja"}}.Encode()
			body, _ := json.Marshal([]map[string]any{{"loc": "高尾山 (東京都)", "lat": "35.6256", "lon": "139.2439", "url": tc.source}})
			entry, _ := json.Marshal(cacheEntry{u, time.Now().Add(-time.Minute), body})
			p := cacheFile(c.CacheDir, u)
			if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(p, entry, 0600); e != nil {
				t.Fatal(e)
			}
			got, e := c.Places(context.Background(), "高尾山", 3, 0)
			if !tc.valid {
				if e == nil {
					t.Fatal("unsupported place URL accepted")
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			item := object(got["items"].([]any)[0])
			want := strings.Split(tc.source, "?")[0]
			if !strings.HasPrefix(want, "https://") {
				want = "https://weathernews.jp" + want
			}
			if item["url"] != want || item["name_ja"] != "高尾山 (東京都)" || item["latitude"] != 35.6256 || item["elevation_m"] != nil || item["elevation_status"] != "not_provided" {
				t.Fatalf("identity changed: %#v", item)
			}
		})
	}
}

func TestCacheEvictionKeepsUnrelatedJSON(t *testing.T) {
	c := NewClient(t.TempDir(), time.Second)
	old := time.Now().Add(-2 * time.Hour)
	otherDigest := sha256.Sum256([]byte("other-tool"))
	otherHex := hex.EncodeToString(otherDigest[:]) + ".json"
	keepers := []string{
		filepath.Join(c.CacheDir, "settings.json"),
		filepath.Join(c.CacheDir, otherHex),
		filepath.Join(c.CacheDir, "wn-"+otherHex),
	}
	ns := filepath.Join(c.CacheDir, cacheNamespace)
	if e := os.MkdirAll(ns, 0700); e != nil {
		t.Fatal(e)
	}
	keepers = append(keepers, filepath.Join(ns, otherHex))
	for _, path := range keepers {
		if e := os.WriteFile(path, []byte(`{"keep":true}`), 0600); e != nil {
			t.Fatal(e)
		}
		if e := os.Chtimes(path, old, old); e != nil {
			t.Fatal(e)
		}
	}
	for i := 0; i < 130; i++ {
		u := fmt.Sprintf("https://weathernews.jp/onebox/tenki/test/%d/", i)
		c.writeCache(cacheEntry{u, time.Now(), []byte("{}")})
	}
	for _, path := range keepers {
		b, e := os.ReadFile(path)
		if e != nil || string(b) != `{"keep":true}` {
			t.Fatalf("unrelated file %s lost or changed: %s %v", path, b, e)
		}
	}
	entries, e := os.ReadDir(ns)
	if e != nil {
		t.Fatal(e)
	}
	owned := 0
	for _, entry := range entries {
		if ownedCacheName(entry.Name()) {
			owned++
		}
	}
	if owned != 128 {
		t.Fatalf("owned cache entries = %d, want 128", owned)
	}
}
