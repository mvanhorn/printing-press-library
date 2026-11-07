package cli

import (
	"bytes"
	"encoding/json"
	"testing"
)

// Un --select in cui nessun nome esiste va ignorato davvero, come dice
// l'avviso: prima il payload usciva con ogni array figlio ridotto a oggetti
// vuoti (`firmatari:[{},{}]` su ddl get, `[{}]` su ogni cerca). Con un
// selettore misto invece la proiezione resta.
func TestSelectTuttoIgnotoNonSvuotaArray(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		fields string
		want   string
	}{
		{"oggetto con array figlio", `{"numero":"779","firmatari":[{"nome":"a"},{"nome":"b"}]}`, "iter", `{"numero":"779","firmatari":[{"nome":"a"},{"nome":"b"}]}`},
		{"lista nuda", `[{"numero":"1"},{"numero":"2"}]`, "pippo", `[{"numero":"1"},{"numero":"2"}]`},
		{"envelope", `{"risultati":[{"numero":"1"}],"troncato":false}`, "pippo,pluto", `{"risultati":[{"numero":"1"}],"troncato":false}`},
		{"misto: la proiezione resta", `{"numero":"779","firmatari":[{"nome":"a"}]}`, "numero,pippo", `{"numero":"779"}`},
		{"misto in lista", `[{"numero":"1","x":1}]`, "numero,pippo", `[{"numero":"1"}]`},
	}
	for _, c := range cases {
		var buf bytes.Buffer
		flags := &rootFlags{selectFields: c.fields, asJSON: true}
		if err := printOutputWithFlags(&buf, json.RawMessage(c.input), flags); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		var got, want any
		if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
			t.Fatalf("%s: output non JSON: %s", c.name, buf.String())
		}
		_ = json.Unmarshal([]byte(c.want), &want)
		g, _ := json.Marshal(got)
		w, _ := json.Marshal(want)
		if string(g) != string(w) {
			t.Errorf("%s: got %s, want %s", c.name, g, w)
		}
	}
}

// La busta ha il suo percorso di uscita: la stessa regola vale lì.
func TestSelectTuttoIgnotoNellaBusta(t *testing.T) {
	var buf bytes.Buffer
	flags := &rootFlags{selectFields: "pippo", asJSON: true}
	if err := emitEnvelope(&buf, []map[string]any{{"numero": "1"}}, false, 0, "", flags); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"numero"`)) {
		t.Errorf("la riga è uscita vuota: %s", buf.String())
	}
	buf.Reset()
	flags.selectFields = "numero,pippo"
	_ = emitEnvelope(&buf, []map[string]any{{"numero": "1", "x": 2}}, false, 0, "", flags)
	if bytes.Contains(buf.Bytes(), []byte(`"x"`)) {
		t.Errorf("il selettore misto non ha filtrato: %s", buf.String())
	}
}
