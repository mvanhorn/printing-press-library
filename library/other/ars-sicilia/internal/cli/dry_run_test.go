package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	icaro "github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/internal/icaroclient"
	"github.com/spf13/cobra"
)

// dryRunOut esegue un'anteprima su un comando finto e ne rilegge il JSON.
func dryRunOut(t *testing.T, fn func(*cobra.Command) error) map[string]any {
	t.Helper()
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	if err := fn(cmd); err != nil {
		t.Fatalf("anteprima fallita: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output non JSON (%q): %v", buf.String(), err)
	}
	return got
}

func requestsOf(t *testing.T, out map[string]any) []map[string]any {
	t.Helper()
	raw, ok := out["requests"].([]any)
	if !ok {
		t.Fatalf("manca `requests` in %v", out)
	}
	reqs := make([]map[string]any, 0, len(raw))
	for _, r := range raw {
		m, ok := r.(map[string]any)
		if !ok {
			t.Fatalf("richiesta non è un oggetto: %v", r)
		}
		reqs = append(reqs, m)
	}
	return reqs
}

// Il difetto: gli archivi delle sedute sono serviti dal backend /bd/, ma
// l'anteprima li descriveva come query Icaro, annunciando un URL che il
// comando non interroga.
func TestDryRunTargetUsaIlBackendGiusto(t *testing.T) {
	casi := []struct {
		slug        string
		vuoleBE     string
		vuoleChiave string
	}{
		{"resoconti", "bd", "would_post"},
		{"sommari", "bd", "would_post"},
		{"convocazioni", "bd", "would_post"},
		{"ddl", "icaro", "would_fetch"},
		{"leggi", "icaro", "would_fetch"},
		{"pareri", "icaro", "would_fetch"},
	}
	for _, c := range casi {
		arc := icaro.BySlug(c.slug)
		if arc == nil {
			t.Fatalf("archivio %s non registrato", c.slug)
		}
		got, _ := dryRunTarget(*arc, map[string]string{"legisl": "18"}, "")
		if got["backend"] != c.vuoleBE {
			t.Errorf("%s: backend = %v, voluto %s", c.slug, got["backend"], c.vuoleBE)
		}
		url, _ := got[c.vuoleChiave].(string)
		if url == "" {
			t.Errorf("%s: manca %s in %v", c.slug, c.vuoleChiave, got)
			continue
		}
		if c.vuoleBE == "bd" && !strings.Contains(url, "/bd/"+c.slug) {
			t.Errorf("%s: would_post = %q, atteso il path /bd/", c.slug, url)
		}
		if c.vuoleBE == "icaro" && !strings.Contains(url, "icaDB="+arc.ID) {
			t.Errorf("%s: would_fetch = %q, atteso icaDB=%s", c.slug, url, arc.ID)
		}
		if _, ha := got["isis_query"]; ha != (c.vuoleBE == "icaro") {
			t.Errorf("%s: isis_query presente=%v, backend=%s", c.slug, ha, c.vuoleBE)
		}
	}
}

// Il difetto: --dry-run era accettato e scartato, uscita vuota ed exit 0.
func TestLeggeCronologiaDryRunAnteprimaLaLegge(t *testing.T) {
	out := dryRunOut(t, func(cmd *cobra.Command) error {
		return emitLeggeCronologiaDryRun(cmd, 18, 1, 2025)
	})
	if out["dry_run"] != true {
		t.Errorf("manca dry_run: %v", out)
	}
	reqs := requestsOf(t, out)
	if len(reqs) != 1 {
		t.Fatalf("richieste = %d, voluta 1 (la legge; il ddl d'origine dipende da questa risposta)", len(reqs))
	}
	q, _ := reqs[0]["isis_query"].(string)
	for _, atteso := range []string{"18.LEGISL", "1.LEGNUM", "2025.LEGANN"} {
		if !strings.Contains(q, atteso) {
			t.Errorf("isis_query = %q, manca %s", q, atteso)
		}
	}
	nota, _ := out["note"].(string)
	if !strings.Contains(nota, "P010/P012") {
		t.Errorf("la nota deve dichiarare il passo non anteprimabile, invece: %q", nota)
	}
}

