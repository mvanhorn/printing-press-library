// pp:data-source live
// pp:client-call
// Novel feature — ricostruisce la cronologia di un DDL leggendo il blocco
// «Iter Storico» della sua scheda nell'archivio DDL (221), l'unico che questo
// comando interroga.
//
// La testata dichiarava di combinare 221 con i sommari di commissione (230) e i
// resoconti d'aula (217): non è mai stato vero, nel file c'è solo `BySlug("ddl")`.
// Chi lo leggeva concludeva che un iter troncato dipendesse da altro, mentre
// dipende proprio dal fatto che 230 non viene guardato — e l'Iter Storico può
// essere indietro rispetto ai lavori reali (issue #3: sul ddl 18/779 l'iter si
// ferma al 10/06/2026 mentre i sommari della I commissione arrivano al 22/07).

package cli

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	icaro "github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/internal/icaroclient"
	"github.com/spf13/cobra"
)

func newNovelDdlIterCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "iter <legisl> <numero>",
		Short:   "Ricostruisce la cronologia completa di un disegno di legge: presentazione, commissione, aula, eventuale legge.",
		Example: "  ars-sicilia-pp-cli ddl iter 18 1500 --json",
		Args:    cobra.MaximumNArgs(2),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "legisl=18;numero=1153",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				if dryRunOK(flags) || cliIsVerify() {
					return cmd.Help()
				}
				return usageErr(fmt.Errorf("richiesti 2 argomenti: <legisl> e <numero>"))
			}
			legisl, errL := atoiArg(args[0], "legisl")
			numero, errN := atoiArg(args[1], "numero")
			if errL != nil || errN != nil {
				// Come in `legge cronologia`: sotto --dry-run e sotto verify gli
				// argomenti possono essere segnaposto, e una sonda non deve
				// uscire in errore per averli letti. Si ripiega sull'help, come
				// il ramo degli argomenti mancanti qui sopra.
				if dryRunOK(flags) || cliIsVerify() {
					return cmd.Help()
				}
				if errL != nil {
					return errL
				}
				return errN
			}
			if dryRunOK(flags) {
				return emitDdlIterDryRun(cmd, legisl, numero)
			}
			return runDdlIter(cmd, flags, legisl, numero)
		},
	}
	return cmd
}

type iterEvent struct {
	Fase   string `json:"fase"`
	Data   string `json:"data,omitempty"`
	Sede   string `json:"sede,omitempty"`
	Titolo string `json:"titolo,omitempty"`
	// Seduta è il numero della seduta in cui l'evento è avvenuto, quando il
	// portale lo dichiara. Prima veniva tagliato via insieme al resto della
	// riga (vedi parseIterFromBody): senza, dall'iter non si può risalire alla
	// seduta in cui un ddl è stato votato, che è la domanda più frequente su
	// un atto — e la data dell'evento da sola si confonde con la data in cui
	// la notizia è stata scritta, che è quasi sempre il giorno dopo.
	Seduta  int    `json:"seduta,omitempty"`
	Oratori string `json:"oratori,omitempty"`
	// Anomalia marca un evento la cui coppia seduta↔data non può essere giusta
	// come la fonte la dichiara: vedi sedutePerDataIncoerenti (stessa data,
	// sedute diverse) e seduteConDateIncoerenti (stessa seduta, date diverse).
	// Senza il marcatore l'evento esce indistinguibile da uno sano, e chi lo
	// incrocia per data nell'archivio resoconti conclude «resoconto mancante»
	// invece di «la fonte si contraddice»: un falso buco a valle.
	Anomalia bool `json:"anomalia,omitempty"`
	// URL è la fonte PIÙ SPECIFICA che si conosce per questo evento: per un
	// passaggio in Aula di cui si sa la seduta, la scheda del resoconto; per
	// tutti gli altri, la scheda dell'atto da cui l'evento è stato letto.
	// `legge cronologia` popola già questo campo per-evento; `ddl iter`
	// ripeteva la stessa scheda su ogni riga perché è da lì che li parsa.
	// Il numero di seduta è l'id della scheda del resoconto (verificato su
	// leg. XVII e XVIII; una seduta inesistente risponde 404, non una pagina
	// vuota), quindi l'URL si costruisce senza una richiesta in più.
	URL       string `json:"url,omitempty"`
	ArchiveID string `json:"archive_id,omitempty"`
	DocID     int    `json:"doc_id,omitempty"`
	// sedutaAula distingue la seduta d'Aula da quella di commissione: le due
	// numerazioni sono indipendenti, quindi solo il marcatore del portale dice
	// a quale serie appartiene il numero. Non esce nel JSON — serve al
	// chiamante per decidere se esiste un resoconto da linkare, e chi legge
	// l'output lo deduce dalla presenza dell'URL.
	sedutaAula bool
}

type iterReport struct {
	Legisl int    `json:"legisl"`
	Numero int    `json:"numero"`
	Titolo string `json:"titolo,omitempty"`
	// URL è la scheda dell'atto di cui si racconta la storia: il ddl per
	// `ddl iter`, la legge per `legge cronologia`. Stava solo dentro ogni
	// evento, ripetuta identica, e nella radice del report — dove uno la
	// cerca — non c'era.
	URL string `json:"url,omitempty"`
	// DdlOriginari sono i numeri dei disegni di legge da cui la legge nasce,
	// popolati dalla sola `legge cronologia`. Il numero c'era già, ma dentro la
	// frase `sede` dell'evento ddl_originario ("Disegno di legge n. 239"):
	// leggibile da un umano, da estrarre con una regex per chi vuole
	// incatenarci `ddl iter`, che è l'unica cosa che si fa con quel numero. È
	// una lista perché una legge può nascere da più ddl abbinati.
	DdlOriginari []int        `json:"ddl_originari,omitempty"`
	Stralcio     *stralcioOut `json:"stralcio,omitempty"`
	Eventi       []iterEvent  `json:"eventi"`
	Note         string       `json:"note,omitempty"`
}

