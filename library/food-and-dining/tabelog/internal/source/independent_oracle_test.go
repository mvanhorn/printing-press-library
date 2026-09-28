package source_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/tabelog/internal/domain"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/tabelog/internal/source"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, e := os.ReadFile(filepath.Join("..", "..", "e2e", "testdata", name))
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func venue(t *testing.T, items []domain.Restaurant, id string) domain.Restaurant {
	t.Helper()
	for _, item := range items {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("missing fixture venue %s", id)
	return domain.Restaurant{}
}
func budget(t *testing.T, b domain.Budget, raw string, min, max int) {
	t.Helper()
	if b.Raw != raw || b.MinJPY == nil || b.MaxJPY == nil || *b.MinJPY != min || *b.MaxJPY != max {
		t.Fatalf("budget lost source bracket: %+v", b)
	}
}

func TestIndependentListingOraclePreservesContrastingFacts(t *testing.T) {
	listing, e := source.ParseListing(fixture(t, "tokyo-ranked.html"), "https://tabelog.com/en/tokyo/rstLst/?SrtT=rt", time.Date(2026, 9, 27, 13, 34, 43, 0, time.UTC))
	if e != nil {
		t.Fatal(e)
	}
	if len(listing.Items) != 20 {
		t.Fatalf("got%d cards", len(listing.Items))
	}
	h := venue(t, listing.Items, "13136847")
	if h.Name != "Shimbashi Hoshino" || h.Rating == nil || *h.Rating != 4.66 || h.ReviewCount == nil || *h.ReviewCount != 331 {
		t.Fatalf("independent identity/rating/count facts changed: %+v", h)
	}
	budget(t, h.DinnerBudget, "JPY 50,000 - JPY 59,999", 50000, 59999)
	if h.LunchBudget.Raw != "-" || h.LunchBudget.MinJPY != nil || h.LunchBudget.MaxJPY != nil {
		t.Fatalf("unknown lunch became a price: %+v", h.LunchBudget)
	}
	if h.NearestStation != "Onarimon Sta." || h.NearestStationDistanceM == nil || *h.NearestStationDistanceM != 416 {
		t.Fatal("station facts differ from independent source annotation")
	}
	if len(h.Awards) != 2 || h.Awards[0] != "The Tabelog Award 2026 Gold winner" {
		t.Fatalf("awards/count conflated or duplicated: %v", h.Awards)
	}
	trace := venue(t, listing.Items, "13246316")
	if trace.NearestStation != "" || trace.NearestStationDistanceM != nil || trace.Closures != nil {
		t.Fatalf("missing source facts were filled: %+v", trace)
	}
	sawada := venue(t, listing.Items, "13001043")
	budget(t, sawada.LunchBudget, "JPY 40,000 - JPY 49,999", 40000, 49999)
	budget(t, sawada.DinnerBudget, "JPY 50,000 - JPY 59,999", 50000, 59999)
}

func TestIndependentDetailOracleKeepsPriceAndStationSourcesSeparate(t *testing.T) {
	r, e := source.ParseDetail(fixture(t, "sushi-detail.html"), "https://tabelog.com/en/tokyo/A1301/A130103/13294162/", time.Now())
	if e != nil {
		t.Fatal(e)
	}
	if r.Name != "Sushi Dokoro Isseki Sanchou" || r.Rating == nil || *r.Rating != 3.47 || r.ReviewCount == nil || *r.ReviewCount != 529 {
		t.Fatal("detail identity/ratingCount differs from source oracle")
	}
	budget(t, r.LunchBudget, "JPY 8,000 - JPY 9,999", 8000, 9999)
	budget(t, r.DinnerBudget, "JPY 10,000 - JPY 14,999", 10000, 14999)
	if r.ReviewDinnerBudget == nil || r.ReviewLunchBudget == nil {
		t.Fatal("review-based budget evidence missing")
	}
	budget(t, *r.ReviewDinnerBudget, "JPY 15,000 - JPY 19,999", 15000, 19999)
	if r.NearestStation != "Shiodome Sta." || r.NearestStationDistanceM == nil || *r.NearestStationDistanceM != 250 {
		var distance any
		if r.NearestStationDistanceM != nil {
			distance = *r.NearestStationDistanceM
		}
		t.Fatalf("nearest station got %q/%v m, want Shiodome Sta./250 m", r.NearestStation, distance)
	}
	if r.Address == nil || *r.Address != "東京都港区新橋4-20-2 新橋フォーワンビル 1F" {
		t.Fatal("empty JSONLD streetAddress hid real table address")
	}
}

func TestRestaurantJSONLDArrayAndGraphRetainRatingCount(t *testing.T) {
	body := string(fixture(t, "sushi-detail.html"))
	pattern := regexp.MustCompile(`<script type="application/ld\+json">(.*?)</script>`)
	matches := pattern.FindAllStringSubmatch(body, -1)
	var original string
	for _, m := range matches {
		var d map[string]any
		if json.Unmarshal([]byte(m[1]), &d) == nil && d["@type"] == "Restaurant" {
			original = m[1]
			break
		}
	}
	if original == "" {
		t.Fatal("real fixture Restaurant JSONLD missing")
	}
	// Keep source facts unchanged and remove visible rating/count fallbacks,
	// making JSONLD shape handling observable rather than a redundant test.
	body = strings.ReplaceAll(body, ">3.47</span>", "></span>")
	body = strings.ReplaceAll(body, ">529</b> reviews", "></b> reviews")
	for name, wrapped := range map[string]string{"array": "[" + original + "]", "graph": "{\"@graph\":[{\"@type\":\"BreadcrumbList\",\"name\":\"not a restaurant\"}," + original + "]}"} {
		t.Run(name, func(t *testing.T) {
			r, e := source.ParseDetail([]byte(strings.Replace(body, original, wrapped, 1)), "https://tabelog.com/en/tokyo/A1301/A130103/13294162/", time.Now())
			if e != nil {
				t.Fatal(e)
			}
			if r.Rating == nil || *r.Rating != 3.47 || r.ReviewCount == nil || *r.ReviewCount != 529 {
				t.Fatalf("Restaurant JSONLD %s lost aggregate ratingCount", name)
			}
		})
	}
}

func TestRealRelocationPairKeepsDistinctSevenDigitIdentities(t *testing.T) {
	for _, c := range []struct {
		name, id  string
		relocated bool
	}{{"miyakawa-old.html", "1046463", true}, {"miyakawa-current.html", "1073214", false}} {
		t.Run(c.id, func(t *testing.T) {
			url := "https://tabelog.com/en/hokkaido/A0101/A010105/" + c.id + "/"
			r, e := source.ParseDetail(fixture(t, c.name), url, time.Now())
			if e != nil {
				t.Fatal(e)
			}
			if r.ID != c.id || r.Name != "Sushi Miyakawa" {
				t.Fatalf("same-name7-digit source identity changed: %+v", r)
			}
			if c.relocated {
				if r.Status == nil || !strings.EqualFold(*r.Status, "relocated") {
					t.Fatal("explicit relocation status lost")
				}
				if !strings.Contains(strings.Join(r.SourceWarnings, " "), "This is information from before the relocation.") {
					t.Fatal("historical warning lost")
				}
			} else if r.Status != nil {
				t.Fatalf("absent lifecycle status became%s", *r.Status)
			}
		})
	}
}

func TestSourceBudgetUnknownAndFullWidthRange(t *testing.T) {
	unknown := source.ParseBudget("-", "listed")
	if unknown.MinJPY != nil || unknown.MaxJPY != nil || unknown.Raw != "-" {
		t.Fatal("dash became a known price")
	}
	// This full-width separator is observed in the real detail JSONLD priceRange.
	b := source.ParseBudget("JPY 10,000～JPY 14,999", "listed")
	budget(t, b, "JPY 10,000～JPY 14,999", 10000, 14999)
}
