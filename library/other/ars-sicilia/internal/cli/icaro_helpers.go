// Helpers that bridge generated Cobra commands to the hand-rolled icaroclient.
// Each <archivio>_cerca.go / <archivio>_get.go file delegates here so the
// search-engine logic lives in one place.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/internal/cliutil"
	icaro "github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/internal/icaroclient"
	"github.com/spf13/cobra"
)

// cercaParams collects the cleaned param map the icaroclient expects, plus
// the few search-time tunables the CLI exposes.
type cercaParams struct {
	Params   map[string]string
	ISISRaw  string
	Limit    int
	MaxPages int
	// AggregaLeggi aggrega le righe-articolo dell'archivio 201 per legge, così
	// che Limit conti leggi e non articoli (vedi leggi_collapse.go). LimitLeggi
	// è il limite chiesto dall'utente, mentre Limit porta le righe da scaricare.
	AggregaLeggi bool
	LimitLeggi   int
}

// runCerca executes a search against an archive and emits JSON or table-shaped
// output according to flags. archiveSlug names one of the entries in
// internal/icaroclient/archives.go (e.g. "leggi", "ddl").
func runCerca(cmd *cobra.Command, flags *rootFlags, archiveSlug string, p cercaParams) error {
	arc := icaro.BySlug(archiveSlug)
	if arc == nil {
		return fmt.Errorf("unknown archive slug: %q", archiveSlug)
	}
	// --escludi is registered on every search command; read it centrally so the
	// exclusion (ISIS NOT) is applied uniformly without per-command plumbing.
	if cmd.Flags().Lookup("escludi") != nil {
		if esc, _ := cmd.Flags().GetString("escludi"); strings.TrimSpace(esc) != "" {
			if p.Params == nil {
				p.Params = map[string]string{}
			}
			p.Params["escludi"] = esc
		}
	}
	if flags.dryRun {
		return emitDryRun(cmd, *arc, p)
	}
	if cliIsVerify() {
		return emitDryRun(cmd, *arc, p)
	}
	// Default MaxPages: if Limit is set and small, one page is enough; if
	// caller asked for >50, fan out multiple pages (Icaro paginates ~10/pg).
	maxPages := p.MaxPages
	if maxPages == 0 {
		if p.Limit > 10 {
			maxPages = (p.Limit + 9) / 10
		} else {
			maxPages = 1
		}
	}
	c, err := icaro.New(nil)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	// Gli archivi /bd/ ricevono i param grezzi: la loro traduzione (codcom/
	// commissione risolti in id, date in AAMMGG o ISO) avviene in searchBD, non
	// con normalizeParams (specifico del motore Icaro).
	searchParams := p.Params
	if !icaro.IsBDArchive(arc.Slug) {
		searchParams = normalizeParams(*arc, p.Params)
	}
	var truncated bool
	// Con l'aggregazione per legge il limite dell'utente è in leggi, non in
	// righe: la paginazione si ferma quando le leggi chieste ci sono tutte,
	// invece di consumare una finestra di righe stimata a priori.
	var stopWhen func([]icaro.Record) bool
	if p.AggregaLeggi && p.LimitLeggi > 0 {
		stopWhen = func(all []icaro.Record) bool {
			return len(collapseLeggi(all)) >= p.LimitLeggi
		}
	}
	recs, err := c.Search(ctx, *arc, icaro.SearchOptions{
		Params:    searchParams,
		ISISRaw:   p.ISISRaw,
		Limit:     p.Limit,
		MaxPages:  maxPages,
		Truncated: &truncated,
		StopWhen:  stopWhen,
	})
	if err != nil {
		if rlErr := new(icaro.HTTPRateLimitError); errors.As(err, &rlErr) {
			return rateLimitErr(fmt.Errorf("ricerca %s: %w", arc.Slug, err))
		}
		// Un valore scritto male è un errore d'uso (exit 2), non un guasto
		// generico (exit 1): chi ha uno script che distingue i codici deve poter
		// capire che è il comando da correggere, non il servizio da riprovare.
		if invalido := new(icaro.InvalidParamError); errors.As(err, &invalido) {
			return usageErr(err)
		}
		if h := restringiHint(err, arc.Slug, searchParams); h != "" {
			fmt.Fprintln(os.Stderr, "hint: "+h)
		}
		return fmt.Errorf("ricerca %s: %w", arc.Slug, err)
	}
	if p.AggregaLeggi {
		leggi := collapseLeggi(recs)
		// Il troncamento va detto qui e non dopo: se la finestra scaricata si è
		// esaurita sugli articoli delle prime leggi, le successive non sono
		// "assenti", sono "non lette" — ed è la differenza fra una risposta
		// giusta e quella sbagliata che questo aggregato esiste per evitare.
		mancanti := p.LimitLeggi > 0 && len(leggi) < p.LimitLeggi
		if p.LimitLeggi > 0 && len(leggi) > p.LimitLeggi {
			leggi = leggi[:p.LimitLeggi]
		}
		// Due modi di restare corti, e vanno detti tutti e due.
		//
		// `mancanti`: la finestra di righe si è esaurita prima di raccogliere le
		// leggi chieste — l'elenco è incompleto e non si vede.
		//
		// L'altro è il limite raggiunto: le leggi chieste sono arrivate tutte,
		// e proprio per questo la paginazione si è fermata senza sapere quante
		// altre ce ne fossero. Qui il ramo taceva e l'envelope dichiarava
		// `troncato: false`, cioè affermava una completezza che nessuno aveva
		// verificato: `leggi cerca --legisl 18 --anno 2026` rispondeva 10 leggi
		// su 14 e diceva che erano tutte. Ogni altra ricerca di questa CLI in
		// quel caso avvisa; questa era l'unica a non farlo, ed è il percorso
		// naturale della domanda «quali leggi nell'anno X».
		hint := hintLeggiCorte(truncated, mancanti, len(recs), len(leggi), p.LimitLeggi)
		// Questo ramo ha un hint tutto suo e ritorna prima di warnTruncated:
		// senza il caso esplicito, `leggi cerca --envelope` sarebbe l'unica
		// ricerca a non avere la busta, e in silenzio.
		if envelopeWanted(cmd.OutOrStdout(), flags) {
			return emitEnvelope(cmd.OutOrStdout(), leggi, truncated, hint, flags)
		}
		if err := printJSONFiltered(cmd.OutOrStdout(), leggi, flags); err != nil {
			return err
		}
		if hint != "" {
			fmt.Fprintf(cmd.ErrOrStderr(), "hint: %s\n", hint)
		}
		return nil
	}
	var firm map[int][]firmatario
	if conFirmatariRequested(cmd) {
		if _, ok := arc.FieldMap["firmatario"]; ok {
			firm = firmatariByDoc(ctx, c, *arc, recs)
		}
	}
	// Pertinenza: il portale ordina per data, non per attinenza, quindi una
	// ricerca testuale può mettere in cima documenti che citano i termini di
	// sfuggita e relegare in fondo quello che li ha nel titolo.
	termini := terminiRicerca(p.Params)
	recs = ordinaPerPertinenza(recs, termini)
	pertHint := pertinenzaHint(recs, termini, arc.Slug)
	omonHint := omonimiHint(recs, arc.Slug)

	if envelopeWanted(cmd.OutOrStdout(), flags) {
		// L'avviso resta anche su stderr: la busta serve a chi legge il JSON,
		// non a togliere l'informazione a chi usa la CLI a mano.
		warnTruncated(truncated, len(recs), arc.Slug)
		warnPertinenza(pertHint)
		warnPertinenza(omonHint)
		return emitEnvelope(cmd.OutOrStdout(), flatRecords(recs, firm), truncated,
			uniscoHint(truncatedHint(truncated, len(recs), arc.Slug), pertHint, omonHint), flags)
	}
	if err := emitRecords(cmd, flags, *arc, recs, firm); err != nil {
		return err
	}
	warnTruncated(truncated, len(recs), arc.Slug)
	warnPertinenza(pertHint)
	warnPertinenza(omonHint)
	return nil
}

