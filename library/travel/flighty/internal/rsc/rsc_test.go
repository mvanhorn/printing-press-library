package rsc

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestExtractChunks(t *testing.T) {
	tests := []struct {
		name    string
		html    string
		want    string
		wantErr bool
	}{
		{
			name: "single chunk with escaped quotes",
			html: `<html><body><script>self.__next_f.push([1,"2:[\"$\",\"div\",null,{\"iata\":\"DEN\"}]\n"])</script></body></html>`,
			want: "2:[\"$\",\"div\",null,{\"iata\":\"DEN\"}]\n",
		},
		{
			name: "multiple chunks concatenated in order",
			html: `<script>self.__next_f.push([1,"a:1\n"])</script>` +
				`<script>self.__next_f.push([1,"b:2"])</script>`,
			want: "a:1\nb:2",
		},
		{
			name:    "no chunks",
			html:    `<html><body>plain</body></html>`,
			wantErr: true,
		},
		{
			name:    "empty document",
			html:    ``,
			wantErr: true,
		},
		{
			name: "escaped backslash before quote survives",
			html: `<script>self.__next_f.push([1,"x:\\\"y\\\""])</script>`,
			want: `x:\"y\"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractChunks([]byte(tt.html))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFindObject(t *testing.T) {
	payload := `1f:[["$","div",null,{"children":["$","$L21",null,{"iata":"DEN","name":"Denver Intl.","nested":{"a":"b\"c{d}e"},"city":"Denver"}]}]]`
	obj, err := FindObject(payload, `{"iata":"DEN"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var parsed struct {
		IATA   string            `json:"iata"`
		Nested map[string]string `json:"nested"`
	}
	if err := json.Unmarshal(obj, &parsed); err != nil {
		t.Fatalf("fragment is not valid JSON: %v", err)
	}
	if parsed.IATA != "DEN" {
		t.Fatalf("iata = %q, want DEN", parsed.IATA)
	}
	if parsed.Nested["a"] != `b"c{d}e` {
		t.Fatalf("nested string with braces/quotes mishandled: %q", parsed.Nested["a"])
	}
}

func TestFindObjectMissingMarker(t *testing.T) {
	if _, err := FindObject(`2:["$","p",null,"hello"]`, `{"regions":`); err == nil {
		t.Fatal("expected error for missing marker")
	}
}

func TestFindObjectUnanchoredMarker(t *testing.T) {
	// A marker that does not include the opening token must error, not guess.
	if _, err := FindObject(`{"a":1}`, `"a":`); err == nil {
		t.Fatal("expected error for unanchored marker")
	}
}

func TestFindArray(t *testing.T) {
	payload := `["$","$L27",null,{"initialFlights":[{"id":"f1","flightNumber":"5072","status":[{"type":"text","text":"4h 1m Late"}]},{"id":"f2","flightNumber":"100","status":[]}]}]`
	arr, err := FindArray(payload, `"initialFlights":[`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var flights []struct {
		ID           string `json:"id"`
		FlightNumber string `json:"flightNumber"`
	}
	if err := json.Unmarshal(arr, &flights); err != nil {
		t.Fatalf("fragment is not valid JSON: %v", err)
	}
	if len(flights) != 2 || flights[0].FlightNumber != "5072" {
		t.Fatalf("unexpected flights: %+v", flights)
	}
}

func TestFindObjectUnbalanced(t *testing.T) {
	// Truncated object should error, not return a partial fragment.
	payload := `{"today":{"departurePerformance":{"onTime":86`
	if _, err := FindObject(payload, `{"today":`); err == nil {
		t.Fatal("expected error for unbalanced fragment")
	}
}

func TestFindBalancedIgnoresBracesInStrings(t *testing.T) {
	payload := `{"a":"}{","b":{"c":1}}`
	obj, err := FindObject(payload, `{`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(obj), `"c":1`) {
		t.Fatalf("fragment cut short by braces inside strings: %s", obj)
	}
	var parsed map[string]any
	if err := json.Unmarshal(obj, &parsed); err != nil {
		t.Fatalf("invalid fragment: %v", err)
	}
	if parsed["b"].(map[string]any)["c"].(float64) != 1 {
		t.Fatalf("unexpected parse: %+v", parsed)
	}
}

func TestFindObjectRegionMarkerEndsBrace(t *testing.T) {
	payload := `2:{"regionNames":["All"],"regions":{"All":{"airports":[{"slug":"manchester-man","iata":"MAN"}]},"Europe":{"airports":[{"slug":"hamburg-ham","iata":"HAM"}]}}}`
	obj, err := FindObject(payload, `"regions":{`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var parsed struct {
		All struct {
			Airports []struct {
				IATA string `json:"iata"`
			} `json:"airports"`
		} `json:"All"`
	}
	if err := json.Unmarshal(obj, &parsed); err != nil {
		t.Fatalf("invalid fragment: %v", err)
	}
	if len(parsed.All.Airports) != 1 || parsed.All.Airports[0].IATA != "MAN" {
		t.Fatalf("unexpected regions parse: %+v", parsed)
	}
}
