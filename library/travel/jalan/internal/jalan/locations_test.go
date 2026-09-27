package jalan

import "testing"

func TestJapanesePrefectureAliasesPreservePlaceNames(t *testing.T) {
	for alias, code := range map[string]string{
		"京都": "260000", "京都府": "260000",
		"東京": "130000", "東京都": "130000",
		"大阪": "270000", "大阪府": "270000",
		"北海道": "010000",
	} {
		t.Run(alias, func(t *testing.T) {
			location, err := resolveLocation(alias)
			if err != nil || location.AreaCode != code {
				t.Fatalf("%s resolved to %+v: %v", alias, location, err)
			}
			q := datedQuery()
			q.Destination = alias
			normalized, err := normalizeQuery(q, true, testNow)
			if err != nil || normalized.values().Get("kenCd") != code {
				t.Fatalf("%s query resolved incorrectly: %+v %v", alias, normalized, err)
			}
		})
	}
	if _, err := resolveLocation("京"); err == nil {
		t.Fatal("repeated suffix trimming retained a truncated Kyoto alias")
	}
}
