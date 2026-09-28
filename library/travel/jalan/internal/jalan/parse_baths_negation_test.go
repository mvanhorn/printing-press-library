package jalan

import (
	"strings"
	"testing"
)

func planWithRoomBathStatement(t *testing.T, statement string, outdoor bool) string {
	t.Helper()
	doc := parserFixture(t, "plan")
	startTag := `<p class="jlnpc-planDetailInfo__roomDetail">`
	start := strings.Index(doc, startTag)
	if start < 0 {
		t.Fatal("plan fixture lacks room description")
	}
	end := strings.Index(doc[start:], "</p>")
	if end < 0 {
		t.Fatal("plan fixture has unclosed room description")
	}
	doc = doc[:start] + startTag + statement + doc[start+end:]
	if !outdoor {
		for _, evidence := range []string{
			"■温泉かけ流し露天付■",
			`<li class="c-label">露天風呂付き客室</li>`,
		} {
			if strings.Count(doc, evidence) != 1 {
				t.Fatalf("plan fixture expected one %q", evidence)
			}
			doc = strings.Replace(doc, evidence, "", 1)
		}
	}
	return doc
}

func TestParsePlanRoomBathroomNegation(t *testing.T) {
	tests := []struct {
		name      string
		statement string
		outdoor   bool
	}{
		{"plain negation", "室内に浴室はございません", false},
		{"alternate particles", "室内には浴室がありません", false},
		{"reversed order", "浴室は室内にはありません", false},
		{"earlier generic mention", "共用浴室をご利用いただけます。室内には浴室がありません", false},
		{"earlier room mention", "室内浴室の設備については下記をご覧ください。室内には浴室がありません", false},
		{"attached outdoor bath", "室内に浴室はございません", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := ParsePlan(planWithRoomBathStatement(t, tt.statement, tt.outdoor), parserSourceURL, "385995", "03912759", "0576806")
			if err != nil {
				t.Fatal(err)
			}
			assertBool(t, "indoor bathroom", p.Baths.Indoor, false)
			if tt.outdoor {
				assertBool(t, "outdoor bath", p.Baths.Outdoor, true)
				assertBool(t, "attached bath", p.Baths.InRoom, true)
			} else if p.Baths.Outdoor != nil || p.Baths.InRoom != nil {
				t.Fatalf("unsupported outdoor or in-room bath: %+v", p.Baths)
			}
			negative := tt.statement
			if lastSentence := strings.LastIndex(tt.statement, "。"); lastSentence >= 0 {
				negative = tt.statement[lastSentence+len("。"):]
			}
			found := false
			for _, e := range p.Baths.Evidence {
				if !tt.outdoor && e.Field == "baths.in_room" {
					t.Fatalf("unsupported in-room evidence: %+v", e)
				}
				if e.Field == "baths.indoor" && e.Text == negative && e.URL == parserSourceURL {
					found = true
				}
			}
			if !found {
				t.Fatalf("negative indoor evidence missing: %+v", p.Baths.Evidence)
			}
		})
	}
}

func TestParsePlanBathroomOutsideRoomIsUnknown(t *testing.T) {
	p, err := ParsePlan(planWithRoomBathStatement(t, "お部屋以外の浴室はございません", false), parserSourceURL, "385995", "03912759", "0576806")
	if err != nil {
		t.Fatal(err)
	}
	if p.Baths.Indoor != nil || p.Baths.InRoom != nil || p.Baths.Outdoor != nil {
		t.Fatalf("outside-room bathroom statement established room bath facts: %+v", p.Baths)
	}
}