// Senza --anno l'anteprima deve dirlo: la cronologia può uscire coerente e
// riferita all'atto sbagliato.
func TestLeggeCronologiaDryRunSenzaAnnoAvverte(t *testing.T) {
	out := dryRunOut(t, func(cmd *cobra.Command) error {
		return emitLeggeCronologiaDryRun(cmd, 18, 26, 0)
	})
	nota, _ := out["note"].(string)
	if !strings.Contains(nota, "--anno") {
		t.Errorf("nota senza avviso su --anno: %q", nota)
	}
	q, _ := requestsOf(t, out)[0]["isis_query"].(string)
	if strings.Contains(q, "LEGANN") {
		t.Errorf("senza --anno la query non deve vincolare l'anno: %q", q)
	}
}

// L'anteprima del dossier vale se mostra la traduzione dell'argomento: `codcom`
// grezzo verso /bd/, ordinale a lettere verso l'ISIS. Normalizzare i parametri
// qui (come fa `*/cerca`) annuncerebbe `commissione: SESTA` anche su /bd/,
// cioè un parametro diverso da quello che parte.
func TestCommissioneDossierDryRunNonNormalizzaCodcom(t *testing.T) {
	out := dryRunOut(t, func(cmd *cobra.Command) error {
		return emitCommissioneDossierDryRun(cmd, "6", 18)
	})
	reqs := requestsOf(t, out)
	if len(reqs) != 4 {
		t.Fatalf("richieste = %d, volute 4 (convocazioni, sommari, pareri, ddl)", len(reqs))
	}
	perArchivio := map[string]map[string]any{}
	for _, r := range reqs {
		slug, _ := r["archive"].(string)
		perArchivio[slug] = r
	}
	bd, ok := perArchivio["convocazioni"]
	if !ok {
		t.Fatalf("manca la sezione convocazioni: %v", perArchivio)
	}
	// `codcom` non e' un campo della POST: il backend vuole un id per
	// legislatura, risolto leggendo le <option> del form. L'anteprima lo deve
	// nominare fra i differiti, non spacciarlo per un campo — e in nessun caso
	// mostrarlo gia' tradotto in `commissione: SESTA`, che e' cio' che
	// normalizeParams farebbe e runCommissioneDossier non fa.
	deferred, _ := bd["deferred"].(map[string]any)
	if deferred["codcom"] == nil {
		t.Errorf("convocazioni: deferred = %v, atteso codcom fra i filtri risolti al momento della richiesta", deferred)
	}
	if deferred["commissione"] != nil {
		t.Errorf("convocazioni: deferred = %v, `commissione` non e' cio' che il comando manda: manda codcom", deferred)
	}
	pareri, ok := perArchivio["pareri"]
	if !ok {
		t.Fatalf("manca la sezione pareri: %v", perArchivio)
	}
	if q, _ := pareri["isis_query"].(string); !strings.Contains(q, "SESTA") {
		t.Errorf("pareri: isis_query = %q, atteso l'ordinale a lettere", q)
	}
}

// Il profilo interroga sei archivi come firmatario più i resoconti come testo
// libero: l'anteprima deve elencarli tutti, ed è quella differenza a spiegare
// perché lo stesso nome renda su un archivio e non sull'altro.
func TestDeputatoProfiloDryRunElencaTuttiGliArchivi(t *testing.T) {
	out := dryRunOut(t, func(cmd *cobra.Command) error {
		return emitDeputatoProfiloDryRun(cmd, "Cracolici", 18, "2024-01-01:2024-12-31")
	})
	reqs := requestsOf(t, out)
	if len(reqs) != len(profiloFirmaArchives)+1 {
		t.Fatalf("richieste = %d, volute %d", len(reqs), len(profiloFirmaArchives)+1)
	}
	for i, slug := range profiloFirmaArchives {
		if reqs[i]["archive"] != slug {
			t.Errorf("richiesta %d = %v, voluto %s", i, reqs[i]["archive"], slug)
		}
		q, _ := reqs[i]["isis_query"].(string)
		if !strings.Contains(q, "Cracolici.FIRMAT") {
			t.Errorf("%s: isis_query = %q, atteso il nome come firmatario", slug, q)
		}
		// --data deve arrivare già in formato ISIS, come a runtime.
		if !strings.Contains(q, "240101/241231.DATPRE") {
			t.Errorf("%s: isis_query = %q, --data non normalizzata come a runtime", slug, q)
		}
	}
	ultima := reqs[len(reqs)-1]
	if ultima["archive"] != "resoconti" {
		t.Fatalf("ultima richiesta = %v, voluti i resoconti", ultima["archive"])
	}
	if ultima["backend"] != "bd" {
		t.Errorf("resoconti: backend = %v, voluto bd", ultima["backend"])
	}
	// Sui resoconti il nome viaggia come testo libero, e il backend /bd/ quel
	// filtro lo riceve sul campo `$TTEXT`: e' quello che l'anteprima deve dire.
	post, _ := ultima["post_fields"].(map[string]any)
	if post["$TTEXT"] != "Cracolici" {
		t.Errorf("resoconti: post_fields = %v, atteso il nome sul campo $TTEXT del backend", post)
	}
}