func runDdlIter(cmd *cobra.Command, flags *rootFlags, legisl, numero int) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	c, err := icaro.New(nil)
	if err != nil {
		return err
	}
	report := iterReport{Legisl: legisl, Numero: numero, Eventi: []iterEvent{}}

	// 1. DDL stesso (archivio 221): presentazione + apertura del documento per
	// leggere l'iter reale (sezione "Attuale … Storico" nel corpo del doc).
	arc := icaro.BySlug("ddl")
	if arc == nil {
		report.Note = "archivio ddl non disponibile"
		return emitIter(cmd, flags, report)
	}
	recs, err := c.Search(ctx, *arc, icaro.SearchOptions{
		Params: ddlIterSearchParams(legisl, numero),
		Limit:  1,
	})
	if err != nil {
		return fmt.Errorf("ricerca ddl: %w", err)
	}
	if len(recs) == 0 {
		report.Note = fmt.Sprintf("DDL %d non trovato nell'archivio della legislatura %d. Verifica legisl e numero con `ars-sicilia-pp-cli ddl cerca`.", numero, legisl)
		return emitIter(cmd, flags, report)
	}
	report.Titolo = recs[0].Title
	report.URL = recs[0].URL
	// Se il ddl è uno stralcio, dirlo prima della cronologia: l'iter di uno
	// stralcio comincia a metà storia, e senza il rimando al ddl base si legge
	// come un atto nato dal nulla.
	report.Stralcio = stralcioDaTesti(numero, recs[0].Excerpt)
	report.Eventi = append(report.Eventi, iterEvent{
		Fase:      "presentazione",
		Data:      recs[0].Fields["Data"],
		Sede:      "Assemblea (presentazione DDL)",
		Titolo:    recs[0].Title,
		URL:       recs[0].URL,
		ArchiveID: arc.ID,
		DocID:     recs[0].DocID,
	})

	// 2. Iter reale: preferisce il blocco HTML etichettato "Iter" che il
	// portale rende separato dal testo del disegno di legge (doc.Fields),
	// con fallback sul corpo del documento solo se quel campo manca. Vedi
	// docIterEvents.
	if doc, derr := c.GetDoc(ctx, *arc, recs[0].DocID); derr == nil {
		// La scheda porta "Riferimenti", più affidabile dell'excerpt della
		// short-list (che il portale a volte restituisce vuoto).
		if s := stralcioDaTesti(numero, doc.Fields["Riferimenti"]); s != nil {
			report.Stralcio = s
		}
		evs := docIterEvents(doc)
		// L'Aula tiene una seduta per data e una seduta ha una data sola:
		// entrambi i versi della coerenza seduta↔data si controllano qui,
		// vedi marcaEventiIncoerenti.
		incoerenti, avviso := marcaEventiIncoerenti(cmd, legisl, evs)
		report.Note = uniscoNote(report.Note, avviso)
		for _, ev := range evs {
			ev.URL = recs[0].URL
			ev.ArchiveID = arc.ID
			ev.DocID = recs[0].DocID
			// Solo l'Aula ha resoconti: i lavori di commissione producono
			// sommari, che stanno su un altro archivio e non su questa rotta.
			// La discriminante è il marcatore del portale (sedutaAula), NON la
			// fase dell'evento: "Esitato per Aula (epa) Seduta n. 260 0400
			// Commissione QUARTA" è un evento di fase aula che cita una seduta
			// di commissione, e linkarne il resoconto porterebbe alla seduta
			// d'Aula n. 260, che è un'altra cosa.
			// Dove il resoconto esiste è la fonte giusta dell'evento e prende
			// il posto della scheda del ddl, che resta nella radice del report.
			if ev.sedutaAula && !incoerenti[ev.Data] && !ev.Anomalia {
				if u := resocontoSchedaURL(legisl, ev.Seduta); u != "" {
					ev.URL = u
					ev.ArchiveID = ""
					ev.DocID = 0
				}
			}
			report.Eventi = append(report.Eventi, ev)
		}
	}

	// Sort events by ISO date when parseable.
	sort.SliceStable(report.Eventi, func(i, j int) bool {
		return iterDateKey(report.Eventi[i].Data) < iterDateKey(report.Eventi[j].Data)
	})
	report.Note = uniscoNote(report.Note, avvisoStralcioAnteriore(report))
	report.Note = uniscoNote(report.Note, avvisoApprovatoSenzaGurs(report.Eventi))
	return emitIter(cmd, flags, report)
}

// emitDdlIterDryRun previews the ddl iter request without sending it. Unlike
// the silent no-op this used to be, it mirrors the ISIS-query preview that
// `*/cerca` commands already show — ddl iter's --dry-run should be as useful
// a diagnostic as the rest of the CLI.
// ddlIterSearchParams sono i parametri con cui si aggancia il ddl, in un posto
// solo: anteprima e ricerca vera devono partire dagli stessi.
func ddlIterSearchParams(legisl, numero int) map[string]string {
	return map[string]string{"legisl": itoa(legisl), "numero": itoa(numero)}
}

func emitDdlIterDryRun(cmd *cobra.Command, legisl, numero int) error {
	arc := icaro.BySlug("ddl")
	if arc == nil {
		return fmt.Errorf("archivio ddl non disponibile")
	}
	expr := icaro.BuildQuery(*arc, ddlIterSearchParams(legisl, numero), "")
	out := map[string]any{
		"archive":     arc.Slug,
		"archive_id":  arc.ID,
		"isis_query":  expr,
		"would_fetch": fmt.Sprintf("%s/icaro/default.jsp?icaDB=%s&icaQuery=%s", icaro.DefaultBaseURL, arc.ID, expr),
		"note":        "pins the DDL via this query, then fetches its document body to parse the iter timeline",
		"dry_run":     true,
	}
	return writeJSON(cmd.OutOrStdout(), out)
}

func emitIter(cmd *cobra.Command, flags *rootFlags, report iterReport) error {
	out := cmd.OutOrStdout()
	if flags.asJSON || !isTerminal(out) {
		return printJSONFiltered(out, report, flags)
	}
	fmt.Fprintf(out, "DDL %d/%d — %s\n", report.Legisl, report.Numero, report.Titolo)
	if report.Note != "" {
		fmt.Fprintf(out, "  %s\n", report.Note)
	}
	for _, e := range report.Eventi {
		fmt.Fprintf(out, "  [%s] %s — %s\n", e.Fase, e.Data, strings.TrimSpace(e.Sede+" "+e.Titolo))
	}
	return nil
}

