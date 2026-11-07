package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

// `commissione dossier` avvolge le righe due volte: sezioni[].risultati[].
// L'avviso guardava un solo livello e dichiarava ignoto `titolo`, anche quando
// il filtro lo applicava alle righe.
const dossierForma = `{"commissione":"5","note":"n","sezioni":[{"archivio":"229","tipo":"convocazioni","risultati":[{"numero":"1","titolo":"t1","data":"d1"}]},{"archivio":"233","tipo":"pareri","risultati":[]}]}`

func TestSelectRigheDueLivelliSotto(t *testing.T) {
	cases := []struct {
		name         string
		fields       string
		ignoti       []string
		tuttoIgnoto  bool
		rigaAttesa   string
		tipoConserva bool
	}{
		{"nome solo nelle righe", "titolo", nil, false, `{"titolo":"t1"}`, true},
		{"nome misto fra sezione e righe", "tipo,titolo", nil, false, `{"titolo":"t1"}`, true},
		{"nome inventato resta ignoto", "pippo", []string{"pippo"}, true, `{"numero":"1","titolo":"t1","data":"d1"}`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := json.RawMessage(dossierForma)
			got := unknownSelectNames(data, tc.fields)
			if strings.Join(got, ",") != strings.Join(tc.ignoti, ",") {
				t.Fatalf("unknownSelectNames(%q) = %v, want %v", tc.fields, got, tc.ignoti)
			}
			if tutto := selettoreTuttoIgnoto(data, tc.fields); tutto != tc.tuttoIgnoto {
				t.Fatalf("selettoreTuttoIgnoto(%q) = %v, want %v", tc.fields, tutto, tc.tuttoIgnoto)
			}
			out := data
			if !tc.tuttoIgnoto {
				out = filterFields(data, tc.fields)
			}
			var p struct {
				Sezioni []struct {
					Tipo      string            `json:"tipo"`
					Risultati []json.RawMessage `json:"risultati"`
				} `json:"sezioni"`
			}
			if err := json.Unmarshal(out, &p); err != nil {
				t.Fatal(err)
			}
			if len(p.Sezioni) != 2 || len(p.Sezioni[0].Risultati) != 1 {
				t.Fatalf("struttura persa: %s", out)
			}
			if riga := string(p.Sezioni[0].Risultati[0]); riga != tc.rigaAttesa {
				t.Fatalf("riga = %s, want %s", riga, tc.rigaAttesa)
			}
			if p.Sezioni[0].Tipo != "convocazioni" {
				t.Fatalf("tipo della sezione perso: %s", out)
			}
		})
	}
}

// `novita` ha la stessa forma: archivi[].nuovi[].
func TestSelectRigheNovita(t *testing.T) {
	data := json.RawMessage(`{"a":"2026-11-07","archivi":[{"archivio":"ddl","conteggio":1,"nuovi":[{"numero":"1","titolo":"t"}]}],"totale":1}`)
	if got := unknownSelectNames(data, "titolo"); got != nil {
		t.Fatalf("unknownSelectNames = %v, want nil", got)
	}
	if out := string(filterFields(data, "titolo")); !strings.Contains(out, `"nuovi":[{"titolo":"t"}]`) {
		t.Fatalf("righe non filtrate: %s", out)
	}
}

// Fra i campi disponibili devono esserci anche quelli delle righe profonde,
// altrimenti chi sbaglia nome non scopre che `titolo` esiste.
func TestTopLevelKeysSegueGliArrayAnnidati(t *testing.T) {
	got := strings.Join(topLevelKeys(json.RawMessage(dossierForma)), ",")
	for _, k := range []string{"sezioni", "tipo", "risultati", "titolo", "data"} {
		if !strings.Contains(","+got+",", ","+k+",") {
			t.Fatalf("manca %q in %s", k, got)
		}
	}
}

