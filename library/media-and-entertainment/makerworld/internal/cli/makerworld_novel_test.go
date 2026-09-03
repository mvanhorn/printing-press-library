// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestParseDesignRow(t *testing.T) {
	raw := []byte(`{"id":2865269,"title":"Egg","slug":"egg","likeCount":905,"collectionCount":2473,"printCount":1948,"downloadCount":2539,"commentCount":14,"designScore":10.18,"hotScore":6.6,"isStaffPicked":true,"is_printable":true,"nsfw":false,"createTime":"2026-06-09T16:55:14Z","designCreator":{"uid":1194156804,"name":"NUK"}}`)
	r, ok := parseDesignRow(raw)
	if !ok {
		t.Fatal("expected ok")
	}
	if r.ID != "2865269" || r.Title != "Egg" || r.Like != 905 || r.Download != 2539 || r.Collection != 2473 {
		t.Errorf("row = %+v", r)
	}
	if r.CreatorID != "1194156804" || r.CreatorName != "NUK" {
		t.Errorf("creator = %s/%s", r.CreatorID, r.CreatorName)
	}
	if !r.StaffPicked || !r.Printable {
		t.Error("expected staff-picked + printable")
	}
}

func TestParseDesignRowRejects(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"id zero", `{"id":0,"title":""}`},
		{"missing id", `{"title":"no id"}`},
		{"invalid json", `not json`},
	}
	for _, c := range cases {
		if _, ok := parseDesignRow([]byte(c.raw)); ok {
			t.Errorf("%s: expected rejection", c.name)
		}
	}
}

func TestMetricValue(t *testing.T) {
	m := snapshotMetrics{Like: 1, Download: 2, Print: 3, Collection: 4}
	cases := map[string]int{"likes": 1, "downloads": 2, "prints": 3, "collections": 4, "unknown": 2}
	for metric, want := range cases {
		if got := metricValue(m, metric); got != want {
			t.Errorf("metric %q = %d want %d", metric, got, want)
		}
	}
}
