// Hand-written addition: tests for output flags on IPA responses — preserve on regeneration.

package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// Shape returned by every IPA web service (trimmed WS06 response for oam_2).
const ipaListResp = `{"result":{"cod_err":0,"desc_err":"","num_items":2},"data":[{"cod_uni_ou":"EEEPHJ","des_ou":"Ufficio, transizione"},{"cod_uni_ou":"UFMFD2","des_ou":"Uff_eFatturaPA"}]}`

func TestIPAPayload(t *testing.T) {
	if got := string(ipaPayload(json.RawMessage(ipaListResp))); !strings.HasPrefix(got, `[{"cod_uni_ou":"EEEPHJ"`) {
		t.Errorf("payload = %s", got)
	}
	if got := string(ipaPayload(json.RawMessage(`{"result":{"cod_err":0},"data":null}`))); got != "[]" {
		t.Errorf("null payload = %s, want []", got)
	}
	// A plain object that happens to have result/data but no cod_err is not an envelope.
	plain := `{"result":"x","data":[1]}`
	if got := string(ipaPayload(json.RawMessage(plain))); got != plain {
		t.Errorf("non-envelope changed: %s", got)
	}
}

func TestFilterFieldsKeepsIPAEnvelope(t *testing.T) {
	var got struct {
		Result map[string]any   `json:"result"`
		Data   []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(filterFields(json.RawMessage(ipaListResp), "cod_uni_ou"), &got); err != nil {
		t.Fatal(err)
	}
	if got.Result == nil || len(got.Data) != 2 || got.Data[0]["cod_uni_ou"] != "EEEPHJ" || len(got.Data[0]) != 1 {
		t.Errorf("filtered = %+v", got)
	}
}

func TestPrintOutputWithFlagsCSVAndPlain(t *testing.T) {
	var b bytes.Buffer
	if err := printOutputWithFlags(&b, json.RawMessage(ipaListResp), &rootFlags{csv: true}); err != nil {
		t.Fatal(err)
	}
	want := "cod_uni_ou,des_ou\nEEEPHJ,\"Ufficio, transizione\"\nUFMFD2,Uff_eFatturaPA\n"
	if b.String() != want {
		t.Errorf("csv = %q, want %q", b.String(), want)
	}

	b.Reset()
	if err := printOutputWithFlags(&b, json.RawMessage(ipaListResp), &rootFlags{plain: true, selectFields: "cod_uni_ou"}); err != nil {
		t.Fatal(err)
	}
	if b.String() != "cod_uni_ou\nEEEPHJ\nUFMFD2\n" {
		t.Errorf("plain+select = %q", b.String())
	}
}

func TestPrintCSVSingleObjectAndNested(t *testing.T) {
	var b bytes.Buffer
	if err := printCSV(&b, json.RawMessage(`{"cf":"84011510041","sfe":[{"cod_amm":"oam_2"}]}`)); err != nil {
		t.Fatal(err)
	}
	want := "cf,sfe\n84011510041,\"[{\"\"cod_amm\"\":\"\"oam_2\"\"}]\"\n"
	if b.String() != want {
		t.Errorf("csv = %q, want %q", b.String(), want)
	}
}

func TestPrintStructuredRoutesFlatFormats(t *testing.T) {
	result := map[string]any{"pec": "ord.mondovi@cert.legalmail.it", "status": "sconosciuto"}

	var b bytes.Buffer
	if handled, err := printStructured(&b, &rootFlags{quiet: true}, result); !handled || err != nil || b.Len() != 0 {
		t.Errorf("quiet: handled=%v err=%v out=%q", handled, err, b.String())
	}

	b.Reset()
	if handled, err := printStructured(&b, &rootFlags{csv: true}, result); !handled || err != nil || b.String() != "pec,status\nord.mondovi@cert.legalmail.it,sconosciuto\n" {
		t.Errorf("csv: handled=%v err=%v out=%q", handled, err, b.String())
	}

	b.Reset()
	if handled, err := printStructured(&b, &rootFlags{asJSON: true, selectFields: "status"}, result); !handled || err != nil || strings.Contains(b.String(), "pec") {
		t.Errorf("json+select: handled=%v err=%v out=%q", handled, err, b.String())
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

func TestPrintPlainReturnsWriteError(t *testing.T) {
	if err := printPlain(failingWriter{}, json.RawMessage(ipaListResp)); err == nil {
		t.Error("printPlain returned nil on a failing writer")
	}
}