// reIterDate matches an Italian short date "<DD> <mese> <YYYY>" used to anchor
// each iter step in the document status block.
var reIterDate = regexp.MustCompile(`(\d{1,2})\s+([a-zàèéìòù]{3,})\s+(\d{4})`)

// reRunVirgolette matches the portal's stray quote runs. Two or more, never
// one: a single quote can be real punctuation, a run of fourteen is the
// rendering quirk. See its use in parseIterFromBody.
var reRunVirgolette = regexp.MustCompile(`"{2,}`)

// reLrAnnotation matches the portal's raw law-registration annotation
// ("Lr <giorno> <mese> alr <anno> nlr <numero> Titolo : ...") that appears as
// a DDL's own iter event once it is promulgated. Everything after "Titolo :"
// just repeats the bill's title — sometimes mangled with runs of stray quote
// characters, a portal rendering quirk — and duplicates information already
// carried by year+numero, so it is dropped in favor of a short, correctly
// classified event (see its use in parseIterFromBody).
var reLrAnnotation = regexp.MustCompile(`(?i)^Lr\s+\d{1,2}\s+\S+\s+alr\s+(\d{4})\s+nlr\s+(\d+)\b`)

var itaMonths = map[string]string{
	"gen": "01", "feb": "02", "mar": "03", "apr": "04", "mag": "05", "giu": "06",
	"lug": "07", "ago": "08", "set": "09", "ott": "10", "nov": "11", "dic": "12",
}

// parseIterFromBody extracts iter events (committee assignment, aula passage,
// approval/rejection, promulgation) from the status block the portal renders at
// the top of a DDL document body. The block runs from "Attuale" up to the bill
// header "(n. …)" and contains both the current status and, after the "Storico"
// label, the full chronological history. Each step is "<date> <action> [Seduta
// n. N …]"; we cut the action at "Seduta" to drop the sitting metadata (and its
// stray digits). The raw "Lr ... alr ... nlr ..." law-registration annotation
// (see reLrAnnotation) is reduced to a short, correctly classified event.
// Returns nil when no status block is present.
func parseIterFromBody(body string) []iterEvent {
	if body == "" {
		return nil
	}
	start := strings.Index(body, "Attuale")
	if start < 0 {
		return nil
	}
	region := body[start+len("Attuale"):]
	// The bill text proper begins either with the "(n. <numero>)" header or,
	// for records lacking that header (e.g. the finanziaria and other
	// governativi that open straight into the masthead), with the fixed
	// "ASSEMBLEA REGIONALE SICILIANA" line. Cut the region at whichever comes
	// first; everything after it is document content, not iter. Without this,
	// long articolati leak into the region and dates cited inside the bill
	// text (e.g. "3 luglio 1950, n. 51") get parsed as iter events.
	cutAt := -1
	for _, marker := range []string{"(n.", "ASSEMBLEA REGIONALE SICILIANA"} {
		if i := strings.Index(region, marker); i >= 0 && (cutAt < 0 || i < cutAt) {
			cutAt = i
		}
	}
	if cutAt >= 0 {
		region = region[:cutAt]
	}
	// "Storico" is a section label between current status and history, not an event.
	region = strings.ReplaceAll(region, "Storico", " ")

	locs := reIterDate.FindAllStringIndex(region, -1)
	subs := reIterDate.FindAllStringSubmatch(region, -1)
	var events []iterEvent
	// dopoLr tiene la data della promulgazione finché le date che seguono la
	// precedono: sono quelle citate dentro il titolo della legge. Vuota fuori da
	// quella finestra. Vedi il suo uso più sotto.
	dopoLr := ""
	for i, loc := range locs {
		dd, mon, yyyy := subs[i][1], strings.ToLower(subs[i][2]), subs[i][3]
		actEnd := len(region)
		if i+1 < len(locs) {
			actEnd = locs[i+1][0]
		}
		action := region[loc[1]:actEnd]
		// Una data che segue una preposizione pendente appartiene alla frase, non
		// è l'inizio dell'evento dopo: «Pubblicazione Gurs n. 10 del 24 febbraio
		// 2026» veniva tagliata a «del», e la data della pubblicazione — la sola
		// cosa che quell'evento aggiunge alla promulgazione — si perdeva, mentre
		// l'evento che ne restava (azione vuota) cadeva subito dopo. Si riattacca
		// la sola data, non tutto il testo fino alla successiva: l'evento i+1
		// resta al suo posto con ciò che segue, e se non segue nulla sparisce
		// come prima. Sui 18 iter di controllo le uniche azioni che finiscono
		// con una preposizione pendente sono le 13 pubblicazioni in Gurs.
		if i+1 < len(locs) && terminaConPreposizione(action) {
			action += region[locs[i+1][0]:locs[i+1][1]]
		}
		// Le run di virgolette sono un artefatto di resa del portale, non
		// contenuto: la fonte emette davvero `ddl"""""""""""""" 229`,
		// `Seduta"""""""""""""" n. 35` e `Commissione QUARTA""""""""""""""`.
		// Si tolgono qui, sull'azione già ritagliata, e non prima sulla regione:
		// una di quelle run sta dentro il titolo di una legge citata
		// («legge regionale 31"""""""""""""" gennaio 2024, n. 3», ddl 738), e
		// toglierla prima farebbe affiorare una data che reIterDate leggerebbe
		// come un evento in più, datato 2024, in mezzo all'iter del 2025.
		action = reRunVirgolette.ReplaceAllString(action, "")
		// Il numero di seduta si legge PRIMA di tagliare: è l'unico posto in
		// cui il portale lo dichiara, e prima finiva nel pezzo scartato.
		seduta, sedutaAula := sedutaDaAzione(action)
		// Anche la commissione sta nel suffisso di seduta, e va letta prima del
		// taglio per la stessa ragione: vedi sedeDaSuffissoSeduta.
		sedeSuffisso := sedeDaSuffissoSeduta(action)
		if s := indiceSeduta(action); s >= 0 {
			action = action[:s]
		}
		action = strings.Join(strings.Fields(action), " ")
		if action == "" {
			continue
		}
		data := fmt.Sprintf("%s %s %s", dd, mon, yyyy)
		if m := reLrAnnotation.FindStringSubmatch(action); m != nil {
			action = fmt.Sprintf("Promulgata legge regionale n. %s/%s", m[2], m[1])
			// Il titolo che segue "Titolo :" cita quasi sempre le norme
			// modificate, con le loro date: quelle date non sono eventi
			// dell'iter, ma reIterDate le aggancia lo stesso e il pezzo di
			// titolo che le segue diventa un evento inesistente in cima alla
			// cronologia — «23 giugno 2011 — ", n. 118. D.F.B. 2023. Mese di
			// febbraio"» sul ddl 350, dodici anni prima dell'atto. Da qui in poi
			// si scarta finché le date tornano indietro: la prima che non
			// precede la promulgazione è di nuovo l'iter vero (la pubblicazione
			// in Gurs), e chiude la finestra.
			dopoLr = iterDateKey(data)
		} else if dopoLr != "" {
			if iterDateKey(data) < dopoLr {
				continue
			}
			dopoLr = ""
		}
		events = append(events, iterEvent{
			Fase:       classifyIterFase(action),
			Data:       data,
			Sede:       iterSedeRisolta(sedeSuffisso, action),
			Titolo:     action,
			Seduta:     seduta,
			sedutaAula: sedutaAula,
		})
	}
	return events
}

