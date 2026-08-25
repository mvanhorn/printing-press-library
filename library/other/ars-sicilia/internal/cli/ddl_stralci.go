// pp:data-source live
// pp:client-call
// Novel feature — naviga il legame fra un disegno di legge e i suoi stralci.
//
// Il portale pubblica il legame in chiaro, con lo stesso testo in due punti:
// il campo "Riferimenti" della scheda di dettaglio e l'excerpt della riga di
// short-list (es. "ddl n. 1030/A Stralcio IV del 27 gennaio 2026", dove 1030 è
// il ddl base e la riga è il ddl 6030). Si legge quel riferimento: la
// numerazione NON è deducibile — gli stralci del 1030 sono 3030…8030, quelli
// del 738 sono 7381/7382 — e qualsiasi formula sui numeri sbaglia.

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

func newNovelDdlStralciCmd(flags *rootFlags) *cobra.Command {
	var flagLimit int
	cmd := &cobra.Command{
		Use:   "stralci <legisl> <numero>",
		Short: "Elenca gli stralci ricavati da un disegno di legge (il verso inverso di 'ddl get'/'ddl iter').",
		Long: `Elenca i disegni di legge ricavati per stralcio da un ddl base.

Il legame è dichiarato dal portale, non dedotto: la numerazione degli stralci
non segue una regola unica (gli stralci del ddl 1030 sono 3030…8030, quelli del
738 sono 7381/7382), quindi il comando legge il riferimento che ogni stralcio
porta con sé invece di calcolarlo.

Il verso opposto — da uno stralcio al suo ddl base — è nel campo 'stralcio' di
'ddl get' e 'ddl iter'.`,
		Example: "  ars-sicilia-pp-cli ddl stralci 18 1030 --json",
		Args:    cobra.MaximumNArgs(2),
		Annotations: map[string]string{
			"mcp:read-only": "true",
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
				// Come sugli altri comandi con posizionali: sotto --dry-run e
				// sotto verify gli argomenti possono essere segnaposto, e una
				// sonda non deve uscire in errore per averli letti.
				if dryRunOK(flags) || cliIsVerify() {
					return cmd.Help()
				}
				if errL != nil {
					return errL
				}
				return errN
			}
			if dryRunOK(flags) {
				// La query NON si riscrive a mano: si costruisce con gli stessi
				// parametri che runDdlStralci passa a Search. Scritta a mano
				// coincideva, ma nulla la teneva agganciata — e un'anteprima
				// libera di divergere dal percorso vivo e' il difetto che
				// questa CLI ha appena finito di togliersi di dosso.
				target, terr := dryRunTargetBySlug("ddl", stralciSearchParams(legisl, numero))
				if terr != nil {
					return terr
				}
				if target == nil {
					return fmt.Errorf("archivio ddl non disponibile")
				}
				return emitDryRunRequests(cmd, []map[string]any{target},
					"cerca il numero base come testo libero: il riferimento dello stralcio lo contiene. Le righe candidate vengono poi filtrate leggendo il campo Riferimenti di ciascuna.")
			}
			return runDdlStralci(cmd, flags, legisl, numero, flagLimit)
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 30, "Max righe candidate da esaminare.")
	return cmd
}

// stralcioRow è una riga dell'elenco degli stralci di un ddl base.
type stralcioRow struct {
	Numero         int    `json:"numero"`
	Data           string `json:"data,omitempty"`
	Etichetta      string `json:"etichetta,omitempty"`
	Titolo         string `json:"titolo,omitempty"`
	BaseDichiarata bool   `json:"base_dichiarata"`
	URL            string `json:"url,omitempty"`
}

type stralciReport struct {
	Legisl  int           `json:"legisl"`
	Numero  int           `json:"numero"`
	Titolo  string        `json:"titolo,omitempty"`
	Stralci []stralcioRow `json:"stralci"`
	Note    string        `json:"note,omitempty"`
}