// uniscoHint concatena gli avvisi non vuoti in un unico campo `hint`: la busta
// ne espone uno solo, e due frasi separate da spazio si leggono meglio di una
// struttura annidata che ogni consumatore dovrebbe imparare.
func uniscoHint(parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(strings.TrimPrefix(p, "hint: ")); p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, " ")
}

func warnPertinenza(hint string) {
	if hint != "" {
		fmt.Fprintln(os.Stderr, hint)
	}
}

// terminiRicerca estrae le parole cercate a testo libero (--testo, --frase).
// Gli operatori ISIS espliciti passati con --isis-query non entrano: lì
// l'utente ha costruito la query e non vogliamo riordinargliela sotto i piedi.
func terminiRicerca(params map[string]string) []string {
	var out []string
	for _, k := range []string{"testo", "frase"} {
		v := strings.TrimSpace(params[k])
		if v == "" {
			continue
		}
		// Una query con parentesi o operatori è roba costruita a mano: si
		// riordina solo il testo libero.
		if strings.ContainsAny(v, "()*$") {
			continue
		}
		for _, w := range strings.Fields(strings.ToLower(v)) {
			if len(w) > 2 && !isOperatoreISIS(w) {
				out = append(out, w)
			}
		}
	}
	return out
}

func isOperatoreISIS(w string) bool {
	switch w {
	case "e", "o", "non", "and", "or", "not", "adj":
		return true
	}
	return false
}

// titoloCapFonte è il numero di caratteri a cui la lista dei risultati del
// portale tronca i titoli: oltre non si va in nessun archivio (ddl, mozioni,
// interrogazioni, leggi).
const titoloCapFonte = 256

// titoloSospetto è la lunghezza da cui un titolo va considerato tagliato. È il
// cap meno uno perché quando il taglio cade su uno spazio la coda viene
// ripulita: in `mozioni cerca --legisl 18` convivono titoli da 256 e da 255,
// entrambi interrotti a metà frase. Alzarla a titoloCapFonte rimetterebbe i
// secondi fra i fuori tema.
const titoloSospetto = titoloCapFonte - 1

// titoloTroncato riporta se il titolo è arrivato al tetto della fonte, cioè se
// quello che vediamo può non essere il titolo intero.
//
// Perché conta: il ddl 199 della XVII si intitola «…riconoscimento degli
// svantaggi derivanti dalla condizione di insularità», ma la lista lo taglia su
// «…svantaggi deriva». Cercando `--frase "condizione di insularità"` il titolo
// visibile non matcha, e senza questo controllo l'atto finiva in coda con
// l'avviso «nessun risultato ha i termini nel titolo»: un falso negativo
// proprio sui titoli lunghi (schemi di progetto di legge, disegni di legge
// voto), che sono i più difficili da trovare.
func titoloTroncato(titolo string) bool {
	return utf8.RuneCountInString(titolo) >= titoloSospetto
}

// ordinaPerPertinenza porta davanti i record che hanno TUTTI i termini nel
// titolo, lasciando invariato l'ordine relativo dentro ciascun gruppo (il
// portale ordina per data, e dentro il gruppo quell'ordine resta leggibile).
//
// I gruppi sono tre, non due: in mezzo stanno i titoli troncati dalla fonte,
// dove il termine può esserci nella parte non mostrata. Non sono un match
// dimostrato, ma nemmeno un no: metterli in coda insieme ai fuori tema
// nasconderebbe l'atto giusto sotto quelli sbagliati.
//
// Limite dichiarato: agisce sulla finestra già scaricata. Se il documento
// pertinente sta oltre --limit, nessun riordino lo fa comparire — per quello
// c'è pertinenzaHint, che dice di alzare il limite invece di lasciar
// concludere che il documento non esista.
func ordinaPerPertinenza(recs []icaro.Record, termini []string) []icaro.Record {
	if len(termini) == 0 || len(recs) < 2 {
		return recs
	}
	testa := make([]icaro.Record, 0, len(recs))
	incerti := make([]icaro.Record, 0, len(recs))
	coda := make([]icaro.Record, 0, len(recs))
	for _, r := range recs {
		switch {
		case titoloMatcha(r.Title, termini):
			testa = append(testa, r)
		case titoloTroncato(r.Title):
			incerti = append(incerti, r)
		default:
			coda = append(coda, r)
		}
	}
	return append(append(testa, incerti...), coda...)
}

// titoloMatcha riporta se il titolo contiene tutti i termini cercati.
func titoloMatcha(titolo string, termini []string) bool {
	t := strings.ToLower(titolo)
	for _, w := range termini {
		if !strings.Contains(t, w) {
			return false
		}
	}
	return true
}

// pertinenzaHint avvisa quando una ricerca testuale ha prodotto risultati in
// cui NESSUN titolo contiene i termini: è il sintomo che il documento cercato
// sta oltre la finestra. Su `--testo "gestione rifiuti"` i primi dieci sono
// debiti fuori bilancio e leggi di stabilità, mentre il ddl con quelle parole
// nel titolo è il 75° — e chi si ferma al limite di default conclude che non
// esista.
//
// Se però fra i risultati ci sono titoli troncati dalla fonte, «nessuno ha i
// termini nel titolo» sarebbe una bugia: il termine può stare nei caratteri
// tagliati. In quel caso l'avviso cambia e dice dove guardare, invece di
// mandare a cercare più in basso un atto che è già sotto gli occhi.
//
// Il rimedio suggerito dipende dall'archivio: `--frase` esiste solo sul flusso
// Icaro, e sui tre serviti da /bd/ (resoconti, sommari, convocazioni) non è
// nemmeno un flag. Consigliarlo lì manderebbe in un vicolo cieco chi segue
// l'avviso alla lettera, che è esattamente chi l'avviso deve aiutare.
func pertinenzaHint(recs []icaro.Record, termini []string, slug string) string {
	if len(termini) == 0 || len(recs) == 0 {
		return ""
	}
	rimedio := "alza --limit, oppure usa --frase per la locuzione esatta"
	if icaro.IsBDArchive(slug) {
		rimedio = "alza --limit"
	}
	troncati := 0
	for _, r := range recs {
		if titoloMatcha(r.Title, termini) {
			return ""
		}
		if titoloTroncato(r.Title) {
			troncati++
		}
	}
	quali := strings.Join(virgolette(termini), " e ")
	if troncati > 0 {
		apri := "apri il documento per il titolo intero"
		if g := comandoGet(slug); g != "" {
			apri = fmt.Sprintf("apri il documento per il titolo intero (`%s`)", g)
		}
		return fmt.Sprintf(
			"hint: nessuno dei %d risultati mostrati ha %s nel titolo visibile, ma %s al limite di %d caratteri della lista del portale: il termine può stare nella parte non mostrata, quindi l'assenza dal titolo non prova nulla. Quei risultati sono stati portati in cima: %s. Altrimenti %s.",
			len(recs), quali, plurale(troncati, "1 titolo è tagliato", "%d titoli sono tagliati"), titoloCapFonte, apri, rimedio)
	}
	return fmt.Sprintf(
		"hint: nessuno dei %d risultati mostrati ha %s nel titolo: la ricerca a testo libero aggancia anche i documenti che citano i termini nel corpo, e il portale ordina per data, non per pertinenza. L'atto che cerchi può stare più in basso: %s.",
		len(recs), quali, rimedio)
}

