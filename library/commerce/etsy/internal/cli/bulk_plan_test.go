package cli

import (
	"fmt"
	"strings"
	"testing"
)

func TestBulkParsePlanJSONAndCSV(t *testing.T) {
	js := `{"version":1,"generated_by":"listings audit","items":[
	 {"listing_id":1000000001,"field":"tags","op":"add","value":["linen apron"],"reason":"missing tag"},
	 {"listing_id":"1000000002","field":"alt_text","value":"Linen apron on a hook","image_id":55}]}`
	p, err := bulkParsePlan([]byte(js), false)
	s2Must(t, err)
	if len(p.Items) != 2 || p.Items[1].ListingID != 1000000002 || p.Items[1].ImageID != 55 || p.GeneratedBy != "listings audit" {
		t.Fatalf("plan = %+v", p)
	}
	csv := "listing_id,field,op,value,image_id,reason\n1000000001,tags,set,apron|linen|kitchen,,fix\n1000000003,price,set,24.5,,\n1000000004,tags,add,\"[\"\"gift\"\"]\",,\n"
	p, err = bulkParsePlan([]byte(csv), true)
	s2Must(t, err)
	if len(p.Items) != 3 {
		t.Fatalf("csv items = %d", len(p.Items))
	}
	if v, _ := bulkValueStrings(p.Items[0].Value); len(v) != 3 || v[1] != "linen" {
		t.Fatalf("csv tag list = %v", p.Items[0].Value)
	}
	if f, ok := bulkValueNumber(p.Items[1].Value); !ok || f != 24.5 {
		t.Fatalf("csv price = %v", p.Items[1].Value)
	}
	if v, _ := bulkValueStrings(p.Items[2].Value); len(v) != 1 || v[0] != "gift" {
		t.Fatalf("csv json list = %v", p.Items[2].Value)
	}
}

func TestBulkParsePlanRejects(t *testing.T) {
	for name, tc := range map[string]struct {
		in, want string
		csv      bool
	}{
		"empty":      {"  ", "empty", false},
		"version":    {`{"version":2,"items":[]}`, "unsupported plan version 2", false},
		"bad json":   {`{"items":[`, "parsing plan JSON", false},
		"csv no id":  {"field,value\ntags,x\n", "listing_id", true},
		"csv bad id": {"listing_id,field\nabc,tags\n", "not an integer", true},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := bulkParsePlan([]byte(tc.in), tc.csv)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func s2Tags(n int) []any {
	out := make([]any, n)
	for i := range out {
		out[i] = fmt.Sprintf("tag %d", i)
	}
	return out
}

func TestBulkValidateStaticLimits(t *testing.T) {
	ok := []bulkPlanItem{
		{ListingID: 1, Field: "tags", Op: "set", Value: s2Tags(13)},
		{ListingID: 2, Field: "title", Value: strings.Repeat("x", 140)},
		{ListingID: 3, Field: "tags", Op: "add", Value: strings.Repeat("y", 20)},
		{ListingID: 4, Field: "price", Value: 19.99},
		{ListingID: 5, Field: "alt_text", Value: "A mug", ImageID: 9},
	}
	if p := bulkValidateStatic(ok); len(p) != 0 {
		t.Fatalf("valid plan rejected: %v", p)
	}
	for name, tc := range map[string]struct {
		it      bulkPlanItem
		listing int64
		field   string
		want    string
	}{
		"14 tags":      {bulkPlanItem{ListingID: 1000000007, Field: "tags", Op: "set", Value: s2Tags(14)}, 1000000007, "tags", "14 tags"},
		"long tag":     {bulkPlanItem{ListingID: 1000000008, Field: "tags", Op: "add", Value: strings.Repeat("z", 21)}, 1000000008, "tags", "21 characters"},
		"long title":   {bulkPlanItem{ListingID: 1000000009, Field: "title", Value: strings.Repeat("t", 141)}, 1000000009, "title", "141 characters"},
		"14 materials": {bulkPlanItem{ListingID: 1000000010, Field: "materials", Op: "set", Value: s2Tags(14)}, 1000000010, "materials", "14 materials"},
		"bad price":    {bulkPlanItem{ListingID: 11, Field: "price", Value: -2.0}, 11, "price", "positive"},
		"alt no image": {bulkPlanItem{ListingID: 12, Field: "alt_text", Value: "x"}, 12, "alt_text", "image_id"},
		"unknown":      {bulkPlanItem{ListingID: 13, Field: "color", Value: "x"}, 13, "color", "unknown field"},
		"bad op":       {bulkPlanItem{ListingID: 14, Field: "title", Op: "add", Value: "x"}, 14, "title", "not supported"},
	} {
		t.Run(name, func(t *testing.T) {
			probs := bulkValidateStatic([]bulkPlanItem{tc.it})
			if len(probs) != 1 || probs[0].ListingID != tc.listing || probs[0].Field != tc.field || !strings.Contains(probs[0].Message, tc.want) {
				t.Fatalf("problems = %+v", probs)
			}
		})
	}
	// a tag of 20 multibyte characters is within the limit (characters, not bytes)
	if p := bulkValidateStatic([]bulkPlanItem{{ListingID: 1, Field: "tags", Op: "add", Value: strings.Repeat("é", 20)}}); len(p) != 0 {
		t.Fatalf("multibyte tag wrongly rejected: %v", p)
	}
}

func TestBulkResolveAddRemoveAndOverflow(t *testing.T) {
	cur := map[int64]*bulkCurrent{
		1: {Found: true, Title: "Old", Tags: []string{"a", "b", "c"}},
		2: {Found: true, Tags: s2Strs(13)},
	}
	items := []bulkPlanItem{
		{ListingID: 1, Field: "tags", Op: "add", Value: []any{"d", "A"}}, // "A" duplicates "a" case-insensitively
		{ListingID: 1, Field: "tags", Op: "remove", Value: "b"},
		{ListingID: 1, Field: "title", Value: "New"},
		{ListingID: 2, Field: "tags", Op: "add", Value: "overflow"},
		{ListingID: 3, Field: "tags", Op: "add", Value: "x"}, // not in store
		{ListingID: 3, Field: "title", Value: "Brand new"},
	}
	ch, probs := bulkResolve(items, cur)
	byKey := map[string]bulkChange{}
	for _, c := range ch {
		byKey[fmt.Sprintf("%d/%s", c.ListingID, c.Field)] = c
	}
	tags := byKey["1/tags"]
	if got := fmt.Sprint(tags.New); got != "[a c d]" || fmt.Sprint(tags.Old) != "[a b c]" {
		t.Fatalf("tags old=%v new=%v", tags.Old, tags.New)
	}
	if byKey["1/title"].Old != "Old" || byKey["1/title"].New != "New" {
		t.Fatalf("title = %+v", byKey["1/title"])
	}
	if len(probs) != 1 || probs[0].ListingID != 2 || probs[0].Field != "tags" || !strings.Contains(probs[0].Message, "14 tags") {
		t.Fatalf("overflow problems = %+v", probs)
	}
	u := byKey["3/tags"]
	if u.Status != bulkStatusUnresolved || u.New != nil || !strings.Contains(u.Note, "sync") {
		t.Fatalf("unresolved = %+v", u)
	}
	if byKey["3/title"].Status != bulkStatusOK || byKey["3/title"].New != "Brand new" {
		t.Fatalf("set on unsynced listing = %+v", byKey["3/title"])
	}
}

func s2Strs(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("tag %d", i)
	}
	return out
}
