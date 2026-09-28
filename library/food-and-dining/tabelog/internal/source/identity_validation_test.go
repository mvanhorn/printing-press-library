package source_test

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/tabelog/internal/source"
)

func TestDetailRequiresIndependentSourceIdentity(t *testing.T) {
	const requested = "https://tabelog.com/en/tokyo/A1301/A130103/13294162/"
	body := string(fixture(t, "sushi-detail.html"))
	pattern := regexp.MustCompile(`(?s)<script[^>]*type="application/ld\+json"[^>]*>(.*?)</script>`)
	var original string
	var schema map[string]any
	for _, m := range pattern.FindAllStringSubmatch(body, -1) {
		var v map[string]any
		if json.Unmarshal([]byte(m[1]), &v) == nil && v["@type"] == "Restaurant" {
			original, schema = m[1], v
			break
		}
	}
	if original == "" {
		t.Fatal("source fixture lost Restaurant schema")
	}
	for _, tc := range []struct {
		name, id, url, canonical string
		wantError                bool
	}{
		{"full_id", requested, "", "", false},
		{"full_id_with_fragment", requested + "#restaurant", "", "", false},
		{"url_field", "", requested, "", false},
		{"canonical_relative", "#restaurant", "", "/en/tokyo/A1301/A130103/13294162/", false},
		{"no_independent_identity", "", "", "", true},
		{"fragment_only", "#restaurant", "", "", true},
		{"relative_self_id", "./", "", "", true},
		{"canonical_fragment", requested, "", "#restaurant", true},
		{"same_id_wrong_route", "https://tabelog.com/en/tokyo/A1301/A130101/13294162/", "", "", true},
		{"conflicting_url", requested, "https://tabelog.com/en/tokyo/A1301/A130101/13005012/", "", true},
		{"conflicting_canonical", requested, "", "https://tabelog.com/en/tokyo/A1301/A130101/13005012/", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := make(map[string]any, len(schema))
			for k, x := range schema {
				v[k] = x
			}
			delete(v, "@id")
			if tc.id != "" {
				v["@id"] = tc.id
			}
			if tc.url != "" {
				v["url"] = tc.url
			}
			encoded, err := json.Marshal(v)
			if err != nil {
				t.Fatal(err)
			}
			page := strings.Replace(body, original, string(encoded), 1)
			if tc.canonical != "" {
				page = strings.Replace(page, "<body>", `<head><link rel="canonical" href="`+tc.canonical+`"></head><body>`, 1)
			}
			r, err := source.ParseDetail([]byte(page), requested, time.Now())
			if tc.wantError {
				if err == nil || !strings.Contains(err.Error(), "identity") {
					t.Fatalf("unverified identity accepted or not explained: %+v, %v", r, err)
				}
				return
			}
			if err != nil || r.ID != "13294162" || r.Name != "Sushi Dokoro Isseki Sanchou" {
				t.Fatalf("valid source identity variant rejected: %+v, %v", r, err)
			}
		})
	}
}