// rePreposizionePendente matches an action that breaks off on a preposition
// introducing a date ("Pubblicazione Gurs n. 10 del"). Only that: an action
// ending on a word means the step is complete, and the next date starts the
// next step.
var rePreposizionePendente = regexp.MustCompile(`(?i)\b(del|dal|nel)\s*$`)

// terminaConPreposizione reports whether the date that follows this action
// belongs to its sentence rather than opening the next event.
func terminaConPreposizione(action string) bool {
	return rePreposizionePendente.MatchString(action)
}

// reSeduta matches the portal's sitting reference inside an iter step, plus
// the marker that follows it. The case-insensitive flag is deliberate: the
// portal writes both "Seduta n. 64" and "seduta n. 114", and the old
// fixed-case cut left the lowercase form inside the event title — same data,
// two renderings, depending on chance. The quote run in the character class
// is not decorative: the portal really does emit
// `Seduta"""""""""""""" n. 35` on some rows.
//
// The trailing group is what tells an Aula sitting from a committee one:
//
//	Seduta n. 261 AULA                      -> Aula
//	Seduta n. 260 0400 Commissione QUARTA   -> IV Committee
//
// The two numbering series are independent, so the marker cannot be guessed
// from the event phase: "Esitato per Aula (epa) Seduta n. 260 0400 Commissione
// QUARTA" classifies as an aula event but cites a COMMITTEE sitting, and
// building a resoconto URL from it lands on the unrelated Aula sitting n. 260
// — a link that resolves and shows the wrong document.
var reSeduta = regexp.MustCompile(`(?i)\bseduta"*\s+n\.?\s*(\d+)\s*([a-z0-9]*)`)

// indiceSeduta returns where the sitting metadata starts in an iter action, or
// -1. Case-insensitive counterpart of the old strings.Index(action, "Seduta").
func indiceSeduta(action string) int {
	if loc := reSeduta.FindStringIndex(action); loc != nil {
		return loc[0]
	}
	return -1
}

// sedutaDaAzione extracts the sitting number from an iter action and reports
// whether it was an Aula sitting (as opposed to a committee one). Returns
// (0, false) when the portal declared no sitting.
func sedutaDaAzione(action string) (int, bool) {
	m := reSeduta.FindStringSubmatch(action)
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, false
	}
	return n, strings.EqualFold(m[2], "AULA")
}

// resocontoSchedaURL builds the /bd/ sitting-record URL from legislature and
// sitting number. The sitting number IS the scheda id: verified on leg. XVII
// (n. 114 → 07/05/2019, n. 149/150 → 05-06/11/2019) and leg. XVIII (n. 267 →
// 28/07/2026); a non-existent sitting returns 404 rather than an empty page,
// so a constructed URL either resolves or fails visibly.
func resocontoSchedaURL(legisl, seduta int) string {
	if legisl <= 0 || seduta <= 0 {
		return ""
	}
	return fmt.Sprintf("%s/bd/resoconti/scheda/%d/%d", icaro.DefaultBaseURL, legisl, seduta)
}

// sedutePerDataIncoerenti riporta le date in cui gli eventi d'Aula di questo
// iter dichiarano numeri di seduta diversi, con i numeri in gioco.
//
// L'Aula tiene una sola seduta per data — l'archivio resoconti ne indicizza una
// per giorno — quindi due numeri sulla stessa data non possono essere entrambi
// giusti, e la fonte non dice quale lo sia. Succede davvero: l'iter del ddl 199
// della XVII dà la votazione finale come «19 feb 2020 — Approvato
// dall'Assemblea — Seduta n. 179», ma la 179 è del 26 febbraio; il voto sta nel
// resoconto della 178, che è quella del 19 (verificato sul resoconto stesso,
// «XVII LEGISLATURA 178a SEDUTA 19 febbraio 2020 … L'Assemblea approva»).
//
// Su quelle date il link al resoconto non si costruisce: un URL che risolve
// mostrando il documento sbagliato è peggio dell'assenza del link, ed è la
// stessa ragione per cui non si linka la seduta di commissione citata da un
// evento di fase aula. La data resta nell'evento, ed è quella la chiave con cui
// trovare la seduta vera.
func sedutePerDataIncoerenti(evs []iterEvent) map[string]bool {
	numeriPerData := map[string]map[int]bool{}
	for _, ev := range evs {
		if !ev.sedutaAula || ev.Seduta <= 0 {
			continue
		}
		if numeriPerData[ev.Data] == nil {
			numeriPerData[ev.Data] = map[int]bool{}
		}
		numeriPerData[ev.Data][ev.Seduta] = true
	}
	out := map[string]bool{}
	for data, numeri := range numeriPerData {
		if len(numeri) > 1 {
			out[data] = true
		}
	}
	return out
}

