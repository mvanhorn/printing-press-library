package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

const itemsDiProva = `[{"ecli":"ECLI:A","sede":"ROMA","snippet":"lungo"},{"ecli":"ECLI:B","sede":"MILANO","snippet":"lungo"}]`

func TestEmitRicercaSenzaAvvisiRestaArrayNudo(t *testing.T) {
	// Il caso ordinario non cambia contratto: chi fa `jq '.[0]'` continua a
	// funzionare finche' non ci sono avvisi da consegnare.
	var buf bytes.Buffer
	if err := emitRicerca(&buf, json.RawMessage(itemsDiProva), nil, &rootFlags{asJSON: true}); err != nil {
		t.Fatalf("emitRicerca: %v", err)
	}
	var v any
	if err := json.Unmarshal(buf.Bytes(), &v); err != nil {
		t.Fatalf("output non e' JSON valido: %v", err)
	}
	if _, isArray := v.([]any); !isArray {
		t.Errorf("senza avvisi l'output deve essere un array, ottenuto %T", v)
	}
}

func TestEmitRicercaConAvvisiIncapsula(t *testing.T) {
	avvisi := []string{"ricorsi gemelli raggruppati", "il portale ne dichiara 2187"}
	var buf bytes.Buffer
	if err := emitRicerca(&buf, json.RawMessage(itemsDiProva), avvisi, &rootFlags{asJSON: true}); err != nil {
		t.Fatalf("emitRicerca: %v", err)
	}
	var env struct {
		Items  []map[string]any `json:"items"`
		Avvisi []string         `json:"avvisi"`
	}
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("output non e' l'envelope atteso: %v", err)
	}
	if len(env.Items) != 2 {
		t.Errorf("items = %d, attesi 2", len(env.Items))
	}
	if len(env.Avvisi) != 2 || env.Avvisi[0] != avvisi[0] {
		t.Errorf("avvisi = %v, attesi %v", env.Avvisi, avvisi)
	}
}

func TestEmitRicercaSelectSiApplicaAgliItems(t *testing.T) {
	// --select deve filtrare i provvedimenti, non le chiavi dell'envelope:
	// altrimenti `--select ecli` restituirebbe un envelope vuoto.
	var buf bytes.Buffer
	flags := &rootFlags{asJSON: true, selectFields: "ecli"}
	if err := emitRicerca(&buf, json.RawMessage(itemsDiProva), []string{"un avviso"}, flags); err != nil {
		t.Fatalf("emitRicerca: %v", err)
	}
	var env struct {
		Items  []map[string]any `json:"items"`
		Avvisi []string         `json:"avvisi"`
	}
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("output non e' l'envelope atteso: %v", err)
	}
	if len(env.Avvisi) != 1 {
		t.Fatalf("gli avvisi devono sopravvivere a --select, ottenuti %v", env.Avvisi)
	}
	if len(env.Items) != 2 {
		t.Fatalf("items = %d, attesi 2", len(env.Items))
	}
	for _, it := range env.Items {
		if _, ok := it["ecli"]; !ok {
			t.Errorf("--select ecli ha perso il campo richiesto: %v", it)
		}
		if _, ok := it["sede"]; ok {
			t.Errorf("--select ecli non ha filtrato: sede e' ancora presente in %v", it)
		}
	}
}

func TestEmitRicercaNonIncapsulaCSVQuietNeUmano(t *testing.T) {
	// Un CSV di un envelope non vuole dire nulla; con --quiet non si stampa
	// niente; a video gli avvisi sono gia' su stderr.
	for nome, flags := range map[string]*rootFlags{
		"csv":   {asJSON: true, csv: true},
		"quiet": {asJSON: true, quiet: true},
		"umano": {asJSON: false},
	} {
		var buf bytes.Buffer
		if err := emitRicerca(&buf, json.RawMessage(itemsDiProva), []string{"un avviso"}, flags); err != nil {
			t.Fatalf("%s: emitRicerca: %v", nome, err)
		}
		if strings.Contains(buf.String(), "avvisi") {
			t.Errorf("%s: l'envelope non deve comparire, ottenuto %q", nome, buf.String())
		}
	}
}