// comandoGet è il sottocomando che apre il documento intero di un archivio, o
// "" per i tre che non ne hanno (convocazioni, sommari, biblioteca).
//
// Serve agli avvisi. «Apri il documento» senza dire con quale comando lascia il
// lavoro proprio a chi l'avviso deve aiutare: chi legge un hint sul titolo
// tagliato di solito non sa che il titolo intero sta in `<archivio> get`, e non
// c'è motivo di fargli cercare il comando.
func comandoGet(slug string) string {
	switch slug {
	case "leggi":
		// Le leggi riusano il numero ogni anno: senza --anno il comando apre la
		// legge di un altro anno. È la stessa ragione per cui esiste
		// runGetExtra, e un avviso che la ignora manda alla legge sbagliata.
		return "leggi get <legisl> <numero> --anno <anno>"
	case "ddl", "resoconti", "pareri", "interrogazioni", "interpellanze", "mozioni", "odg", "risoluzioni":
		return slug + " get <legisl> <numero>"
	}
	return ""
}

// unDocPerNumero riporta se in quell'archivio legislatura+numero identificano
// UN documento. Vale quasi ovunque, ma non su due archivi:
//
//   - leggi (201) è indicizzato per articolo: una legge di venti articoli sono
//     venti righe con lo stesso numero (vedi leggi_collapse.go);
//   - resoconti (217) su Icaro è indicizzato per punto dell'ordine del giorno,
//     quindi una seduta sono più frammenti.
//
// Lì più righe con lo stesso numero sono la norma e segnalarle sarebbe rumore
// a ogni ricerca. Altrove sono una cosa che chi legge deve sapere.
func unDocPerNumero(slug string) bool {
	switch slug {
	case "leggi", "resoconti":
		return false
	}
	return true
}

