package ikyu

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// Replace only amount2 refs in a captured public Nuxt listing. IDs, names,
// destination/stay echoes and source pagination remain the fixture's originals.
func searchFixtureWithoutPrices(t *testing.T, positions ...int) []byte {
	t.Helper()
	original := fixture(t, "search-nuxt.html")
	begin := bytes.IndexByte(original, '>') + 1
	end := bytes.Index(original[begin:], []byte("</script>")) + begin
	if begin <= 0 || end < begin {
		t.Fatal("fixture Nuxt script missing")
	}
	var table []any
	if err := json.Unmarshal(original[begin:end], &table); err != nil {
		t.Fatal(err)
	}
	index := func(value any) int { return int(value.(float64)) }
	dataRef := index(table[1].(map[string]any)["data"])
	dataIndex := index(table[dataRef].([]any)[1])
	var listingIndex int
	for key, value := range table[dataIndex].(map[string]any) {
		if strings.Contains(key, `"o":"ListPageDataIkyu"`) {
			listingIndex = index(value)
			break
		}
	}
	if listingIndex == 0 {
		t.Fatal("listing operation absent")
	}
	list := table[listingIndex].(map[string]any)
	page := table[index(list["listPageIkyu"])].(map[string]any)
	connection := table[index(page["accommodations"])].(map[string]any)
	edges := table[index(connection["edges"])].([]any)
	for _, position := range positions {
		if position < 0 || position >= len(edges) {
			t.Fatalf("invalid fixture index %d", position)
		}
		edge := table[index(edges[position])].(map[string]any)
		node := table[index(edge["node"])].(map[string]any)
		node["amount2"] = float64(-1) // Nuxt undefined; decoded source amount is nil.
	}
	encoded, err := json.Marshal(table)
	if err != nil {
		t.Fatal(err)
	}
	result := append(append([]byte{}, original[:begin]...), encoded...)
	return append(result, original[end:]...)
}

func TestSearchUnpricedPropertiesRemainDiscoverableWithoutFilters(t *testing.T) {
	for _, tc := range []struct {
		name         string
		missing      []int
		limit        int
		wantUnpriced int
	}{
		{"mixed page", []int{0}, 10, 1},
		{"all unpriced", []int{0, 1}, 10, 2},
		{"bounded prefix", []int{0}, 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := searchFixtureWithoutPrices(t, tc.missing...)
			paths := map[string]int{}
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				paths[r.URL.Path]++
				if r.URL.Path == "/" {
					w.Write(fixture(t, "destinations.html"))
					return
				}
				if r.URL.Path != "/tokyo/140000/" {
					t.Errorf("unexpected detail request %s", r.URL.Path)
				}
				w.Write(body)
			}, sourceOptions())
			out, err := c.Search(context.Background(), SearchRequest{Destination: "tokyo", Stay: validStay(), Limit: tc.limit})
			if err != nil {
				t.Fatal(err)
			}
			want := 2
			if tc.limit == 1 {
				want = 1
			}
			if len(out.Data) != want || out.Pagination.Scanned != want || out.Pagination.Returned != want || out.Pagination.Total != 587 || out.Pagination.NextOffset == nil || *out.Pagination.NextOffset != want || !out.Pagination.HasNext || out.Pagination.Complete {
				t.Fatalf("unpriced rows/pagination lost %+v", out)
			}
			if len(out.Coverage.Unknown) != tc.wantUnpriced || paths["/"] != 1 || paths["/tokyo/140000/"] != 1 || c.Stats().Requests != 2 || len(paths) != 2 {
				t.Fatalf("hidden fan-out or missing coverage: paths=%v stats=%+v coverage=%+v", paths, c.Stats(), out.Coverage)
			}
			for i, property := range out.Data {
				if property.ID == "" || property.Name == "" || !strings.Contains(property.URL, "/"+property.ID+"/") || !strings.Contains(property.URL, "cid=20261018") || !strings.Contains(property.URL, "cod=20261019") {
					t.Fatalf("identity/date handoff lost %+v", property)
				}
				unpriced := tc.name == "all unpriced" || i == 0
				if unpriced {
					if property.Price != nil || property.DetailGap == nil || !strings.Contains(*property.DetailGap, "Dated price and availability are unverified") || !strings.Contains(*property.DetailGap, "exact room-plan detail") || property.Match != nil {
						t.Fatalf("unpriced offer fabricated/hidden %+v", property)
					}
					encoded, err := json.Marshal(property)
					if err != nil {
						t.Fatal(err)
					}
					var object map[string]any
					if err = json.Unmarshal(encoded, &object); err != nil {
						t.Fatal(err)
					}
					if value, present := object["from_price"]; !present || value != nil {
						t.Fatalf("price null not explicit %s", encoded)
					}
				} else if property.Price == nil || property.DetailGap != nil {
					t.Fatalf("priced source row degraded %+v", property)
				}
			}
		})
	}
}

func TestSearchUnpricedPropertiesNeverProveRequestedPreferences(t *testing.T) {
	for _, tc := range []struct {
		name    string
		missing []int
		p       Preferences
	}{
		{"mixed page budget", []int{0}, Preferences{MaxBudget: pointer(int64(1))}},
		{"mixed page room preference", []int{0}, Preferences{OutdoorBath: true}},
		{"all unpriced budget", []int{0, 1}, Preferences{MaxBudget: pointer(int64(1))}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := searchFixtureWithoutPrices(t, tc.missing...)
			paths := map[string]int{}
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				paths[r.URL.Path]++
				if r.URL.Path == "/" {
					w.Write(fixture(t, "destinations.html"))
					return
				}
				if r.URL.Path != "/tokyo/140000/" {
					t.Errorf("unexpected detail request %s", r.URL.Path)
				}
				w.Write(body)
			}, sourceOptions())
			out, err := c.Search(context.Background(), SearchRequest{Destination: "tokyo", Stay: validStay(), Limit: 10, Preferences: tc.p})
			if err != nil {
				t.Fatal(err)
			}
			if out.Pagination.Scanned != 2 || out.Pagination.Returned != len(out.Data) || out.Pagination.Total != 587 || out.Pagination.NextOffset == nil || *out.Pagination.NextOffset != 2 || len(out.Coverage.Unknown) < len(tc.missing) || c.Stats().Requests != 2 || len(paths) != 2 {
				t.Fatalf("filtered page or requests misreported %+v paths=%v", out, paths)
			}
			for _, property := range out.Data {
				if property.Price == nil || property.ID == "00000946" {
					t.Fatalf("unpriced property treated as filter match %+v", property)
				}
			}
		})
	}
}