// avvisoSedutaIncoerente dice perché il link manca e come arrivare comunque
// alla seduta: senza l'avviso l'assenza dell'URL su alcuni eventi e non su
// altri sembra un bug della CLI invece di un dato incoerente a monte. Torna il
// testo invece di scriverlo, come annoNonPinnatoHint: così lo stesso avviso va
// su stderr e nel campo `note` del report, ed è verificabile senza catturare
// stderr.
func avvisoSedutaIncoerente(legisl int, incoerenti map[string]bool) string {
	if len(incoerenti) == 0 {
		return ""
	}
	date := make([]string, 0, len(incoerenti))
	for d := range incoerenti {
		date = append(date, d)
	}
	sort.Strings(date)
	return fmt.Sprintf(
		"la fonte dichiara numeri di seduta d'Aula diversi per la stessa data (%s), e l'Aula ne tiene una sola al giorno: almeno uno dei numeri è sbagliato. Quegli eventi portano `anomalia: true` e il link al resoconto è omesso invece di puntare al giorno sbagliato — trova la seduta vera con `resoconti cerca --legisl %d --data <data>`.",
		strings.Join(date, ", "), legisl)
}

// seduteConDateIncoerenti è il verso inverso di sedutePerDataIncoerenti: le
// sedute d'Aula che l'iter dichiara su più di una data.
//
// Una seduta ha una data sola, quindi un numero che compare su due date non
// può essere giusto su entrambe. Succede davvero: l'iter del ddl 733 della XVII
// (la stabilità 2020/2022, poi L.R. 9/2020) dà «28 apr 2020 — Esaminato in Aula
// — Seduta n. 187» e «02 mag 2020 — Approvato dall'Assemblea — Seduta n. 187»,
// ma il 2 maggio 2020 l'Aula non ha tenuto seduta alcuna (`resoconti cerca
// --legisl 17 --data 2020-05-02` torna vuoto; la 187 è del 28 aprile e la 188
// del 6 maggio).
//
// La distinzione è la stessa della guardia gemella: si guardano solo le sedute
// d'Aula, perché le due numerazioni sono indipendenti e la seduta 187 di
// commissione (20 apr 2020, SECONDA) non c'entra nulla con la 187 d'Aula. La
// data si normalizza con iterDateKey: la stessa data la fonte la scrive in due
// forme ("28.04.20" nella short-list, "28 apr 2020" nell'iter) e confrontare le
// stringhe grezze marcherebbe come incoerente un iter sano.
func seduteConDateIncoerenti(evs []iterEvent) map[int]bool {
	datePerSeduta := map[int]map[string]bool{}
	for _, ev := range evs {
		if !ev.sedutaAula || ev.Seduta <= 0 {
			continue
		}
		if datePerSeduta[ev.Seduta] == nil {
			datePerSeduta[ev.Seduta] = map[string]bool{}
		}
		datePerSeduta[ev.Seduta][iterDateKey(ev.Data)] = true
	}
	out := map[int]bool{}
	for seduta, date := range datePerSeduta {
		if len(date) > 1 {
			out[seduta] = true
		}
	}
	return out
}

// marcaEventiIncoerenti applica a un iter tutte e due le guardie di coerenza
// seduta↔data, marca gli eventi che non tornano e torna l'insieme delle date
// incoerenti (che il chiamante usa per non costruire il link al resoconto) e il
// testo dell'avviso, o "" quando l'iter è coerente.
//
// L'avviso esce due volte, e non è una ripetizione: su stderr per chi legge, e
// come `note` nella radice del report per chi legge in `--json` — è il nome
// che iterReport dà al campo, mentre il resto della CLI lo chiama `nota`, e
// campiAvviso li tiene entrambi. È un campo di avviso, quindi sopravvive a --select — e --select
// è proprio il modo in cui l'iter si legge di solito, dove un marcatore
// per-evento verrebbe filtrato via insieme ai campi non chiesti.
func marcaEventiIncoerenti(cmd *cobra.Command, legisl int, evs []iterEvent) (map[string]bool, string) {
	incoerentiPerData := sedutePerDataIncoerenti(evs)
	seduteIncoerenti := seduteConDateIncoerenti(evs)
	for i := range evs {
		ev := &evs[i]
		if !ev.sedutaAula || ev.Seduta <= 0 {
			continue
		}
		if incoerentiPerData[ev.Data] || seduteIncoerenti[ev.Seduta] {
			ev.Anomalia = true
		}
	}
	avviso := strings.TrimSpace(strings.Join([]string{
		avvisoSedutaIncoerente(legisl, incoerentiPerData),
		avvisoSedutaDataIncoerente(legisl, seduteIncoerenti),
	}, " "))
	if avviso != "" {
		fmt.Fprintln(cmd.ErrOrStderr(), "hint: "+avviso)
	}
	return incoerentiPerData, avviso
}

// avvisoSedutaDataIncoerente dice perché quegli eventi portano `anomalia` e
// come arrivare comunque alla seduta giusta. Speculare ad avvisoSedutaIncoerente.
func avvisoSedutaDataIncoerente(legisl int, sedute map[int]bool) string {
	if len(sedute) == 0 {
		return ""
	}
	numeri := make([]int, 0, len(sedute))
	for n := range sedute {
		numeri = append(numeri, n)
	}
	sort.Ints(numeri)
	testo := make([]string, 0, len(numeri))
	for _, n := range numeri {
		testo = append(testo, "n. "+itoa(n))
	}
	// Il ripiego NON è `resoconti cerca --data`, che è quello della guardia
	// gemella: là la data è la metà affidabile e si cerca il numero, qui è la
	// metà contestata e cercarla torna vuoto — sulla data incriminata della
	// L.R. 9/2020 `resoconti cerca --legisl 17 --data 2020-05-02` dà `[]`, cioè
	// proprio la conclusione «resoconto mancante» che il marcatore esiste per
	// evitare. La metà su cui si può contare è il numero: `resoconti get 17
	// 187` porta la data autorevole della seduta (28.04.20, nel campo Data).
	//
	// «una data sola» è la misura, non la deduzione: dall'iter non si può dire
	// quale delle due righe sbagli, e una seduta sospesa e ripresa in un altro
	// giorno darebbe la stessa coppia di eventi. Perciò si dice cosa si è
	// visto e si lascia al lettore la conclusione.
	return fmt.Sprintf(
		"la fonte dichiara la stessa seduta d'Aula (%s) su date diverse, mentre l'archivio resoconti le assegna una data sola: la coppia seduta-data non torna. Quegli eventi portano `anomalia: true` e il link al resoconto è omesso invece di puntare a un giorno che potrebbe essere quello sbagliato — la data autorevole della seduta si legge con `resoconti get %d <numero-seduta>` (campo `fields.Data`).",
		strings.Join(testo, ", "), legisl)
}

