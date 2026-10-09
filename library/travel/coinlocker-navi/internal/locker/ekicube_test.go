// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package locker

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseEkicube(t *testing.T) {
	p, err := ParseEkicube(fixture(t, "ekicube_en.json"), "en")
	if err != nil {
		t.Fatal(err)
	}
	if p.Total == 0 || len(p.Locations) == 0 {
		t.Fatalf("empty page: %+v", p)
	}
	for _, l := range p.Locations {
		if l.Name.EN == nil || l.Name.JA != nil {
			t.Fatalf("en page must fill EN only: %+v", l.Name)
		}
		if l.Gate == nil || l.GateSource != "multiecube" || l.Caveat == "" {
			t.Fatalf("gate/caveat missing: %+v", l)
		}
		if len(l.Boxes) == 0 {
			t.Fatalf("no boxes for %d", l.ID)
		}
		for _, b := range l.Boxes {
			if b.Key == "from_at" || b.Key == "end_at" {
				t.Fatalf("date keys leaked into boxes")
			}
			if b.UsageFeeYen == nil || b.ReservationFeeYen == nil || *b.ReservationFeeYen != 500 {
				t.Fatalf("fees: %+v", b)
			}
		}
		if l.Hours.Kind != "first_to_last_train" {
			t.Fatalf("hours kind = %s", l.Hours.Kind)
		}
	}
}

func TestParseEkicubeBadJSON(t *testing.T) {
	if _, err := ParseEkicube([]byte("<html>"), "ja"); err == nil {
		t.Fatal("want error")
	}
}

func TestEkicubeMergesEnglishByCoordinates(t *testing.T) {
	ja := `{"total":1,"page_no":1,"page_total":1,"locations":[{"id":5,"latitude":"35.68","longitude":"139.76","inside_ticket_gate":true,"outside_ticket_gate":false,"attributes":{"display_name":"中央","distance":null,"business_hours":{"summary":"初電～終電"}},"base":{"attributes":{"display_name":"東京駅"}},"area":{"attributes":{"display_name":"改札内"}},"box_availability":{"l":{"by_service":[{"service_type":1,"num_empty":0,"price":{"basic":500,"std":900}}]}}}]}`
	en := strings.NewReplacer("中央", "Center", "東京駅", "Tokyo Sta.", "改札内", "Inside").Replace(ja)
	var sawCoordEN, sawKeywordEN bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if !strings.Contains(r.URL.RawQuery, "service_type=1,3") {
			w.WriteHeader(400)
			return
		}
		if q.Get("lang") == "en" {
			if q.Get("q") != "" {
				sawKeywordEN = true
				fmt.Fprint(w, `{"total":0,"page_no":1,"page_total":0,"locations":[]}`)
				return
			}
			sawCoordEN = q.Get("latitude") != ""
			fmt.Fprint(w, en)
			return
		}
		fmt.Fprint(w, ja)
	}))
	defer srv.Close()
	out, total, more, err := testClient(srv).Ekicube(context.Background(), EkicubeQuery{Keyword: "東京", Date: "2026-10-09", MaxPages: 2})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || more || len(out) != 1 {
		t.Fatalf("total=%d more=%v n=%d", total, more, len(out))
	}
	if sawKeywordEN || !sawCoordEN {
		t.Fatalf("non-ASCII keyword must fill English by coordinates (kw=%v coord=%v)", sawKeywordEN, sawCoordEN)
	}
	if out[0].Name.EN == nil || *out[0].Name.EN != "Center" || *out[0].Name.JA != "中央" || *out[0].Gate != "inside" {
		t.Fatalf("merged = %+v", out[0])
	}
	if out[0].Boxes[0].ReservableEmpty != 0 || *out[0].Boxes[0].SizeClass != "L" {
		t.Fatalf("box = %+v", out[0].Boxes[0])
	}
}