// runDdlStralci trova gli stralci con una sola ricerca: il riferimento che ogni
// stralcio porta ("ddl n. 1030/A Stralcio IV") contiene il numero base, quindi
// il free-text sul numero recupera l'intera famiglia. Attenzione: cercare la
// forma con la barra ("1030/A") restituisce zero — la barra rompe la query ISIS.
// stralciSearchParams sono i parametri della ricerca degli stralci, in un posto
// solo: l'anteprima --dry-run e la ricerca vera devono partire dagli stessi, o
// la prima smette di descrivere la seconda senza che nulla lo segnali.
func stralciSearchParams(legisl, numero int) map[string]string {
	return map[string]string{"legisl": itoa(legisl), "testo": itoa(numero)}
}

func runDdlStralci(cmd *cobra.Command, flags *rootFlags, legisl, numero, limit int) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	arc := icaro.BySlug("ddl")
	if arc == nil {
		return fmt.Errorf("archivio ddl non disponibile")
	}
	c, err := icaro.New(nil)
	if err != nil {
		return err
	}
	if limit <= 0 {
		limit = 30
	}
	recs, err := c.Search(ctx, *arc, icaro.SearchOptions{
		Params: normalizeParams(*arc, stralciSearchParams(legisl, numero)),
		Limit:  limit,
		// Icaro pagina ~10 righe: senza MaxPages la ricerca si ferma alla
		// prima e gli stralci oltre la decima riga non arrivano mai.
		MaxPages: (limit + 9) / 10,
	})
	if err != nil {
		return fmt.Errorf("ricerca stralci: %w", err)
	}

	// Deduplica PRIMA di filtrare: il portale ripete la stessa riga e a volte
	// una delle copie ha l'excerpt vuoto. Filtrando prima, uno stralcio la cui
	// unica copia rimasta fosse quella vuota sparirebbe senza segnalazione.
	migliori := map[int]icaro.Record{}
	ordine := []int{}
	for _, r := range recs {
		n, err := strconv.Atoi(strings.TrimSpace(r.Fields["Numero"]))
		if err != nil || n <= 0 {
			continue
		}
		prec, visto := migliori[n]
		if !visto {
			ordine = append(ordine, n)
			migliori[n] = r
			continue
		}
		if len(strings.TrimSpace(prec.Excerpt)) < len(strings.TrimSpace(r.Excerpt)) {
			migliori[n] = r
		}
	}

	report := stralciReport{Legisl: legisl, Numero: numero, Stralci: []stralcioRow{}}
	for _, n := range ordine {
		r := migliori[n]
		if n == numero {
			report.Titolo = r.Title // il ddl base: dà il titolo, non è uno stralcio di sé
			continue
		}
		ref, ok := parseStralcioRef(r.Excerpt, n)
		if !ok {
			continue // riga agganciata dal free-text ma senza marcatore: rumore
		}
		dichiarata := false
		for _, b := range ref.Basi {
			if b.Numero == numero {
				dichiarata = true
			}
		}
		// Autoriferito: il portale ha scritto l'id della riga al posto della
		// base (succede nella XVII legislatura). Qui la base rivendicata è il
		// numero cercato, perché è ciò che ha agganciato la riga: si tiene, ma
		// marcata come non dichiarata.
		if !dichiarata && !ref.Autoriferito {
			continue
		}
		report.Stralci = append(report.Stralci, stralcioRow{
			Numero:         n,
			Data:           r.Fields["Data"],
			Etichetta:      ref.Etichetta,
			Titolo:         r.Title,
			BaseDichiarata: dichiarata,
			URL:            r.URL,
		})
	}
	// Data, poi numero: a parità di data il portale restituisce gli stralci in
	// ordine sparso ("Comm bis" prima di "Comm"), e l'ordine dei risultati
	// cambierebbe fra una chiamata e l'altra.
	sort.SliceStable(report.Stralci, func(i, j int) bool {
		di, dj := iterDateKey(report.Stralci[i].Data), iterDateKey(report.Stralci[j].Data)
		if di != dj {
			return di < dj
		}
		return report.Stralci[i].Numero < report.Stralci[j].Numero
	})
	if len(report.Stralci) == 0 {
		report.Note = fmt.Sprintf("nessuno stralcio dichiarato per il ddl %d nella legislatura %d. Verifica il numero con `ars-sicilia-pp-cli ddl get %d %d`.", numero, legisl, legisl, numero)
		fmt.Fprintln(cmd.ErrOrStderr(), "hint: "+report.Note)
	}
	return printJSONFiltered(cmd.OutOrStdout(), report, flags)
}

