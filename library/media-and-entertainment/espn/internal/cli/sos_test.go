package cli

import "testing"

func TestExtractSOSWithDivisions(t *testing.T) {
	body := []byte(`{
		"children":[{
			"name":"AFC",
			"children":[{
				"name":"East",
				"standings":{"entries":[
					{"team":{"abbreviation":"BUF","displayName":"Buffalo"},"stats":[
						{"name":"strengthOfSchedule","value":0.55},
						{"name":"remainingStrengthOfSchedule","value":0.6}
					]},
					{"team":{"abbreviation":"MIA","displayName":"Miami"},"stats":[
						{"name":"strengthOfSchedule","value":0.5},
						{"name":"remainingStrengthOfSchedule","value":0.45}
					]}
				]}
			}]
		}]
	}`)
	got := extractSOS(body)
	if len(got) != 2 {
		t.Fatalf("want 2 entries, got %d", len(got))
	}
	abbrs := []string{got[0].TeamAbbr, got[1].TeamAbbr}
	want := map[string]bool{"BUF": true, "MIA": true}
	for _, a := range abbrs {
		if !want[a] {
			t.Errorf("unexpected team %q", a)
		}
	}
}

func TestExtractSOSFlatStandings(t *testing.T) {
	body := []byte(`{
		"children":[{
			"name":"League",
			"standings":{"entries":[
				{"team":{"abbreviation":"KC","displayName":"KC"},"stats":[
					{"name":"strengthOfSchedule","value":0.7}
				]}
			]}
		}]
	}`)
	got := extractSOS(body)
	if len(got) != 1 {
		t.Fatalf("want 1 entry, got %d", len(got))
	}
	if got[0].SOS != 0.7 {
		t.Errorf("SOS = %v, want 0.7", got[0].SOS)
	}
}

func TestExtractSOSNoData(t *testing.T) {
	body := []byte(`{"children":[]}`)
	got := extractSOS(body)
	if len(got) != 0 {
		t.Errorf("want empty result, got %d", len(got))
	}
}

func TestExtractSOSMalformed(t *testing.T) {
	body := []byte(`not json`)
	got := extractSOS(body)
	if got != nil {
		t.Errorf("malformed JSON should yield nil, got %v", got)
	}
}
