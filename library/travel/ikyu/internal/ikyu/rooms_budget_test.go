package ikyu

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// Source windows deliberately contain plans that server filters did not remove.
// Unknown instant payable is distinct from the known original occupancy quote.
func roomBudgetResponse(t *testing.T, finalPage bool) []byte {
	t.Helper()
	var data map[string]any
	if err := json.Unmarshal(fixture(t, "rooms-response.json"), &data); err != nil {
		t.Fatal(err)
	}
	conn := data["data"].(map[string]any)["accommodation"].(map[string]any)["searchRooms2"].(map[string]any)["rooms"].(map[string]any)
	room := conn["edges"].([]any)[0].(map[string]any)
	amounts := room["node"].(map[string]any)["amounts"].(map[string]any)
	seed, _ := json.Marshal(amounts["edges"].([]any)[0].(map[string]any)["node"])
	plans := []struct {
		id    string
		price *int64
		meal  string
	}{{"11055980", pointer(int64(8000)), "003"}, {"11055981", pointer(int64(15000)), "003"}, {"11055982", pointer(int64(25000)), "003"}, {"11055983", nil, "003"}, {"11055984", pointer(int64(14000)), "001"}, {"11055985", pointer(int64(20000)), "003"}}
	if finalPage {
		plans = plans[:1]
		plans[0].id = "11055990"
		plans[0].price = pointer(int64(11000))
	}
	edges := []any{}
	for _, p := range plans {
		var node map[string]any
		_ = json.Unmarshal(seed, &node)
		node["discountAmount"] = p.price
		plan := node["plan"].(map[string]any)
		plan["planId"] = p.id
		plan["name"] = "公開プラン " + p.id
		plan["meal"] = map[string]any{"code": p.meal, "name": "食事区分"}
		edges = append(edges, map[string]any{"node": node})
	}
	amounts["edges"] = edges
	amounts["totalCount"] = 11
	conn["edges"] = []any{room}
	conn["totalCount"] = 8
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func TestRoomsFilterEveryPlanWithinNativeBudgetWindow(t *testing.T) {
	for _, tc := range []struct {
		name string
		p    Preferences
		ids  []string
	}{
		{"no plan filters", Preferences{}, []string{"11055980", "11055981", "11055982", "11055983", "11055984", "11055985"}},
		{"minimum excludes cheap and unknown", Preferences{MinBudget: pointer(int64(10000))}, []string{"11055981", "11055982", "11055984", "11055985"}},
		{"maximum excludes expensive and unknown", Preferences{MaxBudget: pointer(int64(20000))}, []string{"11055980", "11055981", "11055984", "11055985"}},
		{"inclusive budget and meal intersection", Preferences{MinBudget: pointer(int64(15000)), MaxBudget: pointer(int64(20000)), Meals: []string{"003"}}, []string{"11055981", "11055985"}},
		{"meal filter alone", Preferences{Meals: []string{"001"}}, []string{"11055984"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := roomBudgetResponse(t, false)
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) { w.Write(body) }, sourceOptions())
			out, err := c.Rooms(context.Background(), RoomsRequest{PropertyID: "00002889", Stay: validStay(), Limit: 1, Offset: 3, PlanLimit: 6, PlanOffset: 2, Preferences: tc.p})
			if err != nil || len(out.Data) != 1 {
				t.Fatalf("room filtering %+v %v", out, err)
			}
			room := out.Data[0]
			if len(room.Plans) != len(tc.ids) {
				t.Fatalf("unfiltered plans %+v", room.Plans)
			}
			for i, plan := range room.Plans {
				if plan.ID != tc.ids[i] || !strings.Contains(plan.URL, plan.ID) {
					t.Fatalf("wrong identity/order %+v", room.Plans)
				}
			}
			pg := room.PlanPagination
			if room.PlanTotal == nil || *room.PlanTotal != 11 || pg.Total != 11 || pg.Scanned != 6 || pg.Returned != len(tc.ids) || pg.NextOffset == nil || *pg.NextOffset != 8 || !pg.HasNext || pg.Complete {
				t.Fatalf("plan filtering corrupted source pagination %+v", pg)
			}
			if out.Pagination.Scanned != 1 || out.Pagination.Returned != 1 || out.Pagination.NextOffset == nil || *out.Pagination.NextOffset != 4 {
				t.Fatalf("room paging changed %+v", out.Pagination)
			}
		})
	}
}
func TestRoomsZeroBudgetMatchesCanContinueSourcePlanWindow(t *testing.T) {
	requests := 0
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		var request struct {
			Variables struct {
				PlanOffset int `json:"planOffset"`
			} `json:"variables"`
		}
		_ = json.NewDecoder(r.Body).Decode(&request)
		w.Write(roomBudgetResponse(t, request.Variables.PlanOffset == 8))
	}, sourceOptions())
	request := RoomsRequest{PropertyID: "00002889", Stay: validStay(), Limit: 1, Offset: 3, PlanLimit: 6, PlanOffset: 2, Preferences: Preferences{MinBudget: pointer(int64(10000)), MaxBudget: pointer(int64(12000)), Meals: []string{"003"}}}
	first, err := c.Rooms(context.Background(), request)
	if err != nil || len(first.Data) != 0 || first.Pagination.Returned != 0 || first.Pagination.Scanned != 1 || first.Pagination.Total != 8 || first.Pagination.NextOffset == nil || *first.Pagination.NextOffset != 4 || !first.Pagination.HasNext || !strings.Contains(first.Coverage.Note, "--plan-offset") {
		t.Fatalf("zero-match window misreported %+v %v", first, err)
	}
	request.PlanOffset = 8
	second, err := c.Rooms(context.Background(), request)
	if err != nil || requests != 2 || len(second.Data) != 1 || len(second.Data[0].Plans) != 1 || second.Data[0].Plans[0].ID != "11055990" {
		t.Fatalf("later matching source plan lost %+v %v requests=%d", second, err, requests)
	}
	pg := second.Data[0].PlanPagination
	if pg.Offset != 8 || pg.Scanned != 1 || pg.Returned != 1 || pg.Total != 11 || pg.NextOffset == nil || *pg.NextOffset != 9 {
		t.Fatalf("continuation used filtered counts %+v", pg)
	}
}