// La matrice di collaudo sonda i comandi scritti a mano con argomenti
// segnaposto (`mock-value`). Prima delle anteprime il ramo dry-run usciva 0
// senza guardarli; dopo, un argomento non numerico li faceva fallire — una
// sonda che passava trasformata in errore. Il ripiego e' l'help, come sul ramo
// degli argomenti mancanti.
func TestDryRunConArgomentiSegnapostoNonFallisce(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	casi := []struct {
		nome string
		cmd  *cobra.Command
		args []string
	}{
		{"legge cronologia", newNovelLeggeCronologiaCmd(flags), []string{"mock-value", "mock-value"}},
		{"ddl iter", newNovelDdlIterCmd(flags), []string{"mock-value", "mock-value"}},
	}
	for _, c := range casi {
		var buf bytes.Buffer
		c.cmd.SetOut(&buf)
		c.cmd.SetErr(&buf)
		c.cmd.SetArgs(c.args)
		if err := c.cmd.Execute(); err != nil {
			t.Errorf("%s: --dry-run con argomenti segnaposto ha fallito: %v", c.nome, err)
		}
		if buf.Len() == 0 {
			t.Errorf("%s: uscita muta; una sonda --dry-run deve stampare qualcosa", c.nome)
		}
	}
}

// Senza --dry-run un argomento non numerico resta un errore: il ripiego vale
// solo per le sonde, non nasconde un uso sbagliato.
func TestArgomentoNonNumericoRestaErroreSenzaDryRun(t *testing.T) {
	for _, c := range []struct {
		nome string
		cmd  *cobra.Command
	}{
		{"legge cronologia", newNovelLeggeCronologiaCmd(&rootFlags{})},
		{"ddl iter", newNovelDdlIterCmd(&rootFlags{})},
	} {
		var buf bytes.Buffer
		c.cmd.SetOut(&buf)
		c.cmd.SetErr(&buf)
		c.cmd.SilenceUsage = true
		c.cmd.SilenceErrors = true
		c.cmd.SetArgs([]string{"mock-value", "mock-value"})
		if err := c.cmd.Execute(); err == nil {
			t.Errorf("%s: atteso errore su argomento non numerico senza --dry-run", c.nome)
		}
	}
}

// L'anteprima /bd/ non deve spacciare i filtri della riga di comando per campi
// della POST: searchBD non li spedisce come li riceve. I nomi passano per
// spec.fields, i selettori di modalità si aggiungono da spec.static, e `data`,
// `oratore` e `commissione`/`codcom` si risolvono solo al momento della
// richiesta. Rilievo P1 di Greptile sulla PR #1790.
func TestDryRunBDDistingueCampiPostEFiltriDifferiti(t *testing.T) {
	arc := icaro.BySlug("resoconti")
	if arc == nil {
		t.Fatal("archivio resoconti non registrato")
	}
	got, _ := dryRunTarget(*arc, map[string]string{
		"legisl":  "18",
		"data":    "2026-01-01:2026-08-22",
		"oratore": "Cracolici",
	}, "")

	if _, ha := got["params"]; ha {
		t.Error("`params` non deve esistere: prometteva che i filtri partissero cosi' come scritti")
	}
	post, _ := got["post_fields"].(map[string]string)
	if post["$Ilegislatura"] != "18" {
		t.Errorf("post_fields = %v, atteso il nome di campo del backend per legisl", post)
	}
	if post["$S$TTEXT"] != "all" {
		t.Errorf("post_fields = %v, mancano i selettori di modalita' che il backend riceve sempre", post)
	}
	if _, ha := post["legisl"]; ha {
		t.Errorf("post_fields = %v: la chiave utente non deve comparire come campo POST", post)
	}
	deferred, _ := got["deferred"].(map[string]string)
	for _, k := range []string{"data", "oratore"} {
		if deferred[k] == "" {
			t.Errorf("deferred = %v: %q va nominato, non spacciato per un campo", deferred, k)
		}
	}
	// `--data` non diventa un campo con il suo valore: diventa un ciclo, e il
	// campo che ne esce e' `anno` con UN anno per giro (qui il primo). Il valore
	// grezzo dell'intervallo non deve comparire da nessuna parte fra i campi.
	if post["anno"] != "2026" {
		t.Errorf("post_fields = %v, atteso anno=2026, cioe' il primo giro del ciclo", post)
	}
	for k, v := range post {
		if v == "2026-01-01:2026-08-22" {
			t.Errorf("post_fields[%q] = %q: l'intervallo grezzo non e' un valore che il backend riceve", k, v)
		}
	}
	if _, ha := post["data"]; ha {
		t.Errorf("post_fields = %v: `data` non e' un campo del backend", post)
	}
}

