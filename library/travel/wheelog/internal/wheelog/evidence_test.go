package wheelog

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"
)

func pointer(n int) *int    { return &n }
func text(s string) *string { return &s }

func TestCategoryAndQuestionContracts(t *testing.T) {
	for _, tc := range []struct {
		id       int
		category string
		known    bool
	}{{102, "toilet", true}, {201, "elevator", true}, {409, "shop", true}, {710, "hotel", true}, {1008, "favorite", true}, {0, "", false}, {110, "", false}, {309, "", false}} {
		t.Run(tc.category+string(rune(tc.id)), func(t *testing.T) {
			got, known := QuestionCategory(tc.id)
			if got != tc.category || known != tc.known {
				t.Fatalf("got %q,%v", got, known)
			}
		})
	}
	if len(Categories()) != 10 || ValidCategory("restaurant") || !ValidCategory("barrierSpot") {
		t.Fatal("source categories changed")
	}
}

func TestAggregateCountsAndScalarShapes(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  Count
		bad   bool
	}{{`0`, 0, false}, {`"3"`, 3, false}, {`-1`, 0, true}, {`1.2`, 0, true}, {`1000001`, 0, true}, {`"unknown"`, 0, true}} {
		var got Count
		err := json.Unmarshal([]byte(tc.input), &got)
		if (err != nil) != tc.bad || (!tc.bad && got != tc.want) {
			t.Fatalf("%s got %d,%v", tc.input, got, err)
		}
	}
	for _, input := range []string{`35.7`, `"35.7"`} {
		var got Scalar
		if err := json.Unmarshal([]byte(input), &got); err != nil || got != "35.7" {
			t.Fatalf("scalar %s = %q,%v", input, got, err)
		}
	}
}

func TestCardCoordinatesAreExplicitlyUnknown(t *testing.T) {
	for _, input := range []string{`""`, `null`, `[]`, `0`} {
		var raw RawSpot
		body := `{"id":166345,"name":"Public restroom","spotCategory":"toilet","location":` + input + `}`
		if err := json.Unmarshal([]byte(body), &raw); err != nil {
			t.Fatal(err)
		}
		spot, err := Normalize(raw, time.Now())
		if err != nil || spot.Location != nil {
			t.Fatalf("omitted coordinates became measured location: %+v,%v", spot.Location, err)
		}
	}
}

