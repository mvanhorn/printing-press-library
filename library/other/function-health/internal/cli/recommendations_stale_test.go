// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestTrailingRoundsOutOfOptimal(t *testing.T) {
	// Series are oldest..newest, as loadAllResults / byQuest produce them.
	tests := []struct {
		name   string
		series []resultRow
		want   int
	}{
		{name: "latest in range -> 0", series: []resultRow{draw(200, 50, 150), draw(100, 50, 150)}, want: 0},
		{name: "latest out, prior in -> 1", series: []resultRow{draw(100, 50, 150), draw(200, 50, 150)}, want: 1},
		{name: "two trailing out -> 2", series: []resultRow{draw(100, 50, 150), draw(200, 50, 150), draw(220, 50, 150)}, want: 2},
		{name: "undefined latest breaks streak -> 0", series: []resultRow{draw(200, 50, 150), draw(99, 0, 0)}, want: 0},
		{name: "undefined mid breaks the count", series: []resultRow{draw(200, 50, 150), draw(99, 0, 0), draw(220, 50, 150)}, want: 1},
		{name: "empty series -> 0", series: nil, want: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := trailingRoundsOutOfOptimal(tc.series); got != tc.want {
				t.Errorf("trailingRoundsOutOfOptimal = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestQuestCodeString(t *testing.T) {
	// The live payload encodes Biomarkers entries as either strings or numbers.
	cases := map[string]string{
		`"86009431"`:   "86009431",
		`86009431`:     "86009431",
		` "25024000" `: "25024000",
	}
	for in, want := range cases {
		if got := questCodeString(json.RawMessage(in)); got != want {
			t.Errorf("questCodeString(%s) = %q, want %q", in, got, want)
		}
	}
}

func TestRecEnvelopeParsing(t *testing.T) {
	// Mirrors the real /recommendations shape: nested category groups, each
	// inner item carrying a name and Quest-code Biomarkers (string or number).
	raw := []byte(`{
		"type": "recommendations",
		"recommendations": [
			{"categoryName":"supplements","displayName":"Supplements","recommendations":[
				{"name":"Omega-3 fatty acids","Biomarkers":["86009431", 86002760]}
			]},
			{"categoryName":"selfcares","displayName":"Self care","recommendations":[]}
		]
	}`)
	var env recEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(env.Recommendations) != 2 {
		t.Fatalf("groups = %d, want 2", len(env.Recommendations))
	}
	g := env.Recommendations[0]
	if g.DisplayName != "Supplements" || len(g.Items) != 1 {
		t.Fatalf("unexpected first group: %+v", g)
	}
	it := g.Items[0]
	if it.Name != "Omega-3 fatty acids" || len(it.Biomarkers) != 2 {
		t.Fatalf("unexpected item: %+v", it)
	}
	if got := questCodeString(it.Biomarkers[0]); got != "86009431" {
		t.Errorf("first code = %q, want 86009431", got)
	}
	if got := questCodeString(it.Biomarkers[1]); got != "86002760" {
		t.Errorf("numeric code = %q, want 86002760", got)
	}
}

func TestDedupeStrings(t *testing.T) {
	got := dedupeStrings([]string{"b", "a", "b", "c", "a"})
	want := []string{"b", "a", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("dedupeStrings = %v, want %v", got, want)
	}
}