func TestRoomsEmptyPlanWindowPreservesUnfilteredRoom(t *testing.T) {
	var data map[string]any
	if err := json.Unmarshal(roomBudgetResponse(t, false), &data); err != nil {
		t.Fatal(err)
	}
	conn := data["data"].(map[string]any)["accommodation"].(map[string]any)["searchRooms2"].(map[string]any)["rooms"].(map[string]any)
	node := conn["edges"].([]any)[0].(map[string]any)["node"].(map[string]any)
	id := node["roomId"].(string)
	node["amounts"].(map[string]any)["edges"] = []any{}
	node["attributes"] = []any{}
	body, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		p        Preferences
		returned int
	}{
		{"no plan filter retains room", Preferences{}, 1},
		{"budget needs a matching plan", Preferences{MaxBudget: pointer(int64(20000))}, 0},
		{"meal needs a matching plan", Preferences{Meals: []string{"003"}}, 0},
		{"room bath evidence still required", Preferences{OutdoorBath: true}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) { w.Write(body) }, sourceOptions())
			out, err := c.Rooms(context.Background(), RoomsRequest{PropertyID: "00002889", Stay: validStay(), Limit: 1, Offset: 3, PlanLimit: 6, PlanOffset: 11, Preferences: tc.p})
			if err != nil || len(out.Data) != tc.returned {
				t.Fatalf("empty plan window %+v %v", out, err)
			}
			if out.Pagination.Scanned != 1 || out.Pagination.Returned != tc.returned || out.Pagination.Total != 8 || out.Pagination.NextOffset == nil || *out.Pagination.NextOffset != 4 {
				t.Fatalf("source room pagination changed %+v", out.Pagination)
			}
			if tc.returned == 0 {
				return
			}
			room := out.Data[0]
			pg := room.PlanPagination
			if room.ID != id || room.Name == "" || !strings.Contains(room.URL, id) || room.Plans == nil || len(room.Plans) != 0 || room.PlanTotal == nil || *room.PlanTotal != 11 {
				t.Fatalf("room identity or empty-plan evidence lost %+v", room)
			}
			if pg.Limit != 6 || pg.Offset != 11 || pg.Total != 11 || pg.Scanned != 0 || pg.Returned != 0 || pg.NextOffset != nil || pg.HasNext || pg.Complete {
				t.Fatalf("native empty nested pagination changed %+v", pg)
			}
			if out.OccupancyEchoVerified || out.DateEchoVerified {
				t.Fatal("empty plan window fabricated quote verification")
			}
		})
	}
}