func TestSanitizeEnvelopeRemovesPrivateMaterial(t *testing.T) {
	body := `{"result":[{"resultCode":0}],"content":{"spot":{"id":166345,"name":"成田空港 多機能トイレ","created":"2026-10-02 04:52:05.173565","updated":"","address":"public facility","location":{"lat":"35.7","lng":"140.3","private":"PRIVATE"},"spotCategory":{"id":1,"category":"toilet","user":{"email":"PRIVATE"},"questionList":[{"id":102,"question":"Turning space?","totalGood":1,"totalBad":0,"contributor":"PRIVATE"}]},"outline":"PRIVATE","user":{"email":"private@example.test","dateOfBirth":"PRIVATE"},"photoList":[{"nickname":"PRIVATE"}],"commentList":[{"text":"PRIVATE"}]}}}`
	for _, tc := range []struct {
		body   string
		detail bool
		bad    bool
	}{{body, true, false}, {`{"result":[{"resultCode":"BAD"}],"content":{}}`, true, true}, {`{"content":{}}`, false, true}, {`<html>wall</html>`, false, true}, {`{"result":[{"resultCode":0}],"content":{"timelineList":[],"request":{"word":"none","type":"spot"}}}`, false, false}} {
		out, err := SanitizeEnvelope([]byte(tc.body), tc.detail)
		if (err != nil) != tc.bad {
			t.Fatalf("err=%v", err)
		}
		if err == nil && (strings.Contains(string(out), "PRIVATE") || strings.Contains(string(out), "private@example") || strings.Contains(string(out), `"user"`)) {
			t.Fatalf("private field survived: %s", out)
		}
		if !tc.detail && !tc.bad && !strings.Contains(string(out), `"timelineList":[]`) {
			t.Fatalf("empty results lost: %s", out)
		}
	}
	var envelope Envelope
	if err := json.Unmarshal([]byte(body), &envelope); err != nil {
		t.Fatal(err)
	}
	spot, err := Normalize(*envelope.Content.Spot, time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if spot.Name != "成田空港 多機能トイレ" || spot.Updated != nil || *spot.Created != "2026-10-02T04:52:05.173565Z" || spot.Questions[0].ObservedAt != nil {
		t.Fatalf("lost source semantics: %+v", spot)
	}
}

func TestReportStatesAndRequestedQuestionApplicability(t *testing.T) {
	for _, tc := range []struct {
		good, bad *int
		state     string
	}{{pointer(0), pointer(0), "unreported"}, {pointer(1), pointer(0), "reported_affirmative"}, {pointer(0), pointer(2), "reported_negative"}, {pointer(1), pointer(1), "conflicting"}, {pointer(1), nil, "counts_missing"}, {nil, nil, "counts_missing"}} {
		if got := ReportState(tc.good, tc.bad); got != tc.state {
			t.Fatalf("got %s", got)
		}
		spot := Spot{Category: "toilet", DetailStatus: "checked", Questions: []Question{{ID: 102, State: tc.state, Positive: tc.good, Negative: tc.bad}}}
		rows := Evaluate(spot, []int{102, 103, 201})
		if rows[0].State != tc.state || rows[1].Reason != "question_missing" || rows[2].State != "inapplicable" {
			t.Fatalf("wrong matrix %+v", rows)
		}
	}
	spot := Spot{Category: "toilet", DetailStatus: "unavailable"}
	if Evaluate(spot, []int{102})[0].State != "unavailable" {
		t.Fatal("fetch failure became an unanswered question")
	}
}

func TestSourceAndRetrievalClocksRemainSeparate(t *testing.T) {
	now := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		updated *string
		want    string
	}{{nil, "source_update_date_unknown"}, {text("2025-01-01T00:00:00Z"), "source_record_update_old"}, {text("2027-01-01T00:00:00Z"), "source_update_date_in_future"}} {
		spot := Spot{Updated: tc.updated, Created: text(now.Format(time.RFC3339)), ObservedAt: now.Format(time.RFC3339), DetailStatus: "checked", Questions: []Question{}}
		if reasons := RecheckReasons(spot, nil, 180*24*time.Hour, now); len(reasons) != 1 || reasons[0] != tc.want {
			t.Fatalf("created/retrieval hid update uncertainty: %v", reasons)
		}
	}
	if Age(nil, now) != nil || Age(text("invalid"), now) != nil {
		t.Fatal("unknown time became zero age")
	}
}

func TestCoordinatesAndStraightLineDistance(t *testing.T) {
	for _, tc := range []struct {
		lat, lon float64
		valid    bool
	}{{0, 0, true}, {90, 180, true}, {91, 0, false}, {0, 181, false}, {math.NaN(), 0, false}, {0, math.Inf(1), false}} {
		if ValidCoordinate(tc.lat, tc.lon) != tc.valid {
			t.Fatalf("coordinate %+v", tc)
		}
	}
	for _, tc := range []struct {
		a, b Coordinate
		want float64
	}{{Coordinate{0, 0}, Coordinate{0, 0}, 0}, {Coordinate{0, 0}, Coordinate{0, 1}, 111195.08}, {Coordinate{0, 179.9}, Coordinate{0, -179.9}, 22239.016}} {
		if got := Distance(tc.a, tc.b); math.Abs(got-tc.want) > 1 {
			t.Fatalf("distance %v", got)
		}
	}
}

func TestObservationChangesNeverInventPhysicalChanges(t *testing.T) {
	base := Spot{ID: 1, Name: "public place", Category: "toilet", Questions: []Question{{ID: 102, State: "unreported", Positive: pointer(0), Negative: pointer(0)}}}
	for _, tc := range []struct {
		name   string
		change func(*Spot)
		want   string
	}{{"retrieval only", func(s *Spot) { s.ObservedAt = "later" }, ""}, {"count", func(s *Spot) {
		s.Questions = append([]Question(nil), s.Questions...)
		s.Questions[0].Positive = pointer(1)
		s.Questions[0].State = "reported_affirmative"
	}, "question_102"}, {"date", func(s *Spot) { s.Updated = text("2026-10-03T00:00:00Z") }, "record_updated_at"}} {
		next := base
		tc.change(&next)
		changes := Changes(base, next)
		if tc.want == "" {
			if len(changes) != 0 {
				t.Fatalf("fabricated drift %v", changes)
			}
		} else if len(changes) != 1 || changes[0].Field != tc.want {
			t.Fatalf("%s: %v", tc.name, changes)
		}
	}
}
