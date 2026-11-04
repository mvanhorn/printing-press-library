package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestCompactFieldsKeepsExactNumbers(t *testing.T) {
	obj := json.RawMessage(`{"cik":320193,"val":9007199254740993,"ratio":0.1,"comments":[1]}`)
	out := string(compactFields(obj))
	if !strings.Contains(out, `"val":9007199254740993`) {
		t.Fatalf("object compaction changed a large integer: %s", out)
	}
	if !strings.Contains(out, `"ratio":0.1`) || strings.Contains(out, "comments") {
		t.Fatalf("unexpected object compaction output: %s", out)
	}

	list := json.RawMessage(`[{"id":"a","amount":9007199254740993},{"id":"b","amount":12.5}]`)
	out = string(compactFields(list))
	if !strings.Contains(out, `"amount":9007199254740993`) || !strings.Contains(out, `"amount":12.5`) {
		t.Fatalf("list compaction changed numbers: %s", out)
	}
}

func TestCompactFieldsKeepsExactNumbersInEnvelopeArrays(t *testing.T) {
	env := json.RawMessage(`{"meta":{"total":2},"data":[{"id":"a","amount":9007199254740993},{"id":"b","amount":0.1}]}`)
	out := string(compactFields(env))
	if !strings.Contains(out, `9007199254740993`) || !strings.Contains(out, `0.1`) {
		t.Fatalf("envelope array compaction changed numbers: %s", out)
	}
	if strings.Contains(out, `9007199254740992`) {
		t.Fatalf("envelope array compaction rounded a large integer: %s", out)
	}
}

func TestPrintOutputKeepsExactNumbersInObjects(t *testing.T) {
	var buf bytes.Buffer
	if err := printOutput(&buf, json.RawMessage(`{"val":9007199254740993}`), false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "9007199254740993") {
		t.Fatalf("printOutput changed a large integer: %s", buf.String())
	}
}

func TestUnmarshalUseNumberRejectsTrailingData(t *testing.T) {
	var obj map[string]any
	if err := unmarshalUseNumber([]byte(`{"a":1} {"b":2}`), &obj); err == nil {
		t.Fatal("expected trailing data error")
	}
	var items []map[string]any
	if err := unmarshalUseNumber([]byte(`{"a":1}`), &items); err == nil {
		t.Fatal("expected type error for object into slice")
	}
}
