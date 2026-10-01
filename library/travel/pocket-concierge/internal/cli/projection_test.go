package cli

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

func TestProjectionPreservesIdentityShapeAndFreshness(t *testing.T) {
	v := map[string]any{"items": []any{map[string]any{"id": "1", "name_ja": "ますます増田", "price": map[string]any{"per_guest": 21000, "all_in_total": nil}}}, "meta": map[string]any{"requests": 2}}
	for _, tc := range []struct {
		selectFields string
		want         map[string]any
	}{{"items.id,items.name_ja", map[string]any{"items": []any{map[string]any{"id": "1", "name_ja": "ますます増田"}}, "meta": map[string]any{"requests": float64(2)}}}, {"items.price.all_in_total", map[string]any{"items": []any{map[string]any{"price": map[string]any{"all_in_total": nil}}}, "meta": map[string]any{"requests": float64(2)}}}} {
		got, e := project(v, tc.selectFields)
		if e != nil || !reflect.DeepEqual(got, tc.want) {
			t.Fatal(got, e)
		}
	}
}
func TestProjectionMissAndEmptyCollection(t *testing.T) {
	if _, e := project(map[string]any{"id": "1"}, "wat"); e == nil {
		t.Fatal("silent bad field")
	}
	got, e := project(map[string]any{"items": []any{}}, "items.id")
	if e != nil || !reflect.DeepEqual(got, map[string]any{"items": []any{}}) {
		t.Fatal(got, e)
	}
	if _, e := project(map[string]any{"id": "1"}, "id,"); e == nil {
		t.Fatal("empty path accepted")
	}
}
func TestDryRunDoesNotAccessSourceOrFilesystem(t *testing.T) {
	r := RootCmd()
	r.SetArgs([]string{"availability", "slots", "--dry-run", "--cache-dir", "/no/such/cache"})
	if e := r.Execute(); e != nil {
		t.Fatal(e)
	}
	r = RootCmd()
	r.SetArgs([]string{"availability", "slots", "--id", "1", "--date", "2026-02-29"})
	if e := r.Execute(); e == nil {
		t.Fatal("invalid date accepted")
	}
}
func TestDryRunProjectionDescribesSelectionWithoutInventingData(t *testing.T) {
	root := RootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"restaurants", "search", "--dry-run", "--select", "items.id,items.name_ja"})
	if e := root.Execute(); e != nil {
		t.Fatal(e)
	}
	var result map[string]any
	if json.Unmarshal(out.Bytes(), &result) != nil || result["dry_run"] != true || result["would_select"] != "items.id,items.name_ja" {
		t.Fatal(out.String())
	}
	if _, ok := result["items"]; ok {
		t.Fatal("invented fixture in dry run")
	}
}
