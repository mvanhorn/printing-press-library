package ikyu

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, e := os.ReadFile("../../testdata/ikyu/" + name)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func sourceOptions() Options {
	return Options{Now: func() time.Time { return time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC) }}
}
func TestOfferSourceAcceptance(t *testing.T) {
	for _, tc := range []struct {
		name      string
		alter     func(map[string]any)
		bad       bool
		available bool
	}{{"known quote", nil, false, true}, {"source booking error with amount", func(a map[string]any) { a["booking"].(map[string]any)["error"] = "blocked" }, true, false}, {"source roomplan error", func(a map[string]any) { a["error"] = "invalid plan" }, true, false}, {"adults normalized", func(a map[string]any) { a["booking"].(map[string]any)["amount"].(map[string]any)["peopleCount"] = 3 }, true, false}, {"nightly date changed", func(a map[string]any) {
		a["booking"].(map[string]any)["amount"].(map[string]any)["details"].([]any)[0].(map[string]any)["date"] = "2026-10-20"
	}, true, false}, {"sold out", func(a map[string]any) {
		b := a["booking"].(map[string]any)
		b["amount"] = nil
		b["error"] = "ご指定の条件では空室がありません。"
	}, false, false}} {
		t.Run(tc.name, func(t *testing.T) {
			var a map[string]any
			if e := json.Unmarshal(fixture(t, "offer-response.json"), &a); e != nil {
				t.Fatal(e)
			}
			rp := a["data"].(map[string]any)["accommodation"].(map[string]any)["roomPlan"].(map[string]any)
			if tc.alter != nil {
				tc.alter(rp)
			}
			body, _ := json.Marshal(a)
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
					t.Error("credential sent")
				}
				var req struct {
					Query string `json:"query"`
				}
				_ = json.NewDecoder(r.Body).Decode(&req)
				for _, unsafe := range []string{"urlPath", "BookingButton", "mutation", "inComparisonList"} {
					if strings.Contains(req.Query, unsafe) {
						t.Errorf("unsafe selection %s", unsafe)
					}
				}
				w.Write(body)
			}, sourceOptions())
			out, e := c.Offer(context.Background(), OfferRequest{PropertyID: "00002889", RoomID: "10193741", PlanID: "11055986", Stay: validStay()})
			if (e != nil) != tc.bad {
				t.Fatalf("offer err=%v", e)
			}
			if tc.bad {
				return
			}
			o := out.Data.Offer
			if o.Available == nil || *o.Available != tc.available {
				t.Fatalf("availability %+v", o)
			}
			if !tc.available {
				if o.Price != nil || o.DateVerified {
					t.Fatal("fabricated sold-out quote")
				}
				return
			}
			p := o.Price
			if p == nil || *p.Amount != 30800 || *p.DiscountAmount != 24640 || *p.InstantPoint != 6160 || !o.DateVerified || o.EchoStay == nil {
				t.Fatalf("exact quote %+v", o)
			}
			if len(out.Data.Plan.Cancellation.Rules) != 6 || out.Data.Plan.Cancellation.Rules[5].Day == nil || *out.Data.Plan.Cancellation.Rules[5].Day != 10 {
				t.Fatalf("policy collapsed %+v", out.Data.Plan.Cancellation)
			}
			if out.Data.Room.Bath.HotSpring != nil {
				t.Fatal("property onsen inferred into room")
			}
		})
	}
}
func TestRoomsOffsetPlanWindowAndDateGap(t *testing.T) {
	var req map[string]any
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Write(fixture(t, "rooms-response.json"))
	}, sourceOptions())
	out, e := c.Rooms(context.Background(), RoomsRequest{PropertyID: "00002889", Stay: validStay(), Limit: 3, Offset: 3, PlanLimit: 2, PlanOffset: 4})
	if e != nil {
		t.Fatal(e)
	}
	v := req["variables"].(map[string]any)
	if v["offset"] != float64(3) || v["planOffset"] != float64(4) || v["planFirst"] != float64(2) {
		t.Fatalf("pagination omitted %v", v)
	}
	if out.Pagination.Total != 8 || !out.Pagination.HasNext || !out.OccupancyEchoVerified || out.DateEchoVerified || !strings.Contains(out.Coverage.Note, "omit nightly dates") {
		t.Fatalf("dishonest room page %+v", out)
	}
	if len(out.Data) == 0 || out.Data[0].PlanTotal == nil {
		t.Fatal("missing plan total")
	}
}
func TestSearchDestinationAndBoundedFilterCoverage(t *testing.T) {
	for _, budget := range []int64{0, 1} {
		t.Run(fmt.Sprint(budget), func(t *testing.T) {
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/" {
					w.Write(fixture(t, "destinations.html"))
					return
				}
				if r.URL.Path != "/tokyo/140000/" {
					t.Errorf("wrong destination %s", r.URL.Path)
				}
				if r.URL.Query().Get("cid") != "20261018" {
					t.Error("missing date")
				}
				w.Write(fixture(t, "search-nuxt.html"))
			}, sourceOptions())
			p := Preferences{}
			if budget > 0 {
				p.MaxBudget = &budget
			}
			out, e := c.Search(context.Background(), SearchRequest{Destination: "tokyo", Stay: validStay(), Limit: 10, Preferences: p})
			if e != nil {
				t.Fatal(e)
			}
			if out.Pagination.Total != 587 || out.Pagination.Scanned != 2 || out.Stay != validStay() {
				t.Fatalf("wrong source page %+v", out)
			}
			if budget == 0 {
				if len(out.Data) != 2 || out.Data[0].ID != "00000946" || out.Data[0].Latitude == nil || *out.Data[0].Latitude < 35.5 || *out.Data[0].Latitude > 36.0 {
					t.Fatalf("irrelevant rows %+v", out.Data)
				}
			} else if len(out.Data) != 0 || len(out.Coverage.Applied) == 0 || out.Coverage.Note == "" {
				t.Fatalf("budget/filter gap %+v", out)
			}
		})
	}
}
func TestInvalidStayMakesNoRequest(t *testing.T) {
	count := 0
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) { count++ }, sourceOptions())
	bad := validStay()
	bad.Adults = 0
	if _, e := c.Rooms(context.Background(), RoomsRequest{PropertyID: "00002889", Stay: bad, Limit: 10}); e == nil || count != 0 {
		t.Fatalf("invalid request reached source %d %v", count, e)
	}
	bad = validStay()
	bad.CheckIn = "2026-13-40"
	if _, e := c.Search(context.Background(), SearchRequest{Destination: "tokyo", Stay: bad, Limit: 10}); e == nil || count != 0 {
		t.Fatalf("invalid date reached source %d %v", count, e)
	}
}
func TestSSRParserMalformedFailsClosed(t *testing.T) {
	for _, body := range []string{"<html>denied</html>", `<script id="__NUXT_DATA__">[]</script>`, `<script id="__NUXT_DATA__">[["ShallowReactive",1],{"data":2},["ShallowReactive",3],{"x":99}]</script>`} {
		if _, e := decodeSSR([]byte(body)); e == nil {
			t.Fatal("malformed schema accepted")
		}
	}
	out, e := decodeSSR(fixture(t, "search-nuxt.html"))
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = operation(out, "ListPageDataIkyu"); e != nil {
		t.Fatal(e)
	}
}

