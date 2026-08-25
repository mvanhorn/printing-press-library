package cli

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
)

// I nomi disponibili guidano un avviso su stderr: sbagliarli per difetto
// significa avvisare che un campo non esiste mentre invece viene restituito.
// L'insieme e' quindi additivo (chiavi proprie + chiavi delle righe degli
// array figli), non sostitutivo.
func TestTopLevelKeys(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "lista di oggetti: unione delle chiavi di tutti gli elementi",
			input: `[{"doc_id":1,"titolo":"x"},{"doc_id":2,"titolo":"y"}]`,
			want:  []string{"doc_id", "titolo"},
		},
		{
			// Il caso reale (ddl iter, legge cronologia, deputato profilo):
			// il primo elemento e' quello con meno metadati (presentazione,
			// primo atto), e un campo che compare solo dal secondo in poi
			// non deve leggersi come inesistente. Campionare solo arr[0]
			// dava un falso avviso su "seduta"/"doc_id" pur essendo
			// presenti in output.
			name:  "lista di oggetti eterogenei: il campo del secondo elemento non si perde",
			input: `[{"fase":"presentazione","data":"5.07.24"},{"fase":"commissione","data":"09 set 2024","seduta":132}]`,
			want:  []string{"fase", "data", "seduta"},
		},
		{
			name:  "oggetto singolo: le proprie chiavi",
			input: `{"body":"...","fields":{"Data":"16.03.26"},"title":"x"}`,
			want:  []string{"body", "fields", "title"},
		},
		{
			// Il caso che renderebbe falso l'avviso: "a" e' selezionabile
			// eccome, perche' filterFieldsRec la trova al livello esterno.
			name:  "envelope: chiavi esterne E chiavi delle righe",
			input: `{"a":1,"items":[{"id":"x"}]}`,
			want:  []string{"a", "id", "items"},
		},
		{
			// Stesso caso della lista eterogenea, ma dentro un envelope
			// (ddl iter/legge cronologia sono {eventi:[...]}, non un array
			// nudo): la stessa unione deve valere per l'array figlio.
			name:  "envelope: array figlio eterogeneo, chiavi unite",
			input: `{"eventi":[{"fase":"presentazione"},{"fase":"commissione","seduta":132}]}`,
			want:  []string{"eventi", "fase", "seduta"},
		},
		{
			name:  "lista vuota: nessun nome ispezionabile",
			input: `[]`,
			want:  nil,
		},
		{
			name:  "scalare: nessun nome ispezionabile",
			input: `"stringa"`,
			want:  nil,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := topLevelKeys(json.RawMessage(tc.input))
			sort.Strings(got)
			want := append([]string(nil), tc.want...)
			sort.Strings(want)
			if len(got) != len(want) {
				t.Fatalf("topLevelKeys(%s) = %v, want %v", tc.input, got, want)
			}
			for i := range got {
				if got[i] != want[i] {
					t.Fatalf("topLevelKeys(%s) = %v, want %v", tc.input, got, want)
				}
			}
		})
	}
}

// Un avviso deve uscire solo per i nomi che non matchano NULLA: un --select
// parzialmente valido resta silenzioso, altrimenti l'avviso diventa rumore.
func TestUnknownSelectNames(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		fields string
		want   []string
	}{
		{
			// Il caso reale: `oggetto` non esiste in questi archivi, il campo
			// si chiama `titolo`; `id` e' `doc_id`.
			name:   "nomi inventati segnalati",
			input:  `[{"doc_id":1,"titolo":"x","data":"16.03.26"}]`,
			fields: "id,titolo,oggetto",
			want:   []string{"id", "oggetto"},
		},
		{
			name:   "tutti validi: nessuna segnalazione",
			input:  `[{"doc_id":1,"titolo":"x","data":"16.03.26"}]`,
			fields: "doc_id,titolo,data",
			want:   nil,
		},
		{
			// camelCase->kebab e' una corrispondenza legittima di --select.
			name:   "corrispondenza kebab non e' un nome sconosciuto",
			input:  `[{"orderDate":"2026-01-01"}]`,
			fields: "order-date",
			want:   nil,
		},
		{
			// Solo il primo segmento di un path puntato viene giudicato.
			name:   "path puntato: giudicato sul primo segmento",
			input:  `[{"fields":{"Data":"16.03.26"}}]`,
			fields: "fields.Data,assente.x",
			want:   []string{"assente"},
		},
		{
			name:   "payload non ispezionabile: nessuna segnalazione",
			input:  `[]`,
			fields: "qualsiasi",
			want:   nil,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := unknownSelectNames(json.RawMessage(tc.input), tc.fields)
			if len(got) != len(tc.want) {
				t.Fatalf("unknownSelectNames(%s, %q) = %v, want %v",
					tc.input, tc.fields, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("unknownSelectNames(%s, %q) = %v, want %v",
						tc.input, tc.fields, got, tc.want)
				}
			}
		})
	}
}

// Quando ogni nome chiesto esiste anche nella radice, l'array annidato non
// esce: senza avviso `ddl iter --select numero,titolo` dava 0 eventi su 31 con
// stderr muto, e chi legge conclude che la cronologia non esiste.
func TestAvvisoArrayScartato(t *testing.T) {
	originale := json.RawMessage(`{"numero":6030,"titolo":"un ddl","eventi":[{"data":"13 gen 2026","fase":"commissione"},{"data":"27.01.26","fase":"presentazione"}]}`)

	scartato := json.RawMessage(`{"numero":6030,"titolo":"un ddl"}`)
	msg := avvisoArrayScartato(originale, scartato)
	for _, atteso := range []string{"eventi", "2 righe", "data, fase"} {
		if !strings.Contains(msg, atteso) {
			t.Errorf("l'avviso deve citare %q: %q", atteso, msg)
		}
	}

	// L'array c'e' ancora: niente da dire.
	tenuto := json.RawMessage(`{"titolo":"un ddl","eventi":[{"data":"13 gen 2026"}]}`)
	if msg := avvisoArrayScartato(originale, tenuto); msg != "" {
		t.Errorf("array conservato, nessun avviso atteso: %q", msg)
	}
	// Un array vuoto non e' una perdita.
	vuoto := json.RawMessage(`{"numero":1,"eventi":[]}`)
	if msg := avvisoArrayScartato(vuoto, json.RawMessage(`{"numero":1}`)); msg != "" {
		t.Errorf("array vuoto, nessun avviso atteso: %q", msg)
	}
	// Una lista semplice non e' un envelope.
	lista := json.RawMessage(`[{"numero":1}]`)
	if msg := avvisoArrayScartato(lista, lista); msg != "" {
		t.Errorf("lista semplice, nessun avviso atteso: %q", msg)
	}
}

// Su `ddl get` data e numero vivono dentro `fields`: dire solo che non
// esistono manda a cercare un campo che c'e', col nome del portale.
func TestDoveVivono(t *testing.T) {
	data := json.RawMessage(`{"docno":9513,"fields":{"Data":"27.01.26","Numero":"6030"},"body":"..."}`)

	if got := doveVivono(data, []string{"data"}); len(got) != 1 || got[0] != "fields.Data" {
		t.Errorf("doveVivono(data) = %v, atteso [fields.Data]", got)
	}
	if got := doveVivono(data, []string{"pippo"}); len(got) != 0 {
		t.Errorf("un nome inventato non deve trovare nulla: %v", got)
	}
	if got := doveVivono(data, nil); len(got) != 0 {
		t.Errorf("nessun nome ignorato, nessun path: %v", got)
	}
}
