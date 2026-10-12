package cli

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestTrialListKeysAlwaysPresentInJSON guards the list half of the Trial
// serialization contract: conditions, interventions, countries and
// secondary_ids are always present and marshal as [] (never null, never
// absent) so a consumer can read and iterate them on every row. The fixture
// posts none of the four, and the Trial goes through normalizeStudy because a
// struct literal would prove nothing about the constructor.
func TestTrialListKeysAlwaysPresentInJSON(t *testing.T) {
	raw := marshalToRawMap(t, normalizeUnphasedStudy(t))

	for _, key := range []string{"conditions", "interventions", "countries", "secondary_ids"} {
		key := key
		t.Run(key, func(t *testing.T) {
			v, ok := raw[key]
			if !ok {
				t.Fatalf("Trial marshaled without a %q key: %v", key, keysOf(raw))
			}
			if got := string(v); got != "[]" {
				t.Errorf("Trial %s = %s, want [] (never null, never absent)", key, got)
			}
		})
	}
}

// TestTrialListKeysKeepValues proves the always-present change leaves posted
// values, their order and the country dedup untouched.
func TestTrialListKeysKeepValues(t *testing.T) {
	const study = `{"protocolSection":{
		"identificationModule":{"nctId":"NCT00000001","briefTitle":"A study",
			"secondaryIdInfos":[{"id":"2020-000001-11","type":"OTHER","domain":"EU CTR"}]},
		"statusModule":{"overallStatus":"RECRUITING"},
		"designModule":{},
		"conditionsModule":{"conditions":["Diabetes","Obesity"]},
		"armsInterventionsModule":{"interventions":[{"name":"Metformin","type":"DRUG"},{"name":"Placebo","type":"DRUG"}]},
		"contactsLocationsModule":{"locations":[{"country":"Germany"},{"country":"France"},{"country":"Germany"}]}
	}}`
	trial, ok := normalizeStudy(json.RawMessage(study))
	if !ok {
		t.Fatalf("normalizeStudy rejected the fixture")
	}
	b, err := json.Marshal(trial)
	if err != nil {
		t.Fatalf("json.Marshal error = %v", err)
	}
	var got struct {
		Conditions    []string      `json:"conditions"`
		Interventions []string      `json:"interventions"`
		Countries     []string      `json:"countries"`
		SecondaryIDs  []SecondaryID `json:"secondary_ids"`
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("json.Unmarshal error = %v (marshaled: %s)", err, b)
	}
	want := struct {
		Conditions    []string
		Interventions []string
		Countries     []string
		SecondaryIDs  []SecondaryID
	}{
		[]string{"Diabetes", "Obesity"},
		[]string{"Metformin", "Placebo"},
		[]string{"Germany", "France"},
		[]SecondaryID{{ID: "2020-000001-11", Type: "OTHER", Domain: "EU CTR"}},
	}
	if !reflect.DeepEqual(got.Conditions, want.Conditions) {
		t.Errorf("conditions = %v, want %v", got.Conditions, want.Conditions)
	}
	if !reflect.DeepEqual(got.Interventions, want.Interventions) {
		t.Errorf("interventions = %v, want %v", got.Interventions, want.Interventions)
	}
	if !reflect.DeepEqual(got.Countries, want.Countries) {
		t.Errorf("countries = %v, want %v (duplicate Germany must stay deduped)", got.Countries, want.Countries)
	}
	if !reflect.DeepEqual(got.SecondaryIDs, want.SecondaryIDs) {
		t.Errorf("secondary_ids = %v, want %v", got.SecondaryIDs, want.SecondaryIDs)
	}
}