// stralcioOut è la resa JSON del legame, con tre stati distinti e leggibili:
//   - chiave assente        → il documento non è uno stralcio
//   - "di": [] con base_dichiarata false → è uno stralcio, ma il portale non
//     dichiara da quale ddl (vedi stralcioRef.Autoriferito)
//   - "di" popolato         → basi dichiarate dal portale
type stralcioOut struct {
	Di             []stralcioBase `json:"di"`
	Etichetta      string         `json:"etichetta,omitempty"`
	BaseDichiarata bool           `json:"base_dichiarata"`
}

// stralcioDaTesti costruisce la resa JSON dal primo testo che contiene un
// riferimento riconoscibile: la scheda espone "Riferimenti", la short-list
// l'excerpt, con lo stesso contenuto.
func stralcioDaTesti(numeroRiga int, testi ...string) *stralcioOut {
	for _, t := range testi {
		if strings.TrimSpace(t) == "" {
			continue
		}
		ref, ok := parseStralcioRef(t, numeroRiga)
		if !ok {
			continue
		}
		out := &stralcioOut{
			Di:             ref.Basi,
			Etichetta:      ref.Etichetta,
			BaseDichiarata: len(ref.Basi) > 0,
		}
		if out.Di == nil {
			out.Di = []stralcioBase{}
		}
		return out
	}
	return nil
}

// stralcioBase è un disegno di legge da cui uno stralcio è stato ricavato. Sono
// più d'uno quando lo stralcio nasce da ddl abbinati ("ddl nn. 824-810 - I
// Stralcio").
type stralcioBase struct {
	Numero   int    `json:"numero"`
	Variante string `json:"variante,omitempty"`
}

// stralcioRef è il riferimento parsato da un testo del portale.
//
// Basi vuoto con Autoriferito true è il caso — presente nella XVII legislatura —
// in cui il portale scrive l'id interno della riga al posto del numero base
// ("ddl n. 8931/A stralcio 1 bis" sulla riga 8931). Lì la base non è
// dichiarata: dedurla togliendo l'ultima cifra sarebbe esattamente l'euristica
// sui numeri che questo parser esiste per evitare.
type stralcioRef struct {
	Basi         []stralcioBase
	Etichetta    string
	Autoriferito bool
}

var (
	// "ddl n. 1030/A", "ddl nn. 824-810", "ddl n.1030/A", "ddl 1030/V",
	// "ddl n. 738/". Il gruppo dei numeri accetta la lista unita da "-".
	// La barra della variante non ammette spazi prima delle lettere: in
	// "ddl n. 738/ stralcio I" il suffisso è vuoto, e tollerare lo spazio
	// farebbe catturare la parola "stralcio" come variante.
	reStralcioDdl = regexp.MustCompile(`(?i)\bddl\s+(?:nn?\.\s*)?(\d+(?:\s*-\s*\d+)*)\s*(?:/([A-Za-z]*))?`)
	reStralcioTok = regexp.MustCompile(`(?i)\bstralcio\b`)
	reRomano      = regexp.MustCompile(`(?i)^[ivxlc]+$`)
	reArabo       = regexp.MustCompile(`^\d+$`)
)

// qualificatori sono i token che il portale accoda all'ordinale dello stralcio.
// "bis/BIL" arriva come token unico e viene spezzato sulla barra.
var qualificatori = map[string]bool{
	"bis": true, "ter": true, "quater": true, "quinquies": true,
	"comm": true, "bil": true, "a": true,
}

