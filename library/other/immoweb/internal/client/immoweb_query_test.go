package client

import (
	"net/url"
	"testing"
)

func TestImmowebQueryKeepsCommas(t *testing.T) {
	v := url.Values{}
	v.Set("epcScores", "F,G")
	v.Set("postalCodes", "BE-1030,BE-1050")
	got := pathWithQueryValues("/en/search-results", v)
	if got != "/en/search-results?epcScores=F,G&postalCodes=BE-1030,BE-1050" {
		t.Fatalf("commas must stay literal for Immoweb: %s", got)
	}
}