// Dentro un campo-oggetto non si scende: il filtro non lo fa, e `doveVivono`
// indica già il percorso puntato. Un nome lì dentro resta ignoto.
func TestTopLevelKeysNonScendeNegliOggetti(t *testing.T) {
	got := unknownSelectNames(json.RawMessage(`{"numero":"1","fields":{"Data":"x"},"firmatari":[{"nome":"a"}]}`), "data")
	if strings.Join(got, ",") != "data" {
		t.Fatalf("got %v, want [data]", got)
	}
}

// `variante` sta in un array dentro un oggetto: l'avviso deve dare il percorso,
// e quel percorso deve funzionare.
func TestDoveVivonoAttraversaGliArray(t *testing.T) {
	data := json.RawMessage(`{"numero":"1","stralcio":{"di":[{"numero":1030,"variante":"A"}]},"fields":{"Data":"x"}}`)
	got := doveVivono(data, []string{"variante"})
	if strings.Join(got, ",") != "stralcio.di.variante" {
		t.Fatalf("doveVivono = %v", got)
	}
	if out := string(filterFields(data, got[0])); !strings.Contains(out, `"variante":"A"`) {
		t.Fatalf("il percorso indicato non funziona: %s", out)
	}
}

func TestAvvisoElencoDiSezione(t *testing.T) {
	data := json.RawMessage(`{"archivi":[{"archivio":"ddl","nuovi":[{"archivio":"ddl","numero":"1"},{"archivio":"ddl","numero":"2"}]},{"archivio":"odg","nuovi":[]}]}`)
	msg := avvisoArrayScartato(data, filterFields(data, "archivio"), "archivio")
	selettore := "archivi.archivio,archivi.nuovi.archivio"
	if !strings.Contains(msg, "archivi.nuovi (2 righe)") || !strings.Contains(msg, selettore) {
		t.Fatalf("avviso = %q", msg)
	}
	out := string(filterFields(data, selettore))
	if !strings.Contains(out, `"nuovi":[{"archivio":"ddl"},{"archivio":"ddl"}]`) {
		t.Fatalf("il selettore proposto non apre l'elenco: %s", out)
	}
	// `tipo` non vive nelle righe: nessuno ha perso nulla di ciò che ha chiesto.
	dossier := json.RawMessage(dossierForma)
	if msg := avvisoArrayScartato(dossier, filterFields(dossier, "tipo"), "tipo"); msg != "" {
		t.Fatalf("avviso non dovuto: %q", msg)
	}
	// Se l'elenco esce, non si avvisa.
	if msg := avvisoArrayScartato(dossier, filterFields(dossier, "tipo,titolo"), "tipo,titolo"); msg != "" {
		t.Fatalf("avviso non dovuto: %q", msg)
	}
}

// Seguire l'avviso non deve far perdere nulla di ciò che era stato chiesto: un
// campo della radice e un percorso già puntato restano come sono.
func TestSelettoreConElencoConservaIlResto(t *testing.T) {
	data := json.RawMessage(`{"totale":3,"archivi":[{"archivio":"ddl","conteggio":3,"nuovi":[{"archivio":"ddl","numero":"1"}]}]}`)
	for _, tc := range []struct{ fields, selettore string }{
		{"archivio,totale", "archivi.archivio,totale,archivi.nuovi.archivio"},
		{"archivio,archivi.conteggio", "archivi.archivio,archivi.conteggio,archivi.nuovi.archivio"},
	} {
		msg := avvisoArrayScartato(data, filterFields(data, tc.fields), tc.fields)
		if !strings.Contains(msg, "usa --select "+tc.selettore+".") {
			t.Fatalf("%s: avviso = %q", tc.fields, msg)
		}
		out := string(filterFields(data, tc.selettore))
		if !strings.Contains(out, `"nuovi":[{"archivio":"ddl"}]`) || !strings.Contains(out, `"archivio":"ddl"`) {
			t.Fatalf("%s: il selettore non apre l'elenco: %s", tc.fields, out)
		}
		if tc.fields == "archivio,totale" && !strings.Contains(out, `"totale":3`) {
			t.Fatalf("totale perso: %s", out)
		}
		if tc.fields != "archivio,totale" && !strings.Contains(out, `"conteggio":3`) {
			t.Fatalf("conteggio perso: %s", out)
		}
	}
}