// Con --data il percorso vivo non manda una richiesta: ne manda una per anno
// dell'intervallo, ciascuna paginata. Un'anteprima che mostra una sola voce e
// dice solo "si risolve dopo" non fa capire quante ne partono ne' come rifarle.
// Secondo rilievo P1 di Greptile sulla PR #1790.
func TestDryRunBDEnumeraGliAnniDellIntervallo(t *testing.T) {
	arc := icaro.BySlug("resoconti")
	if arc == nil {
		t.Fatal("archivio resoconti non registrato")
	}

	got, _ := dryRunTarget(*arc, map[string]string{"legisl": "18", "data": "2024-06-01:2026-08-22"}, "")
	anni, _ := got["anni"].([]string)
	if len(anni) != 3 {
		t.Fatalf("anni = %v, volute le tre annate dell'intervallo", got["anni"])
	}
	// L'ordine e' quello in cui searchBD li scorre: dal piu' recente.
	if anni[0] != "2026" || anni[2] != "2024" {
		t.Errorf("anni = %v, atteso dal piu' recente al piu' vecchio", anni)
	}
	if got["richieste"] == nil {
		t.Error("manca il conto delle richieste: da un dry run si deve capire quante ne partono")
	}
	// `page` viaggia su ogni POST e la prima di ogni giro e' 1: senza, nemmeno
	// la prima richiesta si puo' rifare alla lettera.
	if post, _ := got["post_fields"].(map[string]string); post["page"] != "1" {
		t.Errorf("post_fields = %v, atteso page=1 sulla prima richiesta di ogni anno", got["post_fields"])
	}
	// `anno` va fra i CAMPI, non solo nell'elenco a parte: il ciclo lo imposta
	// prima di ogni post, e senza, rigiocando i campi mostrati si manda una
	// richiesta senza vincolo d'anno — l'archivio intero invece della fetta.
	if post, _ := got["post_fields"].(map[string]string); post["anno"] != "2026" {
		t.Errorf("post_fields = %v, atteso anno=2026 (il primo giro)", got["post_fields"])
	}

	// --anno e --data scrivono lo stesso campo server: si intersecano, come nel
	// percorso vivo. Annunciare giri che non partirebbero e' lo stesso difetto.
	got, _ = dryRunTarget(*arc, map[string]string{"legisl": "18", "data": "2024-06-01:2026-08-22", "anno": "2025"}, "")
	anni, _ = got["anni"].([]string)
	if len(anni) != 1 || anni[0] != "2025" {
		t.Errorf("anni = %v, voluto il solo 2025 dopo l'intersezione con --anno", got["anni"])
	}

	// --anno fuori dall'intervallo: la ricerca non restituirebbe nulla, e
	// l'anteprima lo deve dire invece di mostrare una richiesta plausibile.
	got, _ = dryRunTarget(*arc, map[string]string{"legisl": "18", "data": "2024-06-01:2024-12-31", "anno": "2026"}, "")
	if anni, _ := got["anni"].([]string); len(anni) != 0 {
		t.Errorf("anni = %v, atteso vuoto: il 2026 e' fuori dall'intervallo", got["anni"])
	}
	deferred, _ := got["deferred"].(map[string]string)
	if deferred["anno"] == "" {
		t.Errorf("deferred = %v: va detto che nessun anno resta da interrogare", deferred)
	}
	// In quel caso searchBD non manda nulla: mostrare `anno` fra i campi
	// annuncerebbe una richiesta plausibile che non parte.
	if post, _ := got["post_fields"].(map[string]string); post["anno"] != "" {
		t.Errorf("post_fields = %v: senza anni da interrogare `anno` non deve comparire", got["post_fields"])
	}

	// Senza --data non c'e' nessun ciclo, e le chiavi non devono comparire —
	// ma `page` si', perche' quella richiesta parte comunque paginata.
	got, _ = dryRunTarget(*arc, map[string]string{"legisl": "18"}, "")
	if _, ha := got["anni"]; ha {
		t.Errorf("senza --data `anni` non deve esistere: %v", got)
	}
	if post, _ := got["post_fields"].(map[string]string); post["page"] != "1" {
		t.Errorf("post_fields = %v, `page` va dichiarato anche senza --data", got["post_fields"])
	}

	// --anno da solo, senza --data: e' un filtro come gli altri e resta un campo.
	got, _ = dryRunTarget(*arc, map[string]string{"legisl": "18", "anno": "2025"}, "")
	if post, _ := got["post_fields"].(map[string]string); post["anno"] != "2025" {
		t.Errorf("post_fields = %v, atteso anno=2025 anche senza --data", got["post_fields"])
	}
}