// stopParola chiude l'etichetta: oltre queste comincia la data.
var stopParola = map[string]bool{
	"del": true, "dello": true, "della": true, "di": true, "e": true,
}

// parseStralcioRef legge un riferimento del portale e dice se il documento è
// uno stralcio, di quale/i ddl e con quale etichetta.
//
// numeroRiga è il numero del documento a cui il testo appartiene: serve a
// riconoscere l'autoriferimento. Passare 0 se non è noto.
func parseStralcioRef(text string, numeroRiga int) (stralcioRef, bool) {
	loc := reStralcioTok.FindStringIndex(text)
	if loc == nil {
		// Nessun token "stralcio": è un ddl ordinario, non uno stralcio.
		return stralcioRef{}, false
	}
	ref := stralcioRef{Etichetta: etichettaStralcio(text, loc)}

	m := reStralcioDdl.FindStringSubmatch(text)
	if m == nil {
		return ref, true
	}
	variante := strings.ToUpper(strings.TrimSpace(m[2]))
	for _, raw := range strings.Split(m[1], "-") {
		n, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil || n <= 0 {
			continue
		}
		if numeroRiga > 0 && n == numeroRiga {
			// Il testo cita il numero della riga stessa: la base non è
			// dichiarata. Si segnala, non si inventa.
			ref.Autoriferito = true
			continue
		}
		ref.Basi = append(ref.Basi, stralcioBase{Numero: n, Variante: variante})
	}
	return ref, true
}

// etichettaStralcio ritaglia la designazione dello stralcio attorno al token
// trovato: l'ordinale può precederlo ("I Stralcio") o seguirlo ("Stralcio IV
// bis", "stralcio 4", "Stralcio I COMM ter"), e può mancare del tutto
// ("ddl 1030/V Stralcio").
func etichettaStralcio(text string, loc []int) string {
	parola := text[loc[0]:loc[1]]

	// L'ordinale che precede è ammesso solo in cifre romane ("I Stralcio"): un
	// numero arabo prima della parola è il numero del ddl, non un ordinale
	// ("ddl n. 8321 stralcio").
	pre := strings.Fields(text[:loc[0]])
	var prefisso string
	if n := len(pre); n > 0 && isRomano(pre[n-1]) {
		prefisso = strings.Trim(pre[n-1], ".,;:") + " "
	}

	// Un numero arabo vale come ordinale solo se apre la coda ("stralcio 4",
	// "stralcio 1 bis"). Dopo un ordinale già visto è l'inizio della data non
	// preceduta da "del" ("Stralcio IV bis/BIL 15 aprile 2026").
	var coda []string
	var ordinaleVisto bool
	for _, tok := range strings.Fields(text[loc[1]:]) {
		pulito := strings.Trim(tok, ".,;:")
		if pulito == "" || stopParola[strings.ToLower(pulito)] {
			break
		}
		switch {
		case isRomano(pulito):
			ordinaleVisto = true
		case reArabo.MatchString(pulito):
			if ordinaleVisto {
				return prefisso + parola + codaJoin(coda)
			}
			ordinaleVisto = true
		case isQualificatore(pulito):
		default:
			return prefisso + parola + codaJoin(coda)
		}
		coda = append(coda, pulito)
	}
	return prefisso + parola + codaJoin(coda)
}

func codaJoin(coda []string) string {
	if len(coda) == 0 {
		return ""
	}
	return " " + strings.Join(coda, " ")
}

func isRomano(tok string) bool {
	tok = strings.Trim(tok, ".,;:")
	return tok != "" && reRomano.MatchString(tok)
}

// isQualificatore accetta anche i token composti sulla barra ("bis/BIL"),
// purché ogni parte sia a sua volta un qualificatore o un ordinale.
func isQualificatore(tok string) bool {
	parti := strings.Split(strings.Trim(tok, ".,;:"), "/")
	for _, p := range parti {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		if !qualificatori[p] && !reRomano.MatchString(p) {
			return false
		}
	}
	return true
}
