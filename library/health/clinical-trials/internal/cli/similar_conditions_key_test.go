package cli

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestSimilarMatchConditionsKeyAlwaysPresent guards the similar output rows:
// "conditions" is always present and marshals as [] (never null, never absent),
// the same contract as Trial.Phases and the Trial list keys. The first case
// goes through normalizeStudy because a struct literal would prove nothing
// about the Trial the command actually ranks; the second feeds a nil slice
// straight to the constructor.
func TestSimilarMatchConditionsKeyAlwaysPresent(t *testing.T) {
	cases := []struct {
		name  string
		trial Trial
	}{
		{name: "from normalizeStudy", trial: normalizeUnphasedStudy(t)},
		{name: "from nil Conditions", trial: Trial{NCTID: "NCT00000000"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			raw := marshalToRawMap(t, newSimilarMatch(tc.trial, 0))
			v, ok := raw["conditions"]
			if !ok {
				t.Fatalf("similarMatch marshaled without a \"conditions\" key: %v", keysOf(raw))
			}
			if got := string(v); got != "[]" {
				t.Errorf("similarMatch conditions = %s, want [] (never null, never absent)", got)
			}
		})
	}
}

// TestSimilarMatchKeepsValues proves the always-present change leaves every
// posted field and the condition order untouched.
func TestSimilarMatchKeepsValues(t *testing.T) {
	const study = `{"protocolSection":{
		"identificationModule":{"nctId":"NCT00000001","briefTitle":"A study"},
		"statusModule":{"overallStatus":"RECRUITING"},
		"designModule":{"phases":["PHASE2"]},
		"conditionsModule":{"conditions":["Diabetes","Obesity"]}
	}}`
	trial, ok := normalizeStudy(json.RawMessage(study))
	if !ok {
		t.Fatalf("normalizeStudy rejected the fixture")
	}
	b, err := json.Marshal(newSimilarMatch(trial, 7))
	if err != nil {
		t.Fatalf("json.Marshal error = %v", err)
	}
	var got struct {
		ID         string   `json:"id"`
		Title      string   `json:"title"`
		Status     string   `json:"status"`
		Phase      string   `json:"phase"`
		Conditions []string `json:"conditions"`
		MatchScore int      `json:"match_score"`
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("json.Unmarshal error = %v (marshaled: %s)", err, b)
	}
	if got.ID != "NCT00000001" {
		t.Errorf("id = %q, want NCT00000001", got.ID)
	}
	if got.Title != "A study" {
		t.Errorf("title = %q, want %q", got.Title, "A study")
	}
	if got.Status != "RECRUITING" {
		t.Errorf("status = %q, want RECRUITING", got.Status)
	}
	if got.Phase != trial.Phase || got.Phase == "" {
		t.Errorf("phase = %q, want %q (non-empty, from the Trial)", got.Phase, trial.Phase)
	}
	if got.MatchScore != 7 {
		t.Errorf("match_score = %d, want 7", got.MatchScore)
	}
	if want := []string{"Diabetes", "Obesity"}; !reflect.DeepEqual(got.Conditions, want) {
		t.Errorf("conditions = %v, want %v (same order)", got.Conditions, want)
	}
}