// `*/cerca --dry-run` passava SEMPRE da normalizeParams, mentre runCerca lo
// salta sugli archivi /bd/ (la traduzione avviene dentro searchBD). Cosi'
// `--codcom 6` usciva riscritto in `commissione: SESTA` solo nell'anteprima:
// un parametro diverso da quello che il comando processa. Rilievo P1 di
// Greptile sulla PR #1790.
func TestEmitDryRunNonNormalizzaSugliArchiviBD(t *testing.T) {
	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	arc := icaro.BySlug("convocazioni")
	if arc == nil {
		t.Fatal("archivio convocazioni non registrato")
	}
	if err := emitDryRun(cmd, *arc, cercaParams{Params: map[string]string{"legisl": "18", "codcom": "6"}}); err != nil {
		t.Fatalf("anteprima fallita: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output non JSON: %v", err)
	}
	deferred, _ := got["deferred"].(map[string]any)
	if deferred["codcom"] == nil {
		t.Errorf("deferred = %v: su /bd/ viaggia `codcom`, ed e' quello che l'anteprima deve nominare", deferred)
	}
	if deferred["commissione"] != nil {
		t.Errorf("deferred = %v: `commissione` e' la riscrittura di normalizeParams, che su /bd/ non avviene", deferred)
	}

	// Sugli archivi Icaro la normalizzazione invece ci deve essere, perche' il
	// percorso vivo ce la fa: --data diventa AAMMGG dentro la query ISIS.
	buf.Reset()
	cmd2 := &cobra.Command{}
	cmd2.SetOut(&buf)
	ddl := icaro.BySlug("ddl")
	if ddl == nil {
		t.Fatal("archivio ddl non registrato")
	}
	if err := emitDryRun(cmd2, *ddl, cercaParams{Params: map[string]string{"legisl": "18", "data": "2026-01-01:2026-08-22"}}); err != nil {
		t.Fatalf("anteprima fallita: %v", err)
	}
	got = nil
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output non JSON: %v", err)
	}
	if q, _ := got["isis_query"].(string); !strings.Contains(q, "260101/260822.DATPRE") {
		t.Errorf("isis_query = %q: su Icaro --data va normalizzata in AAMMGG come fa runCerca", q)
	}
}

// Un filtro che non si parsa fa fallire searchBD PRIMA di mandare qualunque
// cosa. L'anteprima deve fallire allo stesso modo: stampare una richiesta
// plausibile per un comando che non parte e' il caso peggiore di tutti —
// descrive una richiesta che non esistera' mai. Rilievo P1 di Greptile sulla
// PR #1790.
func TestDryRunBDDataMalformataFallisceComeIlVivo(t *testing.T) {
	arc := icaro.BySlug("resoconti")
	if arc == nil {
		t.Fatal("archivio resoconti non registrato")
	}
	got, err := dryRunTarget(*arc, map[string]string{"legisl": "18", "data": "2025-01-01:garbage"}, "")
	if err == nil {
		t.Fatalf("atteso errore sulla data malformata, invece l'anteprima ha prodotto %v", got)
	}
	if got != nil {
		t.Errorf("con l'errore non si deve produrre nessuna anteprima: %v", got)
	}
	// Stesso codice d'uscita del percorso vivo: e' un errore d'uso, non un
	// guasto, e chi rama sul codice non deve vedere una differenza fra i due.
	var codeErr *cliError
	if !As(err, &codeErr) || codeErr.code != 2 {
		t.Errorf("err = %v: atteso un usage error (exit 2), come sul percorso vivo", err)
	}
	if !strings.Contains(err.Error(), "--data") {
		t.Errorf("err = %v: deve nominare il filtro incriminato", err)
	}

	// Una data valida continua a produrre l'anteprima.
	if _, err := dryRunTarget(*arc, map[string]string{"legisl": "18", "data": "2024-06-01:2026-08-22"}, ""); err != nil {
		t.Errorf("data valida: errore inatteso %v", err)
	}
}