// docIterEvents reads a DDL's iter timeline. It prefers the page's labeled
// "Iter" block, which the portal renders in a div separate from the bill
// text and contains nothing but the "Attuale ... Storico ..." status steps,
// and only falls back to scanning the flattened Body when that block is
// absent. The distinction matters: DDLs whose relazione/articolato quotes
// dates from the law they amend (e.g. "l.r. 8 aprile 2010, n. 9" repeated
// throughout) have no reliable end-of-status marker in Body, so those quoted
// dates used to leak in as spurious iter events. The labeled field has no
// such text to leak from, and is not truncated even for long iters (verified
// on a 40-event, 2.7kB history).
func docIterEvents(doc icaro.Doc) []iterEvent {
	if s := doc.Fields["Iter"]; s != "" {
		if ev := parseIterFromBody(s); len(ev) > 0 {
			return ev
		}
	}
	return parseIterFromBody(doc.Body)
}

// currentIterState returns the DDL's current iter status as a single stable
// string for `sync --deep` to persist as `iter` — the field `ddl drift` compares
// across snapshots. It prefers the page's labeled "Iter" block (the same source
// docIterEvents trusts first) and falls back to the flattened Body.
func currentIterState(doc icaro.Doc) string {
	if s := doc.Fields["Iter"]; s != "" {
		if c := currentIterFromBody(s); c != "" {
			return c
		}
	}
	return currentIterFromBody(doc.Body)
}

// currentIterFromBody extracts the collapsed current-status segment the portal
// renders between the "Attuale" marker and the "Storico" history label (or the
// bill header when there is no history yet). Returns "" when no status block is
// present. Mirrors parseIterFromBody's region boundaries so the two stay in sync.
func currentIterFromBody(body string) string {
	start := strings.Index(body, "Attuale")
	if start < 0 {
		return ""
	}
	region := body[start+len("Attuale"):]
	if end := strings.Index(region, "Storico"); end >= 0 {
		region = region[:end]
	} else {
		// No history yet: cut at whichever bill-header marker comes first.
		cutAt := -1
		for _, marker := range []string{"(n.", "ASSEMBLEA REGIONALE SICILIANA"} {
			if i := strings.Index(region, marker); i >= 0 && (cutAt < 0 || i < cutAt) {
				cutAt = i
			}
		}
		if cutAt >= 0 {
			region = region[:cutAt]
		}
	}
	return strings.Join(strings.Fields(region), " ")
}

type firmatario struct {
	Nome   string `json:"nome"`
	Gruppo string `json:"gruppo,omitempty"`
}

// reFirmEntry matches a "Nome Cognome (Gruppo)" firmatario entry. The name is a
// run of capitalised words; the group is the parenthesised text.
var reFirmEntry = regexp.MustCompile(`([A-ZÀ-Ù][\p{L}'.\-]*(?:\s+[A-ZÀ-Ù][\p{L}'.\-]*){0,3})\s*\(([^)]{2,90})\)`)

// firmLabelWords are non-name capitalised words that can sit right before a
// firmatario in the flattened body (section labels / iniziativa values).
var firmLabelWords = map[string]bool{
	"Parlamentare": true, "Governativa": true, "Popolare": true, "Iniziativa": true,
	"Gruppo": true, "Firmatari": true, "Argomenti": true, "Premier": true, "ARS": true,
}

// docFirmatari reads the signatories of a document. It prefers the page's
// labeled "Firmatari" block, which contains nothing but signatories, and only
// falls back to scanning the whole flattened Body when that block is absent.
// The distinction matters: in Body the neighbouring "Gruppo Parlamentare"
// block runs straight into the first name ("Partito Democratico Chinnici
// Valentina"), and the bullet-segment heuristics drop signatories that share
// a segment.
func docFirmatari(doc icaro.Doc) []firmatario {
	if s := doc.Fields["Firmatari"]; s != "" {
		if f := parseFirmatariBlock(s); len(f) > 0 {
			return f
		}
	}
	return parseDdlFirmatari(doc.Body)
}

// firmatariNames joins signatory names into the comma-separated string form that
// `sync --deep` persists as `$.firmatari` — the shape splitFirmatari re-parses in
// analytics cofirmatari. Only names are kept (party groups dropped) so the split
// yields clean deputy names. Returns "" for no signatories.
func firmatariNames(fs []firmatario) string {
	names := make([]string, 0, len(fs))
	for _, f := range fs {
		if n := strings.TrimSpace(f.Nome); n != "" {
			names = append(names, n)
		}
	}
	return strings.Join(names, ", ")
}

// reFirmTrunc matches a trailing signatory whose group parenthesis the portal
// left unclosed — it truncates the Firmatari field at a fixed width, so the
// last entry can arrive as "Galluzzo Giuseppe (Fratelli d'Italia".
var reFirmTrunc = regexp.MustCompile(`([A-ZÀ-Ù][\p{L}'.\-]*(?:\s+[A-ZÀ-Ù][\p{L}'.\-]*){0,3})\s*\(([^)]*)$`)

// parseFirmatariBlock parses the page's "Firmatari" block: a run of
// "Nome Cognome (Gruppo)" entries separated by bullets, <br> or nothing at
// all. Because the block holds only signatories, every match is taken
// verbatim — no label stripping, no per-segment guessing.
func parseFirmatariBlock(s string) []firmatario {
	var out []firmatario
	seen := map[string]bool{}
	add := func(nome, grp string) {
		nome = strings.Join(strings.Fields(nome), " ")
		grp = strings.Join(strings.Fields(grp), " ")
		if nome == "" || seen[nome+"|"+grp] {
			return
		}
		seen[nome+"|"+grp] = true
		out = append(out, firmatario{Nome: nome, Gruppo: grp})
	}
	locs := reFirmEntry.FindAllStringSubmatchIndex(s, -1)
	for _, l := range locs {
		add(s[l[2]:l[3]], s[l[4]:l[5]])
	}
	// Recover a truncated trailing entry rather than dropping the signatory:
	// the name is intact even when the portal cut its group short.
	tail := s
	if len(locs) > 0 {
		tail = s[locs[len(locs)-1][1]:]
	}
	if m := reFirmTrunc.FindStringSubmatch(tail); m != nil {
		add(m[1], m[2])
	}
	return out
}

