package cli

import (
	"strings"
	"testing"
)

func TestParseSplits(t *testing.T) {
	ok := `[{"merchantName":"Costco","amount":-82.17,"categoryId":"a"},{"merchantName":"Costco","amount":-60.20,"categoryId":"b"}]`
	got, err := parseSplits([]byte(ok), -14237)
	if err != nil || len(got) != 2 || got[1]["amount"] != -60.2 {
		t.Fatalf("valid splits: got %v, %v", got, err)
	}
	bad := map[string]struct {
		json  string
		total int64
		want  string
	}{
		"off by a cent":  {ok, -14238, "off by -0.01"},
		"unknown field":  {`[{"merchantName":"C","amount":-1,"categoryId":"a","tags":[]},{"merchantName":"C","amount":-1,"categoryId":"a"}]`, -200, "unknown field"},
		"one line":       {`[{"merchantName":"C","amount":-1,"categoryId":"a"}]`, -100, "at least two"},
		"three decimals": {`[{"merchantName":"C","amount":-1.005,"categoryId":"a"},{"merchantName":"C","amount":-1,"categoryId":"a"}]`, -200, "two decimal"},
		"wrong sign":     {`[{"merchantName":"C","amount":3,"categoryId":"a"},{"merchantName":"C","amount":-5,"categoryId":"a"}]`, -200, "same sign"},
		"no category":    {`[{"merchantName":"C","amount":-1},{"merchantName":"C","amount":-1,"categoryId":"a"}]`, -200, "categoryId"},
	}
	for name, tc := range bad {
		if _, err := parseSplits([]byte(tc.json), tc.total); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: want error containing %q, got %v", name, tc.want, err)
		}
	}
}
