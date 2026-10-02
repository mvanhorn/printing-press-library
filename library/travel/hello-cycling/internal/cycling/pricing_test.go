package cycling

import (
	"os"
	"testing"
)

func TestParsePublishedPrices(t *testing.T) {
	b, e := os.ReadFile("testdata/price-tokyo.html")
	if e != nil {
		t.Fatal(e)
	}
	rows, e := ParsePrices(b, "東京都")
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		table           int
		area            string
		models          int
		duration, model string
		yen             int
	}{{0, "東京都", 4, "利用開始30分まで", "シティサイクル", 160}, {1, "千代田区", 5, "15分ごと", "チャイルドシート付き", 200}, {2, "板橋区", 4, "利用開始30分まで", "シティサイクル", 180}} {
		r := rows[tc.table]
		if r.Area != tc.area || len(r.Models) != tc.models {
			t.Fatalf("area/model scope lost: %#v", r)
		}
		found := false
		for _, rate := range r.Rows {
			if rate.Duration == tc.duration {
				cell := rate.Rates[tc.model]
				if cell.Yen == nil || *cell.Yen != tc.yen {
					t.Fatal(cell)
				}
				found = true
			}
		}
		if !found {
			t.Fatal("missing duration")
		}
	}
	unset := rows[0].Rows[0].Rates["スポーツタイプ"]
	if unset.Yen != nil || unset.Setting != "not_set" {
		t.Fatal("設定なし was converted to free")
	}
}
func TestAreaDiscoveryAndChangedSchema(t *testing.T) {
	b, e := os.ReadFile("testdata/price-index.html")
	if e != nil {
		t.Fatal(e)
	}
	areas, e := ParseAreas(b)
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, r := range areas {
		if r.Slug == "tokyo" && r.Name == "東京都" && r.URL == PriceURL+"tokyo/" {
			found = true
		}
	}
	if !found {
		t.Fatal("Tokyo discovery missing")
	}
	for _, tc := range []struct {
		name, body string
		area       bool
	}{{"foreign links", `<main><a href="https://evil.example/price/tokyo/">Tokyo</a></main>`, true}, {"no tables", `<main>No data</main>`, false}, {"mismatched columns", `<main><table><thead><tr><td>A</td><td>B</td></tr></thead><tbody><tr><th>15分ごと</th><td>100円</td></tr></tbody></table></main>`, false}} {
		t.Run(tc.name, func(t *testing.T) {
			var e error
			if tc.area {
				_, e = ParseAreas([]byte(tc.body))
			} else {
				_, e = ParsePrices([]byte(tc.body), "test")
			}
			if e == nil {
				t.Fatal("changed/untrusted source accepted")
			}
		})
	}
}
