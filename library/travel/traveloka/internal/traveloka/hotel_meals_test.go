package traveloka_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/traveloka"
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/travelokacompare"
)

type mealTestTransport func(*http.Request) (*http.Response, error)

func (f mealTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSimulatedHotelRoomsPreserveMealTypesForFlexibility(t *testing.T) {
	const roomsPath = "/api/v2/hotel/search/rooms"
	dir := t.TempDir()
	writeJSON := func(name string, value any) string {
		t.Helper()
		b, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, b, 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	cookies := writeJSON("synthetic-cookies.json", []any{map[string]any{"name": "guest", "value": "synthetic-meal-cookie", "domain": ".traveloka.com", "path": "/", "expires": -1, "secure": true}})
	requests := writeJSON("synthetic-requests.json", []any{map[string]any{"method": "POST", "url": "https://www.traveloka.com" + roomsPath, "headers": map[string]string{}, "body": `{"data":{}}`}})
	session := filepath.Join(dir, "synthetic-session.json")
	if _, err := traveloka.ImportSession(cookies, requests, session); err != nil {
		t.Fatal(err)
	}
	client, err := traveloka.NewClient(session)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.SetRateLimit(0); err != nil {
		t.Fatal(err)
	}
	query := traveloka.Query{Kind: "rooms", Market: "SG", Locale: "en-SG", Currency: "SGD", PropertyID: "source-property", PropertyName: "Source Property", CheckIn: time.Now().AddDate(0, 3, 0).Format("2006-01-02"), CheckOut: time.Now().AddDate(0, 3, 2).Format("2006-01-02"), Adults: 2, Rooms: 1, Limit: 2, ChildAges: []int{}}
	for _, tt := range []struct {
		name        string
		secondMeals []string
		pairs       int
	}{
		{"same exposed meal types", []string{"BREAKFAST"}, 1},
		{"different hidden meal types", []string{"BREAKFAST", "DINNER"}, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rate := func(id string, refundable bool, meals []string) map[string]any {
				return map[string]any{
					"hotelRoomInventoryId": id, "isRefundable": refundable, "matchSearchOccupancy": true,
					"maxOccupancy": "2", "maxChildOccupancy": "0", "numChargedRooms": "1",
					"mealPlanDisplay":     map[string]any{"displayMealPlanIncluded": "Breakfast"},
					"isBreakfastIncluded": true, "includedMealTypes": meals, "rateType": "PAY_NOW",
					"ccGuaranteeRequirement": "NOT_REQUIRED", "bookingPolicy": map[string]any{"payment": "PAY_NOW"},
					"finalPrice": map[string]any{"totalPriceRateDisplay": map[string]any{"inclusiveFinalPrice": map[string]any{"currency": "SGD", "amount": "10000"}, "numOfDecimalPoint": "2"}},
				}
			}
			reply := map[string]any{"data": map[string]any{"status": "SUCCESS", "recommendedEntries": []any{map[string]any{"hotelRoomId": "source-room", "name": "Double", "hotelRoomInventoryList": []any{rate("flexible", true, []string{"BREAKFAST"}), rate("fixed", false, tt.secondMeals)}}}}}
			raw, err := json.Marshal(reply)
			if err != nil {
				t.Fatal(err)
			}
			client.SetHTTPTransport(mealTestTransport(func(r *http.Request) (*http.Response, error) {
				if r.URL.Host != "www.traveloka.com" || r.URL.Path != roomsPath || r.Method != "POST" {
					t.Fatal("unexpected source operation")
				}
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(raw)), Request: r}, nil
			}))
			snapshot, err := client.HotelRooms(context.Background(), query)
			if err != nil {
				t.Fatal(err)
			}
			if len(snapshot.Offers) != 2 {
				t.Fatalf("expected two source rate plans, got %d", len(snapshot.Offers))
			}
			sourceRate, ok := snapshot.Offers[1].Details["rate"].(map[string]any)
			if !ok {
				t.Fatal("source rate terms absent")
			}
			wantMeals := make([]any, len(tt.secondMeals))
			for i, meal := range tt.secondMeals {
				wantMeals[i] = meal
			}
			if !reflect.DeepEqual(sourceRate["includedMealTypes"], wantMeals) {
				t.Fatal("exposed meal types were lost in source normalization")
			}
			result, err := travelokacompare.Flexibility(snapshot, 10, 10)
			if err != nil {
				t.Fatal(err)
			}
			if result.PairCount != tt.pairs {
				t.Fatalf("different included meal types paired: got %d, want %d", result.PairCount, tt.pairs)
			}
		})
	}
}
