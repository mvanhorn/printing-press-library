package parks

import (
	"strings"
	"testing"
	"time"

	"golang.org/x/net/html"
)

func TestBathIconAggregationRetainsNarrowerEvidenceAndFeeUncertainty(t *testing.T) {
	for _, narrow := range []string{"on", "off"} {
		doc, err := html.Parse(strings.NewReader(`<img src="/images/ico/bath_indoor_on.gif" alt="入浴施設あり 無料"><img src="/images/ico/facility_hotspring_` + narrow + `.gif" alt="温泉あり 有料">`))
		if err != nil {
			t.Fatal(err)
		}
		p := newPark("yypark/213", time.Now())
		parseIcons(&p, doc)
		bath := p.Facilities["bath"]
		if bath.Status != "yes" || bath.Fee != "unknown" || len(bath.Evidence) != 2 {
			t.Fatalf("lost bath evidence: %+v", bath)
		}
		evidence := strings.Join(bath.Evidence, "\n")
		if !strings.Contains(evidence, "bath_indoor") || !strings.Contains(evidence, "facility_hotspring") {
			t.Fatal("one icon overwrote the other")
		}
	}
}

func TestYYParkBathEvidenceIncludesBothPublishedIcons(t *testing.T) {
	p, err := ParseDetail("yypark/213", fixture(t, "yypark-213.html"), observed)
	if err != nil {
		t.Fatal(err)
	}
	bath := p.Facilities["bath"]
	evidence := strings.Join(bath.Evidence, "\n")
	if !strings.Contains(evidence, "bath_shisetunai") || !strings.Contains(evidence, "bath_hotspring") {
		t.Fatalf("actual duplicate icon evidence lost: %+v", bath)
	}
}