func TestEkicubeDoneStopsPaging(t *testing.T) {
	page := `{"total":60,"page_no":1,"page_total":3,"locations":[{"id":%d,"latitude":"35.68","longitude":"139.76","inside_ticket_gate":false,"outside_ticket_gate":true,"attributes":{"display_name":"x"},"box_availability":{}}]}`
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("lang") != "ja" {
			t.Errorf("JapaneseOnly must not request lang=%s", r.URL.Query().Get("lang"))
		}
		fmt.Fprintf(w, page, calls)
	}))
	defer srv.Close()
	lat, lon := 35.68, 139.76
	var seen []float64
	out, _, more, err := testClient(srv).Ekicube(context.Background(), EkicubeQuery{
		Lat: &lat, Lon: &lon, RadiusM: 500, Date: "2026-10-09", MaxPages: 3, JapaneseOnly: true,
		Done: func(read []EkicubeLocation, covered float64) bool {
			seen = append(seen, covered)
			return len(read) >= 2
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(out) != 2 || !more || len(seen) != 2 {
		t.Fatalf("calls=%d out=%d more=%v seen=%v", calls, len(out), more, seen)
	}
	if out[0].Name.EN != nil {
		t.Fatal("JapaneseOnly must leave name.en null")
	}
}

func TestEkicubeStopAfter(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("lang") == "en" {
			fmt.Fprint(w, `{"total":60,"page_no":1,"page_total":3,"locations":[]}`)
			return
		}
		fmt.Fprintf(w, `{"total":60,"page_no":%d,"page_total":3,"locations":[{"id":%d,"latitude":"35.68","longitude":"139.76","attributes":{"display_name":"x"},"box_availability":{}},{"id":%d,"latitude":"35.68","longitude":"139.76","attributes":{"display_name":"y"},"box_availability":{}}]}`, calls, calls*10, calls*10+1)
	}))
	defer srv.Close()
	out, total, more, err := testClient(srv).Ekicube(context.Background(), EkicubeQuery{Keyword: "shinjuku", Date: "2026-10-09", MaxPages: 3, StopAfter: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || total != 60 || !more || calls != 2 {
		t.Fatalf("out=%d total=%d more=%v calls=%d", len(out), total, more, calls)
	}
}

func TestEkicubeStopAfterNamesOnlyReturnedRows(t *testing.T) {
	loc := `{"id":%d,"latitude":"%s","longitude":"%s","attributes":{"display_name":"%s"},"box_availability":{}}`
	ja := `{"total":4,"page_no":1,"page_total":1,"locations":[` +
		fmt.Sprintf(loc, 1, "35.68", "139.76", "北") + "," + fmt.Sprintf(loc, 2, "35.6801", "139.7601", "南") + "," +
		fmt.Sprintf(loc, 3, "35.70", "139.80", "遠い") + "," + fmt.Sprintf(loc, 4, "35.71", "139.81", "遠い2") + `]}`
	en := `{"total":2,"page_no":1,"page_total":3,"locations":[` +
		fmt.Sprintf(loc, 1, "35.68", "139.76", "North") + "," + fmt.Sprintf(loc, 2, "35.6801", "139.7601", "South") + `]}`
	var enCalls int
	var enRadius string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("lang") == "en" {
			enCalls++
			enRadius = r.URL.Query().Get("distance_max")
			fmt.Fprint(w, en)
			return
		}
		fmt.Fprint(w, ja)
	}))
	defer srv.Close()
	out, _, _, err := testClient(srv).Ekicube(context.Background(), EkicubeQuery{Keyword: "東京", Date: "2026-10-09", MaxPages: 3, StopAfter: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 4 || enCalls != 1 || enRadius != "57" {
		t.Fatalf("out=%d enCalls=%d distance_max=%s", len(out), enCalls, enRadius)
	}
	if out[0].Name.EN == nil || *out[0].Name.EN != "North" || out[1].Name.EN == nil || *out[1].Name.EN != "South" {
		t.Fatalf("returned rows need English names: %+v %+v", out[0].Name, out[1].Name)
	}
	if out[2].Name.EN != nil {
		t.Fatalf("rows past StopAfter are not named: %+v", out[2].Name)
	}
}

func TestParseEkicubeSkipsBadCoordinates(t *testing.T) {
	body := `{"total":2,"page_no":1,"page_total":1,"locations":[{"id":1,"latitude":"","longitude":"139.7","attributes":{"display_name":"x"},"box_availability":{}},{"id":2,"latitude":"35.6","longitude":"139.7","attributes":{"display_name":"y"},"box_availability":{}}]}`
	p, err := ParseEkicube([]byte(body), "ja")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Locations) != 1 || p.Locations[0].ID != 2 {
		t.Fatalf("locations = %+v", p.Locations)
	}
}
