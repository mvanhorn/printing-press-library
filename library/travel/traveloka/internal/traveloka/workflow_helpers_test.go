package traveloka

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestSimulatedCoreCalendarAndIntegerHelpers(t *testing.T) {
	for _, tt := range []struct {
		name  string
		input any
		want  string
	}{{"ISO date", "2028-02-29", "2028-02-29"}, {"object date", map[string]any{"year": "2028", "month": "2", "day": "29"}, "2028-02-29"}, {"invalid leap day", map[string]any{"year": "2027", "month": "2", "day": "29"}, ""}, {"missing date", nil, ""}, {"object midnight", map[string]any{"hour": "0", "minute": "0"}, "00:00"}, {"ISO time", "23:59", "23:59"}, {"seconds", "23:59:01", "23:59:01"}, {"invalid time", map[string]any{"hour": "24", "minute": "0"}, ""}, {"missing time", nil, ""}} {
		t.Run(tt.name, func(t *testing.T) {
			got := sourceDate(tt.input)
			if tt.name == "object midnight" || tt.name == "ISO time" || tt.name == "seconds" || tt.name == "invalid time" || tt.name == "missing time" {
				got = sourceTime(tt.input)
			}
			if got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
	for _, tt := range []struct {
		v    any
		want *int
	}{{"0", intPtrCore(0)}, {json.Number("-420"), intPtrCore(-420)}, {nil, nil}, {"1.5", nil}, {float64(480), nil}, {"", nil}} {
		got := sourceInt(tt.v)
		if (got == nil) != (tt.want == nil) || (got != nil && *got != *tt.want) {
			t.Fatal("integer absence/zero/source offset lost")
		}
	}
}
func intPtrCore(n int) *int { return &n }
func TestSimulatedCoreMoneyPriceAbsenceZeroAndWrongCurrency(t *testing.T) {
	for _, tt := range []struct {
		name    string
		v       any
		amount  string
		unknown bool
		code    string
	}{{"known zero", coreDisplay("0"), "0.00", false, ""}, {"missing", nil, "", true, ""}, {"source null", map[string]any{"currencyValue": map[string]any{"nullOrEmpty": true}, "numOfDecimalPoint": "2"}, "", true, ""}, {"fractional integer", coreDisplay("1.25"), "", false, "MALFORMED_RESPONSE"}, {"wrong currency", map[string]any{"currencyValue": map[string]any{"currency": "USD", "amount": "1"}, "numOfDecimalPoint": "2"}, "", false, "CURRENCY_MISMATCH"}, {"unknown currency and precision", map[string]any{"amount": "123"}, "", false, ""}} {
		t.Run(tt.name, func(t *testing.T) {
			m, e := displayMoney(tt.v, "SGD")
			if tt.code != "" {
				var ae *APIError
				if !errors.As(e, &ae) || ae.Code != tt.code {
					t.Fatalf("wrong error %v", e)
				}
				return
			}
			if e != nil || (m == nil) != tt.unknown {
				t.Fatalf("wrong unknown money: %v", e)
			}
			if m != nil && m.Amount != tt.amount {
				t.Fatal("money rounded or zero lost")
			}
			if tt.name == "unknown currency and precision" && (m.Currency != "" || m.Decimals != nil || m.MinorUnits != "123") {
				t.Fatal("unknown scale/currency guessed")
			}
		})
	}
	for _, tt := range []struct {
		name                 string
		input                any
		wantTotal, wantNight string
	}{{"source totals", coreFinalPrice("47181", "11795"), "471.81", "117.95"}, {"zero", coreFinalPrice("0", "0"), "0.00", "0.00"}, {"missing", nil, "", ""}, {"nightly only", map[string]any{"perRoomPerNightDisplay": map[string]any{"inclusiveFinalPrice": coreMoney("11795"), "numOfDecimalPoint": "2"}}, "", "117.95"}} {
		t.Run(tt.name, func(t *testing.T) {
			p, e := hotelFinalPrice(tt.input, "SGD")
			if e != nil {
				t.Fatal(e)
			}
			total, night := "", ""
			if p.Total != nil {
				total = p.Total.Amount
			}
			if p.PerRoomPerNight != nil {
				night = p.PerRoomPerNight.Amount
			}
			if total != tt.wantTotal || night != tt.wantNight {
				t.Fatal("stay total was derived or source price changed")
			}
		})
	}
}
func TestSimulatedCoreRelevanceAndCapturedNames(t *testing.T) {
	for _, tt := range []struct {
		query  string
		values []any
		want   bool
	}{{"SIN", []any{"SIN", "Changi"}, true}, {"Singapore", []any{"Changi International Airport", "Singapore"}, true}, {"New York", []any{"New York City"}, true}, {"Singapore", []any{"Bali", "DPS"}, false}, {"", []any{"Singapore"}, false}, {"ZZ", []any{nil, "Singapore"}, false}} {
		if got := locationRelevant(tt.query, tt.values...); got != tt.want {
			t.Fatal("resolver relevance includes unrelated/empty suggestions")
		}
	}
	for _, tt := range []struct{ raw, typ, id, want string }{{"https://www.traveloka.com/en-sg/hotel/detail?spec=06-01-2027.08-01-2027.2.2.HOTEL.9.Source+Name.3", "HOTEL", "9", "Source Name"}, {"https://www.traveloka.com/en-sg/hotel/detail?spec=06-01-2027.08-01-2027.2.2.HOTEL.9.Source.Name.3", "HOTEL", "9", "Source.Name"}, {"https://www.traveloka.com/en-sg/hotel/detail?spec=06-01-2027.08-01-2027.2.2.HOTEL.9", "HOTEL", "9", ""}, {"https://www.traveloka.com/en-sg/hotel/detail?spec=06-01-2027.08-01-2027.2.2.HOTEL.9.Wrong.3", "HOTEL", "8", ""}, {"https://evil.example/hotel/detail?spec=06-01-2027.08-01-2027.2.2.HOTEL.9.Wrong.3", "HOTEL", "9", ""}} {
		if got := capturedHotelName(tt.raw, tt.typ, tt.id); got != tt.want {
			t.Fatal("captured source name was guessed or wrong ID accepted")
		}
	}
}
func TestSimulatedCoreSourceDelayIsHonoredAndBounded(t *testing.T) {
	start := time.Now()
	if e := waitSourceRefresh(context.Background(), map[string]any{"refreshDelayMillisecond": "15"}); e != nil || time.Since(start) < 12*time.Millisecond {
		t.Fatal("source refresh delay ignored")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if e := waitSourceRefresh(ctx, map[string]any{"refreshDelay": "1"}); !errors.Is(e, context.Canceled) {
		t.Fatal("source wait ignores canceled context")
	}
	var ae *APIError
	if e := waitSourceRefresh(context.Background(), map[string]any{"refreshDelay": "31"}); !errors.As(e, &ae) || ae.Code != "INCOMPLETE_SEARCH" {
		t.Fatal("unbounded source refresh delay accepted")
	}
}
func TestSimulatedCoreSourceEnvelopeAndKnownUnknownPolicy(t *testing.T) {
	for _, tt := range []struct {
		response map[string]any
		code     string
	}{{map[string]any{"data": map[string]any{"status": "SUCCESS"}}, ""}, {map[string]any{"data": map[string]any{}, "error": false}, ""}, {map[string]any{"data": map[string]any{}, "error": map[string]any{"code": "denied"}}, "UPSTREAM_ERROR"}, {map[string]any{"data": map[string]any{"success": false}}, "UPSTREAM_ERROR"}, {map[string]any{"data": nil}, "MALFORMED_RESPONSE"}} {
		_, e := responseData(tt.response)
		if tt.code == "" {
			if e != nil {
				t.Fatal(e)
			}
		} else {
			var ae *APIError
			if !errors.As(e, &ae) || ae.Code != tt.code {
				t.Fatal("source error/shape collapsed")
			}
		}
	}
	trueValue, falseValue := true, false
	for _, tt := range []struct {
		a, b  *bool
		first bool
		want  *bool
	}{{nil, &trueValue, true, &trueValue}, {&trueValue, &trueValue, false, &trueValue}, {&trueValue, &falseValue, false, &falseValue}, {nil, &falseValue, false, &falseValue}, {&trueValue, nil, false, nil}, {nil, nil, false, nil}} {
		got := combinePolicy(tt.a, tt.b, tt.first)
		if (got == nil) != (tt.want == nil) || (got != nil && *got != *tt.want) {
			t.Fatal("fare policy unknown or explicit false lost")
		}
	}
	l, e := normalizeFlightLeg(map[string]any{"connectingFlightRoutes": []any{map[string]any{"segments": []any{map[string]any{"departureAirport": "SIN", "arrivalAirport": "CGK"}}}}})
	if e != nil || len(l.Segments) != 1 || l.Segments[0].DurationMinutes != nil || l.Segments[0].DepartureUTCOffsetMinutes != nil || l.Segments[0].CheckedBaggage != nil || l.Segments[0].OperatingAirline != "" {
		t.Fatal("unknown segment attributes inferred")
	}
	if _, e = normalizeFlightLeg(map[string]any{}); e == nil {
		t.Fatal("segment-less source candidate accepted")
	}
}