// campoRecord legge un campo della riga con la stessa normalizzazione che usa
// flatRecords ("Legisl." → "legisl"), così l'avviso ragiona sulle chiavi che
// chi legge l'output ha davanti.
func campoRecord(r icaro.Record, chiave string) string {
	for k, v := range r.Fields {
		if strings.ToLower(strings.TrimSuffix(k, ".")) == chiave {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// omonimiHint avvisa che nella finestra ci sono più righe con lo stesso
// legislatura+numero.
//
// Non sono duplicati da scartare: sul ddl 6030 il portale tiene due documenti
// distinti — docno 9513, con il testo del ddl e l'iter aggiornato, e docno
// 9390, la sola scheda ferma a due settimane prima. Nei campi della lista sono
// identici in tutto, titolo e data comprese, quindi senza questo avviso chi
// legge crede di vedere la stessa riga due volte e ne scarta una a caso.
//
// Limite dichiarato, come per pertinenzaHint: conta dentro la finestra già
// scaricata. Con `--limit 1` sul 6030 la seconda riga non c'è e l'avviso tace —
// lì a dire che manca qualcosa è `troncato`/truncatedHint, che in quel caso
// parla sempre. Il silenzio di questo avviso non è la prova che il numero
// agganci un documento solo.
func omonimiHint(recs []icaro.Record, slug string) string {
	if !unDocPerNumero(slug) || len(recs) < 2 {
		return ""
	}
	visti := map[string]int{}
	var doppi []string
	for _, r := range recs {
		n := campoRecord(r, "numero")
		if n == "" {
			continue
		}
		k := campoRecord(r, "legisl") + "/" + n
		visti[k]++
		if visti[k] == 2 {
			doppi = append(doppi, n)
		}
	}
	if len(doppi) == 0 {
		return ""
	}
	apri := ""
	if g := comandoGet(slug); g != "" {
		apri = fmt.Sprintf(" `%s` apre il primo e ne riporta il `docno`, che è l'identificatore stabile del documento.", g)
	}
	return fmt.Sprintf(
		"hint: %s con più di una riga (%s): il portale tiene documenti distinti sotto lo stesso numero — di norma versioni diverse della stessa pratica, non copie da scartare.%s",
		plurale(len(doppi), "1 numero compare", "%d numeri compaiono"), strings.Join(doppi, ", "), apri)
}

// plurale sceglie fra le due forme e formatta il conteggio: un avviso che dice
// «1 titoli» si legge come un bug e toglie credito al resto del messaggio.
func plurale(n int, uno, molti string) string {
	if n == 1 {
		return uno
	}
	return fmt.Sprintf(molti, n)
}

func virgolette(termini []string) []string {
	out := make([]string, 0, len(termini))
	for _, t := range termini {
		out = append(out, "«"+t+"»")
	}
	return out
}

// warnTruncated avvisa su stderr quando la ricerca ha restituito solo una
// finestra dei risultati disponibili. Senza questo avviso l'assenza di un
// documento dalla lista è indistinguibile dalla sua assenza dall'archivio: è
// l'equivoco che ha fatto dare per "non ancora indicizzata" una legge che era
// regolarmente presente, ma oltre i primi 10 risultati.
// L'ordinamento del portale non è per pertinenza, quindi la finestra non
// contiene necessariamente i documenti più attinenti alla ricerca.
//
// Con --envelope l'avviso viaggia anche dentro il JSON (vedi emitEnvelope):
// su stderr resta comunque, perché è lì che lo legge chi usa la CLI a mano.
func warnTruncated(truncated bool, shown int, slug string) {
	if msg := truncatedHint(truncated, shown, slug); msg != "" {
		fmt.Fprintln(os.Stderr, msg)
	}
}

// emitEnvelope stampa i risultati avvolti in {risultati, troncato, hint}.
//
// Esiste perché l'avviso di troncamento nasceva su stderr e basta: in --agent
// il payload è un array puro, quindi chi consuma JSON non lo vedeva mai e
// leggeva una finestra troncata come "il documento non esiste". L'hint c'era,
// stampato accanto, e restava fuori dal dato.
//
// --select viene applicato DENTRO risultati, non alla busta: filtrare il
// livello esterno cancellerebbe i risultati (nessun record ha un campo
// "risultati") e lascerebbe le chiavi di servizio, cioè l'opposto di ciò che
// chiede chi scrive --select data,titolo. Le chiavi della busta restano
// sempre, che è il motivo per cui la busta esiste.
func emitEnvelope(w io.Writer, payload any, troncato bool, hint string, flags *rootFlags) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	// La busta ha un percorso di uscita suo (writeJSON, non
	// printOutputWithFlags), quindi l'iniezione di data_iso va rifatta qui:
	// senza, le stesse righe avevano data_iso senza --envelope e non l'avevano
	// con --envelope.
	raw = iniettaDataISO(raw)
	if flags.selectFields != "" {
		warnUnknownSelectFields(raw, flags.selectFields)
		raw = filterFields(raw, flags.selectFields)
	} else if flags.compact {
		raw = compactFields(raw)
	}
	env := struct {
		Risultati json.RawMessage `json:"risultati"`
		Troncato  bool            `json:"troncato"`
		Hint      string          `json:"hint,omitempty"`
	}{Risultati: raw, Troncato: troncato, Hint: hint}
	if flags.quiet {
		return nil
	}
	return writeJSON(w, env)
}

// envelopeWanted riporta se l'output va avvolto. La busta è JSON: con --csv
// (che rende una tabella) e nelle viste a terminale non ha significato, quindi
// il flag viene ignorato invece di produrre un ibrido.
func envelopeWanted(w io.Writer, flags *rootFlags) bool {
	if !flags.envelope || flags.csv {
		return false
	}
	return flags.asJSON || !isTerminal(w)
}

// truncatedHint torna il testo dell'avviso, o "" quando non c'è nulla da dire.
// Separato da warnTruncated per poterlo verificare senza catturare stderr.
// hintLeggiCorte dice perché l'elenco aggregato può non essere tutto.
//
// Separato dal chiamante per poterlo verificare senza rete, come
// truncatedHint. Torna "" quando non c'è nulla da dire.
func hintLeggiCorte(truncated, mancanti bool, righe, leggi, limite int) string {
	switch {
	case !truncated:
		return ""
	case mancanti:
		return fmt.Sprintf(
			"lette %d righe-articolo e trovate solo %d delle %d leggi chieste: l'elenco può essere incompleto. Alza --limit, oppure restringi con --anno/--numero.",
			righe, leggi, limite)
	default:
		return fmt.Sprintf(
			"mostrate %d leggi, il massimo chiesto: l'archivio ne ha altre e la ricerca si è fermata qui. Alza --limit (es. --limit 50) prima di leggere questo elenco come completo.",
			leggi)
	}
}

func truncatedHint(truncated bool, shown int, slug string) string {
	if !truncated {
		return ""
	}
	return fmt.Sprintf(
		"hint: risultati troncati: mostrati %d, l'archivio %s ne ha altri. L'ordinamento non è per pertinenza, quindi un documento che stai cercando può stare oltre questa finestra: alza --limit (es. --limit 50) o restringi la ricerca prima di concludere che non esiste.",
		shown, slug)
}

// conFirmatariRequested reports whether --con-firmatari was set. Like
// --escludi, the flag is registered per-command and read centrally here so
// the behavior stays uniform without per-command plumbing.
func conFirmatariRequested(cmd *cobra.Command) bool {
	if cmd.Flags().Lookup("con-firmatari") == nil {
		return false
	}
	v, _ := cmd.Flags().GetBool("con-firmatari")
	return v
}

// firmatariByDoc opens each record's document and parses its full signatory
// list. The portal's short list carries only the FIRST signatory (the column
// is literally "Titolo e Primo Firmatario") or none at all for ddl, so a
// --firmatario hit cannot be verified from the list alone. Resolving that
// costs one extra request per row — paced by the client's rate limiter —
// which is why this is opt-in behind --con-firmatari rather than always on.
// A document that fails to load is skipped: one bad row must not sink the
// whole result set.
func firmatariByDoc(ctx context.Context, c *icaro.Client, arc icaro.Archive, recs []icaro.Record) map[int][]firmatario {
	out := make(map[int][]firmatario, len(recs))
	for _, r := range recs {
		doc, err := c.GetDoc(ctx, arc, r.DocID)
		if err != nil {
			continue
		}
		if f := docFirmatari(doc); len(f) > 0 {
			out[r.DocID] = f
		}
	}
	return out
}

// runGet fetches and emits a single document. Get needs a fresh session, so
// we Search first with a narrow query that pins the record, then GetDoc on
// the returned docID. For the typical case where the caller passes legisl
// and numero, the query is `<legisl>.LEGISL E <numero>.<KEY>` where KEY is
// the archive-specific id field.
// bdSchedaFallback cerca il record sul backend /bd/ e ne restituisce la scheda
// con l'URL del documento allegato. Ritorna (nil, nil) se nemmeno /bd/ ce l'ha,
// così il chiamante può emettere il not-found consueto.
//
// Il PDF non viene scaricato né convertito: pesa qualche MB (una seduta d'Aula
// sfiora i 5) e il testo estratto supera i 200.000 caratteri, che non ha senso
// far transitare per default. L'URL è stabile e citabile, quindi chi vuole il
// testo lo prende con lo strumento che preferisce.
func bdSchedaFallback(ctx context.Context, c *icaro.Client, arc icaro.Archive, params map[string]string, legisl, numero int) (map[string]any, error) {
	recs, err := c.Search(ctx, arc, icaro.SearchOptions{
		Params: params, // grezzi: gli archivi /bd/ non passano da normalizeParams
		Limit:  1,
	})
	if err != nil || len(recs) == 0 {
		return nil, err
	}
	r := recs[0]
	out := map[string]any{
		"legisl": legisl,
		"numero": numero,
		"titolo": r.Title,
		"url":    r.URL,
		"fonte":  "bd",
	}
	if d := strings.TrimSpace(r.Fields["Data"]); d != "" {
		out["data"] = d
	}
	// Senza questa nota il record /bd/ mente per omissione: `get` su un record
	// servito da Icaro ha il campo `body`, qui il campo non c'è e basta. Chi
	// legge il JSON — un agente, soprattutto — conclude «testo non disponibile»,
	// mentre il testo c'è ed è nel PDF. È lo stesso principio dell'avviso su
	// `--group-by cofirmatari`: quando il dato non si può dare, si dice dov'è.
	nota := "l'indice testuale di questo archivio è fermo indietro nel tempo e questo record arriva dal backend /bd/: la scheda non ha il campo `body`. Il testo integrale non manca, sta nel PDF."
	pdf, err := c.SchedaAllegatoURL(ctx, r.URL)
	if err != nil || pdf == "" {
		out["nota"] = nota + " Questa volta però la scheda non ha restituito l'allegato: apri `url` a mano."
		return out, nil // la scheda non si è aperta: meglio i metadati che niente
	}
	out["pdf_url"] = pdf
	out["nota"] = nota + " Scaricalo da `pdf_url`."
	return out, nil
}

// restringiHint suggerisce di stringere la ricerca quando il backend /bd/ non
// consegna la risposta. Il portale tronca a metà le pagine grandi e consegna
// intere quelle piccole — misurato il 2026-08-12 su `sommari`: la ricerca di una
// singola seduta (24 KB) è arrivata 8 volte su 8, quella senza filtri (44 KB)
// zero volte su 8. Quindi «riprova» da solo è un consiglio scadente: quello che
// cambia davvero l'esito è chiedere meno righe. Il suggerimento esce solo se
// c'è ancora un filtro da mettere: dirlo a chi ha già ristretto tutto sarebbe
// dare la colpa all'utente di un guasto che non è suo.
//
// Per lo stesso motivo tace su UnresolvedFilterError: lì la ricerca non è caduta
// per la dimensione della risposta ma perché il valore chiesto non esiste in
// anagrafica, e restringere per numero o anno non farebbe comparire un oratore
// che non c'è. L'hint uscirebbe sopra il messaggio d'errore vero, che è quello
// che spiega cosa correggere.
func restringiHint(err error, slug string, params map[string]string) string {
	if !icaro.IsBDArchive(slug) {
		return ""
	}
	if irrisolto := new(icaro.UnresolvedFilterError); errors.As(err, &irrisolto) {
		return ""
	}
	var mancanti []string
	for _, f := range bdFiltriStretti[slug] {
		if strings.TrimSpace(params[f.chiave]) == "" && strings.TrimSpace(params[f.chiaveAlt]) == "" {
			mancanti = append(mancanti, f.flag)
		}
	}
	if len(mancanti) == 0 {
		return ""
	}
	return "il portale tronca le risposte grandi e consegna intere quelle piccole: una ricerca più stretta ha molte più probabilità di riuscire. Aggiungi " +
		strings.Join(mancanti, " o ") + " e riprova."
}

// bdFiltriStretti elenca, per archivio /bd/, i filtri che riducono davvero il
// numero di righe, dal più selettivo al meno. Sono i campi del form del portale:
// convocazioni non ha un numero di seduta (il suo form non lo espone), quindi
// per quell'archivio si può solo suggerire l'anno e la commissione.
var bdFiltriStretti = map[string][]struct{ chiave, chiaveAlt, flag string }{
	"sommari":      {{"numero", "", "--numero"}, {"anno", "data", "--anno"}, {"commissione", "codcom", "--commissione"}},
	"resoconti":    {{"numero", "", "--numero"}, {"anno", "data", "--anno"}},
	"convocazioni": {{"anno", "data", "--anno"}, {"commissione", "codcom", "--commissione"}},
}

// getMissingErr traduce in errore l'esito di un `get` che non ha prodotto il
// documento. Sono due fatti diversi e finivano nella stessa frase: «il record
// non c'è» e «il backend non ha risposto». Il secondo travestito da primo è la
// bugia peggiore che questa CLI possa dire — chi legge, giornalista o agente,
// conclude che la seduta non è mai esistita, mentre il portale semplicemente
// tronca le risposte a intermittenza e il tentativo dopo la restituisce.
// bdErr nil significa che il backend ha risposto e non aveva il record.
func getMissingErr(slug string, legisl, numero int, bdErr error) error {
	if bdErr == nil {
		return fmt.Errorf("nessun documento trovato per legisl=%d numero=%d in %s", legisl, numero, slug)
	}
	// Il 429 ha già il suo codice di uscita e la sua ricetta (attendere): non va
	// confuso con un backend che cade, che invece si ritenta subito.
	if rlErr := new(icaro.HTTPRateLimitError); errors.As(bdErr, &rlErr) {
		return rateLimitErr(fmt.Errorf("backend /bd/ (%s): %w", slug, bdErr))
	}
	return fmt.Errorf("il backend /bd/ non ha risposto per legisl=%d numero=%d in %s: %w; "+
		"non vuol dire che il documento non esista — riprova", legisl, numero, slug, bdErr)
}

// rejectPositionalArgs rifiuta gli argomenti posizionali sui comandi di ricerca,
// che prendono ogni criterio da un flag. Senza, cobra li accetta e li scarta in
// silenzio: `commissioni sommari cerca --commissione X` restituisce lo stesso
// risultato di `commissioni sommari --commissione X`, e chi lo scrive crede di
// aver invocato un sottocomando che non esiste. È così che una gap analysis ha
// concluso che due comandi si comportassero diversamente: era lo stesso comando.
func rejectPositionalArgs(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	return usageErr(fmt.Errorf("argomento inatteso %q: %s non prende argomenti posizionali, i criteri di ricerca sono flag (vedi --help)",
		args[0], cmd.CommandPath()))
}

func runGet(cmd *cobra.Command, flags *rootFlags, archiveSlug string, legisl, numero int) error {
	return runGetExtra(cmd, flags, archiveSlug, legisl, numero, nil)
}

// runGetExtra is runGet with additional pinning params (e.g. --anno) so callers
// can disambiguate records that share legisl+numero (leggi reuse a number per
// year). The extra keys are translated through the archive FieldMap like any
// other criterion.
func runGetExtra(cmd *cobra.Command, flags *rootFlags, archiveSlug string, legisl, numero int, extra map[string]string) error {
	arc := icaro.BySlug(archiveSlug)
	if arc == nil {
		return fmt.Errorf("unknown archive slug: %q", archiveSlug)
	}
	// I parametri si costruiscono PRIMA del ramo d'anteprima, cosi' l'anteprima
	// descrive la ricerca che partirebbe davvero invece di comporne una propria.
	params := getSearchParams(legisl, numero, extra)
	if flags.dryRun || cliIsVerify() {
		return emitGetDryRun(cmd, *arc, legisl, numero, params)
	}
	c, err := icaro.New(nil)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	recs, err := c.Search(ctx, *arc, icaro.SearchOptions{
		Params: normalizeParams(*arc, params),
		// Due e non uno: serve solo a sapere se il numero ne aggancia più d'uno,
		// e non costa una richiesta in più (Limit taglia la pagina già
		// scaricata). Con uno solo `get` sceglieva in silenzio la prima riga, e
		// sul ddl 6030 — dove il portale tiene due documenti diversi sotto lo
		// stesso numero — chi leggeva non sapeva nemmeno che ci fosse una scelta.
		Limit: 2,
		// Il dettaglio (GetDoc) richiede il DocID Icaro; le righe /bd/ non lo
		// hanno e la scheda /bd/ non è implementata. ForceIcaro tiene `get` sul
		// flusso Icaro (trova i record nell'indice Icaro; not-found sui recenti).
		ForceIcaro: true,
	})
	if err != nil {
		if rlErr := new(icaro.HTTPRateLimitError); errors.As(err, &rlErr) {
			return rateLimitErr(fmt.Errorf("locating document: %w", err))
		}
		return fmt.Errorf("locating document: %w", err)
	}
	if len(recs) == 0 {
		// Icaro non ha il record. Per i tre archivi migrati a /bd/ non significa
		// che il documento non esista: l'indice Icaro è fermo indietro nel tempo
		// (sui resoconti, alla seduta 232 del 25.02.2026) mentre /bd/ è corrente.
		// Prima di dichiarare il not-found si guarda lì, e si restituisce la
		// scheda con l'URL del PDF: è il testo integrale, che Icaro non dà
		// nemmeno quando il record ce l'ha.
		if icaro.IsBDArchive(arc.Slug) {
			out, bdErr := bdSchedaFallback(ctx, c, *arc, params, legisl, numero)
			if bdErr == nil && out != nil {
				// Anche su stderr, non solo nel campo `nota`: la ricetta
				// documentata è `resoconti get ... --select pdf_url`, e --select
				// il campo lo filtra via. L'avviso servirebbe a niente proprio
				// nel comando che la documentazione suggerisce di scrivere.
				if n, ok := out["nota"].(string); ok {
					fmt.Fprintln(os.Stderr, "hint: "+n)
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			return getMissingErr(arc.Slug, legisl, numero, bdErr)
		}
		return getMissingErr(arc.Slug, legisl, numero, nil)
	}
	doc, err := c.GetDoc(ctx, *arc, recs[0].DocID)
	if err != nil {
		return err
	}
	// Merge the short-list fields into the doc so callers see legisl, atto, etc.
	for k, v := range recs[0].Fields {
		if _, exists := doc.Fields[k]; !exists {
			doc.Fields[k] = v
		}
	}
	if recs[0].Excerpt != "" && doc.Body == "" {
		doc.Body = recs[0].Excerpt
	}
	// For every archive with signatories (same gate as --con-firmatari in
	// runCerca: ddl, interrogazioni, interpellanze, mozioni, odg, risoluzioni
	// all share the FIRMAT field and the portal's "Nome (Gruppo)." doc
	// format), surface them as a structured field instead of leaving the
	// caller to parse fields.Firmatari by hand. Was ddl-only: interrogazioni
	// get et al. left the raw string in place, forcing a second parsing path
	// for any consumer iterating across atto types.
	var firm []firmatario
	if _, ok := arc.FieldMap["firmatario"]; ok {
		firm = docFirmatari(doc)
	}
	// Sui ddl il portale dichiara nel campo "Riferimenti" se il documento è lo
	// stralcio di un altro ddl, e di quale: si espone come dato strutturato
	// invece di lasciarlo dentro una stringa (vedi ddl_stralci.go).
	var stralcio *stralcioOut
	if arc.Slug == "ddl" {
		stralcio = stralcioDaTesti(numero, doc.Fields["Riferimenti"], recs[0].Excerpt)
	}
	// Il numero aggancia più di un documento: si dice quale si è aperto invece
	// di scegliere in silenzio. Su stderr oltre che nel campo, perché `get` non
	// ha busta e --select il campo lo filtra via.
	nota := ""
	if len(recs) > 1 && unDocPerNumero(arc.Slug) {
		nota = fmt.Sprintf("il portale ha più di un documento per legisl=%d numero=%d — di norma versioni diverse della stessa pratica. Questo è il primo", legisl, numero)
		if doc.DocNo > 0 {
			nota += fmt.Sprintf(" (docno %d)", doc.DocNo)
		}
		nota += fmt.Sprintf("; gli altri si vedono con `%s cerca --legisl %d --numero %d`.", arc.Slug, legisl, numero)
		fmt.Fprintln(os.Stderr, "hint: "+nota)
	}
	// printJSONFiltered (not the bare writeJSON) so --select/--compact/--csv
	// behave the same as on generator-emitted commands — writeJSON always
	// dumped the full payload regardless of --select.
	return printJSONFiltered(cmd.OutOrStdout(), getOut{
		Doc:       doc,
		Legisl:    legisl,
		Numero:    numero,
		Data:      strings.TrimSpace(doc.Fields["Data"]),
		Titolo:    titoloDoc(doc),
		Fonte:     "icaro",
		Firmatari: firm,
		Stralcio:  stralcio,
		Nota:      nota,
	}, flags)
}

// getOut è la forma di `<archivio> get`, ed esiste per farne UNA sola.
//
// I tre archivi migrati a /bd/ hanno due percorsi: se l'indice Icaro il record
// ce l'ha si risponde con la scheda Icaro, altrimenti con la scheda /bd/
// (bdSchedaFallback). Le due uscite avevano forme diverse — la prima teneva
// numero e data dentro `fields` (`fields.Numero`, `fields.Data`), la seconda in
// radice — quindi lo stesso `--select numero,data_iso,titolo` rendeva sulla
// seduta 268 e tornava `{}` sulla 147. Con exit 0: chi legge solo stdout
// conclude che il documento non ha quei dati, o che non esiste.
//
// E il confine fra i due percorsi non è una proprietà del documento: è dove si
// è fermato l'indice Icaro (il 2026-08-22, sui resoconti, alla seduta 232 del
// 25.02.2026 — misurato: 232 annidata, 233 piatta). Si sposta quando il portale
// aggiorna l'indice, quindi non è nemmeno una regola che si possa documentare:
// la stessa seduta può cambiare forma da un giorno all'altro.
//
// Le coordinate stanno quindi in radice su entrambi i rami, con gli stessi nomi
// e gli stessi tipi della scheda /bd/. `legisl` e `numero` vengono dagli
// argomenti del comando, che sono interi e autorevoli, non da un campo di testo
// da riparsare. `data` esce grezza come la scrive la fonte, così `data_iso` la
// affianca il passaggio che lo fa già per tutti gli altri payload
// (iniettaDataISO). `fields` resta dov'è: chi lo legge continua a funzionare,
// perché questa è un'aggiunta, non uno spostamento.
type getOut struct {
	icaro.Doc
	Legisl int    `json:"legisl,omitempty"`
	Numero int    `json:"numero,omitempty"`
	Data   string `json:"data,omitempty"`
	Titolo string `json:"titolo,omitempty"`
	// Fonte dice quale dei due percorsi ha risposto, come già faceva la scheda
	// /bd/. Ora che la forma è una sola servirebbe a poco distinguere a occhio,
	// e senza il marcatore non si distinguerebbe affatto: `body` presente o
	// assente è un indizio, non una risposta.
	Fonte     string       `json:"fonte,omitempty"`
	Firmatari []firmatario `json:"firmatari,omitempty"`
	Stralcio  *stralcioOut `json:"stralcio,omitempty"`
	Nota      string       `json:"nota,omitempty"`
}

// titoloDoc sceglie il titolo dell'atto: la fonte lo mette nel titolo della
// scheda su alcuni archivi e solo nel campo `Titolo` su altri — sui resoconti
// la scheda ha titolo vuoto e il campo pieno («Ordine del giorno della seduta
// successiva»), sui ddl è il contrario.
func titoloDoc(doc icaro.Doc) string {
	if t := strings.TrimSpace(doc.Title); t != "" {
		return t
	}
	return strings.TrimSpace(doc.Fields["Titolo"])
}

// getSearchParams sono i parametri con cui `get` aggancia il documento, in un
// posto solo: l'anteprima --dry-run e la ricerca vera devono partire dagli
// stessi, o la prima smette di descrivere la seconda.
func getSearchParams(legisl, numero int, extra map[string]string) map[string]string {
	params := map[string]string{}
	if legisl > 0 {
		params["legisl"] = fmt.Sprintf("%d", legisl)
	}
	if numero > 0 {
		params["numero"] = fmt.Sprintf("%d", numero)
	}
	for k, v := range extra {
		if v = strings.TrimSpace(v); v != "" {
			params[k] = v
		}
	}
	return params
}

// emitGetDryRun mostra la ricerca che aggancia il documento.
//
// Prima stampava un URL composto a mano: `doc221-1.jsp?icaDocId=N&legisl=18&numero=1185`
// — con una `N` letterale al posto dell'id, e due parametri che quell'URL non
// porta. Non era una richiesta diversa da quella vera: non era una richiesta.
// E taceva che `get` fa due passi, di cui il secondo dipende dal primo.
func emitGetDryRun(cmd *cobra.Command, arc icaro.Archive, legisl, numero int, params map[string]string) error {
	target, err := dryRunTargetBySlug(arc.Slug, normalizeParams(arc, params))
	if err != nil {
		return err
	}
	if target == nil {
		return fmt.Errorf("archivio %s non disponibile", arc.Slug)
	}
	nota := "aggancia il documento, poi ne scarica la scheda: l'URL della scheda contiene l'id che questa ricerca restituisce, quindi non e' anteprimabile."
	if icaro.IsBDArchive(arc.Slug) {
		nota += " Su questo archivio la ricerca e' forzata sull'indice Icaro (serve l'id del documento); se l'indice non ha il record, `get` ripiega sulla scheda del backend /bd/ e restituisce `pdf_url`."
	}
	return emitDryRunRequests(cmd, []map[string]any{target}, nota)
}

// normalizeParams rewrites a few flag inputs to the shape the portal expects:
//   - dates given as YYYY-MM-DD become AAMMGG (the 6-digit numeric form the
//     ISIS date fields store, e.g. DATPRE/DATSED); a range YYYY-MM-DD:YYYY-MM-DD
//     becomes AAMMGG/AAMMGG (ISIS interval syntax)
//   - on ddl, a bare --anno year becomes a DATPRE Jan-1..Dec-31 AAMMGG range
//     (ddl has no year field to qualify a plain year against)
//   - a numeric commission code (--codcom 1..6) is rerouted to the COMMIS field
//     as its Roman ordinal name, since the upstream CODCOM field is not indexed
//   - whitespace is trimmed
func normalizeParams(arc icaro.Archive, in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		switch {
		case k == "data":
			v = toISISDate(v)
		case k == "anno" && arc.Slug == "ddl":
			// ddl has no year field (unlike leggi.LEGANN/resoconti.ANNSED):
			// --anno is qualified on DATPRE (presentation date) as a
			// Jan-1..Dec-31 range. See archives.go's ddl.FieldMap["anno"].
			v = yearToISISRange(v)
		}
		out[k] = v
	}
	// --codcom 1..6 has no working upstream field; reroute to COMMIS by name.
	if code, ok := out["codcom"]; ok {
		if name := commissioneOrdinale(code); name != "" {
			delete(out, "codcom")
			if out["commissione"] == "" {
				out["commissione"] = name
			}
		}
	}
	return out
}

// toISISDate converts a date (or a date range) to the 6-digit AAMMGG form the
// ISIS engine stores. Accepts YYYY-MM-DD and ranges as YYYY-MM-DD:YYYY-MM-DD.
// Values it cannot parse are returned unchanged (so already-AAMMGG input or
// raw expressions still pass through).
func toISISDate(v string) string {
	// Emit a range only when BOTH bounds convert to a valid AAMMGG date.
	// Otherwise (empty, non-numeric, or one-sided bound) fall through to the
	// single-value path so we never produce a malformed "260225/" or
	// "260225/garbage" range expression.
	if lo, hi, isRange := strings.Cut(v, ":"); isRange {
		loC, hiC := aammgg(lo), aammgg(hi)
		if isAAMMGG(loC) && isAAMMGG(hiC) {
			return loC + "/" + hiC
		}
		return aammgg(v)
	}
	return aammgg(v)
}

// yearToISISRange converts a bare 4-digit year to a DATPRE-style AAMMGG range
// spanning the whole year (Jan 1 to Dec 31), e.g. "2024" -> "240101/241231".
// Anything else (already-AAMMGG input, a range from --isis-query, garbage)
// passes through unchanged rather than producing a malformed expression.
func yearToISISRange(v string) string {
	if len(v) != 4 || !isDigits(v) {
		return v
	}
	yy := v[2:]
	return yy + "0101/" + yy + "1231"
}

func aammgg(v string) string {
	v = strings.TrimSpace(v)
	iso := strings.SplitN(v, "-", 3)
	if len(iso) == 3 && len(iso[0]) == 4 && len(iso[1]) == 2 && len(iso[2]) == 2 {
		// Verify all components are numeric before accepting as a date, so the
		// documented pass-through guarantee holds for non-date input.
		if isDigits(iso[0]) && isDigits(iso[1]) && isDigits(iso[2]) {
			return iso[0][2:] + iso[1] + iso[2]
		}
	}
	return v
}

// isAAMMGG reports whether s is the 6-digit numeric date form the ISIS engine
// stores (e.g. "260225").
func isAAMMGG(s string) bool { return len(s) == 6 && isDigits(s) }

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// commissioneOrdinale maps a numeric commission code to its Roman ordinal name
// as stored in the COMMIS field. Returns "" for unknown codes.
func commissioneOrdinale(code string) string {
	switch strings.TrimSpace(code) {
	case "1":
		return "PRIMA"
	case "2":
		return "SECONDA"
	case "3":
		return "TERZA"
	case "4":
		return "QUARTA"
	case "5":
		return "QUINTA"
	case "6":
		return "SESTA"
	}
	return ""
}

// dryRunTarget descrive, nella lingua del backend che serve davvero
// l'archivio, la richiesta che il comando farebbe.
//
// Gli archivi delle sedute (sommari, resoconti, convocazioni) sono migrati al
// backend /bd/ e Search ce li instrada (vedi icaroclient.Client.Search), ma
// l'anteprima li descriveva tutti come query Icaro: `resoconti cerca --dry-run`
// annunciava `/icaro/default.jsp?icaDB=217&icaQuery=(18.LEGISL)`, un URL che
// quel comando non interroga. Su un flag che esiste per diagnosticare, un
// endpoint sbagliato detto con sicurezza è peggio del silenzio: manda a
// cercare il guasto sul backend che non c'entra.
func dryRunTarget(arc icaro.Archive, params map[string]string, isisRaw string) (map[string]any, error) {
	out := map[string]any{"archive": arc.Slug, "archive_id": arc.ID}
	if bd, ok := icaro.BDPreview(icaro.DefaultBaseURL, arc.Slug, params); ok {
		// Un filtro che non si parsa fa fallire searchBD prima di qualunque
		// richiesta: l'anteprima esce con lo stesso errore invece di stampare
		// una richiesta che non partirebbe.
		if bd.Invalid != nil {
			return nil, usageErr(bd.Invalid)
		}
		// Su /bd/ non c'è una query ISIS: i filtri viaggiano come campi di una
		// POST. Ma non viaggiano come li scrive l'utente — i nomi cambiano, i
		// selettori di modalità si aggiungono, e tre filtri si risolvono solo
		// al momento della richiesta. Stamparli come sono arrivati sarebbe la
		// stessa bugia dell'endpoint sbagliato, un livello più giù: vedi
		// icaroclient.BDPreview.
		out["backend"] = "bd"
		out["would_post"] = bd.Endpoint
		out["post_fields"] = bd.PostFields
		if len(bd.Anni) > 0 {
			// Con --data non parte una richiesta: ne parte una per anno, e
			// dentro ciascuna una per pagina. Enumerarli e' l'unico modo perche'
			// da un dry run si capisca quante ne partono e come rifarle a mano.
			out["anni"] = bd.Anni
			out["richieste"] = fmt.Sprintf("almeno %d, una per anno con `page` a 1; dentro ciascun anno `page` cresce fino al numero di pagine che la risposta dichiara, o finché --limit è pieno", len(bd.Anni))
		}
		if len(bd.Deferred) > 0 {
			out["deferred"] = bd.Deferred
		}
		return out, nil
	}
	expr := icaro.BuildQuery(arc, params, isisRaw)
	out["backend"] = "icaro"
	out["isis_query"] = expr
	out["would_fetch"] = fmt.Sprintf("%s/icaro/default.jsp?icaDB=%s&icaQuery=%s", icaro.DefaultBaseURL, arc.ID, expr)
	return out, nil
}

// dryRunTargetBySlug è dryRunTarget per i comandi che conoscono l'archivio per
// slug; torna nil sugli slug sconosciuti, così il chiamante li salta come li
// salterebbe a runtime.
//
// I parametri NON vengono normalizzati qui: normalizeParams riscrive i valori
// (fra l'altro dirotta `codcom: 6` su `commissione: SESTA`) e non tutti i
// chiamanti ci passano — `commissione dossier` manda `codcom` grezzo al
// backend /bd/. Applicarlo d'ufficio farebbe annunciare all'anteprima un
// parametro diverso da quello che parte davvero, cioè il difetto che questa
// anteprima esiste per non avere. Chi normalizza a runtime lo fa anche qui.
func dryRunTargetBySlug(slug string, params map[string]string) (map[string]any, error) {
	arc := icaro.BySlug(slug)
	if arc == nil {
		return nil, nil
	}
	return dryRunTarget(*arc, params, "")
}

// emitDryRunRequests stampa l'anteprima dei comandi che interrogano più di un
// archivio: una riga per richiesta, nell'ordine in cui partirebbero.
func emitDryRunRequests(cmd *cobra.Command, requests []map[string]any, note string) error {
	out := map[string]any{"dry_run": true, "requests": requests}
	if note != "" {
		out["note"] = note
	}
	return writeJSON(cmd.OutOrStdout(), out)
}

// emitDryRun prints the would-be query without hitting the network, useful
// for --dry-run flows and Printing Press verify checks.
func emitDryRun(cmd *cobra.Command, arc icaro.Archive, p cercaParams) error {
	// Stessa condizione di runCerca, e per lo stesso motivo: gli archivi /bd/
	// ricevono i parametri grezzi, la loro traduzione avviene dentro searchBD.
	// Applicare normalizeParams qui faceva annunciare all'anteprima un
	// parametro diverso da quello che il comando processa — `--codcom 6`
	// riscritto in `commissione: SESTA`, che su /bd/ non e' cio' che viaggia.
	// Se questa riga e quella di runCerca divergono di nuovo, l'anteprima
	// ricomincia a mentire: vanno lette insieme.
	searchParams := p.Params
	if !icaro.IsBDArchive(arc.Slug) {
		searchParams = normalizeParams(arc, p.Params)
	}
	out, err := dryRunTarget(arc, searchParams, p.ISISRaw)
	if err != nil {
		return err
	}
	out["dry_run"] = true
	return writeJSON(cmd.OutOrStdout(), out)
}

// emitRecords prints search records honoring --json/--csv/table formats.
// When the user did not pass --json explicitly and stdout is a TTY, we
// produce a small table; otherwise we default to JSON for pipe friendliness.
// firmatari, when non-nil, carries the full signatory list per doc ID (see
// firmatariByDoc); a nil map simply leaves the field out.
func emitRecords(cmd *cobra.Command, flags *rootFlags, arc icaro.Archive, recs []icaro.Record, firmatari map[int][]firmatario) error {
	out := cmd.OutOrStdout()
	asJSON := flags.asJSON || (!isTerminal(out) && !flags.csv && !flags.quiet && !flags.plain)
	if asJSON {
		// printJSONFiltered (not the bare writeJSON) so --select/--compact
		// behave the same as on generator-emitted commands — writeJSON
		// always dumped the full array regardless of --select.
		return printJSONFiltered(out, flatRecords(recs, firmatari), flags)
	}
	if flags.csv {
		return writeRecordsCSV(out, arc, recs, firmatari)
	}
	// Table view (default for TTY).
	if len(recs) == 0 {
		fmt.Fprintln(out, "Nessun risultato.")
		return nil
	}
	for _, r := range recs {
		if r.DocID > 0 {
			fmt.Fprintf(out, "#%d  %s\n", r.DocID, r.Title)
		} else {
			// Righe /bd/: nessun DocID Icaro da mostrare (vedi emitRecords).
			fmt.Fprintf(out, "%s\n", r.Title)
		}
		for i, col := range arc.Columns {
			if i == len(arc.Columns)-1 {
				continue // last col is the title block, already printed
			}
			if v, ok := r.Fields[col]; ok {
				fmt.Fprintf(out, "  %-10s %s\n", col, v)
			}
		}
		if r.Excerpt != "" {
			fmt.Fprintf(out, "  %s\n", r.Excerpt)
		}
		if f, ok := firmatari[r.DocID]; ok {
			fmt.Fprintf(out, "  %-10s %s\n", "Firmatari", firmatariLine(f))
		}
		fmt.Fprintln(out)
	}
	return nil
}

// titoloAlias torna il titolo del record nel nome con cui lo chiamano gli
// altri archivi (`titolo`, non `title`). Sugli archivi Icaro il campo ISIS
// "Titolo" arriva già dentro Fields e coincide con Record.Title; sui tre
// archivi /bd/ (resoconti, sommari, convocazioni) quel campo non esiste nel
// parsing e senza fallback la colonna/chiave "titolo" resterebbe vuota
// mentre "title" è popolato — vedi docs/news-agent/2026-08-07_08-43.md.
func titoloAlias(r icaro.Record) string {
	if v := r.Fields["Titolo"]; v != "" {
		return v
	}
	return r.Title
}

// flatRecords converts records to the flat JSON shape
// {doc_id, title, titolo, excerpt, url, <fields...>}. Estratta da emitRecords
// perché la serve anche il percorso --envelope, che deve filtrare i
// risultati dentro la busta invece di stamparli direttamente.
func flatRecords(recs []icaro.Record, firmatari map[int][]firmatario) []map[string]any {
	flat := make([]map[string]any, 0, len(recs))
	for _, r := range recs {
		row := map[string]any{
			"title":   r.Title,
			"excerpt": r.Excerpt,
			"url":     r.URL,
		}
		// Le righe del backend /bd/ non hanno un DocID Icaro (vedi
		// parseBDRow): esporre lo zero come identificativo farebbe
		// credere che ogni record sia il documento #0. Meglio assente.
		if r.DocID > 0 {
			row["doc_id"] = r.DocID
		}
		for k, v := range r.Fields {
			row[strings.ToLower(strings.TrimSuffix(k, "."))] = v
		}
		row["titolo"] = titoloAlias(r)
		if f, ok := firmatari[r.DocID]; ok {
			row["firmatari"] = f
		}
		flat = append(flat, row)
	}
	return flat
}

// firmatariLine renders a signatory list as "Nome (Gruppo); Nome (Gruppo)"
// for the flat table and CSV views.
func firmatariLine(f []firmatario) string {
	parts := make([]string, 0, len(f))
	for _, x := range f {
		if x.Gruppo != "" {
			parts = append(parts, x.Nome+" ("+x.Gruppo+")")
			continue
		}
		parts = append(parts, x.Nome)
	}
	return strings.Join(parts, "; ")
}

func writeRecordsCSV(out io.Writer, arc icaro.Archive, recs []icaro.Record, firmatari map[int][]firmatario) error {
	// Header. Unnamed columns are portal placeholders (see Archive.Columns)
	// and get no CSV column of their own.
	cols := make([]string, 0, len(arc.Columns))
	for _, c := range arc.Columns {
		if c != "" {
			cols = append(cols, c)
		}
	}
	hdr := []string{"doc_id", "title", "excerpt", "url"}
	for _, c := range cols {
		nome := strings.ToLower(strings.TrimSuffix(c, "."))
		hdr = append(hdr, nome)
		// Il CSV è la forma con cui questi dati finiscono in duckdb o in un
		// foglio, ed è lì che le quattro grafie di data della fonte costano di
		// più: la colonna normalizzata viaggia accanto all'originale.
		if nome == "data" {
			hdr = append(hdr, "data_iso")
		}
	}
	if firmatari != nil {
		hdr = append(hdr, "firmatari")
	}
	for i, h := range hdr {
		if i > 0 {
			fmt.Fprint(out, ",")
		}
		fmt.Fprint(out, csvEscape(h))
	}
	fmt.Fprintln(out)
	for _, r := range recs {
		docID := ""
		if r.DocID > 0 { // righe /bd/: colonna vuota, non uno zero fittizio
			docID = strconv.Itoa(r.DocID)
		}
		row := []string{docID, r.Title, r.Excerpt, r.URL}
		for _, c := range cols {
			if c == "Titolo" {
				row = append(row, titoloAlias(r))
				continue
			}
			row = append(row, r.Fields[c])
			if strings.ToLower(strings.TrimSuffix(c, ".")) == "data" {
				row = append(row, dataISO(r.Fields[c]))
			}
		}
		if firmatari != nil {
			row = append(row, firmatariLine(firmatari[r.DocID]))
		}
		for i, v := range row {
			if i > 0 {
				fmt.Fprint(out, ",")
			}
			fmt.Fprint(out, csvEscape(v))
		}
		fmt.Fprintln(out)
	}
	return nil
}

func csvEscape(s string) string {
	if strings.ContainsAny(s, ",\"\n\r") {
		return "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\""
	}
	return s
}

func writeJSON(out io.Writer, v any) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// cliIsVerify mirrors cliutil.IsVerifyEnv so callers can short-circuit
// outbound network calls during Printing Press verify runs.
func cliIsVerify() bool {
	return cliutil.IsVerifyEnv()
}

// itoa is a tiny shorthand so cerca-wrapper commands don't need strconv.
func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}

// atoiArg parses a positional CLI argument as an int, returning a
// human-friendly Italian error when the input is malformed.
func atoiArg(s, name string) (int, error) {
	var n int
	if _, err := fmt.Sscanf(strings.TrimSpace(s), "%d", &n); err != nil {
		return 0, fmt.Errorf("argomento %q non valido (atteso numero intero): %s", name, s)
	}
	return n, nil
}
