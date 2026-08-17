package client

import (
	"encoding/json"
	"testing"
)

func TestSanitizeJSONResponseRedactsCoordinatesAndPrivateFields(t *testing.T) {
	body := sanitizeJSONResponse([]byte(`{"results":[{"location":"1,2","geojson":{"coordinates":[2,1]},"private_location":"3,4","private_sensitive":"secret","place_geometry":{"coordinates":[2,1]},"geoprivacy":"obscured","obscured":true,"taxon":{"name":"Aves"}}]}`))
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal sanitized response: %v", err)
	}
	observation := got["results"].([]any)[0].(map[string]any)
	for _, key := range []string{"location", "geojson", "private_location", "private_sensitive", "place_geometry"} {
		if _, ok := observation[key]; ok {
			t.Fatalf("coordinate field %q was not redacted: %#v", key, observation)
		}
	}
	if observation["geoprivacy"] != "obscured" || observation["obscured"] != true {
		t.Fatalf("privacy state was not preserved: %#v", observation)
	}
}