// parseDdlFirmatari extracts the bill signatories from the document body. It
// handles the structured sidebar form ("Nome (Gruppo). • Nome (Gruppo).", with
// party groups) and the relazione form ("presentato dai deputati: A, B, C",
// names only). Returns nil when none is found (e.g. some governativi).
// Prefer docFirmatari, which reads the labeled block when the page has one.
func parseDdlFirmatari(body string) []firmatario {
	if body == "" {
		return nil
	}
	// Form A: bullet-separated "Nome (Gruppo)" entries.
	if strings.Contains(body, "•") {
		var out []firmatario
		seen := map[string]bool{}
		for _, seg := range strings.Split(body, "•") {
			ms := reFirmEntry.FindAllStringSubmatch(seg, -1)
			if len(ms) == 0 {
				continue
			}
			m := ms[len(ms)-1] // firmatario sits at the end of the segment
			nome := cleanFirmatarioName(m[1])
			grp := strings.Join(strings.Fields(m[2]), " ")
			if nome == "" || seen[nome+"|"+grp] {
				continue
			}
			seen[nome+"|"+grp] = true
			out = append(out, firmatario{Nome: nome, Gruppo: grp})
		}
		if len(out) > 0 {
			return out
		}
	}
	// Form B: "presentato dai deputati: A, B, C" (no groups).
	for _, marker := range []string{"presentato dai deputati", "presentato dal deputato", "presentato dalla deputata"} {
		if i := strings.Index(strings.ToLower(body), marker); i >= 0 {
			rest := body[i+len(marker):]
			rest = strings.TrimLeft(rest, ": ")
			if e := strings.IndexAny(rest, ".\n"); e >= 0 {
				rest = rest[:e]
			}
			var out []firmatario
			for _, n := range strings.Split(rest, ",") {
				if n = strings.Join(strings.Fields(n), " "); n != "" {
					out = append(out, firmatario{Nome: n})
				}
			}
			if len(out) > 0 {
				return out
			}
		}
	}
	return nil
}

// cleanFirmatarioName strips leading section-label words and caps the name to
// its trailing capitalised tokens.
func cleanFirmatarioName(s string) string {
	toks := strings.Fields(s)
	for len(toks) > 0 && firmLabelWords[toks[0]] {
		toks = toks[1:]
	}
	if len(toks) > 4 {
		toks = toks[len(toks)-4:]
	}
	return strings.Join(toks, " ")
}

func classifyIterFase(action string) string {
	a := strings.ToLower(action)
	switch {
	case strings.Contains(a, "promulgat") || strings.Contains(a, "legge regionale") || strings.Contains(a, "l.r."):
		return "legge"
	case strings.Contains(a, "aula") || strings.Contains(a, "assemblea") || strings.Contains(a, "approvat"):
		return "aula"
	case strings.Contains(a, "commissione") || strings.Contains(a, "esame") || strings.Contains(a, "parere"):
		return "commissione"
	default:
		return "iter"
	}
}

// iterSede returns the committee name when the action references one.
func iterSede(action string) string {
	if i := strings.Index(strings.ToLower(action), "commissione"); i >= 0 {
		return strings.TrimSpace(action[i:])
	}
	return ""
}

// reSedeSuffisso reads the committee the portal writes AFTER the sitting
// number, in the same suffix reSeduta matches: "Seduta n. 184 0400 Commissione
// QUARTA". The four-digit code between the two is optional only in the sense
// that some rows carry the AULA marker instead — where it appears, it is the
// committee's own code (0100…0600, plus 1200 for the special committees).
//
// The name is taken to the end of the action and not as a single word: the
// standing committees go by ordinal, but the special ones have real names that
// a one-word capture would cut in half ("Commissione speciale per lo Statuto
// della Regione", "Commissione riforma statuto" — leg. XVII, ddl 66). It stops
// at an open parenthesis, which is where the portal appends a note about the
// event and not about the committee ("Commissione PRIMA (Articolo 3
// stralciato)"), and at a newline, because the Body fallback of docIterEvents
// is not flattened and a capture must not run past its own row.
var reSedeSuffisso = regexp.MustCompile(`(?i)\bseduta"*\s+n\.?\s*\d+\s*(?:\d{4})?\s*(commissione\b[^(\n]*)`)

// sedeDaSuffissoSeduta returns the committee declared in the sitting suffix, or
// "". It must run on the untruncated action: indiceSeduta cuts exactly the
// stretch of text this reads, which is why the committee used to be lost even
// when the source stated it. The verb alone names it only sometimes, and when
// it does it is not the canonical form —
//
//	Esaminato in commissione Seduta n. 184 0400 Commissione QUARTA  -> "commissione"
//	Parere espresso Commissione * Seduta n. 68 0500 Commissione QUINTA -> "Commissione *"
//	Esitato per Aula (epa) Seduta n. 185 0100 Commissione PRIMA    -> ""
//	Parere Commissione Bilancio Seduta n. 153 0200 Commissione SECONDA -> "Commissione Bilancio"
//
// — while the suffix always gives the ordinal, which is the form the rest of
// the CLI accepts (`commissioni sommari --commissione SECONDA`). The informal
// name of the last case is not lost: `titolo` keeps the action verbatim.
func sedeDaSuffissoSeduta(action string) string {
	if m := reSedeSuffisso.FindStringSubmatch(action); m != nil {
		return strings.Join(strings.Fields(m[1]), " ")
	}
	return ""
}

// iterSedeRisolta prefers the committee read from the sitting suffix and falls
// back to the one the verb names, with its raw code resolved.
func iterSedeRisolta(sedeSuffisso, action string) string {
	if sedeSuffisso != "" {
		return sedeSuffisso
	}
	return risolviCodiceCommissione(iterSede(action))
}