func TestSearchCombinedFiltersRequireOneRoomPlan(t *testing.T) {
	var base map[string]any
	if e := json.Unmarshal(fixture(t, "offer-response.json"), &base); e != nil {
		t.Fatal(e)
	}
	rawAmountJSON := base["data"].(map[string]any)["accommodation"].(map[string]any)["roomPlan"].(map[string]any)["booking"].(map[string]any)["amount"]
	encoded, _ := json.Marshal(rawAmountJSON)
	var amount rawAmount
	_ = json.Unmarshal(encoded, &amount)
	amount.Plan = &rawPlan{ID: "11055986", Meal: Meal{Code: "003", Name: "夕朝食付"}}
	for _, tc := range []struct {
		name  string
		price int64
		want  bool
	}{{"cheap different room is not a joint match", 50000, false}, {"same outdoor room with matching price and meal", 25000, true}} {
		t.Run(tc.name, func(t *testing.T) {
			cheap := amount
			cheap.DiscountAmount = pointer(int64(20000))
			outdoor := amount
			outdoor.DiscountAmount = &tc.price
			asConnection := func(a rawAmount) *connection[rawAmount] {
				return &connection[rawAmount]{Edges: []struct {
					Node rawAmount `json:"node"`
				}{{a}}}
			}
			p := rawProperty{ID: "00002889"}
			p.SearchRooms = &struct {
				Rooms *connection[rawRoom] `json:"rooms"`
			}{Rooms: &connection[rawRoom]{}}
			p.SearchRooms.Rooms.Edges = append(p.SearchRooms.Rooms.Edges, struct {
				Node rawRoom `json:"node"`
			}{rawRoom{ID: "10193741", Name: "cheap room", Amounts: asConnection(cheap)}}, struct {
				Node rawRoom `json:"node"`
			}{rawRoom{ID: "10193727", Name: "outdoor room", Attributes: []Attribute{{Value: "18", Name: "露天風呂付"}}, Amounts: asConnection(outdoor)}})

			a, r, e := qualifyingPreview(p, validStay(), Preferences{OutdoorBath: true, MaxBudget: pointer(int64(30000)), Meals: []string{"003"}})
			if e != nil {
				t.Fatal(e)
			}
			if (a != nil) != tc.want {
				t.Fatalf("split criteria matched=%v want=%v", a != nil, tc.want)
			}
			if tc.want && (r.ID != "10193727" || *a.DiscountAmount != 25000) {
				t.Fatal("headline came from wrong offer")
			}
		})
	}
}
