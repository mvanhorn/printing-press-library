package cli

import (
	"encoding/json"
	"reflect"
	"testing"
)

// Fixtures mirror real AppTweak responses: every payload sits under
// {"result": {"<app id>": ...}}.

func TestExtractKeywordRanksResultEnvelope(t *testing.T) {
	data := json.RawMessage(`{"result":{"324684580":{
		"music":{"rank":{"value":5,"fetch_performed":true}},
		"podcasts":{"rank":{"value":null,"fetch_performed":true}}}}}`)
	got := extractKeywordRanks(data)
	want := map[string]int{"music": 5} // null value = unranked, left out
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestExtractKeywordRankChangesResultEnvelope(t *testing.T) {
	data := json.RawMessage(`{"result":{"324684580":{"music":{"rank":[
		{"date":"2026-09-10","value":5},{"date":"2026-09-11","value":null},{"date":"2026-09-12","value":3}]}}}}`)
	got := extractKeywordRankChanges(data)
	want := []keywordRankChange{{Keyword: "music", RankStart: 5, RankEnd: 3, Delta: -2, Trend: "up"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestExtractPaidKeywordsResultEnvelope(t *testing.T) {
	data := json.RawMessage(`{"result":{"401626263":{"bids":[{"keyword":"airbnb","count":22},{"keyword":"webook","count":22}]}}}`)
	got := extractPaidKeywords(data)
	if !reflect.DeepEqual(got, []string{"airbnb", "webook"}) {
		t.Fatalf("got %v", got)
	}
}

func TestExtractReviewRatingsResultEnvelope(t *testing.T) {
	data := json.RawMessage(`{"result":{"324684580":{"reviews":[{"rating":1},{"rating":5},{"rating":4}]}}}`)
	got := extractReviewRatings(data)
	if !reflect.DeepEqual(got, []int{1, 5, 4}) {
		t.Fatalf("got %v", got)
	}
}

func TestExtractFirstMetadataFieldsResultEnvelope(t *testing.T) {
	data := json.RawMessage(`{"result":{"324684580":{"metadata":{"title":"Spotify","subtitle":"Music and Podcasts"}}}}`)
	got := extractFirstMetadataFields(data)
	if got["title"] != "Spotify" || got["subtitle"] != "Music and Podcasts" {
		t.Fatalf("got %v", got)
	}
}

func TestParseCreditsResponseResultEnvelope(t *testing.T) {
	data := []byte(`{"result":{"current_monthly_api_credits":24752,"total_monthly_api_credits":25000}}`)
	out := parseCreditsResponse(data)
	if out.CreditsRemaining == nil || *out.CreditsRemaining != 24752 {
		t.Fatalf("got %+v", out)
	}
}