// reSedeCodice matches a sede left as the portal's internal committee code
// ("Rinviato Commissione 0400"), the one row shape where the suffix carries the
// AULA marker instead of the committee name and the code is all there is.
var reSedeCodice = regexp.MustCompile(`(?i)^commissione\s+(\d{3,4})$`)

// risolviCodiceCommissione turns "Commissione 0400" into "Commissione QUARTA".
// The iter codes are the ordinal times 100, so they need dividing and not
// trimming: TrimSpace("0400") is "400", which commissioneOrdinale does not know.
func risolviCodiceCommissione(sede string) string {
	m := reSedeCodice.FindStringSubmatch(sede)
	if m == nil {
		return sede
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n%100 != 0 {
		return sede
	}
	if ord := commissioneOrdinale(itoa(n / 100)); ord != "" {
		return "Commissione " + ord
	}
	return sede
}

// iterDateKey returns a sortable "YYYY-MM-DD" key for both the short-list date
// form (DD.MM.YY) and the document status form ("DD mese YYYY"). Entrambe le
// forme, e le altre due che il portale usa altrove, stanno in dataISO; qui
// resta il ripiego sulla stringa grezza, che i chiamanti storici si aspettano.
func iterDateKey(s string) string {
	s = strings.TrimSpace(s)
	if iso := dataISO(s); iso != "" {
		return iso
	}
	return s
}

func min3(n int) int {
	if n < 3 {
		return n
	}
	return 3
}

// parseICaroDate converts DD.MM.YYYY (or DD.MM.YY) into a sortable string
// "YYYY-MM-DD"; returns the input as-is when the format isn't recognized.
// Il riconoscimento sta tutto in dataISO, che copre anche le forme degli altri
// due motori del portale; qui resta il ripiego sulla stringa grezza.
func parseICaroDate(s string) string {
	if iso := dataISO(s); iso != "" {
		return iso
	}
	return strings.TrimSpace(s)
}

// uniscoNote accoda un avviso alla nota del report senza perdere quella che
// c'era già: `note` è un campo solo, e i motivi per riempirlo sono più d'uno.
func uniscoNote(nota, avviso string) string {
	switch {
	case avviso == "":
		return nota
	case strings.TrimSpace(nota) == "":
		return avviso
	default:
		return nota + " " + avviso
	}
}

// avvisoStralcioAnteriore spiega la timeline di uno stralcio che comincia prima
// della propria presentazione.
//
// Su `ddl iter 18 6030` il primo evento è l'assegnazione alla Commissione
// QUARTA del 13 gennaio 2026, mentre la presentazione è del 27: letto senza
// contesto sembra un dato sballato. Non lo è — lo stralcio nasce da una
// decisione d'Aula che lo ritaglia dal ddl base e lo instrada alla commissione
// competente, e come documento autonomo viene registrato giorni dopo.
//
// Per questo qui NON si usa `anomalia`. Quel marcatore dice «così come la fonte
// lo dichiara, questo non può essere vero» (l'Aula tiene una seduta al giorno,
// una seduta ha una data sola); applicarlo a un ordine spiegabile lo
// svaluterebbe e produrrebbe a valle il falso buco che esiste per evitare.
// Misurato il 21/08/2026: nessuno degli 8 ddl non-stralcio campionati ha un
// evento anteriore alla presentazione, e fra gli stralci lo hanno solo 6030 e
// 8030, entrambi con primo evento il 13 gennaio, cioè il giorno della delibera
// di stralcio.
func avvisoStralcioAnteriore(report iterReport) string {
	if report.Stralcio == nil {
		return ""
	}
	var presentazione, primo string
	for _, ev := range report.Eventi {
		k := iterDateKey(ev.Data)
		if k == "" {
			continue
		}
		if ev.Fase == "presentazione" && presentazione == "" {
			presentazione = k
		}
		if primo == "" || k < primo {
			primo = k
		}
	}
	if presentazione == "" || primo == "" || primo >= presentazione {
		return ""
	}
	return fmt.Sprintf(
		"la cronologia comincia il %s, prima della presentazione del %s: è uno stralcio, e i lavori che lo hanno ritagliato dal ddl base precedono la sua registrazione come documento autonomo. Il ddl di provenienza è nel campo `stralcio`.",
		primo, presentazione)
}

// avvisoApprovatoSenzaGurs dice perché un iter che finisce con l'approvazione
// non porta la legge.
//
// I due archivi hanno ritardi diversi: il ddl arriva a 24 giorni, le leggi a
// 30 (misurato il 21/08/2026 con `novita`). Chi parte da un ddl approvato di
// recente trova la timeline che si ferma a «Approvato dall'Assemblea» o
// «Inviato Presidenza della Regione», cerca la legge e non la trova — e non ha
// modo di distinguere «non è ancora stata promulgata» da «l'archivio non l'ha
// ancora indicizzata». È lo stesso buco che `legge cronologia` copre dall'altro
// verso, e rimanda allo stesso comando.
//
// Il segnale è strutturale, non una soglia di giorni inventata: quando la fonte
// ha registrato la pubblicazione, l'evento «Pubblicazione Gurs» sta nell'iter.
// Verificato su quattro ddl approvati e promulgati da tempo (17/733, 17/587,
// 18/4991, 18/1030): tutti e quattro ce l'hanno. Il ddl 6030, approvato il 29
// luglio 2026 e pubblicato in Gazzetta il 7 agosto, no — ed è l'unico dei sei
// campionati su cui la nota compare. Un ddl mai arrivato in Aula (18/1171) non
// la riceve, perché manca l'approvazione.
func avvisoApprovatoSenzaGurs(eventi []iterEvent) string {
	var approvato string
	for _, ev := range eventi {
		if strings.Contains(strings.ToLower(ev.Titolo), "gurs") {
			return ""
		}
		if strings.Contains(ev.Titolo, "Approvato dall'Assemblea") {
			approvato = ev.Data
		}
	}
	if approvato == "" {
		return ""
	}
	return fmt.Sprintf(
		"approvato dall'Assemblea il %s, ma la fonte non registra ancora la pubblicazione in Gazzetta: l'iter si ferma qui. Se cerchi la legge promulgata, l'archivio leggi pubblica con settimane di ritardo — `ars-sicilia-pp-cli novita --archivi leggi` dice fin dove arriva la fonte.",
		approvato)
}
